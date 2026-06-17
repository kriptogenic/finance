<script setup lang="ts">
// Renders a category icon. Tabler icon names (e.g. "shopping-cart") render as
// an SVG; legacy values (emoji, free text) fall back to plain text so older
// data keeps working. `size` is the px square; `color` tints via currentColor.
import { computed } from 'vue'
import { isIconName, tablerIcon } from '../lib/tablerIcon'

const props = withDefaults(
  defineProps<{ icon?: string | null; color?: string | null; size?: number }>(),
  { icon: null, color: null, size: 20 },
)

const named = computed(() => isIconName(props.icon))
const component = computed(() => (named.value ? tablerIcon(props.icon as string) : null))
</script>

<template>
  <component
    :is="component"
    v-if="named && component"
    :size="size"
    :stroke-width="1.75"
    :style="color ? { color } : undefined"
    aria-hidden="true"
  />
  <span v-else-if="icon" :style="color ? { color } : undefined">{{ icon }}</span>
</template>
