<template>
  <div class="inst-panel p-4 mb-4">
    <!-- Unplugged Alert Banner (Triggers in <500ms) -->
    <div
      v-if="deviceWarning"
      class="mb-3 p-3 bg-red-950/80 border border-red-500 rounded flex items-center justify-between text-red-200"
    >
      <div class="flex items-center gap-2">
        <span class="text-xl">⚠️</span>
        <div>
          <div class="font-bold">设备连接警告 (Device Hardware Alert)</div>
          <div class="text-sm">{{ deviceWarning }}</div>
        </div>
      </div>
      <button class="btn-secondary text-xs" @click="refreshDevices">重新检测</button>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-4">
      <!-- Device Dropdown Selection -->
      <div class="flex-1 min-w-[280px]">
        <label class="block text-xs uppercase tracking-wider text-gray-400 mb-1 font-semibold">
          输出音频设备 (Audio Output Device)
        </label>
        <div class="flex gap-2">
          <select
            v-model="selectedId"
            class="inst-input flex-1 font-mono-num text-sm"
            @change="onDeviceChange"
          >
            <option v-for="d in devices" :key="d.id" :value="d.id">
              {{ d.name }} {{ d.is_default ? '★ [系统默认]' : '' }} {{ d.state && d.state !== 'Active' ? ' (' + d.state + ')' : '' }}
            </option>
          </select>
          <button
            class="btn-secondary text-xs px-3"
            title="探测候选格式支持能力"
            :disabled="probing"
            @click="probeCapabilities"
          >
            {{ probing ? '探测中...' : '能力探测' }}
          </button>
        </div>
      </div>

      <!-- Exclusive Mode Switch -->
      <div class="flex items-center gap-3 bg-black/30 p-2 rounded border border-gray-800">
        <label class="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            v-model="isExclusive"
            class="sr-only peer"
            @change="onExclusiveChange"
          />
          <div
            class="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"
          ></div>
        </label>
        <div>
          <div class="text-sm font-semibold text-gray-200">WASAPI 独占排他模式</div>
          <div class="text-[11px] text-gray-400">AUDCLNT_SHAREMODE_EXCLUSIVE (防系统音效干扰)</div>
        </div>
      </div>

      <!-- Hardware Negotiated Readout Panel -->
      <div class="flex items-center gap-4 bg-gray-900/90 px-4 py-2 rounded border border-gray-800 font-mono-num text-xs">
        <div>
          <div class="text-gray-500 uppercase text-[10px]">协商采样率</div>
          <div class="text-cyan-400 font-bold text-sm">{{ diagnostics.sample_rate ? diagnostics.sample_rate.toLocaleString() + ' Hz' : '--' }}</div>
        </div>
        <div class="border-l border-gray-700 pl-4">
          <div class="text-gray-500 uppercase text-[10px]">位深 / 容器格式</div>
          <div class="text-cyan-400 font-bold text-sm">{{ diagnostics.sample_format || '--' }}</div>
        </div>
        <div class="border-l border-gray-700 pl-4">
          <div class="text-gray-500 uppercase text-[10px]">缓冲区周期</div>
          <div class="text-cyan-400 font-bold text-sm">
            {{ diagnostics.buffer_period_ms ? diagnostics.buffer_period_ms.toFixed(1) + ' ms' : '--' }}
            <span class="text-gray-400 font-normal">({{ diagnostics.buffer_frames }} frames)</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Capabilities Probe Result Modal -->
    <div
      v-if="showProbeResults"
      class="mt-4 p-3 bg-gray-950 border border-gray-800 rounded font-mono-num text-xs"
    >
      <div class="flex justify-between items-center mb-2">
        <span class="font-bold text-gray-300">硬件能力矩阵探测结果 (Candidate Matrix Probe)</span>
        <button class="text-gray-400 hover:text-white" @click="showProbeResults = false">✕ 关闭</button>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
        <div
          v-for="(r, idx) in probeResults"
          :key="idx"
          class="p-2 rounded flex justify-between items-center text-xs"
          :class="r.status === 'Supported' ? 'bg-emerald-950/40 border border-emerald-800 text-emerald-300' : 'bg-red-950/20 border border-red-900 text-red-400'"
        >
          <span>{{ r.format.sample_rate / 1000 }}kHz / {{ formatName(r.format.sample_format) }}</span>
          <span class="font-bold uppercase text-[10px]">{{ r.status }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '../api'
import type { DeviceInfo, FormatSupportResult, DiagnosticsReport } from '../types'

const props = defineProps<{
  diagnostics: DiagnosticsReport
}>()

const emit = defineEmits<{
  (e: 'device-selected', deviceId: string): void
  (e: 'exclusive-toggled', exclusive: boolean): void
}>()

const devices = ref<DeviceInfo[]>([])
const selectedId = ref<string>('')
const isExclusive = ref<boolean>(true)
const deviceWarning = ref<string>('')
const probing = ref<boolean>(false)
const showProbeResults = ref<boolean>(false)
const probeResults = ref<FormatSupportResult[]>([])

let hotplugTimer: number | null = null

async function refreshDevices() {
  try {
    const list = await api.getDevices()
    devices.value = list
    if (!selectedId.value && list.length > 0) {
      const def =
        list.find((d) => d.is_default && d.state === 'Active') ||
        list.find((d) => d.is_default) ||
        list.find((d) => d.state === 'Active') ||
        list[0]
      selectedId.value = def.id
      emit('device-selected', def.id)
    }
  } catch (err: any) {
    console.error('Failed to enumerate devices:', err)
  }
}

async function onDeviceChange() {
  deviceWarning.value = ''
  try {
    await api.selectDevice(selectedId.value)
    emit('device-selected', selectedId.value)
  } catch (err: any) {
    deviceWarning.value = '选择设备失败: ' + (err.message || err)
  }
}

async function onExclusiveChange() {
  await api.setExclusiveMode(isExclusive.value)
  emit('exclusive-toggled', isExclusive.value)
}

async function probeCapabilities() {
  probing.value = true
  try {
    const res = await api.probeDeviceCapabilities()
    probeResults.value = res
    showProbeResults.value = true
  } catch (err: any) {
    deviceWarning.value = '格式探测失败: ' + (err.message || err)
  } finally {
    probing.value = false
  }
}

function formatName(fmt: number): string {
  switch (fmt) {
    case 0: return '16-bit Int'
    case 1: return '24-bit Packed'
    case 2: return '24-in-32 LE'
    case 3: return '32-bit Float'
    default: return 'Unknown'
  }
}

// 250ms hotplug poller (meets <500ms warning requirement)
async function checkHotplug() {
  try {
    const event = await api.pollDeviceChange()
    if (event) {
      if (event.type === 'unplugged') {
        deviceWarning.value = event.active_warning || '当前音频输出设备已被拔出！输出已安全静音。'
      } else if (event.type === 'plugged') {
        deviceWarning.value = ''
        await refreshDevices()
      }
    }
  } catch (_) {
    // Ignore transient IPC errors during background polling
  }
}

onMounted(async () => {
  await refreshDevices()
  isExclusive.value = await api.isExclusiveMode()
  hotplugTimer = window.setInterval(checkHotplug, 250)
})

onUnmounted(() => {
  if (hotplugTimer !== null) {
    clearInterval(hotplugTimer)
  }
})
</script>
