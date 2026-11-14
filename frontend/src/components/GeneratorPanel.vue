<template>
  <div class="inst-panel p-5 mb-4">
    <div class="flex items-center justify-between mb-4 border-b border-gray-800 pb-3">
      <div class="flex items-center gap-3">
        <h2 class="text-base font-bold text-gray-200 tracking-wide uppercase">
          全功能信号源发生器 (Waveform Signal Generator)
        </h2>
        <span
          class="px-2 py-0.5 rounded text-[11px] font-mono-num font-semibold uppercase"
          :class="isRunning ? 'bg-emerald-950 text-emerald-400 border border-emerald-700' : 'bg-gray-800 text-gray-400 border border-gray-700'"
        >
          {{ isRunning ? '● 正在推流 (ACTIVE)' : '○ 已静音 (MUTED)' }}
        </span>
      </div>

      <!-- Quick Preset Frequencies -->
      <div class="flex gap-1.5" v-if="supportsFrequency">
        <button
          v-for="f in [100, 1000, 10000, 20000]"
          :key="f"
          class="px-2 py-1 bg-gray-900 border border-gray-700 hover:border-gray-500 rounded text-[11px] font-mono-num text-gray-300"
          @click="setQuickFreq(f)"
        >
          {{ f >= 1000 ? f / 1000 + ' kHz' : f + ' Hz' }}
        </button>
      </div>
    </div>

    <!-- Waveform Selector Tabs -->
    <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-9 gap-1.5 mb-5">
      <button
        v-for="w in waveforms"
        :key="w.id"
        class="py-2 px-2 text-xs font-semibold rounded border transition-all flex flex-col items-center justify-center gap-1"
        :class="
          request.waveform === w.id
            ? 'bg-blue-900/60 border-blue-500 text-blue-200 shadow-[0_0_8px_rgba(59,130,246,0.3)]'
            : 'bg-gray-900/80 border-gray-800 text-gray-400 hover:bg-gray-850 hover:text-gray-200'
        "
        @click="selectWaveform(w.id)"
      >
        <span class="text-base">{{ w.icon }}</span>
        <span class="truncate text-[11px]">{{ w.label }}</span>
      </button>
    </div>

    <!-- Generator Dynamic Parameters -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 mb-5">
      <!-- Left Column: Frequency & Basic Parameters -->
      <div class="lg:col-span-6 inst-subpanel p-4">
        <!-- Frequency Input with Nyquist Safety Validation -->
        <div v-if="supportsFrequency" class="mb-4">
          <div class="flex justify-between items-center mb-1">
            <label class="text-xs font-bold text-gray-300 uppercase tracking-wider">
              信号频率 (Frequency)
            </label>
            <span class="text-[11px] font-mono-num text-gray-400">
              奈奎斯特限制: &le; {{ maxSafeFreq.toLocaleString() }} Hz (0.45&times;Fs)
            </span>
          </div>

          <div class="flex items-center gap-2">
            <input
              type="number"
              v-model.number="request.frequency"
              step="1"
              min="1"
              :class="['inst-input font-mono-num text-lg font-bold flex-1', isNyquistExceeded ? 'input-error' : '']"
              @input="onParamChange"
            />
            <span class="font-mono-num text-sm text-gray-400 px-2 font-bold">Hz</span>
          </div>

          <!-- Red Error Box for Nyquist Exceeded -->
          <div v-if="isNyquistExceeded" class="mt-2 text-xs text-red-400 font-semibold flex items-center gap-1">
            <span>❌</span>
            <span>频率超限：请求频率 {{ request.frequency }} Hz 已超过安全奈奎斯特上限 ({{ maxSafeFreq }} Hz)！已自动禁用启动。</span>
          </div>
        </div>

        <!-- Waveform Specific: Sweep Options -->
        <div v-if="request.waveform === 'linear_sweep' || request.waveform === 'log_sweep'" class="space-y-3">
          <div class="text-xs font-bold text-blue-400 uppercase">扫频参数 (Chirp Sweep Settings)</div>
          <div class="grid grid-cols-2 gap-3 font-mono-num text-xs">
            <div>
              <label class="text-gray-400 block mb-1">起始频率 (Start Freq)</label>
              <input
                type="number"
                v-model.number="request.sweep!.start_freq"
                class="inst-input w-full"
                @input="onParamChange"
              />
            </div>
            <div>
              <label class="text-gray-400 block mb-1">终止频率 (End Freq)</label>
              <input
                type="number"
                v-model.number="request.sweep!.end_freq"
                :class="['inst-input w-full', request.sweep!.end_freq > maxSafeFreq ? 'input-error' : '']"
                @input="onParamChange"
              />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-3 font-mono-num text-xs">
            <div>
              <label class="text-gray-400 block mb-1">扫频时长 (Duration s)</label>
              <input
                type="number"
                v-model.number="request.sweep!.duration_sec"
                step="0.5"
                min="0.5"
                class="inst-input w-full"
                @input="onParamChange"
              />
            </div>
            <div class="flex items-center gap-2 pt-4">
              <input
                type="checkbox"
                id="sweepLoop"
                v-model="request.sweep!.loop"
                class="rounded bg-gray-900 border-gray-700 text-blue-500"
                @change="onParamChange"
              />
              <label for="sweepLoop" class="text-xs text-gray-300 cursor-pointer">循环持续扫频 (Loop)</label>
            </div>
          </div>
        </div>

        <!-- Waveform Specific: Noise Seed -->
        <div v-if="request.waveform === 'white_noise' || request.waveform === 'pink_noise'" class="space-y-3">
          <div class="text-xs font-bold text-blue-400 uppercase">噪声参数 (Noise PRNG Seed)</div>
          <div>
            <label class="text-gray-400 block text-xs mb-1">PRNG 随机种子 (可复现实验验证)</label>
            <input
              type="number"
              v-model.number="request.noise!.seed"
              class="inst-input w-full font-mono-num text-sm"
              @input="onParamChange"
            />
          </div>
        </div>

        <!-- Waveform Specific: DoP Mode -->
        <div v-if="request.waveform === 'dop'" class="space-y-3">
          <div class="text-xs font-bold text-amber-400 uppercase">DoP DSD 载波封装 (dCS DoP v1.1)</div>
          <div class="grid grid-cols-2 gap-3 text-xs">
            <div>
              <label class="text-gray-400 block mb-1">DSD 规格 (Bitstream Rate)</label>
              <select
                v-model="request.dop!.dsd_format"
                class="inst-input w-full font-mono-num text-xs"
                @change="onDoPFormatChange"
              >
                <option value="DSD64">DSD64 (176.4 kHz PCM 载波)</option>
                <option value="DSD128">DSD128 (352.8 kHz PCM 载波)</option>
              </select>
            </div>
            <div>
              <label class="text-gray-400 block mb-1">端序规范 (Bit Order)</label>
              <select v-model.number="request.dop!.bit_order" class="inst-input w-full font-mono-num text-xs">
                <option :value="0">LSB First (标准 dCS v1.1)</option>
                <option :value="1">MSB First</option>
              </select>
            </div>
          </div>
          <div class="p-2 bg-amber-950/40 border border-amber-800/80 rounded text-[11px] text-amber-300">
            ★ DoP 模式强制保持 0 dBFS Unity Gain 且禁用淡化，以保障黄金测试向量位完美解析。
          </div>
        </div>

        <!-- Waveform Specific: Multitone Presets -->
        <div v-if="request.waveform.startsWith('multitone')" class="space-y-3">
          <div class="text-xs font-bold text-blue-400 uppercase">多音互调参数 (Multitone Configuration)</div>
          <div v-if="request.waveform === 'multitone_smpte'" class="text-xs text-gray-300 font-mono-num bg-black/30 p-2 rounded">
            标准 SMPTE 互调信号：60 Hz + 7,000 Hz，振幅比 4:1 (12.04 dB 功率差)，自动 PAPR 峰值保护。
          </div>
          <div v-if="request.waveform === 'multitone_ccif'" class="text-xs text-gray-300 font-mono-num bg-black/30 p-2 rounded">
            标准 CCIF 差频互调信号：19,000 Hz + 20,000 Hz，振幅比 1:1，自动 PAPR 峰值保护。
          </div>
        </div>
      </div>

      <!-- Right Column: Dual-Channel Independent dBFS Volume Controls -->
      <div class="lg:col-span-6 inst-subpanel p-4 flex flex-col justify-between">
        <div>
          <div class="flex items-center justify-between mb-3 border-b border-gray-800 pb-2">
            <span class="text-xs font-bold text-gray-300 uppercase tracking-wider">
              双通道独立电平 (Dual-Channel dBFS Attenuation)
            </span>
            <div class="flex items-center gap-2">
              <input
                type="checkbox"
                id="linkCh"
                v-model="linkChannels"
                class="rounded bg-gray-900 border-gray-700 text-cyan-500"
              />
              <label for="linkCh" class="text-xs text-gray-400 cursor-pointer">声道联动 (Link L/R)</label>
            </div>
          </div>

          <!-- Channel 1 (Left) -->
          <div class="mb-4">
            <div class="flex justify-between items-center mb-1 font-mono-num text-xs">
              <span class="text-cyan-400 font-bold">CH 1 [左声道 / Left]</span>
              <span class="text-cyan-300 font-bold">{{ request.left_volume_dbfs.toFixed(1) }} dBFS</span>
            </div>
            <input
              type="range"
              min="-144"
              max="0"
              step="0.5"
              v-model.number="request.left_volume_dbfs"
              class="inst-slider"
              :disabled="request.waveform === 'dop'"
              @input="onLeftVolChange"
            />
          </div>

          <!-- Channel 2 (Right) -->
          <div class="mb-4">
            <div class="flex justify-between items-center mb-1 font-mono-num text-xs">
              <span class="text-cyan-400 font-bold">CH 2 [右声道 / Right]</span>
              <span class="text-cyan-300 font-bold">{{ request.right_volume_dbfs.toFixed(1) }} dBFS</span>
            </div>
            <input
              type="range"
              min="-144"
              max="0"
              step="0.5"
              v-model.number="request.right_volume_dbfs"
              class="inst-slider"
              :disabled="request.waveform === 'dop'"
              @input="onRightVolChange"
            />
          </div>

          <!-- Linear Amplitude Master -->
          <div class="p-2.5 bg-black/40 rounded border border-gray-800 flex justify-between items-center font-mono-num text-xs">
            <span class="text-gray-400">总输出峰值预测 (Max Linear Peak):</span>
            <span class="text-emerald-400 font-bold">{{ Math.pow(10, Math.max(request.left_volume_dbfs, request.right_volume_dbfs) / 20).toFixed(4) }} (0 dBFS 最大 1.0000)</span>
          </div>
        </div>

        <!-- Master Playback Action Bar -->
        <div class="pt-4 mt-3 border-t border-gray-800 flex items-center justify-between gap-3">
          <div class="text-xs text-gray-400">
            状态: <span class="font-bold text-gray-200">{{ diagnostics.state }}</span>
          </div>

          <div class="flex gap-2">
            <button
              class="btn-primary flex items-center gap-2"
              :disabled="isNyquistExceeded || isRunning"
              @click="handleStart"
            >
              <span>▶</span>
              <span>开始推流 (START)</span>
            </button>
            <button
              class="btn-stop flex items-center gap-2"
              :disabled="!isRunning"
              @click="handleStop"
            >
              <span>■</span>
              <span>平滑停止 (STOP)</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { ExtendedSignalRequest, WaveformType, DiagnosticsReport } from '../types'

