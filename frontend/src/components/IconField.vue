<script setup lang="ts">
// Category icon field: shows the current selection, an LLM-suggested set (fed
// from the parent as the name is typed), and a browse grid covering the full
// Tabler icon set (searchable, rendered incrementally so 6k SVGs don't mount at
// once). v-model is the bare Tabler icon name, matching the category API.
import { ref, computed, watch } from 'vue'
import { ALL_ICON_NAMES } from '../lib/tablerIcon'
import CategoryIcon from './CategoryIcon.vue'

const props = defineProps<{
  modelValue: string
  color?: string
  suggestions?: string[]
  loading?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const browseOpen = ref(false)
const query = ref('')

// The model occasionally invents names that aren't real Tabler icons; drop
// those so we never render a broken glyph.
const iconSet = new Set(ALL_ICON_NAMES)
const validSuggestions = computed(() => (props.suggestions ?? []).filter((n) => iconSet.has(n)))

const PAGE = 120
const visibleCount = ref(PAGE)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? ALL_ICON_NAMES.filter((n) => n.includes(q)) : ALL_ICON_NAMES
})
const visible = computed(() => filtered.value.slice(0, visibleCount.value))

// Reset the window when the query changes so results start from the top.
watch(query, () => {
  visibleCount.value = PAGE
})

function onScroll(e: Event) {
  const el = e.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 200 && visibleCount.value < filtered.value.length) {
    visibleCount.value += PAGE
  }
}

function pick(name: string) {
  emit('update:modelValue', props.modelValue === name ? '' : name)
}
</script>

<template>
  <div class="space-y-2">
    <!-- current selection -->
    <div class="flex items-center gap-2">
      <span class="grid h-10 w-10 place-items-center rounded-lg bg-slate-50 text-slate-600 ring-1 ring-slate-200">
        <CategoryIcon v-if="modelValue" :icon="modelValue" :color="color" :size="22" />
        <span v-else class="text-xs text-slate-300">—</span>
      </span>
      <span class="text-sm" :class="modelValue ? 'text-slate-700' : 'text-slate-400'">{{ modelValue || 'No icon' }}</span>
      <button v-if="modelValue" type="button" class="ml-auto text-xs text-slate-400 hover:text-rose-600" @click="emit('update:modelValue', '')">
        Clear
      </button>
    </div>

    <!-- LLM suggestions -->
    <div>
      <div class="mb-1 flex items-center gap-1.5 text-[11px] font-medium text-slate-400">
        <span>Suggested</span>
        <span v-if="loading" class="h-3 w-3 animate-spin rounded-full border-[1.5px] border-slate-300 border-t-slate-500" />
      </div>
      <div v-if="validSuggestions.length" class="flex flex-wrap gap-1">
        <button
          v-for="name in validSuggestions"
          :key="name"
          type="button"
          :title="name"
          class="grid h-9 w-9 place-items-center rounded-lg transition hover:bg-slate-100"
          :class="modelValue === name ? 'bg-slate-900 text-white hover:bg-slate-800' : 'text-slate-600'"
          @click="pick(name)"
        >
          <CategoryIcon :icon="name" :color="modelValue === name ? null : color" :size="20" />
        </button>
      </div>
      <p v-else-if="!loading" class="text-xs text-slate-400">Type a name to get icon suggestions.</p>
    </div>

    <!-- browse the full icon set -->
    <button type="button" class="text-xs text-slate-400 hover:text-slate-600" @click="browseOpen = !browseOpen">
      {{ browseOpen ? 'Hide icons' : 'Browse all icons' }}
    </button>
    <div v-if="browseOpen" class="rounded-xl border border-slate-200 p-3">
      <input v-model="query" class="field mb-2" placeholder="Search all icons…" />
      <div class="max-h-56 overflow-y-auto" @scroll="onScroll">
        <div class="grid grid-cols-7 gap-1">
          <button
            v-for="name in visible"
            :key="name"
            type="button"
            :title="name"
            class="grid h-8 w-8 place-items-center rounded-lg transition hover:bg-slate-100"
            :class="modelValue === name ? 'bg-slate-900 text-white hover:bg-slate-800' : 'text-slate-600'"
            @click="pick(name)"
          >
            <CategoryIcon :icon="name" :size="18" />
          </button>
        </div>
        <p v-if="!filtered.length" class="py-2 text-center text-sm text-slate-400">No icons found</p>
      </div>
      <p class="mt-1.5 text-[11px] text-slate-400">
        {{ filtered.length }} icons<span v-if="visible.length < filtered.length"> · scroll for more</span>
      </p>
    </div>
  </div>
</template>
