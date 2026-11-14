/**
 * ASIO & Native DSD Driver Subsystem Service
 *
 * Adheres to low-coupling: handles only ASIO driver selection,
 * hardware clock source switches, buffer sizing, latency reports,
 * and Native DSD guard configuration.
 */

import { isWailsRuntime, callBackend } from './ipc'
import { defaultMockAsioStatus } from './mockData'
import type { AsioStatus } from '../types'

export const asioService = {
  async getAsioStatus(): Promise<AsioStatus> {
    if (isWailsRuntime()) {
      return callBackend<AsioStatus>('GetAsioStatus')
    }
    return Promise.resolve({ ...defaultMockAsioStatus })
  },

  async selectAsioDriver(id: string): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SelectAsioDriver', id)
    }
    return Promise.resolve()
  },

  async setAsioClockSource(index: number): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SetAsioClockSource', index)
    }
    return Promise.resolve()
  },

  async setAsioBufferSize(frames: number): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('SetAsioBufferSize', frames)
    }
    return Promise.resolve()
  },

  async configureNativeDSD(format: string): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('ConfigureNativeDSD', format)
    }
    return Promise.resolve()
  },

  async confirmNativeDSDRisk(confirmation: string): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('ConfirmNativeDSDRisk', confirmation)
    }
    return Promise.resolve()
  },

  async disableNativeDSD(): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('DisableNativeDSD')
    }
    return Promise.resolve()
  }
}
