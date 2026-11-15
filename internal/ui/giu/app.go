package giuui

import (
	"fmt"
	"math"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
)

const (
	defaultWindowWidth  = 1440
	defaultWindowHeight = 780
	minimumWindowWidth  = 1180
	minimumWindowHeight = 700

	rootPadding        = float32(20)
	rootGap            = float32(8)
	topBarHeight       = float32(60)
	diagnosticsHeight  = float32(92)
	transportBarHeight = float32(64)
	devicePanelWidth   = float32(332)

	dopConfirmationPopupID = "DoP DSD 载波输出安全确认##dop-confirm"
	nativeDSDPopupID       = "Native DSD 1-bit 直通安全确认##native-dsd-confirm"
	genericSafetyPopupID   = "通用安全风险确认##generic-safety-confirm"
)

type GiuApp struct {
	uiService *controller.UIService
	state     *UIState
	window    *g.MasterWindow

	topBar           *TopBar
	devicePanel      *DevicePanel
	generatorPanel   *GeneratorPanel
	diagnosticsPanel *DiagnosticsPanel
	transportBar     *TransportBar

	stopPolling chan struct{}
	isRunning   bool
}

func NewGiuApp(uiService *controller.UIService) *GiuApp {
	state := NewUIState()

	app := &GiuApp{
		uiService:   uiService,
		state:       state,
		stopPolling: make(chan struct{}),
	}

	app.devicePanel = NewDevicePanel(uiService, state, app.ApplyConfiguration, app.RequestNativeDSDConfirmation)
	app.generatorPanel = NewGeneratorPanel(uiService, state, app.ApplyConfiguration, app.RequestDoPConfirmation)
	app.diagnosticsPanel = NewDiagnosticsPanel(uiService, state)
	app.topBar = NewTopBar(state, app.StopPlayback)
	app.transportBar = NewTransportBar(
		state,
		app.StartPlayback,
		app.StopPlayback,
		app.generatorPanel.IsNyquistExceeded,
	)

	return app
}

func (a *GiuApp) StartPlayback() {
	a.generatorPanel.StartPlayback()
	a.state.IsActiveStream = a.state.ErrorMessage == "" &&
		!a.state.PendingSafetyStart && !a.state.ShowSafetyConfirm && !a.state.ShowDoPConfirm
	if a.state.IsActiveStream {
		a.state.StreamStartTime = time.Now()
	} else {
		a.state.StreamStartTime = time.Time{}
	}
	a.safeUpdate()
}

func (a *GiuApp) StopPlayback() {
	a.state.StreamStartTime = time.Time{}
	a.generatorPanel.StopPlayback()
	if a.state.ErrorMessage == "" {
		a.state.IsActiveStream = false
	}
	a.safeUpdate()
}

// BuildExtendedRequest translates GUI state into the domain request. Indexes
// are clamped defensively so a transient device refresh cannot panic the UI.
func (a *GiuApp) BuildExtendedRequest() controller.ExtendedSignalRequest {
	devID := ""
	if int(a.state.SelectedDeviceIndex) >= 0 && int(a.state.SelectedDeviceIndex) < len(a.state.DeviceIDs) {
		devID = a.state.DeviceIDs[a.state.SelectedDeviceIndex]
	}

	rateIndex := clampIndex(a.state.SelectedSampleRateIndex, len(SampleRateOptions))
	formatIndex := clampIndex(a.state.SelectedSampleFormatIndex, len(SampleFormatOptions))
	waveIndex := clampIndex(a.state.SelectedWaveformIndex, len(WaveformOptions))
	channelIndex := clampIndex(a.state.SelectedChannelMode, len(ChannelModeLabels))

	sampleRate := SampleRateOptions[rateIndex]
	sampleFmt := SampleFormatOptions[formatIndex]
	waveType := WaveformOptions[waveIndex].ID
	dopFormat := a.state.SelectedDoPFormat()
	dopBitOrder := a.state.SelectedDoPBitOrder()

	lVol := float64(a.state.LeftVolumeDBFS)
	rVol := float64(a.state.RightVolumeDBFS)

	if waveType == controller.WaveformDoP {
		// DoP has hard transport invariants and must never inherit an arbitrary
		// PCM format, channel trim or calibration gain from the UI state.
		sampleRate = dopFormat.DoPCarrierSampleRate()
		sampleFmt = audio.SampleFormat24In32
		lVol = 0.0
		rVol = 0.0
	} else {
		switch ChannelMode(channelIndex) {
		case ChannelModeSoloL:
			rVol = -144.0
		case ChannelModeSoloR:
			lVol = -144.0
		case ChannelModeMono:
			monoVol := (lVol + rVol) * 0.5
			lVol = monoVol
			rVol = monoVol
		}

		trim := float64(a.state.MasterTrimDB)
		lVol = math.Min(0.0, lVol+trim)
		rVol = math.Min(0.0, rVol+trim)
	}

	maxVol := math.Max(lVol, rVol)
	linearAmp := math.Pow(10, maxVol/20.0)
	if waveType == controller.WaveformDoP {
		linearAmp = 1.0
	}

	return controller.ExtendedSignalRequest{
		DeviceID:        devID,
		SampleRate:      sampleRate,
		SampleFormat:    sampleFmt,
		Channels:        2,
		Waveform:        waveType,
		Frequency:       float64(a.state.Frequency),
		Amplitude:       linearAmp,
		LeftVolumeDBFS:  lVol,
		RightVolumeDBFS: rVol,
		Sweep: controller.SweepParams{
			StartFreq:     float64(a.state.SweepStartFreq),
			EndFreq:       float64(a.state.SweepEndFreq),
			DurationSec:   float64(a.state.SweepDurationSec),
			TaperFraction: 0.10,
			Loop:          a.state.SweepLoop,
		},
		Noise: controller.NoiseParams{Seed: int64(a.state.NoiseSeed)},
		DoP: controller.DoPParams{
			DSDFormat: dopFormat,
			BitOrder:  dopBitOrder,
		},
	}
}

