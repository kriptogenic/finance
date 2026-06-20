<script setup lang="ts">
import { reactive, ref, computed, watch, onUnmounted } from 'vue'
import { categoriesApi } from '../api/categories'
import { errMessage } from '../api/client'
import type { Category, CategoryType, CreateCategoryRequest, UpdateCategoryRequest } from '../api/types'
import { categoryColors, randomCategoryColor } from '../lib/palette'
import Modal from './Modal.vue'
import IconField from './IconField.vue'

const props = withDefaults(
  defineProps<{ category?: Category | null; categories?: Category[] }>(),
  { category: null, categories: () => [] },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.category)
const error = ref('')
const saving = ref(false)

const form = reactive({
  name: props.category?.name ?? '',
  type: (props.category?.type ?? 'expense') as CategoryType,
  parent_id: props.category?.parent_id ?? '',
  icon: props.category?.icon ?? '',
  // new categories get a random palette color; editing keeps the saved one
  color: props.category ? (props.category.color ?? '') : randomCategoryColor(),
  archived: props.category?.archived ?? false,
})

const parentOptions = computed(() =>
  props.categories.filter((c) => c.type === form.type && !c.parent_id),
)

// LLM icon suggestions: refetched ~600ms after the user stops editing the name
// (or switches type). A request id guards against out-of-order responses.
const suggestions = ref<string[]>([])
const suggestLoading = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | undefined
let reqId = 0

watch(
  () => [form.name, form.type] as const,
  ([name, type]) => {
    clearTimeout(debounceTimer)
    const trimmed = name.trim()
    if (trimmed.length < 2) {
      suggestions.value = []
      suggestLoading.value = false
      return
    }
    suggestLoading.value = true
    debounceTimer = setTimeout(async () => {
      const myId = ++reqId
      try {
        const icons = await categoriesApi.suggestIcons({ name: trimmed, type })
        if (myId === reqId) suggestions.value = icons
      } catch {
        if (myId === reqId) suggestions.value = []
      } finally {
        if (myId === reqId) suggestLoading.value = false
      }
    }, 600)
  },
)

onUnmounted(() => clearTimeout(debounceTimer))

async function submit() {
  error.value = ''
  saving.value = true
  try {
    if (props.category) {
      const body: UpdateCategoryRequest = {
        name: form.name,
        archived: form.archived,
        // send empty strings too, so clearing the icon/color persists
        icon: form.icon,
        color: form.color,
      }
      // only touch the parent when it's editable (not for parents-with-children)
      if (showParent.value) body.parent_id = form.parent_id || null
      await categoriesApi.update(props.category.id, body)
    } else {
      const body: CreateCategoryRequest = {
        name: form.name,
        type: form.type,
        ...(form.parent_id ? { parent_id: form.parent_id } : {}),
        ...(form.icon ? { icon: form.icon } : {}),
        ...(form.color ? { color: form.color } : {}),
      }
      await categoriesApi.create(body)
    }
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal :title="editing ? 'Edit category' : 'New category'" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="submit">
      <div>
        <label class="lbl">Name</label>
        <input v-model="form.name" class="field" required placeholder="e.g. Groceries" />
      </div>

      <template v-if="!editing">
        <div>
          <label class="lbl">Type</label>
          <select v-model="form.type" class="field" @change="form.parent_id = ''">
            <option value="expense">Expense</option>
            <option value="income">Income</option>
          </select>
        </div>
        <div>
          <label class="lbl">Parent (optional)</label>
          <select v-model="form.parent_id" class="field">
            <option value="">— top level —</option>
            <option v-for="c in parentOptions" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
      </template>

      <div v-else class="flex items-center gap-2">
        <input id="cat-archived" v-model="form.archived" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
        <label for="cat-archived" class="text-sm text-slate-600">Archived</label>
      </div>

      <div>
        <div class="mb-1 flex items-center justify-between">
          <label class="lbl mb-0">Icon (optional)</label>
          <label class="flex items-center gap-1.5 text-xs text-slate-500">
            Color
            <input v-model="form.color" type="color" class="h-6 w-8 rounded border border-slate-300 p-0.5" />
          </label>
        </div>
        <div class="mb-2 flex flex-wrap gap-1.5">
          <button
            v-for="c in categoryColors"
            :key="c"
            type="button"
            class="h-6 w-6 rounded-full ring-2 ring-offset-1 transition hover:scale-110"
            :class="form.color === c ? 'ring-slate-500' : 'ring-transparent'"
            :style="{ backgroundColor: c }"
            :title="c"
            @click="form.color = c"
          />
        </div>
        <IconField v-model="form.icon" :color="form.color" :suggestions="suggestions" :loading="suggestLoading" />
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex justify-end gap-2 pt-1">
        <button type="button" class="btn btn-soft" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>
