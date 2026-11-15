/**
 * Main Frontend API Façade
 *
 * Adheres strictly to the low-coupling principle:
 * Aggregates focused domain services (Device, Playback, Safety, Preset, ASIO)
 * without concentrating excessive functionality in a single module.
 */

import { isWailsRuntime } from './services/ipc'
import { deviceService } from './services/deviceService'
import { playbackService } from './services/playbackService'
import { safetyService } from './services/safetyService'
import { presetService } from './services/presetService'
import { asioService } from './services/asioService'

export { isWailsRuntime }

export const api = {
  // Device & Capabilities Management
  getDevices: deviceService.getDevices,
  selectDevice: deviceService.selectDevice,
  probeDeviceCapabilities: deviceService.probeDeviceCapabilities,
  setExclusiveMode: deviceService.setExclusiveMode,
  isExclusiveMode: deviceService.isExclusiveMode,
  pollDeviceChange: deviceService.pollDeviceChange,

  // Playback & Signal Configuration
  configureSignal: playbackService.configureSignal,
  startPlayback: playbackService.startPlayback,
  stopPlayback: playbackService.stopPlayback,
  getDiagnostics: playbackService.getDiagnostics,

  // Safety & Hazard Assessment
  checkSafety: safetyService.checkSafety,
  confirmSafety: safetyService.confirmSafety,

  // Preset Storage & Application
  savePreset: presetService.savePreset,
  loadPreset: presetService.loadPreset,
  applyPreset: presetService.applyPreset,

  // ASIO & Native DSD Subsystem
  getAsioStatus: asioService.getAsioStatus,
  selectAsioDriver: asioService.selectAsioDriver,
  setAsioClockSource: asioService.setAsioClockSource,
  setAsioBufferSize: asioService.setAsioBufferSize,
  configureNativeDSD: asioService.configureNativeDSD,
  confirmNativeDSDRisk: asioService.confirmNativeDSDRisk,
  cancelNativeDSDEntry: asioService.cancelNativeDSDEntry,
  disableNativeDSD: asioService.disableNativeDSD
}
