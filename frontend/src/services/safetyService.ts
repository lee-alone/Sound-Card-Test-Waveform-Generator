/**
 * Hazard Safety Assessment Service
 *
 * Adheres to low-coupling: handles only high-risk safety evaluation
 * and user explicit confirmations (e.g. ultrasound, DoP, high crest factor).
 */

import { isWailsRuntime, callBackend } from './ipc'
import { mockConfirmedRisks } from './mockData'
import type { ExtendedSignalRequest, SafetyAssessment } from '../types'

export const safetyService = {
  async checkSafety(req: ExtendedSignalRequest): Promise<SafetyAssessment> {
    if (isWailsRuntime()) {
      return callBackend<SafetyAssessment>('CheckSafety', req)
    }

    if (req.waveform === 'dop') {
      return {
        has_risk: true,
        category: 'dop_carrier',
        title: '高危操作警示：DoP DSD 载波直通',
        warning_message:
          '若连接的 DAC 不支持 DoP，将直接输出全幅强噪声损伤听力与扬声器！请降低功放增益或断开耳机。',
        requires_confirmation: true,
        is_confirmed: !!mockConfirmedRisks['dop_carrier']
      }
    }
    if (req.frequency > 20000) {
      return {
        has_risk: true,
        category: 'ultrasound',
        title: '安全警示：超声频段测试信号 (>20kHz)',
        warning_message: '高频超声持续输出可能导致功放振荡或高音单元音圈过热烧毁。',
        requires_confirmation: true,
        is_confirmed: !!mockConfirmedRisks['ultrasound']
      }
    }
    if (req.waveform === 'square') {
      return {
        has_risk: true,
        category: 'square_wave',
        title: '安全提示：带限方波信号',
        warning_message: '方波富含高能量谐波，大音量输出可能对高音扬声器产生热应力。',
        requires_confirmation: true,
        is_confirmed: !!mockConfirmedRisks['square_wave']
      }
    }
    if (req.waveform.startsWith('multitone')) {
      return {
        has_risk: true,
        category: 'multitone_crest',
        title: '提示：多音互调与波峰因数',
        warning_message: '多音频号具有较高的峰值因数 (PAPR)，请防止瞬态过冲。',
        requires_confirmation: true,
        is_confirmed: !!mockConfirmedRisks['multitone_crest']
      }
    }
    return {
      has_risk: false,
      category: 'none',
      title: '',
      warning_message: '',
      requires_confirmation: false,
      is_confirmed: true
    }
  },

  async confirmSafety(category: string, confirmed: boolean): Promise<void> {
    if (isWailsRuntime()) {
      return callBackend<void>('ConfirmSafety', category, confirmed)
    }
    mockConfirmedRisks[category] = confirmed
    return Promise.resolve()
  }
}