const props = defineProps<{
  request: ExtendedSignalRequest
  diagnostics: DiagnosticsReport
}>()

const emit = defineEmits<{
  (e: 'update:request', req: ExtendedSignalRequest): void
  (e: 'start-requested'): void
  (e: 'stop-requested'): void
  (e: 'param-changed'): void
}>()

const linkChannels = ref<boolean>(true)

const waveforms: { id: WaveformType; label: string; icon: string }[] = [
  { id: 'sine', label: '纯正弦波', icon: '∿' },
  { id: 'square', label: 'PolyBLEP方波', icon: '⊓' },
  { id: 'triangle', label: 'PolyBLEP三角波', icon: '▲' },
  { id: 'linear_sweep', label: '线性扫频', icon: '↗' },
  { id: 'log_sweep', label: '对数扫频', icon: '⤴' },
  { id: 'white_noise', label: '高斯白噪', icon: '░' },
  { id: 'pink_noise', label: 'Voss粉噪', icon: '▒' },
  { id: 'multitone_smpte', label: 'SMPTE双音', icon: '〰' },
  { id: 'multitone_ccif', label: 'CCIF双音', icon: '≈' },
  { id: 'dop', label: 'DoP (DSD)', icon: '◈' }
]

const isRunning = computed(() => {
  const s = String(props.diagnostics.state || '').toLowerCase()
  return s === 'running' || s === 'fadingout' || s === '4' || s === '5'
})

