/**
 * Wails v3 IPC Transport Client
 *
 * Implements low-level RPC over the Wails v3 /wails/runtime POST bridge.
 * Adheres to low-coupling: handles only transport serialization and network delivery.
 */

declare global {
  interface Window {
    _wails?: any
  }
}

let clientId =
  typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : 'client-' + Math.random().toString(36).substring(2, 10)

/**
 * Checks if the code is executing within the Wails WebView2 runtime.
 */
export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  // Inside Wails WebView2 on Windows, host is wails.localhost and window._wails is injected
  return !!window._wails || window.location.hostname === 'wails.localhost'
}

/**
 * Calls a bound Go service method on Wails v3 by method name.
 *
 * @param methodName Name of method on UIService, e.g. "GetDevices"
 * @param args Arguments to pass to the Go method
 */
export async function callBackend<T = any>(methodName: string, ...args: any[]): Promise<T> {
  const callId = 'call-' + Math.random().toString(36).substring(2, 9) + '-' + Date.now()
  const fqn = `sound-signal-generator/internal/controller.UIService.${methodName}`

  const payload = {
    object: 0, // callRequest
    method: 0, // callBinding
    args: {
      'call-id': callId,
      methodName: fqn,
      args: args
    }
  }

  const response = await fetch('/wails/runtime', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-wails-client-id': clientId
    },
    body: JSON.stringify(payload)
  })

  if (!response.ok) {
    const errorText = await response.text()
    let errorMsg = errorText
    try {
      const errJson = JSON.parse(errorText)
      if (errJson.message) {
        errorMsg = errJson.message
      }
    } catch (_) {
      // Keep errorText as is
    }
    throw new Error(errorMsg)
  }

  const contentType = response.headers.get('Content-Type') || ''
  if (contentType.includes('application/json')) {
    return (await response.json()) as T
  }
  return (await response.text()) as any
}