func clampIndex(value int32, length int) int {
	if length <= 0 {
		return 0
	}
	if int(value) < 0 || int(value) >= length {
		return 0
	}
	return int(value)
}

func (a *GiuApp) safeUpdate() {
	if a.isRunning {
		g.Update()
	}
}

func (a *GiuApp) ApplyConfiguration() {
	if a.state.IsDoPSelected() && !a.state.DoPConfirmed {
		a.state.StatusMessage = "DoP 待安全确认：输出保持静音"
		return
	}

	wasRunning := a.state.IsOutputRunning()

	req := a.BuildExtendedRequest()
	assessment := a.uiService.CheckSafety(req)
	if assessment.HasRisk && assessment.RequiresConfirmation && !assessment.IsConfirmed {
		if wasRunning {
			a.state.PendingSafetyStart = true
		}
		a.requestSafetyConfirmation(assessment)
		return
	}

	if err := a.uiService.ConfigureSignal(req); err != nil {
		a.state.ErrorMessage = err.Error()
	} else {
		a.state.ErrorMessage = ""
	}
	a.safeUpdate()
}

func (a *GiuApp) requestSafetyConfirmation(assessment controller.SafetyAssessment) {
	// A risk selected while audio is active must fail safe immediately.
	_ = a.uiService.StopPlayback()
	a.state.IsActiveStream = false
	a.state.StreamStartTime = time.Time{}
	a.state.ErrorMessage = ""
	a.state.SafetyAssessment = assessment
	a.state.StatusMessage = "安全确认待处理：输出保持静音"
	a.state.SuppressSafetyPrompt = false

	if !a.state.ShowSafetyConfirm {
		a.state.ShowSafetyConfirm = true
		g.OpenPopup(genericSafetyPopupID)
	}
	a.safeUpdate()
}

func (a *GiuApp) confirmSafety() {
	assessment := a.state.SafetyAssessment
	saveWarning := ""
	if a.state.SuppressSafetyPrompt {
		if err := a.uiService.SuppressSafety(string(assessment.Category)); err != nil {
			saveWarning = err.Error()
		}
	} else {
		a.uiService.ConfirmSafety(string(assessment.Category), true)
	}

	a.state.ShowSafetyConfirm = false
	a.state.SuppressSafetyPrompt = false
	g.CloseCurrentPopup()
	a.ApplyConfiguration()

	if a.state.PendingSafetyStart {
		a.state.PendingSafetyStart = false
		if a.state.ErrorMessage == "" {
			a.state.StreamStartTime = time.Now()
			if err := a.uiService.StartPlayback(); err != nil {
				a.state.ErrorMessage = fmt.Sprintf("启动输出失败: %v", err)
			} else {
				a.state.IsActiveStream = true
				a.state.StatusMessage = "信号输出中"
			}
		}
	} else if a.state.ErrorMessage == "" {
		a.state.StatusMessage = "安全确认已应用，输出保持静音"
	}

	if saveWarning != "" {
		a.state.StatusMessage = fmt.Sprintf("安全确认已应用，但保存“以后不再提示”失败: %s", saveWarning)
	}
	a.safeUpdate()
}

