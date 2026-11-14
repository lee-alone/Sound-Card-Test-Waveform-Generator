<template>
  <div class="bg-gray-900/90 border border-cyan-900/60 rounded-lg p-4 md:p-5 mb-5 shadow-xl backdrop-blur">
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-3 mb-4 pb-3 border-b border-gray-800">
      <div class="flex items-center gap-2">
        <span class="w-3 h-3 rounded-full bg-cyan-400 animate-pulse shadow-[0_0_8px_rgba(6,182,212,0.8)]"></span>
        <h2 class="text-base font-bold text-gray-100 uppercase tracking-wider flex items-center gap-2">
          <span>ASIO 驱动与 Native DSD 原生直通控制面板</span>
          <span class="text-xs px-2 py-0.5 bg-cyan-950 text-cyan-300 border border-cyan-800 rounded font-mono font-semibold">C-BRIDGE</span>
        </h2>
      </div>

      <div class="flex items-center gap-2 font-mono text-xs">
        <span class="px-2 py-0.5 bg-gray-800 text-gray-300 rounded border border-gray-700">
          驱动版本: {{ status.driver_version || 'N/A' }}
        </span>
        <span class="px-2 py-0.5 bg-indigo-950 text-indigo-300 rounded border border-indigo-800 font-semibold">
          SampleType: {{ status.sample_type || 'Unknown' }}
        </span>
      </div>
    </div>

    <!-- Main Controls Grid -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
      <!-- 1. Driver Selection -->
      <div class="bg-gray-950/70 p-3.5 rounded border border-gray-800">
        <label class="block text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">ASIO 驱动选择</label>
        <select
          v-model="selectedDriver"
          @change="handleDriverChange"
          class="w-full bg-gray-900 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-cyan-500 font-mono"
        >
          <option value="ASIO Combo384 Driver">ASIO Combo384 Driver (Native DSD)</option>
          <option value="Thesycon USB Audio 2.0 Class Driver">Thesycon USB Audio 2.0 (DSD512)</option>
          <option value="Yamaha Steinberg USB ASIO">Yamaha Steinberg USB ASIO (PCM)</option>
          <option value="ASIO4ALL v2">ASIO4ALL v2 (WDM-to-ASIO)</option>
        </select>
        <p class="text-[11px] text-gray-500 mt-2 font-mono">
          底层驱动: {{ status.active_driver || selectedDriver }}
        </p>
      </div>

      <!-- 2. Hardware Clock Source -->
      <div class="bg-gray-950/70 p-3.5 rounded border border-gray-800">
        <label class="block text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">硬件时钟源 (Clock Source)</label>
        <select
          v-model="selectedClock"
          @change="handleClockChange"
          class="w-full bg-gray-900 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-cyan-500 font-mono"
        >
          <option
            v-for="clk in status.clock_sources"
            :key="clk.index"
            :value="clk.index"
          >
            {{ clk.name }} {{ clk.is_current_source ? '★ (Active)' : '' }}
          </option>
        </select>
        <p class="text-[11px] text-gray-500 mt-2 font-mono">
          支持外部字时钟 (Word Clock) 与内部晶振切换
        </p>
      </div>

      <!-- 3. Buffer Size & Latency Tuning -->
      <div class="bg-gray-950/70 p-3.5 rounded border border-gray-800">
        <label class="block text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">ASIO 缓冲区帧数 (Buffer Size)</label>
        <select
          v-model.number="selectedBufferSize"
          @change="handleBufferSizeChange"
          class="w-full bg-gray-900 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-cyan-500 font-mono"
        >
          <option :value="64">64 frames (超低延迟 1.33ms)</option>
          <option :value="128">128 frames (推荐极低延迟 2.67ms)</option>
          <option :value="256">256 frames (仪器测试基准 5.33ms)</option>
          <option :value="512">512 frames (高稳定性 10.67ms)</option>
          <option :value="1024">1024 frames (极限稳定性 21.33ms)</option>
        </select>
        <p class="text-[11px] text-cyan-400 mt-2 font-mono">
          限制范围: [{{ status.buffer_sizes.min_size }}, {{ status.buffer_sizes.max_size }}] 帧
        </p>
      </div>
    </div>

    <!-- Latency & Jitter Monitor (Task T5.2) -->
    <div v-if="status.latency_report" class="bg-gray-950/90 rounded border border-gray-800 p-3.5 mb-4 font-mono text-xs">
      <div class="flex flex-wrap items-center justify-between gap-2 mb-2 pb-2 border-b border-gray-800/80">
        <span class="font-bold text-gray-300">ASIOGetLatencies 报告与回环脉冲实测 (100次重复统计)</span>
        <span class="px-2 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded">
          抖动标准差: &sigma; = {{ status.latency_report.jitter_standard_deviation.toFixed(4) }} 样本 (&le; 1.0 样本合格)
        </span>
      </div>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-gray-400">
        <div>
          <span class="text-gray-500">报告输入延迟:</span>
          <span class="text-gray-200 font-bold ml-1">{{ status.latency_report.reported_input_latency }} 帧</span>
        </div>
        <div>
          <span class="text-gray-500">报告输出延迟:</span>
          <span class="text-gray-200 font-bold ml-1">{{ status.latency_report.reported_output_latency }} 帧</span>
        </div>
        <div>
          <span class="text-gray-500">实测往返延迟:</span>
          <span class="text-cyan-300 font-bold ml-1">{{ status.latency_report.measured_round_trip_latency.toFixed(2) }} 帧</span>
        </div>
        <div>
          <span class="text-gray-500">固定转换器偏移:</span>
          <span class="text-yellow-300 font-bold ml-1">{{ status.latency_report.converter_offset.toFixed(2) }} 帧</span>
        </div>
      </div>
    </div>

    <!-- Native DSD Section (Task T5.3 & T5.5) -->
    <div class="bg-gradient-to-r from-gray-950 via-gray-900 to-indigo-950/40 p-4 rounded border border-indigo-900/50">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-sm font-bold text-gray-200 uppercase tracking-wider">Native DSD 原生直通直传</h3>
            <span v-if="status.native_dsd_mode" class="px-2 py-0.5 bg-indigo-600 text-white text-[10px] font-bold rounded animate-pulse">
              ACTIVE
            </span>
            <span v-else class="px-2 py-0.5 bg-gray-800 text-gray-400 text-[10px] rounded">
              PCM MODE
            </span>
          </div>
          <p class="text-xs text-gray-400 mt-1">
            独立通道无交织缓冲 &bull; 纯净 1-bit 比特流 &bull; 无任何 DoP Marker &bull; 支持 DSD256 (11.2896MHz) 与 DSD512 (22.5792MHz)
          </p>
        </div>

        <div class="flex items-center gap-3">
          <select
            v-model="selectedDSDFormat"
            class="bg-gray-900 border border-indigo-800 rounded px-3 py-1.5 text-xs text-gray-200 font-mono"
            :disabled="status.native_dsd_mode"
          >
            <option value="DSD256">DSD256 (11.2896 MHz)</option>
            <option value="DSD512">DSD512 (22.5792 MHz 驱动可选)</option>
          </select>

          <button
            v-if="!status.native_dsd_mode"
            @click="triggerNativeDSDActivation"
            class="px-4 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold rounded shadow-[0_0_12px_rgba(99,102,241,0.5)] transition"
          >
            启用 Native DSD 直通
          </button>
          <button
            v-else
            @click="disableNativeDSD"
            class="px-4 py-1.5 bg-gray-700 hover:bg-gray-600 text-gray-200 text-xs font-bold rounded transition"
          >
            恢复 PCM 模式
          </button>
        </div>
      </div>
    </div>

    <!-- Mandatory Native DSD Safety Warning Modal (T5.5 Unskippable Confirmation) -->
    <div
      v-if="showSafetyWarning"
      class="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4 backdrop-blur-sm"
    >
      <div class="bg-[#12151e] border-2 border-red-600 rounded-lg max-w-lg w-full p-6 shadow-[0_0_40px_rgba(220,38,38,0.4)]">
        <div class="flex items-center gap-3 text-red-500 mb-4">
          <span class="text-2xl">⚠️</span>
          <h4 class="text-base font-black uppercase tracking-wide">不可跳过的 Native DSD 高危操作安全确认</h4>
        </div>

        <div class="text-xs text-gray-300 leading-relaxed mb-5 bg-red-950/40 p-3.5 rounded border border-red-800/60 font-mono">
          <p class="mb-2 text-red-300 font-bold">
            【高风险警示】Native DSD 模式将直接向物理 DAC 传输 1-bit 高频脉冲：
          </p>
          <ul class="list-disc pl-5 space-y-1.5 text-gray-300">
            <li>若后端功放或扬声器处于高增益状态，可能导致设备损坏或听力永久损伤；</li>
            <li>在开启前，必须将后级放大器音量旋钮调至最低或断开耳机；</li>
            <li>系统已在 Controller 核心层实施硬件守卫：绕过界面直接调用 RPC 也必须完成校验。</li>
          </ul>
        </div>

        <div class="mb-5">
          <label class="block text-xs font-bold text-gray-300 mb-1.5">
            请在下方输入 <span class="text-red-400 font-mono">YES</span> 以解除核心安全拦截锁：
          </label>
          <input
            v-model="confirmationInput"
            type="text"
            placeholder="在此输入 YES"
            class="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-sm text-gray-100 font-mono focus:border-red-500 focus:outline-none uppercase"
          />
          <p v-if="confirmationError" class="text-xs text-red-400 mt-1.5 font-mono">
            {{ confirmationError }}
          </p>
        </div>

        <div class="flex justify-end gap-3">
          <button
            @click="cancelNativeDSD"
            class="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded text-xs font-semibold"
          >
            取消操作
          </button>
          <button
            @click="confirmNativeDSD"
            class="px-5 py-2 bg-red-600 hover:bg-red-500 text-white rounded text-xs font-bold shadow-[0_0_15px_rgba(220,38,38,0.6)]"
          >
            确认并解锁 Native DSD
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api'
import type { AsioStatus } from '../types'

