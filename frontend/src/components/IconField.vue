<script setup lang="ts">
// Category icon field: shows the current selection, an LLM-suggested set (fed
// from the parent as the name is typed), and an optional browse grid of common
// icons. v-model is the bare Tabler icon name, matching the category API.
import { ref, computed } from 'vue'
import { ICON_GROUPS } from '../lib/icons'
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

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return ICON_GROUPS.map((g) => ({
    label: g.label,
    icons: q
      ? g.icons.filter((i) => i.name.includes(q) || (i.keywords ?? '').includes(q))
      : g.icons,
  })).filter((g) => g.icons.length)
})

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
      <div v-if="suggestions && suggestions.length" class="flex flex-wrap gap-1">
        <button
          v-for="name in suggestions"
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

    <!-- manual browse -->
    <button type="button" class="text-xs text-slate-400 hover:text-slate-600" @click="browseOpen = !browseOpen">
      {{ browseOpen ? 'Hide icons' : 'Browse icons' }}
    </button>
    <div v-if="browseOpen" class="max-h-56 overflow-y-auto rounded-xl border border-slate-200 p-3">
      <input v-model="query" class="field mb-2" placeholder="Search icons…" />
      <div v-for="g in filtered" :key="g.label" class="mb-3 last:mb-0">
        <p class="mb-1 text-[11px] font-semibold tracking-wide text-slate-400 uppercase">{{ g.label }}</p>
        <div class="grid grid-cols-7 gap-1">
          <button
            v-for="i in g.icons"
            :key="i.name"
            type="button"
            :title="i.name"
            class="grid h-8 w-8 place-items-center rounded-lg transition hover:bg-slate-100"
            :class="modelValue === i.name ? 'bg-slate-900 text-white hover:bg-slate-800' : 'text-slate-600'"
            @click="pick(i.name)"
          >
            <CategoryIcon :icon="i.name" :size="18" />
          </button>
        </div>
      </div>
      <p v-if="!filtered.length" class="py-2 text-center text-sm text-slate-400">No icons found</p>
    </div>
  </div>
</template>
