# Windows 专业音频测试信号发生器
## 架构与开发白皮书 v2.0

---

## 1. 项目定位、背景与目标

### 1.1 产品定位

本项目第一阶段定位为一款面向 Windows 平台的高精度音频测试信号发生器，为声卡、外置 USB DAC、解码耳放一体机及其他音频设备提供可控、可复现、可验证的数字测试信号。

第一阶段负责准确生成和输出信号，不直接宣称能够独立测量 DAC 的 THD、THD+N、SNR、频率响应等指标。完整测量需要经过被测设备、模拟采集设备和分析算法构成闭环，相关能力作为后续扩展目标：

    Signal Generator → Render Backend → DUT
                                      ↓
    Analyzer         ← Capture Backend ← ADC / Loopback

因此：

- 当前核心产品：专业音频测试信号发生器。
- 后续扩展产品：包含采集、校准和分析功能的闭环音频测试系统。

### 1.2 现状与痛点

传统 Windows 音频软件在专业硬件测试场景中通常存在以下限制：

1. **共享音频路径不可完全控制**：WASAPI Shared Mode 使用系统音频引擎的混合格式。当应用采样率与设备 Mix Format 不一致，或启用了系统音量、APO、音效及其他处理时，输出样本可能发生格式转换或内容修改，因此不适合作为需要严格控制样本内容的测量路径。
2. **缺乏可验证的位完美输出**：普通播放器往往不能明确显示请求格式、实际打开格式、软件增益、有效位宽和信号处理状态，用户难以证明从生成器到驱动的数据是否保持不变。
3. **高采样率与超声测试配置复杂**：20kHz 以上信号需要匹配足够高的输出采样率，并同时考虑 DAC 数字滤波、模拟输出带宽和外部测量设备带宽。
4. **DSD 测试工具不足**：常规信号发生器较少提供可验证的 DSD 测试向量、DoP 封装和 Native DSD 输出路径。
5. **安全风险容易被忽略**：超声、多音、方波、DoP 误解码和高电平输出可能损伤扬声器、高音单元、耳机或后级放大器。

### 1.3 项目目标

- 提供 WASAPI Exclusive PCM 输出。
- 提供可选的 ASIO PCM 与 Native DSD 输出。
- 支持高采样率正弦、扫频、噪声、双音、多音和带限周期波形。
- 支持 DSD 测试向量生成、DoP 封装和设备识别验证。
- 明确展示请求格式、实际格式、信号峰值和输出状态。
- 对关键算法、格式封装和实时性能建立自动化验收标准。
- 所有失败路径优先进入静音状态。

### 1.4 非目标

第一版不承诺：

- 仅凭 DAC 面板点亮 DSD 指示灯即可证明模拟输出质量。
- 仅凭 WASAPI Exclusive 或 ASIO backend 名称即可自动证明位完美。
- 使用普通声卡模拟回环即可获得实验室级测量精度。
- 在未经校准的条件下给出绝对 THD、THD+N、SNR 或声压结论。

### 1.5 核心设计原则

- **低耦合**：硬件驱动、格式封装、DSP、实时调度、业务控制和图形界面分层实现。
- **零隐式重采样**：生成器采样率必须与已打开的硬件流采样率一致；不在内部静默重采样。
- **稳定播放阶段可验证位完美**：只有在格式匹配、软件增益为 unity、无 DSP 修改且验证通过时，才标记为 Bit-Perfect Eligible。
- **实时线程最小化**：渲染热路径不得分配内存、记录日志、调用 UI、阻塞或执行设备管理操作。
- **失败即静音**：设备失效、underrun、格式变化、DoP 标记中断或控制端异常时立即静音。
- **先验证后扩展**：先完成 PCM 输出和字节级验证，再开发 DoP，最后开发 Native DSD 和闭环分析。

---

## 2. 总体架构

