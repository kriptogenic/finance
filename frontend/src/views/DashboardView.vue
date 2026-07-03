<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import type { DateRange } from '../api/reports'
import { errMessage } from '../api/client'
import { useNetWorthQuery, useSpendingQuery, useCashFlowQuery } from '../api/queries'
import CategorizeCard from '../components/CategorizeCard.vue'
import DonutChart from '../components/DonutChart.vue'

const range = reactive({ from: '', to: '' })

// Spending + cash flow both key on the selected range; changing it refetches.
const reportParams = computed<DateRange>(() => {
  const p: DateRange = {}
  if (range.from) p.date_from = new Date(range.from + 'T00:00:00').toISOString()
  if (range.to) p.date_to = new Date(range.to + 'T23:59:59.999').toISOString()
  return p
})

const nwQuery = useNetWorthQuery()
const spQuery = useSpendingQuery(reportParams)
const cfQuery = useCashFlowQuery(reportParams)

const netWorth = computed(() => nwQuery.data.value ?? null)
const spending = computed(() => spQuery.data.value ?? null)
const cashFlow = computed(() => cfQuery.data.value ?? null)
// isLoading is true only on the first fetch (no cache); revisits render cached
// data immediately while a background refetch runs.
const loading = computed(() => nwQuery.isLoading.value || spQuery.isLoading.value || cfQuery.isLoading.value)
const error = computed(() => {
  const e = nwQuery.error.value || spQuery.error.value || cfQuery.error.value
  return e ? errMessage(e) : ''
})
const activePreset = ref('all')

const presets = [
  { key: '30d', label: '30D' },
  { key: '3m', label: '3M' },
  { key: '6m', label: '6M' },
  { key: '12m', label: '12M' },
  { key: 'ytd', label: 'YTD' },
  { key: 'all', label: 'All' },
]