const status = ref<AsioStatus>({
  active_driver: 'ASIO Combo384 Driver',
  driver_version: '0x0103',
  sample_type: 'ASIOSTDSDInt8LSB1',
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
  latency_report: {
    buffer_size: 512,
    sample_rate: 48000,
    reported_input_latency: 512,
    reported_output_latency: 560,
    reported_total_latency: 1072,
    measured_round_trip_latency: 1074.007,
    converter_offset: 2.007,
    jitter_standard_deviation: 0.0872,
    repetitions: 100
  },
  native_dsd_mode: false,
  native_dsd_confirmed: false,
  selected_dsd_format: 'DSD256'
})

const selectedDriver = ref('ASIO Combo384 Driver')
const selectedClock = ref(0)
const selectedBufferSize = ref(512)
const selectedDSDFormat = ref('DSD256')

const showSafetyWarning = ref(false)
const confirmationInput = ref('')
const confirmationError = ref('')

async function refreshStatus() {
  try {
    const s = await api.getAsioStatus()
    status.value = s
    selectedDriver.value = s.active_driver || selectedDriver.value
    selectedClock.value = s.active_clock_index
    selectedBufferSize.value = s.active_buffer_size
  } catch (e) {
    console.error('Failed to get ASIO status:', e)
  }
}