~~~mermaid
flowchart TD
    subgraph UI_Layer ["表现层（GIU）"]
        View["仪表盘与参数交互"]
        State["UI 状态存储"]
    end

    subgraph Controller_Layer ["控制层（pkg/controller）"]
        AppController["应用控制器"]
        Validator["参数与安全校验"]
    end

    subgraph Engine_Layer ["实时引擎层（pkg/engine）"]
        AudioEngine["音频引擎"]
        StateMachine["生命周期状态机"]
        CommandQueue["无阻塞控制命令队列"]
        Fader["样本级淡入淡出"]
        Diagnostics["实时诊断计数器"]
    end

    subgraph DSP_Layer ["DSP 层（pkg/dsp）"]
        GenInterface["WaveformGenerator"]
        SineGen["正弦波"]
        PeriodicGen["带限方波/三角波"]
        SweepGen["线性/对数扫频"]
        NoiseGen["白噪声/粉红噪声"]
        MultiGen["双音/多音"]
    end

    subgraph Format_Layer ["格式层（pkg/format）"]
        PCMEncoder["PCM 量化与封装"]
        PeakGuard["峰值与削波校验"]
        ChannelMapper["通道映射"]
    end

    subgraph DSD_Layer ["DSD 层（pkg/dsd）"]
        DSDVector["DSD 测试向量"]
        Modulator["Sigma-Delta 调制器"]
        DoPPacker["DoP 封装器"]
    end

    subgraph Audio_Layer ["硬件层（pkg/audio）"]
        BackendInterface["AudioBackend"]
        WASAPIBackend["WASAPI Exclusive"]
        ASIOBackend["ASIO"]
        CapProber["能力探测器"]
    end

    View --> AppController
    AppController --> Validator
    Validator --> AudioEngine
    AudioEngine --> StateMachine
    AudioEngine --> CommandQueue
    AudioEngine --> DSP_Layer
    DSP_Layer --> Format_Layer
    DSD_Layer --> Format_Layer
    Format_Layer --> Audio_Layer
    AudioEngine --> Diagnostics
    Audio_Layer --> Hardware["DAC / 声卡"]
~~~

### 2.1 依赖规则

- UI 只能调用 Controller，不得直接访问 Engine、DSP 或 Audio Backend。
- DSP 不得依赖 Windows API、GUI、设备对象或日志组件。
- Audio Backend 不得包含波形算法。
- DSD 调制与 DoP 传输必须独立，便于分别验证。
- 实时线程只读取准备完成的不可变配置或无阻塞命令。
- Controller 不得持有实时缓冲区的可写引用。
- Backend 错误必须保留原始错误码，并转换成统一诊断事件。

### 2.2 建议源码结构

    sound-signal-generator/
    ├── cmd/
    │   ├── app-giu/
    │   │   └── main.go
    │   └── cli-mvp/
    │       └── main.go
    ├── internal/
    │   ├── audio/
    │   │   ├── backend.go
    │   │   ├── device.go
    │   │   ├── capability.go
    │   │   ├── wasapi/
    │   │   │   ├── backend_windows.go
    │   │   │   ├── stream_windows.go
    │   │   │   └── com_windows.go
    │   │   └── asio/
    │   │       ├── bridge_windows.go
    │   │       └── stream_windows.go
    │   ├── format/
    │   │   ├── pcm.go
    │   │   ├── quantizer.go
    │   │   ├── channel.go
    │   │   └── peak.go
    │   ├── dsp/
    │   │   ├── generator.go
    │   │   ├── sine.go
    │   │   ├── polyblep_square.go
    │   │   ├── polyblep_triangle.go
    │   │   ├── sweep.go
    │   │   ├── noise.go
    │   │   └── multitone.go
    │   ├── dsd/
    │   │   ├── types.go
    │   │   ├── vector.go
    │   │   ├── modulator.go
    │   │   └── dop.go
    │   ├── engine/
    │   │   ├── engine.go
    │   │   ├── state.go
    │   │   ├── commands.go
    │   │   ├── buffer.go
    │   │   ├── fader.go
    │   │   └── diagnostics.go
    │   ├── controller/
    │   │   ├── controller.go
    │   │   └── validation.go
    │   └── ui/
    │       └── giu/
    │           ├── app.go
    │           ├── device_panel.go
    │           ├── generator_panel.go
    │           ├── diagnostics_panel.go
    │           └── theme.go
    ├── testdata/
    │   ├── pcm/
    │   └── dop/
    └── docs/
        ├── format-contract.md
        ├── safety.md
        └── validation-matrix.md

业务实现优先放在 internal 下，避免在接口尚未稳定时形成对外兼容承诺。只有确实需要被外部项目复用的稳定模块才迁移到 pkg。

---

## 3. 输出格式与位完美契约

### 3.1 输出格式模型

每个音频流必须记录并显示：