func (a *GiuApp) cancelSafety() {
	a.state.ShowSafetyConfirm = false
	a.state.SuppressSafetyPrompt = false
	a.state.PendingSafetyStart = false
	a.state.SafetyAssessment = controller.SafetyAssessment{}
	g.CloseCurrentPopup()
	a.state.StatusMessage = "安全确认已取消，输出保持静音"
	a.safeUpdate()
}

func (a *GiuApp) genericSafetyConfirmationModal() g.Widget {
	confirmLabel := "确认并应用"
	if a.state.PendingSafetyStart {
		confirmLabel = "确认并开始输出"
	}
	return g.PopupModal(genericSafetyPopupID).
		Flags(g.WindowFlagsAlwaysAutoResize).
		Layout(
			g.Dummy(px(580), 1),
			fontLabel("安全风险确认", 16, ColorAlarmRed),
			fontLabel(a.state.SafetyAssessment.Title, 12.5, ColorTextBright),
			g.Separator(),
			g.Label(a.state.SafetyAssessment.WarningMessage).Wrapped(true),
			g.Spacing(),
			fontLabel("确认前输出保持静音。请降低监听音量并检查后级设备。", 11, ColorAmberWarning),
			g.Spacing(),
			g.Checkbox("以后不再提示此类风险（保存到本机）", &a.state.SuppressSafetyPrompt),
			g.Separator(),
			g.Row(
				StyleQuietButton(g.Button("取消并保持静音").Size(px(190), px(38)).OnClick(a.cancelSafety)),
				g.Spacing(),
				StyleDangerButton(g.Button(confirmLabel).Size(px(250), px(38)).OnClick(a.confirmSafety)),
			),
		)
}

func (a *GiuApp) RequestDoPConfirmation() {
	if !a.state.IsDoPSelected() {
		return
	}
	a.state.ShowDoPConfirm = true
	a.state.StatusMessage = "DoP 待安全确认：输出保持静音"
	g.OpenPopup(dopConfirmationPopupID)
}

func (a *GiuApp) confirmDoP() {
	wasRunning := a.state.IsOutputRunning()
	a.uiService.ConfirmSafety(string(controller.RiskDoP), true)
	a.state.DoPConfirmed = true
	a.state.ShowDoPConfirm = false
	g.CloseCurrentPopup()
	a.state.ApplyDoPConstraints()
	a.ApplyConfiguration()
	if a.state.ErrorMessage == "" {
		if wasRunning {
			a.state.IsActiveStream = true
			a.state.StreamStartTime = time.Now()
		}
		a.state.StatusMessage = "DoP 已解锁，载波参数已锁定"
	}
}

func (a *GiuApp) cancelDoPConfirmation() {
	a.state.ShowDoPConfirm = false
	a.state.DoPConfirmed = false
	g.CloseCurrentPopup()
	a.state.StatusMessage = "DoP 尚未解锁，输出保持静音"
}

func (a *GiuApp) dopConfirmationModal() g.Widget {
	return g.PopupModal(dopConfirmationPopupID).
		Flags(g.WindowFlagsAlwaysAutoResize).
		Layout(
			g.Dummy(px(540), 1),
			fontLabel("DoP DSD 载波输出 · 不可跳过的安全确认", 16, ColorAlarmRed),
			g.Separator(),
			g.Label(controller.DoPRiskWarningPrompt).Wrapped(true),
			g.Spacing(),
			fontLabel("本操作将强制使用 24-in-32、独占模式、0 dBFS Unity Gain，并禁用淡化和左右电平调整。", 11, ColorAmberWarning),
			fontLabel("若 DAC 不支持 dCS DoP v1.1，解码结果可能是不可预期的全幅噪声。", 11, ColorAmberWarning),
			g.Separator(),
			g.Row(
				StyleQuietButton(g.Button("取消并保持静音").Size(px(190), px(36)).OnClick(a.cancelDoPConfirmation)),
				g.Spacing(),
				StyleDangerButton(g.Button("我已知晓风险，启用 DoP").Size(px(270), px(36)).OnClick(a.confirmDoP)),
			),
		)
}

func (a *GiuApp) RequestNativeDSDConfirmation() {
	if !a.state.IsAsioSelected() {
		a.state.NativeDSDError = "Native DSD 仅支持 ASIO 输出设备"
		return
	}
	a.state.NativeDSDConfirmation = ""
	a.state.NativeDSDError = ""
	a.state.ShowNativeDSDConfirm = true
	g.OpenPopup(nativeDSDPopupID)
}

