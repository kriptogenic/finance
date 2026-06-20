// PWA app-icon badge (Badging API). No-op where unsupported (e.g. iOS Safari,
// or when the app isn't installed). Errors are swallowed — the badge is cosmetic.

interface BadgingNavigator {
  setAppBadge?: (count?: number) => Promise<void>
  clearAppBadge?: () => Promise<void>
}

export function setAppBadge(count: number) {
  const nav = navigator as BadgingNavigator
  if (!nav.setAppBadge) return
  if (count > 0) nav.setAppBadge(count).catch(() => {})
  else nav.clearAppBadge?.().catch(() => {})
}