onMounted(() => {
  refreshStatus()
})

async function handleDriverChange() {
  try {
    await api.selectAsioDriver(selectedDriver.value)
    await refreshStatus()
  } catch (e) {
    console.error('Failed to switch ASIO driver:', e)
  }
}

async function handleClockChange() {
  try {
    await api.setAsioClockSource(selectedClock.value)
    await refreshStatus()
  } catch (e) {
    console.error('Failed to switch clock source:', e)
  }
}

async function handleBufferSizeChange() {
  try {
    await api.setAsioBufferSize(selectedBufferSize.value)
    await refreshStatus()
  } catch (e) {
    console.error('Failed to switch buffer size:', e)
  }
}

function triggerNativeDSDActivation() {
  if (status.value.native_dsd_confirmed) {
    enableNativeDSD()
  } else {
    confirmationInput.value = ''
    confirmationError.value = ''
    showSafetyWarning.value = true
  }
}

async function confirmNativeDSD() {
  if (confirmationInput.value.trim().toUpperCase() !== 'YES') {
    confirmationError.value = '必须准确输入 YES 方可确认'
    return
  }

  try {
    await api.confirmNativeDSDRisk('YES')
    showSafetyWarning.value = false
    await enableNativeDSD()
  } catch (e: any) {
    confirmationError.value = e.message || '确认失败'
  }
}

function cancelNativeDSD() {
  showSafetyWarning.value = false
}

async function enableNativeDSD() {
  try {
    await api.configureNativeDSD(selectedDSDFormat.value)
    await refreshStatus()
  } catch (e) {
    console.error('Failed to configure Native DSD:', e)
  }
}

async function disableNativeDSD() {
  try {
    await api.disableNativeDSD()
    await refreshStatus()
  } catch (e) {
    console.error('Failed to disable Native DSD:', e)
  }
}
</script>
