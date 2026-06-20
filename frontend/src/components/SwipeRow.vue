<script setup lang="ts">
// Touch swipe wrapper for list rows. Swipe right reveals "Edit", swipe left
// reveals "Delete"; releasing past the threshold fires the matching event. A
// drag suppresses the tap that would otherwise follow. Mouse devices never fire
// touch events, so desktop keeps its hover buttons untouched.
import { ref } from 'vue'

const props = withDefaults(defineProps<{ threshold?: number; max?: number }>(), {
  threshold: 72,
  max: 88,
})
const emit = defineEmits<{ swipeLeft: []; swipeRight: [] }>()

const offset = ref(0)
const dragging = ref(false)

let startX = 0
let startY = 0
let active = false
let didDrag = false

function onStart(e: TouchEvent) {
  if (e.touches.length !== 1) return
  startX = e.touches[0].clientX
  startY = e.touches[0].clientY
  active = true
  didDrag = false
  dragging.value = false
}

function onMove(e: TouchEvent) {
  if (!active) return
  const dx = e.touches[0].clientX - startX
  const dy = e.touches[0].clientY - startY
  if (!dragging.value) {
    if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return
    if (Math.abs(dy) >= Math.abs(dx)) {
      active = false // vertical intent — let the list scroll
      return
    }
    dragging.value = true
    didDrag = true
  }
  offset.value = Math.max(-props.max, Math.min(props.max, dx))
  e.preventDefault() // hold the page still while swiping horizontally
}

function onEnd() {
  if (!active) return
  active = false
  if (offset.value <= -props.threshold) emit('swipeLeft')
  else if (offset.value >= props.threshold) emit('swipeRight')
  offset.value = 0
  dragging.value = false
}

// A swipe ends in a synthetic click; swallow it so the row doesn't also open.
function onClickCapture(e: MouseEvent) {
  if (didDrag) {
    e.stopPropagation()
    e.preventDefault()
    didDrag = false
  }
}
</script>

<template>
  <div class="relative overflow-hidden" @click.capture="onClickCapture">
    <!-- action layer (revealed as the foreground slides) -->
    <div class="pointer-events-none absolute inset-0 flex items-stretch">
      <div class="flex flex-1 items-center gap-1.5 bg-amber-400 px-5 text-sm font-semibold text-slate-900 transition-opacity" :style="{ opacity: offset > 0 ? 1 : 0 }">
        <i class="ti ti-pencil text-base" /> Edit
      </div>
      <div class="flex flex-1 items-center justify-end gap-1.5 bg-rose-500 px-5 text-sm font-semibold text-white transition-opacity" :style="{ opacity: offset < 0 ? 1 : 0 }">
        Delete <i class="ti ti-trash text-base" />
      </div>
    </div>

    <!-- foreground -->
    <div
      class="relative bg-white"
      :class="dragging ? '' : 'transition-transform duration-200'"
      :style="{ transform: `translateX(${offset}px)` }"
      @touchstart.passive="onStart"
      @touchmove="onMove"
      @touchend="onEnd"
      @touchcancel="onEnd"
    >
      <slot />
    </div>
  </div>
</template>
