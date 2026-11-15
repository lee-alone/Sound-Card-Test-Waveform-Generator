import { afterEach, describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DeviceSelector from '../components/DeviceSelector.vue'
import GeneratorPanel from '../components/GeneratorPanel.vue'
import DiagnosticsView from '../components/DiagnosticsView.vue'
import SafetyModal from '../components/SafetyModal.vue'
import AsioPanel from '../components/AsioPanel.vue'
import { api } from '../api'
import type { AsioStatus, DiagnosticsReport, ExtendedSignalRequest, SafetyAssessment } from '../types'

function createPCMAsioStatus(): AsioStatus {
  return {
    active_driver: 'ASIO Combo384 Driver',
    driver_version: '0x0103',
    sample_type: 'ASIOSTInt32LSB',
    operating_mode: 'PCM',
    clock_sources: [
      { index: 0, name: 'Internal Master Clock', is_current_source: true },
      { index: 1, name: 'S/PDIF Word Clock', is_current_source: false }
    ],
    active_clock_index: 0,
    buffer_sizes: {
      min_size: 64,
      max_size: 4096,
      preferred_size: 512,
      granularity: -1
    },
    active_buffer_size: 512,
    native_dsd_mode: false,
    native_dsd_active: false,
    native_dsd_confirmed: false,
    selected_dsd_format: '',
    native_dsd_capabilities: []
  }
}

function installAsioApiMock() {
  let current = createPCMAsioStatus()
  vi.spyOn(api, 'getAsioStatus').mockImplementation(async () => ({
    ...current,
    clock_sources: current.clock_sources.map(source => ({ ...source })),
    buffer_sizes: { ...current.buffer_sizes },
    native_dsd_capabilities: current.native_dsd_capabilities?.map(cap => ({ ...cap }))
  }))
  vi.spyOn(api, 'selectAsioDriver').mockResolvedValue()
  vi.spyOn(api, 'setAsioClockSource').mockResolvedValue()
  vi.spyOn(api, 'setAsioBufferSize').mockResolvedValue()
  vi.spyOn(api, 'confirmNativeDSDRisk').mockResolvedValue()
  vi.spyOn(api, 'cancelNativeDSDEntry').mockResolvedValue()
  const configure = vi.spyOn(api, 'configureNativeDSD').mockImplementation(async format => {
    current = {
      ...current,
      sample_type: 'ASIOSTDSDInt8LSB1',
      operating_mode: 'Native DSD',
      native_dsd_mode: true,
      native_dsd_active: true,
      native_dsd_confirmed: true,
      selected_dsd_format: format,
      native_dsd_buffer_frames: 256
    }
  })
  vi.spyOn(api, 'disableNativeDSD').mockImplementation(async () => {
    current = createPCMAsioStatus()
  })
  return { configure, getCurrent: () => current }
}

describe('Frontend Component Unit Tests', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  const dummyDiagnostics: DiagnosticsReport = {
    state: 'Running',
    device_name: 'Realtek High Definition Audio',
    exclusive: true,
    sample_rate: 96000,
    sample_format: '24-in-32 LE',
    channels: 2,
    buffer_period_ms: 10.0,
    buffer_frames: 960,
    bit_perfect_status: 'Eligible',
    render_callback_count: 1500,
    underrun_count: 0,
    late_callback_count: 0,
    last_render_time_us: 145.0,
    max_render_time_us: 420.0,
    p999_render_time_us: 280.0,
    render_time_percent_of_buffer: 1.45,
    max_percent_of_buffer: 4.2,
    p999_percent_of_buffer: 2.8,
    timestamp: new Date().toISOString()
  }

  it('DeviceSelector renders device info and negotiated parameters', () => {
    const wrapper = mount(DeviceSelector, {
      props: {
        diagnostics: dummyDiagnostics
      }
    })

    expect(wrapper.text()).toContain('WASAPI 独占排他模式')
    expect(wrapper.text()).toContain('96,000 Hz')
    expect(wrapper.text()).toContain('24-in-32 LE')
    expect(wrapper.text()).toContain('10.0 ms')
  })

  it('GeneratorPanel validates Nyquist limit (0.45 * Fs) and blocks start', async () => {
    const req: ExtendedSignalRequest = {
      device_id: 'mock-1',
      sample_rate: 48000, // 0.45 * 48000 = 21600 Hz limit
      sample_format: 0,
      channels: 2,
      waveform: 'sine',
      frequency: 25000.0, // Exceeds Nyquist!
      amplitude: 0.5,
      dc_offset: 0.0,
      left_volume_dbfs: -6.0,
      right_volume_dbfs: -6.0
    }

    const wrapper = mount(GeneratorPanel, {
      props: {
        request: req,
        diagnostics: { ...dummyDiagnostics, sample_rate: 48000, state: 'OpenMuted' }
      }
    })

    // Should indicate error and disable start button
    expect(wrapper.find('.input-error').exists()).toBe(true)
    const startBtn = wrapper.find('.btn-primary')
    expect((startBtn.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('频率超限')
  })

  it('DiagnosticsView accurately displays Bit-Perfect status and P99.9 metrics', () => {
    const wrapper = mount(DiagnosticsView, {
      props: {
        diagnostics: dummyDiagnostics
      }
    })

    expect(wrapper.text()).toContain('Eligible')
    expect(wrapper.text()).toContain('145.0 μs')
    expect(wrapper.text()).toContain('280.0 μs')
    expect(wrapper.text()).toContain('P99.9 门禁线')
  })

  it('SafetyModal enforces explicit user checkbox before unlocking', async () => {
    const assessment: SafetyAssessment = {
      has_risk: true,
      category: 'dop_carrier',
      title: '高危操作警示：DoP DSD 载波直通',
      warning_message: '若连接的 DAC 不支持 DoP，输出全幅强噪声！',
      requires_confirmation: true,
      is_confirmed: false
    }

    const wrapper = mount(SafetyModal, {
      props: {
        visible: true,
        assessment
      }
    })

    const unlockBtn = wrapper.find('.btn-stop')
    expect((unlockBtn.element as HTMLButtonElement).disabled).toBe(true)

    // Check confirmation checkbox
    const checkbox = wrapper.find('#confirmRiskCheck')
    await checkbox.setValue(true)

    // Unlock button must now be enabled
    expect((unlockBtn.element as HTMLButtonElement).disabled).toBe(false)

    // Click confirm
    await unlockBtn.trigger('click')
    expect(wrapper.emitted('confirm')).toBeTruthy()
    expect(wrapper.emitted('confirm')![0]).toEqual(['dop_carrier'])
  })

  it('AsioPanel starts in PCM and hides Native DSD test controls', async () => {
    installAsioApiMock()
    const wrapper = mount(AsioPanel)
    await flushPromises()

    expect(wrapper.text()).toContain('ASIO 驱动控制面板')
    expect(wrapper.text()).toContain('当前输出：PCM')
    expect(wrapper.text()).toContain('进入 Native DSD 测试模式…')
    expect(wrapper.text()).not.toContain('关闭 Native DSD · 恢复 PCM')
    expect(wrapper.text()).not.toContain('Native DSD 驱动能力检测')

    const options = wrapper.findAll('option').map(option => option.text()).join(' ')
    expect(options).not.toContain('DSD256 (11.2896 MHz)')
    expect(options).not.toContain('DSD512 (22.5792 MHz 驱动可选)')
  })

  it('AsioPanel requires explicit DSD entry and returns to PCM on exit', async () => {
    const mocked = installAsioApiMock()
    const wrapper = mount(AsioPanel)
    await flushPromises()

    const entryButton = wrapper.findAll('button').find(button => button.text().includes('进入 Native DSD 测试模式'))
    expect(entryButton).toBeDefined()
    await entryButton!.trigger('click')

    expect(wrapper.text()).toContain('不可跳过的 Native DSD 高危操作安全确认')
    const formatOptions = wrapper.findAll('option').map(option => option.text()).join(' ')
    expect(formatOptions).toContain('DSD256')

    const confirmation = wrapper.find('input[type="text"]')
    await confirmation.setValue('YES')
    const confirmButton = wrapper.findAll('button').find(button => button.text().includes('确认并解锁 Native DSD'))
    expect(confirmButton).toBeDefined()
    await confirmButton!.trigger('click')
    await flushPromises()

    expect(mocked.configure).toHaveBeenCalledWith('DSD256')
    expect(mocked.getCurrent().native_dsd_mode).toBe(true)
    expect(wrapper.text()).toContain('Native DSD 测试模式')
    expect(wrapper.text()).toContain('关闭 Native DSD · 恢复 PCM')

    const exitButton = wrapper.findAll('button').find(button => button.text().includes('关闭 Native DSD'))
    expect(exitButton).toBeDefined()
    await exitButton!.trigger('click')
    await flushPromises()

    expect(mocked.getCurrent().native_dsd_mode).toBe(false)
    expect(mocked.getCurrent().operating_mode).toBe('PCM')
    expect(wrapper.text()).toContain('当前输出：PCM')
    expect(wrapper.text()).toContain('进入 Native DSD 测试模式…')
    expect(wrapper.text()).not.toContain('关闭 Native DSD · 恢复 PCM')
  })

})
