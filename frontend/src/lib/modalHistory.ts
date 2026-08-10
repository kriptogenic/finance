import { onMounted, onUnmounted } from 'vue'

// Closes the topmost modal on browser/device "back" instead of navigating away.
//
// Every open Modal registers a close callback. While any modal is open we keep a
// single history "sentinel" entry; pressing back pops it and closes the top
// modal, re-guarding if more remain. Closing a modal by other means (X, backdrop,
// save) pops the sentinel so the history stack stays clean. Cleanup is deferred a
// microtask so a modal that immediately opens another (a swap) doesn't pop+repush.

type Closer = () => void

const stack: Closer[] = []
let hasSentinel = false
let sentinelHref = ''

function ensureSentinel() {
  if (stack.length > 0 && !hasSentinel) {
    // Reuse the current state and URL so vue-router stays in sync — popping this
    // entry changes nothing about the location, so it triggers no navigation.
    history.pushState(history.state, '')
    hasSentinel = true
    sentinelHref = location.href
  }
}

function onPopState() {
  if (!hasSentinel) return // not our sentinel — let the router/app handle it
  hasSentinel = false
  stack[stack.length - 1]?.()
  // Closing the top may reveal another modal underneath; guard it too.
  queueMicrotask(ensureSentinel)
}

function scheduleCleanup() {
  queueMicrotask(() => {
    if (stack.length === 0 && hasSentinel) {
      hasSentinel = false
      // A modal that navigated on close (e.g. "View transactions") pushed its own
      // entry above the sentinel; popping now would undo that navigation.
      if (location.href === sentinelHref) history.back()
    }
  })
}

export function useModalBackClose(close: Closer) {
  onMounted(() => {
    if (stack.length === 0) window.addEventListener('popstate', onPopState)
    stack.push(close)
    ensureSentinel()
  })

  onUnmounted(() => {
    const i = stack.lastIndexOf(close)
    if (i >= 0) stack.splice(i, 1)
    if (stack.length === 0) {
      window.removeEventListener('popstate', onPopState)
      scheduleCleanup()
    }
  })
}