const supportsFrequency = computed(() => {
  return props.request.waveform === 'sine' ||
    props.request.waveform === 'square' ||
    props.request.waveform === 'triangle'
})

const maxSafeFreq = computed(() => {
  const rate = props.diagnostics.sample_rate || props.request.sample_rate || 48000
  return rate * 0.45
})

const isNyquistExceeded = computed(() => {
  if (supportsFrequency.value) {
    return props.request.frequency > maxSafeFreq.value
  }
  if (props.request.waveform === 'linear_sweep' || props.request.waveform === 'log_sweep') {
    return props.request.sweep?.end_freq ? props.request.sweep.end_freq > maxSafeFreq.value : false
  }
  return false
})

function selectWaveform(type: WaveformType) {
  props.request.waveform = type
  if (type === 'dop') {
    props.request.sample_rate = 176400
    props.request.left_volume_dbfs = 0.0
    props.request.right_volume_dbfs = 0.0
  }
  onParamChange()
}

function setQuickFreq(freq: number) {
  if (freq <= maxSafeFreq.value) {
    props.request.frequency = freq
    onParamChange()
  }
}

function onLeftVolChange() {
  if (linkChannels.value) {
    props.request.right_volume_dbfs = props.request.left_volume_dbfs
  }
  onParamChange()
}

function onRightVolChange() {
  if (linkChannels.value) {
    props.request.left_volume_dbfs = props.request.right_volume_dbfs
  }
  onParamChange()
}

function onDoPFormatChange() {
  if (props.request.dop?.dsd_format === 'DSD128') {
    props.request.sample_rate = 352800
  } else {
    props.request.sample_rate = 176400
  }
  onParamChange()
}

function onParamChange() {
  emit('param-changed')
}

function handleStart() {
  if (!isNyquistExceeded.value) {
    emit('start-requested')
  }
}

function handleStop() {
  emit('stop-requested')
}
</script>
