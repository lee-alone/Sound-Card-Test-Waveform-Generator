/**
 * Audio Playback & Signal Configuration Service
 *
 * Adheres to low-coupling: handles only signal configuration,
 * playback lifecycle (start/stop), and 10Hz diagnostics retrieval.
 */

import { isWailsRuntime, callBackend } from './ipc'
import {
  mockDevices,
  mockSelectedDevice,
  setMockPlaying,
  createMockDiagnostics
} from './mockData'
import type { ExtendedSignalRequest, DiagnosticsReport } from '../types'

export const playbackService = {
  async configureSignal(req: ExtendedSignalRequest): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('ConfigureSignal', req)
    }
    return Promise.resolve()
  },

  async startPlayback(): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('StartPlayback')
    }
    setMockPlaying(true)
    return Promise.resolve()
  },

  async stopPlayback(): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('StopPlayback')
    }
    setMockPlaying(false)
    return Promise.resolve()
  },

  async getDiagnostics(): Promise<DiagnosticsReport> {
    if (isWailsRuntime()) {
      return callBackend<DiagnosticsReport>('GetDiagnostics')
    }
    const dev = mockDevices.find((d) => d.id === mockSelectedDevice) || mockDevices[0]
    return Promise.resolve(createMockDiagnostics(dev.name))
  }
}
