<template>
  <div
    v-if="visible"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4"
  >
    <div
      class="inst-panel max-w-lg w-full p-6 border-2 border-red-500 shadow-[0_0_30px_rgba(239,68,68,0.4)] animate-in fade-in zoom-in duration-200"
    >
      <!-- Hazard Header -->
      <div class="flex items-center gap-3 mb-4 text-red-400 border-b border-red-900/60 pb-3">
        <span class="text-3xl">⚠️</span>
        <div>
          <h2 class="text-lg font-bold text-red-200 uppercase tracking-wide">
            {{ assessment.title || '高危操作强制安全确认' }}
          </h2>
          <div class="text-xs text-red-400 font-mono-num uppercase">
            风险类别: {{ assessment.category }}
          </div>
        </div>
      </div>

      <!-- Warning Message Body -->
      <div class="p-4 bg-red-950/40 rounded border border-red-900/60 text-sm text-gray-200 leading-relaxed mb-5">
        {{ assessment.warning_message }}
      </div>

      <!-- Safety Checklist Confirmation Checkbox -->
      <div class="p-3 bg-black/50 border border-gray-800 rounded mb-5 flex items-start gap-3">
        <input
          type="checkbox"
          id="confirmRiskCheck"
          v-model="userAcknowledged"
          class="mt-1 w-4 h-4 rounded bg-gray-900 border-red-500 text-red-600 focus:ring-red-500 cursor-pointer"
        />
        <label for="confirmRiskCheck" class="text-xs text-gray-300 cursor-pointer select-none">
          <span class="font-bold text-red-300">我已明确理解以上风险：</span>
          已确认断开入耳式耳机/贵重扬声器，或已将后级放大器音量旋钮调整至安全静音电平。
        </label>
      </div>

      <!-- Action Buttons -->
      <div class="flex justify-end gap-3">
        <button
          class="btn-secondary text-xs px-4 py-2 font-semibold"
          @click="handleCancel"
        >
          取消并保持安全静音 (Cancel & Mute)
        </button>
        <button
          class="btn-stop text-xs px-5 py-2 font-bold uppercase flex items-center gap-1.5"
          :disabled="!userAcknowledged"
          @click="handleConfirm"
        >
          <span>🔓</span>
          <span>确认解锁并输出 (Unlock Output)</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { SafetyAssessment } from '../types'

const props = defineProps<{
  visible: boolean
  assessment: SafetyAssessment
}>()

const emit = defineEmits<{
  (e: 'confirm', category: string): void
  (e: 'cancel'): void
}>()

const userAcknowledged = ref<boolean>(false)

watch(
  () => props.visible,
  (val) => {
    if (val) {
      userAcknowledged.value = false
    }
  }
)

function handleConfirm() {
  if (userAcknowledged.value) {
    emit('confirm', props.assessment.category)
  }
}

function handleCancel() {
  userAcknowledged.value = false
  emit('cancel')
}
</script>
