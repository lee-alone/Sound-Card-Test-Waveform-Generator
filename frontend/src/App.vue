<template>
  <div class="min-h-screen bg-[#0a0c12] text-gray-100 p-4 md:p-6 font-sans">
    <div class="max-w-7xl mx-auto">
      <!-- App Header Bar -->
      <header class="flex flex-wrap items-center justify-between gap-4 mb-5 pb-4 border-b border-gray-800/80">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded bg-gradient-to-br from-blue-600 to-cyan-600 flex items-center justify-center text-xl font-black shadow-[0_0_15px_rgba(6,182,212,0.4)]">
            ∿
          </div>
          <div>
            <h1 class="text-lg md:text-xl font-black tracking-wider uppercase text-gray-100 flex items-center gap-2">
              <span>Windows 专业音频测试信号发生器</span>
              <span class="text-xs px-2 py-0.5 bg-blue-950 text-blue-400 border border-blue-800 rounded font-mono font-semibold">v2.0 PRO</span>
            </h1>
            <p class="text-xs text-gray-400">
              WASAPI 独占排他模式 &bull; 仪器级带限 DSP &bull; dCS DoP v1.1 &bull; 零隐式重采样 &bull; 失败即静音
            </p>
          </div>
        </div>

        <div class="flex items-center gap-3 font-mono-num text-xs">
          <span class="px-2.5 py-1 bg-emerald-950/60 border border-emerald-800 text-emerald-300 rounded flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
            <span>WebView2 Evergreen 引擎就绪</span>
          </span>
          <span class="px-2.5 py-1 bg-gray-900 border border-gray-800 text-gray-400 rounded">
            MMCSS Pro Audio 优先级
          </span>
        </div>
      </header>

      <!-- Playback Error Alert Banner -->
      <div
        v-if="playbackError"
        class="mb-4 p-3 bg-red-950/90 border border-red-500 rounded flex items-center justify-between text-red-200"
      >
        <div class="flex items-center gap-2">
          <span class="text-xl">⚠️</span>
          <div>
            <div class="font-bold">推流启动错误 (Playback Error)</div>
            <div class="text-sm font-mono-num">{{ playbackError }}</div>
          </div>
        </div>
        <button class="btn-secondary text-xs" @click="playbackError = ''">✕ 关闭</button>
      </div>

      <!-- T4.2: Device Selector Panel -->
      <DeviceSelector
        :diagnostics="diagnostics"
        @device-selected="handleDeviceSelected"
        @exclusive-toggled="handleExclusiveToggled"
      />

      <!-- T5.5: ASIO & Native DSD Control Panel -->
      <AsioPanel />

      <!-- T4.3: Full-Featured Signal Generator Panel -->
      <GeneratorPanel
        :request="currentRequest"
        :diagnostics="diagnostics"
        @update:request="currentRequest = $event"
        @start-requested="handleStartRequested"
        @stop-requested="handleStopRequested"
        @param-changed="handleParamChanged"
      />

      <!-- T4.4: 10Hz Real-Time Performance & Bit-Perfect Diagnostics View -->
      <DiagnosticsView :diagnostics="diagnostics" />

      <!-- T4.6: Presets & Configuration Persistence Panel -->
      <PresetPanel
        :current-request="currentRequest"
        @preset-applied="handlePresetApplied"
      />

      <!-- T4.5: Full-Coverage High-Risk Safety Modal -->
      <SafetyModal
        :visible="showSafetyModal"
        :assessment="currentAssessment"
        @confirm="handleSafetyConfirmed"
        @cancel="handleSafetyCancelled"
      />

      <!-- Footer Info -->
      <footer class="mt-8 pt-4 border-t border-gray-900 text-center text-xs text-gray-500 font-mono-num">
        Windows Professional Audio Test Waveform Generator &bull; 符合白皮书 v2.0 规范 &bull; P99.9 延迟严格小于 30% 缓冲区周期
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from './api'
import type { ExtendedSignalRequest, DiagnosticsReport, SafetyAssessment } from './types'

import DeviceSelector from './components/DeviceSelector.vue'
import AsioPanel from './components/AsioPanel.vue'
import GeneratorPanel from './components/GeneratorPanel.vue'
import DiagnosticsView from './components/DiagnosticsView.vue'
import PresetPanel from './components/PresetPanel.vue'
import SafetyModal from './components/SafetyModal.vue'

