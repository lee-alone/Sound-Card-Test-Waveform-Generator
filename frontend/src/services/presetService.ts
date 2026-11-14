/**
 * Preset & Configuration Persistence Service
 *
 * Adheres to low-coupling: handles only test configuration saving,
 * loading from JSON files, and safe preset application.
 */

import { isWailsRuntime, callBackend } from './ipc'
import { mockPresetStore, setMockPlaying } from './mockData'
import type { ExtendedSignalRequest, SignalPreset } from '../types'

export const presetService = {
  async savePreset(
    filePath: string,
    name: string,
    desc: string,
    req: ExtendedSignalRequest
  ): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SavePreset', filePath, name, desc, req)
    }
    mockPresetStore[filePath] = {
      schema_version: '1.0',
      name,
      description: desc,
      created_at: new Date().toISOString(),
      request: req
    }
    return Promise.resolve()
  },

  async loadPreset(filePath: string): Promise<SignalPreset> {
    if (isWailsRuntime()) {
      return callBackend<SignalPreset>('LoadPreset', filePath)
    }
    if (mockPresetStore[filePath]) {
      return Promise.resolve(mockPresetStore[filePath])
    }
    throw new Error('Preset file not found: ' + filePath)
  },

  async applyPreset(preset: SignalPreset): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('ApplyPreset', preset)
    }
    setMockPlaying(false) // Safe invariant: must remain OpenMuted
    return Promise.resolve()
  }
}
