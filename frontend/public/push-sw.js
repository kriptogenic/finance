// Imported by the Workbox-generated service worker (see vite.config workbox.importScripts).
// On a push, mirror the uncategorized count onto the PWA app-icon badge. Badge-only:
// we intentionally show no notification (Chromium may surface a generic one if its
// silent-push budget is exhausted).

self.addEventListener('push', (event) => {
  event.waitUntil(updateBadge(event))
})

async function updateBadge(event) {
  let count = 0
  try {
    count = event.data ? (event.data.json().count ?? 0) : 0
  } catch {
    count = 0
  }

  if (!self.navigator.setAppBadge) return
  if (count > 0) await self.navigator.setAppBadge(count)
  else if (self.navigator.clearAppBadge) await self.navigator.clearAppBadge()
}
