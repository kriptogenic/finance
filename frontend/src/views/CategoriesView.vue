<script setup>
import { ref, computed, onMounted } from 'vue'
import { categoriesApi } from '../api/categories'

const categories = ref([])
const loading = ref(true)
const error = ref('')

function tree(type) {
  const items = categories.value.filter((c) => c.type === type)
  return items
    .filter((c) => !c.parent_id)
    .map((top) => ({ ...top, children: items.filter((c) => c.parent_id === top.id) }))
}

const expense = computed(() => tree('expense'))
const income = computed(() => tree('income'))

onMounted(async () => {
  try {
    categories.value = await categoriesApi.list()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Categories</h1>
      <p class="text-sm text-slate-500">Two-level expense and income trees</p>
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
          <li v-for="top in group.items" :key="top.id">
            <p class="font-semibold text-slate-800">{{ top.name }}</p>
            <div v-if="top.children.length" class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="child in top.children"
                :key="child.id"
                class="rounded-full px-2.5 py-1 text-xs font-medium"
                :class="group.chip"
              >
                {{ child.name }}
              </span>
            </div>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>