function fmt(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function setPreset(key: string) {
  activePreset.value = key
  if (key === 'all') {
    range.from = ''
    range.to = ''
    return
  }
  const now = new Date()
  const from = new Date(now)
  if (key === '30d') from.setDate(from.getDate() - 30)
  else if (key === '3m') from.setMonth(from.getMonth() - 3)
  else if (key === '6m') from.setMonth(from.getMonth() - 6)
  else if (key === '12m') from.setMonth(from.getMonth() - 12)
  else if (key === 'ytd') from.setMonth(0, 1)
  range.from = fmt(from)
  range.to = fmt(now)
}

// Solid colors for the donut + legend (SVG strokes can't use gradient classes).
const palette = ['#6366f1', '#0ea5e9', '#ec4899', '#f59e0b', '#10b981', '#8b5cf6', '#14b8a6', '#f43f5e']

const donut = computed(() => {
  const cats = spending.value?.categories ?? []
  const total = spending.value?.total.amount ?? 0
  return cats.map((c, i) => ({
    id: c.category_id,
    name: c.category_name,
    amount: c.amount,
    color: palette[i % palette.length],
    pct: total > 0 ? (c.amount.amount / total) * 100 : 0,
  }))
})

const flowMax = computed(() =>
  Math.max(1, ...(cashFlow.value?.months ?? []).flatMap((m) => [m.income.amount, m.expense.amount])),
)

function pct(value: number, max: number): string {
  return Math.max(2, (value / max) * 100) + '%'
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Dashboard</h1>
      <p class="text-sm text-slate-500">Your money at a glance</p>
    </div>

    <!-- uncategorized review; the mobile FAB covers this on small screens -->
    <CategorizeCard :base="netWorth?.base ?? 'UZS'" class="hidden md:block" />

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else-if="netWorth && spending && cashFlow">
      <!-- hero net worth -->
      <div class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-emerald-950 via-emerald-950 to-emerald-900 p-6 text-white shadow-lg shadow-emerald-950/30 sm:p-7">
        <div class="absolute -top-12 -right-10 h-44 w-44 rounded-full bg-amber-400/20 blur-xl"></div>
        <div class="absolute -bottom-16 -left-10 h-44 w-44 rounded-full bg-amber-400/10 blur-xl"></div>
        <div class="relative">
          <div class="flex items-center justify-between">
            <p class="text-sm font-medium text-slate-300">Net worth</p>
            <span class="rounded-full bg-white/10 px-2.5 py-0.5 text-xs font-medium text-amber-300">{{ netWorth.base }}</span>
          </div>
          <p class="tabular mt-2 text-4xl font-bold tracking-tight whitespace-nowrap">{{ netWorth.net_worth.formatShort() }}</p>

          <div class="mt-6 grid grid-cols-2 gap-3">
            <div class="rounded-2xl bg-white/5 p-3.5 ring-1 ring-white/10">
              <div class="flex items-center gap-1.5 text-xs text-slate-300">
                <span class="h-2 w-2 rounded-full bg-emerald-400" /> Assets
              </div>
              <p class="tabular mt-1 text-lg font-semibold whitespace-nowrap">{{ netWorth.assets.formatShort() }}</p>
            </div>
            <div class="rounded-2xl bg-white/5 p-3.5 ring-1 ring-white/10">
              <div class="flex items-center gap-1.5 text-xs text-slate-300">
                <span class="h-2 w-2 rounded-full bg-rose-400" /> Liabilities
              </div>
              <p class="tabular mt-1 text-lg font-semibold whitespace-nowrap">{{ netWorth.liabilities.formatShort() }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- period control: drives spending + cash flow -->
      <div class="card flex flex-wrap items-center gap-2 p-2">
        <span class="px-2 text-xs font-semibold tracking-wide text-slate-400 uppercase">Period</span>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="p in presets"
            :key="p.key"
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
            :class="activePreset === p.key ? 'bg-emerald-950 text-white' : 'text-slate-500 hover:bg-slate-100'"
            @click="setPreset(p.key)"
          >
            {{ p.label }}
          </button>
        </div>
        <div class="flex w-full items-center gap-2 sm:ml-auto sm:w-auto">
          <input v-model="range.from" type="date" class="field min-w-0 flex-1 !py-1.5 sm:!w-auto sm:flex-none" title="From" @change="activePreset = 'custom'" />
          <span class="text-slate-300">–</span>
          <input v-model="range.to" type="date" class="field min-w-0 flex-1 !py-1.5 sm:!w-auto sm:flex-none" title="To" @change="activePreset = 'custom'" />
        </div>
      </div>

      <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <!-- spending donut -->
        <section class="card p-6">
          <div class="mb-5 flex items-baseline justify-between">
            <h2 class="text-base font-semibold text-slate-900">Spending by category</h2>
            <span class="tabular text-sm font-medium text-slate-400">{{ spending.total.format() }}</span>
          </div>

          <div v-if="!spending.categories.length" class="py-6 text-center text-sm text-slate-400">No spending in this period.</div>

          <div v-else class="flex flex-col items-center gap-6 sm:flex-row sm:items-center">
            <DonutChart :segments="donut.map((d) => ({ color: d.color, value: d.pct, key: d.id }))" :size="160">
              <span class="text-xs text-slate-400">Total</span>
              <span class="tabular text-sm font-bold text-slate-900">{{ spending.total.formatShort() }}</span>
            </DonutChart>

            <ul class="w-full space-y-2.5">
              <li v-for="seg in donut" :key="seg.id" class="flex items-center gap-2.5 text-sm">
                <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: seg.color }" />
                <span class="min-w-0 flex-1 truncate text-slate-700">{{ seg.name }}</span>
                <span class="tabular shrink-0 font-medium text-slate-500">{{ Math.round(seg.pct) }}%</span>
                <span class="tabular w-20 shrink-0 text-right text-slate-800">{{ seg.amount.formatShort() }}</span>
              </li>
            </ul>
          </div>
        </section>

        <!-- cash flow -->
        <section class="card p-6">
          <div class="mb-5 flex items-center justify-between">
            <h2 class="text-base font-semibold text-slate-900">Cash flow</h2>
            <div class="flex gap-4 text-xs text-slate-500">
              <span class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-emerald-400" /> Income</span>
              <span class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-rose-400" /> Expense</span>
            </div>
          </div>
          <div v-if="!cashFlow.months.length" class="py-6 text-center text-sm text-slate-400">No data in this period.</div>
          <div class="space-y-5">
            <div v-for="m in cashFlow.months" :key="m.month">
              <div class="mb-2 flex justify-between text-sm">
                <span class="font-medium text-slate-700">{{ m.month }}</span>
                <span class="tabular font-semibold" :class="m.net.isNegative() ? 'text-rose-600' : 'text-emerald-600'">{{ m.net.format() }}</span>
              </div>
              <div class="space-y-1.5">
                <div class="h-2.5 overflow-hidden rounded-full bg-slate-100">
                  <div class="h-full rounded-full bg-emerald-400" :style="{ width: pct(m.income.amount, flowMax) }" />
                </div>
                <div class="h-2.5 overflow-hidden rounded-full bg-slate-100">
                  <div class="h-full rounded-full bg-rose-400" :style="{ width: pct(m.expense.amount, flowMax) }" />
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>
