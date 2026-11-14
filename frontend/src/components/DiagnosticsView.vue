<template>
  <div class="inst-panel p-4 mb-4 font-mono-num text-xs">
    <div class="flex items-center justify-between mb-3 border-b border-gray-800 pb-2">
      <div class="flex items-center gap-2">
        <span class="text-sm">📊</span>
        <h3 class="font-bold text-gray-200 tracking-wider uppercase">
          实时性能诊断与位完美监控 (10Hz Real-Time Performance & Diagnostics)
        </h3>
      </div>
      <div class="text-[11px] text-gray-400">
        刷新频率: 10 Hz | 轮询同步中
      </div>
    </div>

    <!-- Top Row: State Machine & Bit-Perfect Badges -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
      <!-- 1. Engine State Machine -->
      <div class="inst-subpanel p-3">
        <div class="text-gray-500 text-[10px] uppercase font-bold mb-1">引擎状态机 (Engine State)</div>
        <div class="flex items-center gap-2">
          <span class="w-3 h-3 rounded-full animate-pulse" :class="stateColorClass"></span>
          <span class="text-sm font-bold text-gray-100 uppercase">{{ diagnostics.state || 'Closed' }}</span>
        </div>
        <div class="text-[10px] text-gray-400 mt-1">
          生命周期: {{ stateDescription }}
        </div>
      </div>

      <!-- 2. Bit-Perfect Transmission Status -->
      <div class="inst-subpanel p-3">
        <div class="text-gray-500 text-[10px] uppercase font-bold mb-1">位完美纯度 (Bit-Perfect Status)</div>
        <div class="flex items-center gap-2">
          <span
            class="px-2 py-0.5 rounded text-xs font-bold uppercase tracking-wider"
            :class="bitPerfectClass"
          >
            {{ diagnostics.bit_perfect_status || 'Unknown' }}
          </span>
        </div>
        <div class="text-[10px] text-gray-400 mt-1">
          {{ bitPerfectDescription }}
        </div>
      </div>

      <!-- 3. Underruns Counter -->
      <div class="inst-subpanel p-3">
        <div class="text-gray-500 text-[10px] uppercase font-bold mb-1">缓冲欠载 (Buffer Underruns)</div>
        <div class="flex items-baseline gap-2">
          <span
            class="text-xl font-bold"
            :class="diagnostics.underrun_count > 0 ? 'text-red-400' : 'text-emerald-400'"
          >
            {{ diagnostics.underrun_count || 0 }}
          </span>
          <span class="text-[10px] text-gray-400">次 (Target: 0)</span>
        </div>
        <div class="text-[10px] text-gray-500 mt-1">
          推流回调中断/缺包统计
        </div>
      </div>

      <!-- 4. Late Callbacks Counter -->
      <div class="inst-subpanel p-3">
        <div class="text-gray-500 text-[10px] uppercase font-bold mb-1">唤醒迟到 (Late Wakeups)</div>
        <div class="flex items-baseline gap-2">
          <span
            class="text-xl font-bold"
            :class="diagnostics.late_callback_count > 0 ? 'text-amber-400' : 'text-emerald-400'"
          >
            {{ diagnostics.late_callback_count || 0 }}
          </span>
          <span class="text-[10px] text-gray-400">次 (MMCSS 调度)</span>
        </div>
        <div class="text-[10px] text-gray-500 mt-1">
          系统线程调度延迟超出事件
        </div>
      </div>
    </div>

    <!-- Bottom Row: Real-time Render Timing & P99.9 Benchmark Meters -->
    <div class="inst-subpanel p-3">
      <div class="flex flex-wrap items-center justify-between mb-2">
        <div class="font-bold text-gray-300">
          渲染线程执行延迟基准 (Render Hot-Path Latency vs 10ms Buffer Period)
        </div>
        <div class="text-[11px] text-gray-400">
          已执行回调: <span class="text-cyan-400 font-bold">{{ diagnostics.render_callback_count ? diagnostics.render_callback_count.toLocaleString() : 0 }}</span>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-3">
        <!-- Last Render -->
        <div class="bg-black/40 p-2.5 rounded border border-gray-800">
          <div class="text-gray-400 text-[11px]">最新单次耗时 (Last Render)</div>
          <div class="text-base font-bold text-cyan-300">
            {{ diagnostics.last_render_time_us ? diagnostics.last_render_time_us.toFixed(1) : '0.0' }} &mu;s
          </div>
          <div class="text-[11px] text-gray-400">
            占周期: <span class="font-semibold text-gray-300">{{ diagnostics.render_time_percent_of_buffer ? diagnostics.render_time_percent_of_buffer.toFixed(2) : '0.00' }}%</span>
          </div>
        </div>

        <!-- P99.9 Percentile Render -->
        <div class="bg-black/40 p-2.5 rounded border border-gray-800">
          <div class="text-gray-400 text-[11px]">P99.9 分位数耗时 (P99.9 Latency)</div>
          <div class="text-base font-bold text-cyan-400">
            {{ diagnostics.p999_render_time_us ? diagnostics.p999_render_time_us.toFixed(1) : '0.0' }} &mu;s
          </div>
          <div class="text-[11px]" :class="diagnostics.p999_percent_of_buffer < 30 ? 'text-emerald-400' : 'text-red-400'">
            占周期: <span class="font-bold">{{ diagnostics.p999_percent_of_buffer ? diagnostics.p999_percent_of_buffer.toFixed(2) : '0.00' }}%</span> (门禁上限: &lt; 30%)
          </div>
        </div>

        <!-- Max Render -->
        <div class="bg-black/40 p-2.5 rounded border border-gray-800">
          <div class="text-gray-400 text-[11px]">历史单次最大耗时 (Max Single Render)</div>
          <div class="text-base font-bold text-cyan-400">
            {{ diagnostics.max_render_time_us ? diagnostics.max_render_time_us.toFixed(1) : '0.0' }} &mu;s
          </div>
          <div class="text-[11px]" :class="diagnostics.max_percent_of_buffer < 70 ? 'text-emerald-400' : 'text-red-400'">
            占周期: <span class="font-bold">{{ diagnostics.max_percent_of_buffer ? diagnostics.max_percent_of_buffer.toFixed(2) : '0.00' }}%</span> (门禁上限: &lt; 70%)
          </div>
        </div>
      </div>

      <!-- Visual Load Progress Meter -->
      <div>
        <div class="flex justify-between text-[10px] text-gray-400 mb-1">
          <span>0%</span>
          <span class="text-emerald-500 font-bold">P99.9 门禁线 (30%)</span>
          <span class="text-amber-500 font-bold">最大门禁线 (70%)</span>
          <span>100% (Underrun 溢出线)</span>
        </div>
        <div class="w-full bg-gray-900 h-3 rounded-full overflow-hidden relative border border-gray-800">
          <!-- 30% Marker -->
          <div class="absolute left-[30%] top-0 bottom-0 w-0.5 bg-emerald-500/80 z-10"></div>
          <!-- 70% Marker -->
          <div class="absolute left-[70%] top-0 bottom-0 w-0.5 bg-amber-500/80 z-10"></div>
          <!-- Current Fill -->
          <div
            class="h-full transition-all duration-150"
            :class="loadMeterColorClass"
            :style="{ width: Math.min(100, Math.max(1, diagnostics.p999_percent_of_buffer || 2)) + '%' }"
          ></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DiagnosticsReport } from '../types'

