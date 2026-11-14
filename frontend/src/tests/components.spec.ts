import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DeviceSelector from '../components/DeviceSelector.vue'
import GeneratorPanel from '../components/GeneratorPanel.vue'
import DiagnosticsView from '../components/DiagnosticsView.vue'
import SafetyModal from '../components/SafetyModal.vue'
import AsioPanel from '../components/AsioPanel.vue'
import type { DiagnosticsReport, ExtendedSignalRequest, SafetyAssessment } from '../types'

describe('Frontend Component Unit Tests', () => {
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

  it('AsioPanel renders controls and blocks unconfirmed Native DSD with safety modal', async () => {
    const wrapper = mount(AsioPanel)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('ASIO 驱动与 Native DSD 原生直通控制面板')
    expect(wrapper.text()).toContain('ASIOGetLatencies 报告与回环脉冲实测')

    // Click "启用 Native DSD 直通"
    const enableBtn = wrapper.findAll('button').find(b => b.text().includes('启用 Native DSD 直通'))
    expect(enableBtn).toBeDefined()
    await enableBtn!.trigger('click')

    // Safety Warning Modal should be visible
    expect(wrapper.text()).toContain('不可跳过的 Native DSD 高危操作安全确认')
  })
})
