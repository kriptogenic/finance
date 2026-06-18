<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { categoriesApi } from '../api/categories'
import { errMessage } from '../api/client'
import type { Category, CategoryType } from '../api/types'
import CategoryForm from '../components/CategoryForm.vue'
import CategoryIcon from '../components/CategoryIcon.vue'
import CategoryRulesManager from '../components/CategoryRulesManager.vue'

const categories = ref<Category[]>([])
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<Category | null>(null)

function tree(type: CategoryType) {
  const items = categories.value.filter((c) => c.type === type)
  return items
    .filter((c) => !c.parent_id)
    .map((top) => ({ ...top, children: items.filter((c) => c.parent_id === top.id) }))
}
const expense = computed(() => tree('expense'))
const income = computed(() => tree('income'))

async function load() {
  loading.value = true
  try {
    categories.value = await categoriesApi.list()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}
function openNew() {
  editing.value = null
  formOpen.value = true
}
function openEdit(c: Category) {
  editing.value = c
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(c: Category) {
  if (!confirm(`Delete category "${c.name}"?`)) return
  try {
    await categoriesApi.remove(c.id)
    load()
  } catch (e) {
    alert(errMessage(e))
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Categories</h1>
        <p class="text-sm text-slate-500">Two-level expense and income trees</p>
      </div>
      <button class="btn btn-primary" @click="openNew">+ New category</button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else class="grid grid-cols-1 gap-6 sm:grid-cols-2">
      <section
        v-for="group in [
          { title: 'Expense', items: expense, accent: 'bg-rose-400', chip: 'bg-rose-50 text-rose-600' },
          { title: 'Income', items: income, accent: 'bg-emerald-400', chip: 'bg-emerald-50 text-emerald-600' },
        ]"
        :key="group.title"
        class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70"
      >
        <div class="mb-4 flex items-center gap-2">
          <span class="h-2.5 w-2.5 rounded-full" :class="group.accent" />
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">{{ group.title }}</h2>
        </div>

        <div v-if="!group.items.length" class="text-sm text-slate-400">None</div>
        <ul class="space-y-4">
          <li v-for="top in group.items" :key="top.id" class="group">
            <div class="flex items-center gap-2">
              <CategoryIcon :icon="top.icon" :color="top.color" class="text-lg leading-none text-slate-500" />
              <p class="font-semibold text-slate-800">{{ top.name }}</p>
              <div class="flex gap-1 opacity-0 transition group-hover:opacity-100">
                <button class="text-xs text-slate-400 hover:text-slate-700" title="Edit" @click="openEdit(top)">✎</button>
                <button class="text-xs text-slate-400 hover:text-rose-600" title="Delete" @click="remove(top)">🗑</button>
              </div>
            </div>
            <div v-if="top.children.length" class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="child in top.children"
                :key="child.id"
                class="group/c flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium"
                :class="group.chip"
              >
                <CategoryIcon v-if="child.icon" :icon="child.icon" :color="child.color" class="leading-none" />
                {{ child.name }}
                <button class="opacity-0 transition group-hover/c:opacity-100" title="Delete" @click="remove(child)">×</button>
              </span>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <CategoryRulesManager v-if="!loading && !error" :categories="categories" />

    <CategoryForm v-if="formOpen" :category="editing" :categories="categories" @close="formOpen = false" @saved="onSaved" />
  </div>
</template>
