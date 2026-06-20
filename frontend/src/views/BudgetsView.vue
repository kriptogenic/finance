<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { budgetsApi } from '../api/budgets'
import { categoriesApi } from '../api/categories'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import type { Budget, Category } from '../api/types'
import { formatDate } from '../lib/format'
import BudgetForm from '../components/BudgetForm.vue'

const budgets = ref<Budget[]>([])
const categories = ref<Category[]>([])
const base = ref('UZS')
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<Budget | null>(null)

const budgetedIds = computed(() => budgets.value.map((b) => b.category_id))

const periodLabel: Record<string, string> = { weekly: 'This week', monthly: 'This month', yearly: 'This year' }

function tone(pct: number) {
  if (pct >= 100) return { bar: 'bg-rose-500', text: 'text-rose-600', chip: 'bg-rose-50 text-rose-600' }
  if (pct >= 80) return { bar: 'bg-amber-500', text: 'text-amber-600', chip: 'bg-amber-50 text-amber-600' }
  return { bar: 'bg-emerald-500', text: 'text-slate-700', chip: 'bg-slate-100 text-slate-500' }
}

async function load() {
  loading.value = true
  try {
    const [bs, cats] = await Promise.all([budgetsApi.list(), categoriesApi.list({ type: 'expense' })])
    budgets.value = bs
    categories.value = cats
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
function openEdit(b: Budget) {
  editing.value = b
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(b: Budget) {
  if (!confirm(`Delete the budget for "${b.category_name}"?`)) return
  try {
    await budgetsApi.remove(b.id)
    load()
  } catch (e) {
    alert(errMessage(e))
  }
}

onMounted(() => window.addEventListener('data:refresh', load))
onUnmounted(() => window.removeEventListener('data:refresh', load))

onMounted(async () => {
  try {
    base.value = (await reportsApi.netWorth()).base
  } catch {
    /* keep default */
  }
  load()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Budgets</h1>
        <p class="text-sm text-slate-500">Spending limits by category, current period</p>
      </div>
      <button class="btn btn-primary shrink-0" @click="openNew">+ New<span class="hidden sm:inline"> budget</span></button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else-if="!budgets.length" class="card p-10 text-center text-sm text-slate-400">
      No budgets yet. Create one to track spending against a limit.
    </div>

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div v-for="b in budgets" :key="b.id" class="card group p-5">
        <div class="flex items-start justify-between">
          <div>
            <p class="font-semibold text-slate-800">{{ b.category_name }}</p>
            <p class="text-xs text-slate-400">{{ periodLabel[b.period] || b.period }} · {{ formatDate(b.period_start) }}–{{ formatDate(b.period_end) }}</p>
          </div>
          <div class="flex items-center gap-2">
            <span class="rounded-full px-2 py-0.5 text-xs font-semibold" :class="tone(b.percent).chip">{{ Math.round(b.percent) }}%</span>
            <div class="flex gap-1 opacity-100 transition can-hover:opacity-0 can-hover:group-hover:opacity-100">
              <button class="text-sm text-slate-400 hover:text-slate-700" title="Edit" @click="openEdit(b)"><i class="ti ti-pencil" /></button>
              <button class="text-sm text-slate-400 hover:text-rose-600" title="Delete" @click="remove(b)"><i class="ti ti-trash" /></button>
            </div>
          </div>
        </div>

        <div class="mt-4 mb-2 flex items-baseline justify-between text-sm">
          <span class="tabular font-semibold" :class="tone(b.percent).text">{{ b.spent.format() }}</span>
          <span class="tabular text-slate-400">of {{ b.amount.format() }}</span>
        </div>
        <div class="h-2.5 overflow-hidden rounded-full bg-slate-100">
          <div class="h-full rounded-full transition-all" :class="tone(b.percent).bar" :style="{ width: Math.min(100, b.percent) + '%' }" />
        </div>
        <p class="mt-2 text-xs" :class="b.remaining.isNegative() ? 'text-rose-600' : 'text-slate-400'">
          {{ b.remaining.isNegative() ? 'Over by ' + b.remaining.abs().format() : b.remaining.format() + ' left' }}
        </p>
      </div>
    </div>

    <BudgetForm
      v-if="formOpen"
      :budget="editing"
      :categories="categories"
      :budgeted-ids="budgetedIds"
      :base="base"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