const props = defineProps<{
  diagnostics: DiagnosticsReport
}>()

const stateColorClass = computed(() => {
  switch (props.diagnostics.state) {
    case 'Running': return 'bg-emerald-500 shadow-[0_0_8px_#10b981]'
    case 'OpenMuted': return 'bg-blue-500 shadow-[0_0_8px_#3b82f6]'
    case 'Primed': return 'bg-cyan-500 shadow-[0_0_8px_#06b6d4]'
    case 'FadingOut': return 'bg-amber-500 shadow-[0_0_8px_#f59e0b]'
    case 'Faulted': return 'bg-red-500 shadow-[0_0_8px_#ef4444]'
    default: return 'bg-gray-500'
  }
})

const stateDescription = computed(() => {
  switch (props.diagnostics.state) {
    case 'Running': return '稳定推流中，淡入已完成'
    case 'OpenMuted': return '设备已打开，处于静音安全位'
    case 'Primed': return '缓冲区已预填，等待推流命令'
    case 'FadingOut': return '正在平滑淡出，准备进入静音'
    case 'Faulted': return '检测到驱动或硬件异常，紧急静音'
    default: return '流未打开'
  }
})

const bitPerfectClass = computed(() => {
  switch (props.diagnostics.bit_perfect_status) {
    case 'Verified': return 'bg-emerald-950 text-emerald-300 border border-emerald-600'
    case 'Eligible': return 'bg-cyan-950 text-cyan-300 border border-cyan-600'
    case 'Configured': return 'bg-amber-950 text-amber-300 border border-amber-600'
    default: return 'bg-gray-800 text-gray-400 border border-gray-700'
  }
})

const bitPerfectDescription = computed(() => {
  switch (props.diagnostics.bit_perfect_status) {
    case 'Verified': return '★ 已通过外部可信数字捕获硬件逐样本比对归档'
    case 'Eligible': return '独占运行中，硬件格式严格匹配且 0dBFS 零修改'
    case 'Configured': return '独占参数已配置，等待播放进入稳定状态'
    default: return '非独占或格式/音量非透传，无法判定位完美'
  }
})

const loadMeterColorClass = computed(() => {
  const p = props.diagnostics.p999_percent_of_buffer || 0
  if (p < 30) return 'bg-cyan-500'
  if (p < 70) return 'bg-amber-500'
  return 'bg-red-500'
})
</script>
