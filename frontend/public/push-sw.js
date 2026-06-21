// Imported by the Workbox-generated service worker (see vite.config workbox.importScripts).
// On a push, update the PWA app-icon badge and show a notification. The notification
// is required: we subscribe with userVisibleOnly:true, so every push must surface one
// (silent pushes are dropped on iOS and exhaust Chromium's budget).

self.addEventListener('push', (event) => {
  event.waitUntil(handlePush(event))
})

async function handlePush(event) {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = {}
  }
  const count = data.count ?? 0

  if (self.navigator.setAppBadge) {
    if (count > 0) await self.navigator.setAppBadge(count)
    else if (self.navigator.clearAppBadge) await self.navigator.clearAppBadge()
  }

  if (count > 0) {
    // Title: "<merchant> · <amount>", falling back to whichever is present.
    const parts = [data.merchant, data.amount].filter(Boolean)
    const title = parts.length ? parts.join(' · ') : 'New transaction'
    await self.registration.showNotification(title, {
      body: count + ' to categorize',
      tag: 'uncategorized', // replaces the previous badge notification
      renotify: true,
      data: { url: '/' },
    })
  }
}

// Tapping the notification focuses the app (or opens it).
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = event.notification.data?.url || '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ('focus' in client) return client.focus()
      }
      return self.clients.openWindow(url)
    }),
  )
})