- Device ID 与设备名称。
- Backend 类型。
- Exclusive、Shared 或 ASIO 模式。
- 请求采样率与实际采样率。
- 通道数与 Channel Mask。
- Sample Type：PCM Integer、IEEE Float、DoP 或 Native DSD。
- Container Bits 与 Valid Bits。
- Frame Size、Buffer Frames 和 Device Period。
- 软件增益、淡入淡出和其他样本修改状态。

能力探测不得只依赖设备宣传信息。程序应针对候选格式调用 backend 的格式探测接口，并区分：

- Supported
- Unsupported
- Invalid Configuration
- Device Busy
- Driver Error

只有实际成功打开并启动流后，才能把格式标记为 Operational。

### 3.2 位完美定义

满足以下全部条件时，稳定播放区间才可标记为 Bit-Perfect Eligible：

1. 生成采样率与实际设备采样率一致。
2. 无软件重采样。
3. 软件增益为 0dB，且未进行静音、淡化、限幅或其他样本修改。
4. PCM 的量化、有效位宽、容器位宽、端序和通道交织符合目标格式。
5. 未启用会修改数据的 APO 或应用 DSP。
6. Backend 未报告格式回退或隐式转换。
7. 已通过对应格式的数字样本验证。

界面应区分：

- **Configured**：配置看起来满足条件。
- **Eligible**：软件内部链路满足条件。
- **Verified**：已通过外部数字捕获或其他可信方式逐样本验证。

独占模式或 ASIO 模式本身不能直接显示为 Verified。

### 3.3 PCM 量化与封装

格式层必须明确：

- 浮点输入的有效范围。
- 超范围样本的处理策略。
- 浮点到整数的舍入规则。
- 16-bit、24-bit packed、24-valid-in-32 和 32-bit PCM 的布局。
- 小端字节序和通道交织顺序。
- TPDF Dither 的开关与适用场景。
- 任何 Dither 或 Noise Shaping 启用后，位完美状态必须相应更新。

不得使用隐藏 limiter 或自动增益修复削波。若预测峰值超过格式范围，应拒绝启动并提示用户降低电平。

---

## 4. DSP 信号生成规范

### 4.1 统一生成器契约

建议生成器具有以下职责边界：

~~~go
type Generator interface {
    Prepare(Config) error
    Reset(ResetPolicy)
    Render(dst []float64)
    MaxPeak() float64
}
~~~

- Prepare 在非实时线程执行，可分配内存并完成系数计算。
- Render 在实时线程执行，不得分配、阻塞或记录日志。
- 参数切换通过预构建配置和无阻塞命令完成。
- 频率、电平和相位改变必须明确是连续切换还是重新起相。

### 4.2 频率与采样率约束

默认使用以下安全规则：

    f_max ≤ 0.45 × Fs

建议的最低采样率：

| 信号频率 | 最低输出采样率 | 建议采样率 |
|---|---:|---:|
| ≤ 20kHz | 48kHz | 96kHz |
| 20～40kHz | 96kHz | 192kHz |
| 40～85kHz | 192kHz | 384kHz |
| 85～170kHz | 384kHz | 更高规格或专用硬件 |

这些规则只保证数字采样余量，不保证 DAC 模拟输出级、滤波器和外部仪器具备相同带宽。

### 4.3 正弦波

- 使用高精度相位累加器。
- 长时间运行时相位不得无界增长。
- 频率切换默认保持相位连续。
- 支持独立设置初始相位、声道相位差和极性。
- 自动化测试应检查频率误差、振幅误差、DC 偏置和非预期杂散。

### 4.4 带限方波与三角波

方波和三角波可使用 PolyBLEP 或其他带限算法减少混叠，但不得对所有频率统一承诺固定的 -80dBFS 抑制能力。

验收应覆盖多个 f/Fs 比值，并测量：

- 理论谐波。
- 非预期混叠杂散。
- DC 偏置。
- 峰值误差。
- 高频接近 Nyquist 时的退化行为。

带限方波适合观察受控谐波响应，不应被描述为未经处理的理想阶跃。瞬态和传递函数测试应另行提供脉冲、MLS、扫频或多音方案。

### 4.5 扫频

扫频配置至少包含：

- 线性或对数模式。
- 起止频率和时长。
- 起止相位策略。
- 幅度包络和前后静音时间。
- 窗函数。
- 是否生成 inverse filter 或时间映射数据。

### 4.6 噪声

