import { api, call } from './client'
import type { PushSubscriptionRequest } from './types'

export const pushApi = {
  // VAPID public key; empty string means push is disabled server-side.
  publicKey: (): Promise<string> => call(api.GET('/push/vapid-public-key')).then((d) => d.key),

  subscribe: (body: PushSubscriptionRequest): Promise<unknown> =>
    call(api.POST('/push/subscriptions', { body })),

  unsubscribe: (endpoint: string): Promise<unknown> =>
    call(api.DELETE('/push/subscriptions', { params: { query: { endpoint } } })),
}