func (a *GiuApp) confirmNativeDSD() {
	if !a.state.IsAsioSelected() {
		a.state.NativeDSDError = "Native DSD 仅支持 ASIO 输出设备"
		return
	}
	if a.state.NativeDSDConfirmation != "YES" {
		a.state.NativeDSDError = "必须准确输入大写 YES 方可确认"
		return
	}
	if err := a.uiService.ConfirmNativeDSDRisk(a.state.NativeDSDConfirmation); err != nil {
		a.state.NativeDSDError = err.Error()
		return
	}

	nativeFormat := a.state.SelectedNativeDSDFormat().String()
	if err := a.uiService.ConfigureNativeDSD(nativeFormat); err != nil {
		a.state.NativeDSDError = err.Error()
		a.state.StatusMessage = "Native DSD 未启用：输出保持静音"
		a.safeUpdate()
		return
	}

	a.state.AsioStatus.OperatingMode = "Native DSD"
	a.state.AsioStatus.NativeDSDMode = true
	a.state.AsioStatus.NativeDSDActive = true
	a.state.AsioStatus.NativeDSDConfirmed = true
	a.state.AsioStatus.SelectedDSDFormat = nativeFormat
	a.state.NativeDSDConfirmed = true
	a.state.ShowNativeDSDConfirm = false
	g.CloseCurrentPopup()
	a.state.NativeDSDError = ""
	a.state.StatusMessage = "Native DSD 已创建：1-bit 直通缓冲已预载并保持静音"
	a.devicePanel.RefreshAsioStatus()
	a.safeUpdate()
}

func (a *GiuApp) cancelNativeDSDConfirmation() {
	if err := a.uiService.CancelNativeDSDEntry(); err != nil {
		a.state.NativeDSDError = err.Error()
	} else {
		a.state.NativeDSDError = "Native DSD 尚未确认，输出保持静音"
	}
	a.state.ResetNativeDSDSession()
	g.CloseCurrentPopup()
}

func (a *GiuApp) nativeDSDConfirmationModal() g.Widget {
	formatIndex := clampIndex(a.state.NativeDSDFormatIndex, len(NativeDSDFormatLabels))
	return g.PopupModal(nativeDSDPopupID).
		Flags(g.WindowFlagsAlwaysAutoResize).
		Layout(
			g.Dummy(px(560), 1),
			fontLabel("Native DSD 1-bit 直通 · 不可跳过的安全确认", 16, ColorAlarmRed),
			g.Separator(),
			g.Label("Native DSD 将绕过 PCM 与 DoP 封装，直接把 1-bit 位流写入 ASIO 硬件缓冲。错误的采样率、字节位序或 DAC 固件状态可能产生不可预期的全幅高频噪声。").Wrapped(true),
			g.Spacing(),
			fontLabel("请断开扬声器/耳机和后级功放，或将其增益降至最低。", 11, ColorAmberWarning),
			fontLabel("驱动不支持所选格式时，控制器会拒绝启用并保持静音。", 11, ColorAmberWarning),
			g.Separator(),
			captionLabel("Native DSD 测试格式"),
			g.Combo("##nativeDsdEntryFormat", NativeDSDFormatLabels[formatIndex], NativeDSDFormatLabels, &a.state.NativeDSDFormatIndex).
				Size(px(560)).
				OnChange(func() { a.state.NativeDSDError = "" }),
			captionLabel("请输入精确大写 YES 以解除核心安全拦截锁"),
			g.InputText(&a.state.NativeDSDConfirmation).
				Hint("YES").
				Flags(g.InputTextFlagsCharsUppercase).
				Size(px(560)),
			func() g.Widget {
				if a.state.NativeDSDError != "" && a.state.ShowNativeDSDConfirm {
					return fontLabel(a.state.NativeDSDError, 10.5, ColorAlarmRed)
				}
				return g.Dummy(px(1), px(1))
			}(),
			g.Spacing(),
			g.Row(
				StyleQuietButton(g.Button("取消并保持静音").Size(px(190), px(36)).OnClick(a.cancelNativeDSDConfirmation)),
				g.Spacing(),
				StyleDangerButton(g.Button("确认 YES 并启用 Native DSD").Size(px(300), px(36)).OnClick(a.confirmNativeDSD)),
			),
		)
}

