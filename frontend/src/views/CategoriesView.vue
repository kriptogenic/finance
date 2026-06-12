<script setup>
import { ref, computed, onMounted } from 'vue'
import { categoriesApi } from '../api/categories'

const categories = ref([])
const loading = ref(true)
const error = ref('')

function tree(type) {
  const items = categories.value.filter((c) => c.type === type)
  const tops = items.filter((c) => !c.parent_id)
  return tops.map((top) => ({
    ...top,
    children: items.filter((c) => c.parent_id === top.id),
  }))
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
    <h1 class="text-2xl font-semibold">Categories</h1>

    <p v-if="error" class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else class="grid grid-cols-1 gap-6 sm:grid-cols-2">
      <section
        v-for="group in [{ title: 'Expense', items: expense }, { title: 'Income', items: income }]"
        :key="group.title"
        class="rounded-xl border border-slate-200 bg-white p-5"
      >
        <h2 class="mb-3 text-lg font-medium">{{ group.title }}</h2>
        <div v-if="!group.items.length" class="text-sm text-slate-400">None</div>
        <ul class="space-y-2">
          <li v-for="top in group.items" :key="top.id">
            <span class="font-medium">{{ top.name }}</span>
            <ul v-if="top.children.length" class="mt-1 ml-4 space-y-1 border-l border-slate-200 pl-3">
              <li v-for="child in top.children" :key="child.id" class="text-sm text-slate-600">
                {{ child.name }}
              </li>
            </ul>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>