- 白噪声随机种子必须可选且可复现。
- 明确高斯或均匀分布。
- 粉红噪声必须记录所用算法和目标频谱容差。
- 界面同时显示 Peak 和 RMS 估计。
- 噪声输出不得默认使用接近满幅的电平。

### 4.7 双音与多音

- 支持常用互调测试预设，同时允许自定义频率和幅度。
- 在启动前计算理论峰值和 crest factor。
- 进行峰值归一化时必须明确显示缩放量。
- 禁止各正弦简单相加后发生隐藏削波。
- 支持可复现的初始相位组合。

---

## 5. DSD 与 DoP 设计

### 5.1 分层原则

DSD 功能分为两个可独立验证的部分：

1. **DSD 内容生成**：生成可信的 1-bit 测试向量。
2. **DSD 传输封装**：通过 DoP 或 Native DSD 将已有 bitstream 传输给设备。

第一阶段优先使用离线生成并经过验证的 DSD 测试向量，以便把调制算法问题与传输问题分离。

### 5.2 Sigma-Delta 调制器

设计文档必须记录：

- 调制器阶数、拓扑和系数。
- 输入电平范围和稳定边界。
- DC 输入行为。
- 空闲音和 limit-cycle 行为。
- Noise Transfer Function。
- 带内噪声和超声噪声目标。
- 状态溢出及长时间稳定性。
- DSD64、128、256 下的独立验收结果。

在完成稳定性和频谱验证前，实时调制器不得作为默认输出来源。

### 5.3 DoP 封装

DoP 采用交替 marker 的 PCM 容器传输 DSD 数据。实现必须明确并测试：

- 每声道每个 PCM frame 携带 16 位 DSD 数据。
- marker 使用 0x05 与 0xFA，并按 PCM frame 交替。
- 同一个 PCM frame 的所有声道使用一致 marker。
- DSD bit order。
- 24-bit packed 与 24-valid-in-32-bit container 的内存布局。
- Windows 小端内存中的字节顺序。
- 暂停、underrun、重开设备后 marker 序列的恢复策略。

常见承载关系：

| DSD 规格 | DSD 码率 | DoP PCM 采样率 |
|---|---:|---:|
| DSD64 | 2.8224MHz | 176.4kHz |
| DSD128 | 5.6448MHz | 352.8kHz |
| DSD256 | 11.2896MHz | 705.6kHz |

最终是否可用取决于设备和驱动是否支持对应 PCM 容器格式与采样率。

### 5.4 Native DSD

Native DSD 通过 ASIO 等驱动接口实现。该路径预计需要 C 或 C++ bridge，并与 Go 实时线程模型隔离。

在实现前必须完成：

- SDK 授权和发行方式确认。
- 32/64 位 ABI 与驱动兼容性验证。
- 驱动回调线程限制验证。
- PCM、DoP 和 Native DSD 模式切换测试。
- 多厂商驱动兼容性测试。

### 5.5 DSD 判定原则

- DAC 点亮 DSD 指示灯，只证明设备识别到了相应数据流。
- 它不能单独证明调制器正确、模拟输出纯净或失真达标。
- DoP 被当作 PCM 播放时，输出内容、频谱和电平不可预设。
- 禁止让用户通过耳机或扬声器试听噪声来判断 DoP 是否成功。

---

## 6. 实时线程、缓冲与生命周期

### 6.1 执行域

系统至少划分三个执行域：

1. **UI / Controller**
   - RPC、参数校验、状态展示和用户操作。
   - 允许内存分配和日志。

2. **Control Worker**
   - 设备枚举、格式探测、打开和关闭流。
   - 创建不可变运行配置。
   - 处理设备失效与恢复。

3. **Render Thread**
   - 等待设备事件。
   - 生成或复制样本。
   - 量化、封装并提交设备缓冲。
   - 更新预分配的诊断计数器。

涉及 COM、MMCSS 或线程局部驱动状态的 goroutine 必须绑定到固定 OS Thread。

### 6.2 热路径约束

Render 热路径禁止：

- make、new 和 slice 扩容。
- fmt、反射和动态错误字符串构造。
- 日志、文件操作、网络操作和 UI 调用。
- 阻塞 channel。
- mutex 和未知时长的系统调用。
- COM 对象创建、释放或设备枚举。

所有临时缓冲、量化缓冲和状态对象必须提前分配。

### 6.3 缓冲策略

第一阶段同时评估：

1. 事件唤醒后直接生成到 backend buffer。
2. DSP Producer + SPSC Ring Buffer + Backend Consumer。

