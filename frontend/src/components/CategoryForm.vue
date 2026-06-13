<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { categoriesApi } from '../api/categories'
import { errMessage } from '../api/client'
import type { Category, CategoryType, CreateCategoryRequest, UpdateCategoryRequest } from '../api/types'
import Modal from './Modal.vue'

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
  color: props.category?.color ?? '',
  archived: props.category?.archived ?? false,
})

const parentOptions = computed(() =>
  props.categories.filter((c) => c.type === form.type && !c.parent_id),
)

async function submit() {
  error.value = ''
  saving.value = true
  try {
    if (props.category) {
      const body: UpdateCategoryRequest = {
        name: form.name,
        archived: form.archived,
        ...(form.icon ? { icon: form.icon } : {}),
        ...(form.color ? { color: form.color } : {}),
      }
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

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="lbl">Icon (optional)</label>
          <input v-model="form.icon" class="field" placeholder="🍔" />
        </div>
        <div>
          <label class="lbl">Color (optional)</label>
          <input v-model="form.color" type="color" class="field h-10 p-1" />
        </div>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex justify-end gap-2 pt-1">
        <button type="button" class="btn btn-soft" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>
