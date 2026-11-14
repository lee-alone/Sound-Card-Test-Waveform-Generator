/**
 * Audio Device & Capabilities Service
 *
 * Adheres to low-coupling: handles only audio endpoint enumeration,
 * device selection, exclusive mode toggling, and hotplug polling.
 */

import { isWailsRuntime, callBackend } from './ipc'
import {
  mockDevices,
  mockFormatSupportResults,
  mockExclusive,
  setMockSelectedDevice,
  setMockExclusive
} from './mockData'
import type { DeviceInfo, FormatSupportResult, DeviceChangeEvent } from '../types'

export const deviceService = {
  async getDevices(): Promise<DeviceInfo[]> {
    if (isWailsRuntime()) {
      return callBackend<DeviceInfo[]>('GetDevices')
    }
    return Promise.resolve([...mockDevices])
  },

  async selectDevice(id: string): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SelectDevice', id)
    }
    setMockSelectedDevice(id)
    return Promise.resolve()
  },

  async probeDeviceCapabilities(): Promise<FormatSupportResult[]> {
    if (isWailsRuntime()) {
      return callBackend<FormatSupportResult[]>('ProbeDeviceCapabilities')
    }
    return Promise.resolve([...mockFormatSupportResults])
  },

  async setExclusiveMode(exclusive: boolean): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SetExclusiveMode', exclusive)
    }
    setMockExclusive(exclusive)
    return Promise.resolve()
  },

  async isExclusiveMode(): Promise<boolean> {
    if (isWailsRuntime()) {
      return callBackend<boolean>('IsExclusiveMode')
    }
    return Promise.resolve(mockExclusive)
  },

  async pollDeviceChange(): Promise<DeviceChangeEvent | null> {
    if (isWailsRuntime()) {
      return callBackend<DeviceChangeEvent | null>('PollDeviceChange')
    }
    return Promise.resolve(null)
  }
}
