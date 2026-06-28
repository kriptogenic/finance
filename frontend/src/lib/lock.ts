import { ref } from 'vue'

// App-lock: an optional 4-digit PIN gate on top of the API credentials. The PIN
// is hashed (SHA-256) before being stored in localStorage — we never persist the
// raw digits. Re-locks on launch and when the PWA returns from the background.

const PIN_KEY = 'lock.pinHash'
const RELOCK_AFTER_MS = 30_000

export const pinSet = ref(localStorage.getItem(PIN_KEY) !== null)
export const locked = ref(pinSet.value)

async function hash(pin: string): Promise<string> {
  const data = new TextEncoder().encode(pin)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('')
}

export async function setPin(pin: string): Promise<void> {
  localStorage.setItem(PIN_KEY, await hash(pin))
  pinSet.value = true
  locked.value = false
}

export async function verifyPin(pin: string): Promise<boolean> {
  const stored = localStorage.getItem(PIN_KEY)
  return stored !== null && stored === (await hash(pin))
}

export function clearPin(): void {
  localStorage.removeItem(PIN_KEY)
  pinSet.value = false
  locked.value = false
}

export function unlock(): void {
  locked.value = false
}

// Re-lock when the app has been backgrounded long enough. Quick app-switches
// (under the grace window) don't force a re-entry.
let hiddenAt = 0
function onVisibility() {
  if (document.visibilityState === 'hidden') {
    hiddenAt = Date.now()
  } else if (pinSet.value && hiddenAt && Date.now() - hiddenAt > RELOCK_AFTER_MS) {
    locked.value = true
  }
}

export function installLock(): void {
  document.addEventListener('visibilitychange', onVisibility)
}
