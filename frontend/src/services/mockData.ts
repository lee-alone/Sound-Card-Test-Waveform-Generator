/**
 * Mock Data Store for Unit Testing / Fallback
 *
 * Adheres to low-coupling: isolates test doubles from real production RPC logic.
 */

import type {
  DeviceInfo,
  FormatSupportResult,
  DiagnosticsReport,
  SignalPreset,
  AsioStatus
} from '../types'

export const mockDevices: DeviceInfo[] = [
  { id: 'mock-1', name: 'Realtek High Definition Audio (Default)', is_default: true, state: 'Active' },
  { id: 'mock-2', name: 'Amanero Combo384 USB DAC (DSD/DoP Capable)', is_default: false, state: 'Active' },
  { id: 'mock-3', name: 'Universal Audio Apollo x8 (ASIO/Exclusive)', is_default: false, state: 'Active' }
]

export const mockFormatSupportResults: FormatSupportResult[] = [
  { format: { sample_rate: 44100, channels: 2, sample_format: 0 }, status: 'Supported' },
  { format: { sample_rate: 48000, channels: 2, sample_format: 0 }, status: 'Supported' },
  { format: { sample_rate: 96000, channels: 2, sample_format: 2 }, status: 'Supported' },
  { format: { sample_rate: 192000, channels: 2, sample_format: 2 }, status: 'Supported' },
  { format: { sample_rate: 176400, channels: 2, sample_format: 2 }, status: 'Supported' },
  { format: { sample_rate: 352800, channels: 2, sample_format: 2 }, status: 'Supported' }
]

export let mockSelectedDevice = 'mock-1'
export let mockExclusive = true
export let mockPlaying = false
export const mockConfirmedRisks: Record<string, boolean> = {}
export const mockPresetStore: Record<string, SignalPreset> = {}

export function setMockSelectedDevice(id: string) {
  mockSelectedDevice = id
}

export function setMockExclusive(exclusive: boolean) {
  mockExclusive = exclusive
}

export function setMockPlaying(playing: boolean) {
  mockPlaying = playing
}

export function createMockDiagnostics(deviceName: string): DiagnosticsReport {
  return {
    state: mockPlaying ? 'Running' : 'OpenMuted',
    device_name: deviceName,
    exclusive: mockExclusive,
    sample_rate: 96000,
    sample_format: '24-in-32 LE',
    channels: 2,
    buffer_period_ms: 10.0,
    buffer_frames: 960,
    bit_perfect_status: mockPlaying ? 'Eligible' : 'Configured',
    render_callback_count: mockPlaying ? 1250 : 0,
    underrun_count: 0,
    late_callback_count: 0,
    last_render_time_us: 145.2,
    max_render_time_us: 480.0,
    p999_render_time_us: 280.5,
    render_time_percent_of_buffer: 1.45,
    max_percent_of_buffer: 4.8,
    p999_percent_of_buffer: 2.8,
    timestamp: new Date().toISOString()
  }
}

export const defaultMockAsioStatus: AsioStatus = {
  active_driver: 'ASIO Combo384 Driver',
  driver_version: '0x0103',
  sample_type: 'ASIOSTInt32LSB',
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
  operating_mode: 'PCM',
  native_dsd_mode: false,
  native_dsd_active: false,
  native_dsd_confirmed: false,
  selected_dsd_format: ''
}