// configureWindow sizes the OS window in physical pixels while preserving the
// 1440x780 logical design. Without this, a 250% display turns the intended
// desktop console into a tiny 576x312 workspace.
func (a *GiuApp) configureWindow(scale float32) {
	setUIScale(scale)

	workX, workY, workW, workH := primaryWorkArea()
	margin := int(math.Round(float64(px(18))))
	targetW := int(math.Round(float64(defaultWindowWidth) * float64(uiScale)))
	targetH := int(math.Round(float64(defaultWindowHeight) * float64(uiScale)))

	if maxW := workW - margin*2; targetW > maxW {
		targetW = maxW
	}
	if maxH := workH - margin*2; targetH > maxH {
		targetH = maxH
	}
	if targetW < 640 {
		targetW = 640
	}
	if targetH < 480 {
		targetH = 480
	}

	minW := int(math.Round(float64(minimumWindowWidth) * float64(uiScale)))
	minH := int(math.Round(float64(minimumWindowHeight) * float64(uiScale)))
	if maxW := workW - margin*2; minW > maxW {
		minW = maxW
	}
	if maxH := workH - margin*2; minH > maxH {
		minH = maxH
	}
	if minW > targetW {
		minW = targetW
	}
	if minH > targetH {
		minH = targetH
	}

	a.window.SetSizeLimits(minW, minH, -1, -1)
	a.window.SetSize(targetW, targetH)
	a.window.SetPos(workX+(workW-targetW)/2, workY+(workH-targetH)/2)
}

func (a *GiuApp) loop() {
	a.isRunning = true
	if backendScale := imgui.MainViewport().DpiScale(); backendScale > 0 {
		setUIScale(backendScale)
	}

	theme := ApplyLabTheme()
	theme.Push()
	defer theme.Pop()

	viewport := imgui.MainViewport().Size()
	viewportW, viewportH := viewport.X, viewport.Y
	if viewportW <= 0 || viewportH <= 0 {
		w, h := a.window.GetSize()
		viewportW, viewportH = float32(w), float32(h)
	}

	contentW := viewportW - px(rootPadding*2)
	contentH := viewportH - px(rootPadding*2)
	if contentW < px(900) {
		contentW = px(900)
	}
	if contentH < px(560) {
		contentH = px(560)
	}

	topH := px(topBarHeight)
	diagH := px(diagnosticsHeight)
	transH := px(transportBarHeight)
	gap := px(rootGap)
	mainH := contentH - topH - diagH - transH - gap*3
	if mainH < px(380) {
		mainH = px(380)
	}

	leftW := px(devicePanelWidth)
	if contentW < px(1260) {
		leftW = px(306)
	}
	generatorW := contentW - leftW - gap
	if generatorW < px(600) {
		generatorW = px(600)
	}

	g.SingleWindow().Size(viewportW, viewportH).Layout(
		a.topBar.Build(contentW, topH),
		g.Row(
			a.devicePanel.Build(leftW, mainH),
			a.generatorPanel.Build(generatorW, mainH),
		),
		a.diagnosticsPanel.Build(contentW, diagH),
		a.transportBar.Build(contentW, transH),
		a.dopConfirmationModal(),
		a.nativeDSDConfirmationModal(),
		a.genericSafetyConfirmationModal(),
	)
}

func (a *GiuApp) Run() error {
	a.window = g.NewMasterWindow("声卡测试信号发生器 · Pro Audio Lab", defaultWindowWidth, defaultWindowHeight, 0)
	a.window.SetBgColor(ColorCanvas)
	a.window.SetTargetFPS(60)

	scale := float32(imgui.CurrentStyle().FontScaleDpi())
	if scale <= 1.01 {
		scale = systemScale()
	}
	a.configureWindow(scale)

	a.devicePanel.RefreshDevices()
	a.ApplyConfiguration()

	go func() {
		// Giu stops refreshing when there is no input. Run the visual clock at
		// ~30 FPS so noise and sweep previews keep moving, while the heavier
		// hardware diagnostics remain on their original 10 Hz cadence.
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-ticker.C:
				if !a.isRunning {
					frame++
					continue
				}
				if frame%3 == 0 {
					a.diagnosticsPanel.PollMetrics()
				}
				if a.generatorPanel.ScopeNeedsAnimation() || frame%3 == 0 {
					g.Update()
				}
				frame++
			case <-a.stopPolling:
				return
			}
		}
	}()

	a.window.Run(a.loop)

	a.isRunning = false
	close(a.stopPolling)
	return nil
}
