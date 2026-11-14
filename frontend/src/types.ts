export type WaveformType =
  | 'sine'
  | 'square'
  | 'triangle'
  | 'linear_sweep'
  | 'log_sweep'
  | 'white_noise'
  | 'pink_noise'
  | 'multitone_smpte'
  | 'multitone_ccif'
  | 'multitone_custom'
  | 'dop'

export type BitPerfectTier = 'Configured' | 'Eligible' | 'Verified' | 'Unknown'

export interface DeviceInfo {
  id: string
  name: string
  is_default: boolean
  state: string
}

export interface FormatSupportResult {
  format: {
    sample_rate: number
    channels: number
    sample_format: number
  }
  status: string
  hr_result?: number
  error_message?: string
}

export interface SweepParams {
  start_freq: number
  end_freq: number
  duration_sec: number
  taper_fraction: number
  loop: boolean
}

export interface NoiseParams {
  seed: number
}

export interface MultitoneParams {
  frequencies: number[]
  amplitudes: number[]
  phases?: number[]
}

export interface DoPParams {
  dsd_format: string
  bit_order: number
}

export interface ExtendedSignalRequest {
  device_id: string
  sample_rate: number
  sample_format: number
  channels: number
  waveform: WaveformType
  frequency: number
  amplitude: number
  dc_offset: number
  left_volume_dbfs: number
  right_volume_dbfs: number
  sweep?: SweepParams
  noise?: NoiseParams
  multitone?: MultitoneParams
  dop?: DoPParams
}

export interface DiagnosticsReport {
  state: string
  device_name: string
  exclusive: boolean
  sample_rate: number
  sample_format: string
  channels: number
  buffer_period_ms: number
  buffer_frames: number
  bit_perfect_status: BitPerfectTier
  render_callback_count: number
  underrun_count: number
  late_callback_count: number
  last_render_time_us: number
  max_render_time_us: number
  p999_render_time_us: number
  render_time_percent_of_buffer: number
  max_percent_of_buffer: number
  p999_percent_of_buffer: number
  timestamp: string
}

export interface SafetyAssessment {
  has_risk: boolean
  category: string
  title: string
  warning_message: string
  requires_confirmation: boolean
  is_confirmed: boolean
}

export interface SignalPreset {
  schema_version: string
  name: string
  description?: string
  created_at: string
  request: ExtendedSignalRequest
}

export interface DeviceChangeEvent {
  type: 'plugged' | 'unplugged' | 'default_changed'
  device_id: string
  device_name: string
  timestamp: string
  active_warning?: string
}

export interface ClockSource {
  index: number
  name: string
  is_current_source: boolean
}

export interface BufferSizeInfo {
  min_size: number
  max_size: number
  preferred_size: number
  granularity: number
}

export interface LatencyReport {
  buffer_size: number
  sample_rate: number
  reported_input_latency: number
  reported_output_latency: number
  reported_total_latency: number
  measured_round_trip_latency: number
  converter_offset: number
  jitter_standard_deviation: number
  repetitions: number
}

export interface AsioStatus {
  active_driver: string
  driver_version: string
  sample_type: string
  clock_sources: ClockSource[]
  active_clock_index: number
  buffer_sizes: BufferSizeInfo
  active_buffer_size: number
  latency_report?: LatencyReport
  native_dsd_mode: boolean
  native_dsd_confirmed: boolean
  selected_dsd_format: string
}

