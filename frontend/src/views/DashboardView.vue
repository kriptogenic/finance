<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import type { CashFlowReport, NetWorthReport, SpendingReport } from '../api/types'
import { formatMinor } from '../lib/format'

const netWorth = ref<NetWorthReport | null>(null)
const spending = ref<SpendingReport | null>(null)
const cashFlow = ref<CashFlowReport | null>(null)
const loading = ref(true)
const error = ref('')

const range = reactive({ from: '', to: '' })
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

const spendingMax = computed(() =>
  Math.max(1, ...(spending.value?.categories ?? []).map((c) => c.amount.amount)),
)
const flowMax = computed(() =>
  Math.max(1, ...(cashFlow.value?.months ?? []).flatMap((m) => [m.income.amount, m.expense.amount])),
)
const barColors = ['from-violet-500 to-indigo-500', 'from-sky-500 to-cyan-500', 'from-fuchsia-500 to-pink-500', 'from-amber-500 to-orange-500', 'from-emerald-500 to-teal-500']

function pct(value: number, max: number): string {
  return Math.max(2, (value / max) * 100) + '%'
}

async function loadReports() {
  const params: { date_from?: string; date_to?: string } = {}
  if (range.from) params.date_from = new Date(range.from + 'T00:00:00').toISOString()
  if (range.to) params.date_to = new Date(range.to + 'T23:59:59.999').toISOString()
  try {
    const [sp, cf] = await Promise.all([reportsApi.spending(params), reportsApi.cashFlow(params)])
    spending.value = sp
    cashFlow.value = cf
  } catch (e) {
    error.value = errMessage(e)
  }
}

let timer: number | undefined
watch(
  range,
  () => {
    clearTimeout(timer)
    timer = window.setTimeout(loadReports, 150)
  },
  { deep: true },
)

onMounted(async () => {
  try {
    netWorth.value = await reportsApi.netWorth()
    await loadReports()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-8">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Dashboard</h1>
      <p class="text-sm text-slate-500">Your money at a glance</p>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else-if="netWorth && spending && cashFlow">
      <!-- hero + stats -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <div class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-violet-600 via-indigo-600 to-indigo-700 p-7 text-white shadow-lg shadow-indigo-500/20">
          <div class="absolute -top-10 -right-10 h-40 w-40 rounded-full bg-white/10"></div>
          <div class="absolute -bottom-12 -left-8 h-40 w-40 rounded-full bg-white/5"></div>
          <p class="text-sm font-medium text-indigo-100">Net worth</p>
          <p class="tabular mt-2 text-3xl font-bold tracking-tight whitespace-nowrap">{{ netWorth.net_worth.formatShort() }}</p>
          <p class="mt-1 text-xs text-indigo-200">in {{ netWorth.base }}</p>
        </div>

        <div class="rounded-3xl bg-white p-7 shadow-sm ring-1 ring-slate-200/70">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-xl bg-emerald-50 text-emerald-600">↑</span>
            <p class="text-sm font-medium text-slate-500">Assets</p>
          </div>
          <p class="tabular mt-4 text-3xl font-bold whitespace-nowrap text-slate-900">{{ netWorth.assets.formatShort() }}</p>
        </div>

        <div class="rounded-3xl bg-white p-7 shadow-sm ring-1 ring-slate-200/70">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-xl bg-rose-50 text-rose-600">↓</span>
            <p class="text-sm font-medium text-slate-500">Liabilities</p>
          </div>
          <p class="tabular mt-4 text-3xl font-bold whitespace-nowrap text-slate-900">{{ netWorth.liabilities.formatShort() }}</p>
        </div>
      </div>

      <!-- currency exposure (current snapshot) -->
      <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
        <h2 class="mb-4 text-base font-semibold text-slate-900">Currency exposure</h2>
        <ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <li v-for="e in netWorth.by_currency" :key="e.currency" class="flex items-center justify-between rounded-2xl bg-slate-50 px-4 py-3">
            <span class="grid h-9 w-9 place-items-center rounded-xl bg-white text-xs font-bold text-slate-600 ring-1 ring-slate-200">{{ e.currency }}</span>
            <div class="text-right">
              <p class="tabular text-sm font-semibold" :class="e.net < 0 ? 'text-rose-600' : 'text-slate-800'">{{ formatMinor(e.net, e.currency) }}</p>
              <p class="tabular text-xs text-slate-400">{{ e.net_in_base ? e.net_in_base.format() : 'no rate' }}</p>
            </div>
          </li>
        </ul>
      </section>

      <!-- period control: drives spending + cash flow -->
      <div class="flex flex-wrap items-center gap-2 rounded-2xl bg-white p-2 shadow-sm ring-1 ring-slate-200/70">
        <span class="px-2 text-xs font-semibold tracking-wide text-slate-400 uppercase">Period</span>
        <div class="flex gap-1">
          <button
            v-for="p in presets"
            :key="p.key"
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
            :class="activePreset === p.key ? 'bg-slate-900 text-white' : 'text-slate-500 hover:bg-slate-100'"
            @click="setPreset(p.key)"
          >
            {{ p.label }}
          </button>
        </div>
        <div class="ml-auto flex items-center gap-2">
          <input v-model="range.from" type="date" class="field !w-auto !py-1.5" title="From" @change="activePreset = 'custom'" />
          <span class="text-slate-300">–</span>
          <input v-model="range.to" type="date" class="field !w-auto !py-1.5" title="To" @change="activePreset = 'custom'" />
        </div>
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <!-- spending -->
        <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
          <div class="mb-5 flex items-baseline justify-between">
            <h2 class="text-base font-semibold text-slate-900">Spending by category</h2>
            <span class="tabular text-sm font-medium text-slate-400">{{ spending.total.format() }}</span>
          </div>
          <div v-if="!spending.categories.length" class="py-6 text-center text-sm text-slate-400">No spending in this period.</div>
          <ul class="space-y-4">
            <li v-for="(c, i) in spending.categories" :key="c.category_id">
              <div class="mb-1.5 flex justify-between text-sm">
                <span class="font-medium text-slate-700">{{ c.category_name }}</span>
                <span class="tabular text-slate-500">{{ c.amount.format() }}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full bg-slate-100">
                <div class="h-full rounded-full bg-gradient-to-r" :class="barColors[i % barColors.length]" :style="{ width: pct(c.amount.amount, spendingMax) }" />
              </div>
            </li>
          </ul>
        </section>

        <!-- cash flow -->
        <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
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