最终以以下指标决定：

- underrun 数量。
- 端到端延迟。
- CPU 占用和抖动。
- 实现复杂度。
- 参数切换行为。

不得仅因为“无锁”而默认采用双环形缓冲。若使用 Ring Buffer，必须明确生产者、消费者、内存序、满/空策略和计数器回绕处理。

### 6.4 生命周期状态机

~~~mermaid
stateDiagram-v2
    [*] --> Closed
    Closed --> Probing
    Probing --> OpenMuted
    OpenMuted --> Primed
    Primed --> Running
    Running --> FadingOut
    FadingOut --> OpenMuted
    OpenMuted --> Closed

    Probing --> Faulted
    OpenMuted --> Recovering
    Primed --> Recovering
    Running --> Recovering
    FadingOut --> Recovering
    Recovering --> OpenMuted
    Recovering --> Faulted
    Faulted --> Closed
~~~

关键规则：

- 设备打开后必须先处于 OpenMuted。
- 缓冲预填完成后才能进入 Running。
- 设备、采样率、位深、通道和输出模式变化必须先淡出。
- 设备重连后不得自动恢复有声输出。
- underrun、设备失效或 DoP 连续性破坏时立即转入静音或恢复状态。

### 6.5 淡入淡出

- 使用按样本计数的平滑增益包络。
- 时长可配置，默认值应根据输出模式和风险确定。
- 5ms 不作为所有设备和信号的固定保证。
- 波形切换可采用淡出、替换生成器、预填、淡入流程。
- 位完美状态只适用于淡入完成后的稳定播放区间。

---

## 7. 安全设计

### 7.1 默认安全行为

- 应用启动时输出静音。
- 新设备首次选择时使用保守默认电平。
- 不自动恢复上一次高电平输出。
- 设备热插拔或驱动重置后保持静音。
- 超声、方波、多音、DoP 和 Native DSD 模式显示明显风险提示。
- 所有失败路径执行 fail-safe mute。

### 7.2 启动前检查

启动输出前检查：

- 目标设备和格式仍然有效。
- 理论峰值不超过数字满幅。
- 软件增益和预计 RMS 已显示。
- 高风险模式已由用户明确确认。
- DoP/DSD 模式已提示断开扬声器或降低后级增益。

### 7.3 禁止行为

- 禁止用听到的噪声判断 DoP 是否成功。
- 禁止在后台自动切换到另一个输出设备并继续播放。
- 禁止通过隐藏 limiter、AGC 或自动归一化掩盖错误配置。
- 禁止在错误恢复后自动解除静音。

---

## 8. 诊断与可观测性

UI 和日志应记录：

- Device ID、驱动和 backend。
- 请求格式与实际打开格式。
- Buffer Frames、Device Period 和估计延迟。
- 渲染回调次数。
- underrun、late callback 和恢复次数。
- 最大 Render 用时及相对 buffer period 的占比。
- 最后一次 backend 原始错误码。
- 当前状态机状态。
- 当前生成器、频率、电平和理论峰值。
- Bit-Perfect 状态：Configured、Eligible 或 Verified。

实时线程只更新预分配的原子计数器；日志格式化由普通线程完成。

---

## 9. 自动化测试与验收

### 9.1 单元测试

- 相位累加和频率精度。
- 波形峰值、RMS 和 DC。
- 多音 crest factor 与削波预判。
- PCM 舍入、端序、有效位宽和通道交织。
- DoP marker、bit order、声道同步和连续性。
- DSD 固定输入对应的 golden bitstream。
- 状态机合法与非法转换。

### 9.2 频谱测试

针对每种生成器建立离线 FFT 测试：

- 目标频率误差。
- 目标幅度误差。
- 非预期杂散。
- 混叠分量。
- 噪声频谱斜率及容差。
- 不同 f/Fs 比值下的退化。

### 9.3 实时性能测试

- Render 热路径每次调用零分配。
- 长时间播放无 underrun。
- 参数连续变化无死锁、崩溃或爆音。
- 高 CPU 负载下正确记录 underrun 并安全恢复。
- 设备热插拔、休眠恢复和独占冲突测试。

建议至少提供短时 CI 测试和 1、8、24 小时人工或实验室 soak test。

### 9.4 位完美验证

优先使用已知伪随机样本序列和可信数字捕获路径逐样本比较。

验证记录应包含：

