<script setup lang="ts">
// Manage merchant → category ingest routing rules. When a bank notification is
// ingested, the merchant text is matched against these patterns (case-insensitive
// substring); the longest match wins and sets the transaction's category.
import { ref, computed, onMounted } from 'vue'
import { categoryRulesApi } from '../api/categories'
import { errMessage } from '../api/client'
import type { Category, CategoryRule } from '../api/types'

const props = defineProps<{ categories: Category[] }>()

const rules = ref<CategoryRule[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref(false)

const pattern = ref('')
const categoryId = ref('')

// Flat option list with subcategories indented, mirroring TransactionForm.
const options = computed(() =>
  props.categories
    .filter((c) => !c.archived)
    .map((c) => ({ id: c.id, label: (c.parent_id ? '— ' : '') + c.name })),
)

async function load() {
  loading.value = true
  try {
    rules.value = await categoryRulesApi.list()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

async function add() {
  const p = pattern.value.trim()
  if (!p || !categoryId.value) return
  saving.value = true
  error.value = ''
  try {
    await categoryRulesApi.create({ pattern: p, category_id: categoryId.value })
    pattern.value = ''
    categoryId.value = ''
    await load()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}

async function remove(r: CategoryRule) {
  if (!confirm(`Delete rule "${r.pattern}"?`)) return
  try {
    await categoryRulesApi.remove(r.id)
    await load()
  } catch (e) {
    alert(errMessage(e))
  }
}

onMounted(load)
</script>

<template>
  <section class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
    <div class="mb-1 flex items-center gap-2">
      <span class="h-2.5 w-2.5 rounded-full bg-indigo-400" />
      <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">Ingest routing rules</h2>
    </div>
    <p class="mb-4 text-sm text-slate-500">
      Bank-notification ingest matches the merchant text against these patterns
      (case-insensitive substring); the longest match wins and sets the category.
    </p>

    <form class="mb-4 flex flex-col gap-2 sm:flex-row" @submit.prevent="add">
      <input v-model="pattern" class="field sm:flex-1" placeholder="Merchant text, e.g. UBER" />
      <select v-model="categoryId" class="field sm:w-56">
        <option value="" disabled>Category…</option>
        <option v-for="o in options" :key="o.id" :value="o.id">{{ o.label }}</option>
      </select>
      <button type="submit" class="btn btn-primary" :disabled="saving || !pattern.trim() || !categoryId">Add</button>
    </form>

    <p v-if="error" class="mb-3 rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

    <p v-if="loading" class="text-sm text-slate-400">Loading…</p>
    <p v-else-if="!rules.length" class="text-sm text-slate-400">No rules yet.</p>
    <ul v-else class="divide-y divide-slate-100">
      <li v-for="r in rules" :key="r.id" class="group flex items-center gap-3 py-2">
        <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-700">{{ r.pattern }}</code>
        <span class="text-slate-300">→</span>
        <span class="rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-600">{{ r.category_name }}</span>
        <button
          class="ml-auto text-xs text-slate-400 opacity-0 transition group-hover:opacity-100 hover:text-rose-600"
          title="Delete"
          @click="remove(r)"
        >
          Delete
        </button>
      </li>
    </ul>
  </section>
</template>
