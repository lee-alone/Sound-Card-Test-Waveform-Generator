<template>
  <div class="inst-panel p-4 mb-4">
    <div class="flex items-center justify-between mb-3 border-b border-gray-800 pb-2">
      <div class="flex items-center gap-2">
        <span class="text-sm">💾</span>
        <h3 class="text-xs font-bold text-gray-200 tracking-wider uppercase">
          测试配置与预设管理 (Test Presets & Configuration Persistence)
        </h3>
      </div>
      <div class="text-[11px] text-emerald-400 font-mono-num flex items-center gap-1">
        <span>🛡️</span>
        <span>安全准则: 载入预设后严格锁定于 OpenMuted 静音态</span>
      </div>
    </div>

    <!-- Factory Presets Selector & Custom File Operations -->
    <div class="grid grid-cols-1 md:grid-cols-12 gap-3 items-center">
      <!-- Quick Factory Presets -->
      <div class="md:col-span-8 flex flex-wrap gap-2">
        <button
          v-for="fp in factoryPresets"
          :key="fp.name"
          class="btn-secondary text-[11px] font-mono-num py-1.5 px-2.5 flex items-center gap-1"
          @click="applyFactoryPreset(fp)"
        >
          <span>✦</span>
          <span>{{ fp.name }}</span>
        </button>
      </div>

      <!-- Custom File Save/Load Actions -->
      <div class="md:col-span-4 flex justify-end gap-2">
        <button
          class="btn-secondary text-xs flex items-center gap-1"
          @click="showSaveModal = true"
        >
          <span>💾 保存当前配置</span>
        </button>
        <button
          class="btn-secondary text-xs flex items-center gap-1"
          @click="showLoadModal = true"
        >
          <span>📂 载入预设文件</span>
        </button>
      </div>
    </div>

    <!-- Feedback Notice Banner -->
    <div
      v-if="noticeMessage"
      class="mt-3 p-2.5 bg-blue-950/60 border border-blue-600 rounded text-xs text-blue-200 flex justify-between items-center"
    >
      <span>{{ noticeMessage }}</span>
      <button class="text-gray-400 hover:text-white" @click="noticeMessage = ''">✕</button>
    </div>

    <!-- Save Modal Dialog -->
    <div
      v-if="showSaveModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
    >
      <div class="inst-panel max-w-md w-full p-5 border border-blue-500 shadow-xl">
        <h3 class="font-bold text-gray-200 text-sm mb-3">保存预设配置 (Save Preset to JSON)</h3>
        <div class="space-y-3 text-xs font-mono-num mb-4">
          <div>
            <label class="block text-gray-400 mb-1">预设名称 (Preset Name)</label>
            <input type="text" v-model="saveName" class="inst-input w-full" placeholder="e.g. THD Measurement 1kHz" />
          </div>
          <div>
            <label class="block text-gray-400 mb-1">测试说明 (Description)</label>
            <input type="text" v-model="saveDesc" class="inst-input w-full" placeholder="e.g. Standard 0 dBFS sine test" />
          </div>
          <div>
            <label class="block text-gray-400 mb-1">保存文件路径 (File Path)</label>
            <input type="text" v-model="savePath" class="inst-input w-full" placeholder="presets/custom_preset.json" />
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <button class="btn-secondary text-xs" @click="showSaveModal = false">取消</button>
          <button class="btn-primary text-xs" @click="handleSavePreset">确认保存</button>
        </div>
      </div>
    </div>

    <!-- Load Modal Dialog -->
    <div
      v-if="showLoadModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
    >
      <div class="inst-panel max-w-md w-full p-5 border border-blue-500 shadow-xl">
        <h3 class="font-bold text-gray-200 text-sm mb-3">载入预设文件 (Load Preset from JSON)</h3>
        <div class="space-y-3 text-xs font-mono-num mb-4">
          <div>
            <label class="block text-gray-400 mb-1">预设文件路径 (File Path)</label>
            <input type="text" v-model="loadPath" class="inst-input w-full" placeholder="presets/custom_preset.json" />
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <button class="btn-secondary text-xs" @click="showLoadModal = false">取消</button>
          <button class="btn-primary text-xs" @click="handleLoadPreset">载入预设 (Muted)</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api'
import type { ExtendedSignalRequest, SignalPreset } from '../types'

const props = defineProps<{
  currentRequest: ExtendedSignalRequest
}>()

const emit = defineEmits<{
  (e: 'preset-applied', req: ExtendedSignalRequest): void
}>()

const showSaveModal = ref<boolean>(false)
const showLoadModal = ref<boolean>(false)
const saveName = ref<string>('Standard 1kHz Test')
const saveDesc = ref<string>('0 dBFS Sine Reference')
const savePath = ref<string>('presets/standard_1k.json')
const loadPath = ref<string>('presets/standard_1k.json')
const noticeMessage = ref<string>('')

interface FactoryPresetItem {
  name: string
  req: Partial<ExtendedSignalRequest>
}

const factoryPresets: FactoryPresetItem[] = [
  {
    name: '1kHz 0dBFS 位完美',
    req: { waveform: 'sine', frequency: 1000.0, left_volume_dbfs: 0.0, right_volume_dbfs: 0.0 }
  },
  {
    name: '20Hz-20k 对数扫频',
    req: {
      waveform: 'log_sweep',
      sweep: { start_freq: 20.0, end_freq: 20000.0, duration_sec: 5.0, taper_fraction: 0.1, loop: true }
    }
  },
  {
    name: 'SMPTE 互调分析 (4:1)',
    req: { waveform: 'multitone_smpte', left_volume_dbfs: -3.0, right_volume_dbfs: -3.0 }
  },
  {
    name: 'CCIF 差频互调 (1:1)',
    req: { waveform: 'multitone_ccif', left_volume_dbfs: -3.0, right_volume_dbfs: -3.0 }
  },
  {
    name: '带限方波 10kHz',
    req: { waveform: 'square', frequency: 10000.0, left_volume_dbfs: -6.0, right_volume_dbfs: -6.0 }
  },
  {
    name: 'DoP DSD64 黄金向量',
    req: { waveform: 'dop', sample_rate: 176400, left_volume_dbfs: 0.0, right_volume_dbfs: 0.0 }
  }
]

async function applyFactoryPreset(fp: FactoryPresetItem) {
  const merged: ExtendedSignalRequest = {
    ...props.currentRequest,
    ...fp.req
  }
  noticeMessage.value = `已载入预设「${fp.name}」。安全机制生效：输出已强制设定为 OpenMuted 静音态。`
  emit('preset-applied', merged)
}

async function handleSavePreset() {
  try {
    await api.savePreset(savePath.value, saveName.value, saveDesc.value, props.currentRequest)
    noticeMessage.value = `预设配置已成功保存至: ${savePath.value}`
    showSaveModal.value = false
  } catch (err: any) {
    noticeMessage.value = `保存失败: ${err.message || err}`
  }
}

async function handleLoadPreset() {
  try {
    const preset = await api.loadPreset(loadPath.value)
    await api.applyPreset(preset)
    noticeMessage.value = `已成功载入预设「${preset.name}」。安全机制生效：输出已安全置于 OpenMuted 静音态。`
    emit('preset-applied', preset.request)
    showLoadModal.value = false
  } catch (err: any) {
    noticeMessage.value = `载入失败: ${err.message || err}`
  }
}
</script>