- 测试设备和驱动版本。
- 请求与实际格式。
- 原始样本 hash。
- 捕获样本 hash 或差异位置。
- 是否经过淡入淡出区间。
- 是否启用 Dither、增益或其他处理。

模拟回环可用于评价整条模拟链路，但不能单独证明内部数字路径位完美。

### 9.5 硬件兼容矩阵

至少覆盖：

- 板载 HDA 设备。
- USB Audio Class DAC。
- 支持 176.4/192kHz 的设备。
- 支持 352.8/384kHz 的设备。
- 支持 DoP 的 DAC。
- 不支持 DoP 的 PCM 设备。
- 多个厂商的 ASIO 驱动。

---

## 10. 分阶段研发计划

### Phase 0：高风险技术验证

- 建立最小 WASAPI Exclusive 事件驱动原型。
- 验证 Go、COM、MMCSS 和固定 OS Thread 模型。
- 完成 PCM 16/24/32-bit 字节布局实验。
- 验证设备失效和热插拔恢复行为。
- 调研并确认 ASIO SDK 授权、ABI 和发布方式。

退出条件：

- 至少一台真实设备可稳定输出静音和已知 PCM pattern。
- 能显示请求格式、实际格式和 backend 错误。
- 已确认核心线程模型可行。

### Phase 1：安全 PCM MVP

- 完成设备枚举、能力探测和 WASAPI Exclusive backend。
- 实现正弦波、静音和基础通道映射。
- 实现 PCM 量化与封装。
- 实现状态机、默认静音、淡入淡出和诊断计数器。

退出条件：

- 96/192kHz 输出在目标设备上通过验证。
- Render 热路径零分配。
- 设备拔出和格式错误不会产生持续异常输出。

### Phase 2：DSP 与自动验证

- 扫频、噪声、双音、多音和带限周期波形。
- 建立离线频谱测试和 golden vectors。
- 增加削波预判、crest factor 和可复现随机种子。

退出条件：

- 各算法通过明确的数值和频谱验收阈值。
- 高频测试由外部示波器、频谱仪或宽带 ADC 交叉验证。

### Phase 3：DoP

- 引入已验证的离线 DSD 测试向量。
- 完成 DoP 封装、格式探测和安全确认流程。
- 建立支持和不支持 DoP 的设备测试矩阵。

退出条件：

- DoP golden byte vectors 全部通过。
- 目标 DAC 能稳定识别 DSD。
- 错误设备和中断路径均能安全静音。

### Phase 4：完整 GUI

- 构建仪器风格 GIU 原生 GUI。
- 实现设备、格式、波形、频率和电平控制。
- 显示状态机、实际格式、风险提示和实时诊断。
- 支持配置导入导出，但不自动恢复有声输出。

### Phase 5：ASIO 与 Native DSD

- 实现 ASIO bridge。
- 支持 ASIO PCM 和 Native DSD。
- 完成多厂商驱动兼容测试。
- 完成安装包、签名和发布流程。

### Phase 6：闭环测量扩展

- 增加 Capture Backend。
- 增加 FFT、频响、THD、THD+N、SNR、IMD 和串扰分析。
- 增加时钟漂移、延迟对齐、校准文件和测试报告。

---

## 11. 发布门槛

任何公开版本必须满足：

1. 不存在已知的高电平自动恢复路径。
2. 设备失效、underrun 和格式切换能够安全静音。
3. 所有格式封装具有 golden vector 测试。
4. 所有“位完美”声明均区分 Eligible 与 Verified。
5. DoP 和 Native DSD 带有不可跳过的首次安全提示。
6. 实时线程不执行已知的动态分配、日志或阻塞操作。
7. 发布说明列出已验证的设备、格式和已知限制。
8. ASIO 等第三方 SDK 的授权和分发条件已经确认。

---

## 12. 开发行动准则

1. 不在单一文件中混合界面、DSP、格式封装和硬件驱动。
2. 不用 backend 名称替代实际输出验证。
3. 不在运行时静默降低采样率、位深或通道数。
4. 不使用未经测试的 DSD 调制器驱动真实高功率输出链路。
5. 每增加一种格式，必须同时增加 golden vector 和错误路径测试。
6. 每增加一种波形，必须同时定义峰值、RMS、频谱和安全边界。
7. 对无法证明的性能指标使用“目标”而非“保证”。
8. 架构服从可测性和安全性，不为了形式上的分层增加实时路径复杂度。