const currentRequest = ref<ExtendedSignalRequest>({
  device_id: '',
  sample_rate: 48000,
  sample_format: 0,
  channels: 2,
  waveform: 'sine',
  frequency: 1000.0,
  amplitude: 0.501187,
  dc_offset: 0.0,
  left_volume_dbfs: -6.0,
  right_volume_dbfs: -6.0,
  sweep: {
    start_freq: 20.0,
    end_freq: 20000.0,
    duration_sec: 5.0,
    taper_fraction: 0.10,
    loop: true
  },
  noise: {
    seed: 12345
  },
  multitone: {
    frequencies: [60, 7000],
    amplitudes: [0.8, 0.2]
  },
  dop: {
    dsd_format: 'DSD64',
    bit_order: 0
  }
})

const diagnostics = ref<DiagnosticsReport>({
  state: 'OpenMuted',
  device_name: 'Scanning...',
  exclusive: true,
  sample_rate: 48000,
  sample_format: '16-bit Integer',
  channels: 2,
  buffer_period_ms: 10.0,
  buffer_frames: 480,
  bit_perfect_status: 'Configured',
  render_callback_count: 0,
  underrun_count: 0,
  late_callback_count: 0,
  last_render_time_us: 0.0,
  max_render_time_us: 0.0,
  p999_render_time_us: 0.0,
  render_time_percent_of_buffer: 0.0,
  max_percent_of_buffer: 0.0,
  p999_percent_of_buffer: 0.0,
  timestamp: new Date().toISOString()
})

const showSafetyModal = ref<boolean>(false)
const playbackError = ref<string>('')
const currentAssessment = ref<SafetyAssessment>({
  has_risk: false,
  category: 'none',
  title: '',
  warning_message: '',
  requires_confirmation: false,
  is_confirmed: true
})

let diagPoller: number | null = null

// 10Hz Polling interval (100ms)
async function fetchDiagnostics() {
  try {
    const report = await api.getDiagnostics()
    diagnostics.value = report
  } catch (err) {
    // Suppress background poll errors
  }
}

async function handleDeviceSelected(id: string) {
  currentRequest.value.device_id = id
  await api.selectDevice(id)
  await handleParamChanged()
}

async function handleExclusiveToggled(exclusive: boolean) {
  await api.setExclusiveMode(exclusive)
  await handleParamChanged()
}

async function handleParamChanged() {
  try {
    await api.configureSignal(currentRequest.value)
  } catch (err: any) {
    console.warn('Parameter configure deferred:', err)
  }
}

// Start playback with mandatory safety guard verification (T4.5)
async function handleStartRequested() {
  playbackError.value = ''
  const assessment = await api.checkSafety(currentRequest.value)
  if (assessment.has_risk && assessment.requires_confirmation && !assessment.is_confirmed) {
    currentAssessment.value = assessment
    showSafetyModal.value = true
    return
  }

  try {
    await api.configureSignal(currentRequest.value)
    await api.startPlayback()
  } catch (err: any) {
    console.error('Failed to start playback:', err)
    playbackError.value = err.message || String(err)
  }
}

async function handleStopRequested() {
  playbackError.value = ''
  try {
    await api.stopPlayback()
  } catch (err: any) {
    console.error('Failed to stop playback:', err)
    playbackError.value = err.message || String(err)
  }
}

async function handleSafetyConfirmed(category: string) {
  showSafetyModal.value = false
  playbackError.value = ''
  await api.confirmSafety(category, true)
  try {
    await api.configureSignal(currentRequest.value)
    await api.startPlayback()
  } catch (err: any) {
    console.error('Failed to start playback after safety confirmation:', err)
    playbackError.value = err.message || String(err)
  }
}

function handleSafetyCancelled() {
  showSafetyModal.value = false
}

async function handlePresetApplied(req: ExtendedSignalRequest) {
  currentRequest.value = req
  await api.configureSignal(req)
}

onMounted(() => {
  diagPoller = window.setInterval(fetchDiagnostics, 100) // 10Hz poller
  fetchDiagnostics()
})

onUnmounted(() => {
  if (diagPoller !== null) {
    clearInterval(diagPoller)
  }
})
</script>
