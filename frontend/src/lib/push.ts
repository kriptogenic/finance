import { ref } from 'vue'
import { pushApi } from '../api/push'

// Web Push subscription management for the PWA badge. Enabling subscribes this
// browser; the service worker (public/push-sw.js) updates the app-icon badge
// when a push arrives while the app is closed.

export const pushSupported =
  'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window

// Reflects whether this browser currently holds an active push subscription.
export const pushEnabled = ref(false)
export const pushBusy = ref(false)

// VAPID keys are base64url; the subscribe() call wants a Uint8Array.
function urlBase64ToUint8Array(base64: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64.length % 4)) % 4)
  const raw = atob((base64 + padding).replace(/-/g, '+').replace(/_/g, '/'))
  const out = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

// Sync pushEnabled with the browser's actual subscription state.
export async function refreshPushState(): Promise<void> {
  if (!pushSupported) return
  const reg = await navigator.serviceWorker.getRegistration()
  const sub = await reg?.pushManager.getSubscription()
  pushEnabled.value = !!sub
}

export async function enablePush(): Promise<void> {
  if (!pushSupported) throw new Error('Notifications are not supported on this device')
  pushBusy.value = true
  try {
    const key = await pushApi.publicKey()
    if (!key) throw new Error('Push is not configured on the server')

    const permission = await Notification.requestPermission()
    if (permission !== 'granted') throw new Error('Notification permission was denied')

    const reg = await navigator.serviceWorker.ready
    const sub = await reg.pushManager.subscribe({
      userVisibleOnly: true, // required by Chromium even for badge-only pushes
      applicationServerKey: urlBase64ToUint8Array(key),
    })

    const json = sub.toJSON()
    await pushApi.subscribe({
      endpoint: json.endpoint!,
      keys: { p256dh: json.keys!.p256dh, auth: json.keys!.auth },
    })
    pushEnabled.value = true
  } finally {
    pushBusy.value = false
  }
}

export async function disablePush(): Promise<void> {
  pushBusy.value = true
  try {
    const reg = await navigator.serviceWorker.getRegistration()
    const sub = await reg?.pushManager.getSubscription()
    if (sub) {
      await pushApi.unsubscribe(sub.endpoint).catch(() => {}) // best-effort server cleanup
      await sub.unsubscribe()
    }
    pushEnabled.value = false
  } finally {
    pushBusy.value = false
  }
}
