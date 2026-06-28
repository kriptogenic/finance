<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { scheduledTransactionsApi } from '../api/scheduledTransactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type { Account, Category, ForecastReport, ScheduledTransaction } from '../api/types'
import ScheduledTransactionForm from '../components/ScheduledTransactionForm.vue'

const schedules = ref<ScheduledTransaction[]>([])
const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const forecast = ref<ForecastReport | null>(null)
const base = ref('UZS')
const month = ref(new Date().toISOString().slice(0, 7)) // YYYY-MM
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<ScheduledTransaction | null>(null)

const accountName = computed(() => Object.fromEntries(accounts.value.map((a) => [a.id, a.name])))
const categoryName = computed(() => Object.fromEntries(categories.value.map((c) => [c.id, c.name])))

// schedule_id -> its projected contribution this month
const lineBySchedule = computed(() =>
  Object.fromEntries((forecast.value?.lines ?? []).map((l) => [l.schedule_id, l])),
)
const budgetLines = computed(() => forecast.value?.budget_lines ?? [])

const periodLabel: Record<string, string> = { weekly: 'Weekly', monthly: 'Monthly', yearly: 'Yearly' }

const unit: Record<string, string> = { daily: 'day', weekly: 'week', monthly: 'month', yearly: 'year' }
function cadence(s: ScheduledTransaction): string {
  if (s.frequency === 'once') return 'One-time'
  const u = unit[s.frequency] ?? s.frequency
  return s.interval === 1 ? `Every ${u}` : `Every ${s.interval} ${u}s`
}

function subtitle(s: ScheduledTransaction): string {
  if (s.type === 'transfer') {
    return `${accountName.value[s.from_account_id ?? ''] ?? '—'} → ${accountName.value[s.to_account_id ?? ''] ?? '—'}`
  }
  const acc = accountName.value[(s.from_account_id ?? s.to_account_id) ?? ''] ?? '—'
  const cat = categoryName.value[s.category_id ?? ''] ?? '—'
  return `${cat} · ${acc}`
}

const groups = [
  { type: 'income', label: 'Income', dot: 'bg-emerald-500' },
  { type: 'expense', label: 'Expenses', dot: 'bg-rose-500' },
  { type: 'transfer', label: 'Transfers', dot: 'bg-sky-500' },
] as const

function itemsFor(type: string): ScheduledTransaction[] {
  return schedules.value.filter((s) => s.type === type)
}

const monthLabel = computed(() =>
  new Date(month.value + '-01').toLocaleDateString(undefined, { month: 'long', year: 'numeric' }),
)

async function loadForecast() {
  try {
    forecast.value = await reportsApi.forecast(month.value)
  } catch (e) {
    error.value = errMessage(e)
  }
}

async function load() {
  loading.value = true
  try {
    const [scs, accs, cats] = await Promise.all([
      scheduledTransactionsApi.list(),
      accountsApi.list(),
      categoriesApi.list({}),
    ])
    schedules.value = scs
    accounts.value = accs
    categories.value = cats
    await loadForecast()
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
function openEdit(s: ScheduledTransaction) {
  editing.value = s
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(s: ScheduledTransaction) {
  if (!(await confirm({ title: 'Delete planned item?', message: 'This planned transaction will be removed from the forecast.' }))) return
  try {
    await scheduledTransactionsApi.remove(s.id)
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
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Forecast</h1>
      </div>
      <div class="flex items-center gap-2">
        <input v-model="month" type="month" class="field !w-auto !py-1.5" @change="loadForecast" />
        <button class="btn btn-primary shrink-0" @click="openNew">+ New<span class="hidden sm:inline"> plan</span></button>
      </div>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <!-- summary -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-slate-400">Income</p>
          <p class="tabular mt-1 text-lg font-semibold text-emerald-600">{{ forecast?.income.format() }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-slate-400">Expenses</p>
          <p class="tabular mt-1 text-lg font-semibold text-rose-600">{{ forecast?.expense.format() }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-slate-400">Transfers</p>
          <p class="tabular mt-1 text-lg font-semibold text-sky-600">{{ forecast?.transfers.format() }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-slate-400">Net · {{ monthLabel }}</p>
          <p class="tabular mt-1 text-lg font-semibold" :class="forecast && forecast.net.isNegative() ? 'text-rose-600' : 'text-slate-800'">
            {{ forecast?.net.format() }}
          </p>
        </div>
      </div>

      <p v-if="forecast?.missing_rates?.length" class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-700 ring-1 ring-amber-100">
        No exchange rate for {{ forecast.missing_rates.join(', ') }} — those plans are excluded from the totals.
      </p>

      <div v-if="!schedules.length && !budgetLines.length" class="card p-10 text-center text-sm text-slate-400">
        No planned transactions yet. Add your salary, rent, loan and card payments to forecast the month.
      </div>

      <!-- grouped plan items -->
      <div v-for="g in groups" v-else :key="g.type" class="space-y-2">
        <template v-if="itemsFor(g.type).length || (g.type === 'expense' && budgetLines.length)">
          <div class="flex items-center gap-2 px-1">
            <span class="h-2 w-2 rounded-full" :class="g.dot" />
            <h2 class="text-sm font-semibold text-slate-700">{{ g.label }}</h2>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div v-for="s in itemsFor(g.type)" :key="s.id" class="card group flex items-center justify-between p-4">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <p class="truncate font-semibold text-slate-800">{{ s.name || categoryName[s.category_id ?? ''] || cadence(s) }}</p>
                  <span v-if="s.paused" class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-500">Paused</span>
                </div>
                <p class="mt-0.5 truncate text-xs text-slate-400">{{ subtitle(s) }} · {{ cadence(s) }}</p>
              </div>
              <div class="flex items-center gap-3 pl-3">
                <div class="text-right">
                  <p v-if="lineBySchedule[s.id]" class="tabular font-semibold text-slate-800">{{ lineBySchedule[s.id].amount.format() }}</p>
                  <p v-else class="text-xs text-slate-300">Not this month</p>
                  <p v-if="lineBySchedule[s.id] && lineBySchedule[s.id].occurrences > 1" class="text-[11px] text-slate-400">
                    ×{{ lineBySchedule[s.id].occurrences }}
                  </p>
                </div>
                <div class="flex gap-1 opacity-100 transition can-hover:opacity-0 can-hover:group-hover:opacity-100">
                  <button class="text-sm text-slate-400 hover:text-slate-700" title="Edit" @click="openEdit(s)"><i class="ti ti-pencil" /></button>
                  <button class="text-sm text-slate-400 hover:text-rose-600" title="Delete" @click="remove(s)"><i class="ti ti-trash" /></button>
                </div>
              </div>
            </div>

            <!-- budgeted spending (managed on the Budgets tab) -->
            <RouterLink
              v-for="b in (g.type === 'expense' ? budgetLines : [])"
              :key="b.budget_id"
              to="/budgets"
              class="card flex items-center justify-between p-4 ring-1 ring-dashed ring-slate-200 hover:ring-slate-300"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <p class="truncate font-semibold text-slate-800">{{ categoryName[b.category_id] ?? 'Budget' }}</p>
                  <span class="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-600">Budget</span>
                </div>
                <p class="mt-0.5 truncate text-xs text-slate-400">{{ periodLabel[b.period] ?? b.period }} limit</p>
              </div>
              <p class="tabular font-semibold text-slate-800">{{ b.amount.format() }}</p>
            </RouterLink>
          </div>
        </template>
      </div>
    </template>

    <ScheduledTransactionForm
      v-if="formOpen"
      :schedule="editing"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
