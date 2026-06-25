import { api, call } from './client'
import type { AppConfig } from './types'

// The app config rarely changes, so fetch it once and share the promise across
// callers. A failed fetch is not cached, so it can be retried.
let cached: Promise<AppConfig> | null = null

export const configApi = {
  get: (): Promise<AppConfig> => {
    if (!cached) {
      cached = call(api.GET('/config')).catch((e) => {
        cached = null
        throw e
      })
    }
    return cached
  },
  invalidate: () => {
    cached = null
  },
}

// Drop the memoized config on an explicit data refresh so it is re-read next time.
if (typeof window !== 'undefined') {
  window.addEventListener('data:refresh', () => configApi.invalidate())
}
