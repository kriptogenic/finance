<script setup lang="ts">
// Reusable SVG donut. Circumference is normalized to 100 (r = 15.915) so each
// segment's dash length is its percentage; cumulative negative dashoffset walks
// the ring clockwise, and -rotate-90 starts it at 12 o'clock.
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    segments: { color: string; value: number; key?: string }[]
    size?: number
    stroke?: number
    track?: string
  }>(),
  { size: 160, stroke: 3.6, track: '#f1f5f9' },
)

const arcs = computed(() => {
  const total = props.segments.reduce((s, x) => s + Math.max(0, x.value), 0)
  let cum = 0
  return props.segments.map((seg, i) => {
    const pct = total > 0 ? (Math.max(0, seg.value) / total) * 100 : 0
    const arc = { key: seg.key ?? String(i), color: seg.color, dash: pct, offset: -cum }
    cum += pct
    return arc
  })
})
</script>

<template>
  <div class="relative shrink-0" :style="{ width: `${size}px`, height: `${size}px` }">
    <svg viewBox="0 0 36 36" class="-rotate-90" :width="size" :height="size">
      <circle cx="18" cy="18" r="15.915" fill="none" :stroke="track" :stroke-width="stroke" />
      <circle
        v-for="a in arcs"
        :key="a.key"
        cx="18"
        cy="18"
        r="15.915"
        fill="none"
        :stroke="a.color"
        :stroke-width="stroke"
        :stroke-dasharray="`${a.dash} ${100 - a.dash}`"
        :stroke-dashoffset="a.offset"
      />
    </svg>
    <div class="absolute inset-0 flex flex-col items-center justify-center text-center">
      <slot />
    </div>
  </div>
</template>
