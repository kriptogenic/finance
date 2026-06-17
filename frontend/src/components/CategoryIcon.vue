<script setup lang="ts">
// Renders a category icon from the Tabler webfont. Tabler names (e.g.
// "shopping-cart") render as a glyph; legacy values (emoji, free text) fall
// back to plain text so older data keeps working. `size` is the px font-size;
// `color` tints the glyph.
import { computed } from 'vue'
import { isIconName } from '../lib/tablerIcon'

const props = withDefaults(
  defineProps<{ icon?: string | null; color?: string | null; size?: number }>(),
  { icon: null, color: null, size: 20 },
)

const named = computed(() => isIconName(props.icon))
const style = computed(() => ({
  fontSize: `${props.size}px`,
  lineHeight: 1,
  ...(props.color ? { color: props.color } : {}),
}))
</script>

<template>
  <i v-if="named" :class="`ti ti-${icon}`" :style="style" aria-hidden="true" />
  <span v-else-if="icon" :style="color ? { color } : undefined">{{ icon }}</span>
</template>
