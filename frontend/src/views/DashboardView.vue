<script setup>
import { ref, computed, onMounted } from 'vue'
import { reportsApi } from '../api/reports'
import { formatMoney, formatMoneyShort, formatMinor } from '../lib/format'

const netWorth = ref(null)
const spending = ref(null)
const cashFlow = ref(null)
const loading = ref(true)
const error = ref('')

const spendingMax = computed(() =>
  Math.max(1, ...(spending.value?.categories ?? []).map((c) => c.amount.amount)),
)
const flowMax = computed(() =>
  Math.max(1, ...(cashFlow.value?.months ?? []).flatMap((m) => [m.income.amount, m.expense.amount])),
)
const barColors = ['from-violet-500 to-indigo-500', 'from-sky-500 to-cyan-500', 'from-fuchsia-500 to-pink-500', 'from-amber-500 to-orange-500', 'from-emerald-500 to-teal-500']

function pct(value, max) {
  return Math.max(2, (value / max) * 100) + '%'
}

onMounted(async () => {
  try {
    const [nw, sp, cf] = await Promise.all([
      reportsApi.netWorth(),
      reportsApi.spending(),
      reportsApi.cashFlow(),
    ])
    netWorth.value = nw
    spending.value = sp
    cashFlow.value = cf
  } catch (e) {
    error.value = e.response?.data?.error || e.message
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

    <template v-else>
      <!-- hero + stats -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <div class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-violet-600 via-indigo-600 to-indigo-700 p-7 text-white shadow-lg shadow-indigo-500/20">
          <div class="absolute -right-10 -top-10 h-40 w-40 rounded-full bg-white/10"></div>
          <div class="absolute -bottom-12 -left-8 h-40 w-40 rounded-full bg-white/5"></div>
          <p class="text-sm font-medium text-indigo-100">Net worth</p>
          <p class="tabular mt-2 text-3xl font-bold tracking-tight whitespace-nowrap">{{ formatMoneyShort(netWorth.net_worth) }}</p>
          <p class="mt-1 text-xs text-indigo-200">in {{ netWorth.base }}</p>
        </div>

        <div class="rounded-3xl bg-white p-7 shadow-sm ring-1 ring-slate-200/70">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-xl bg-emerald-50 text-emerald-600">↑</span>
            <p class="text-sm font-medium text-slate-500">Assets</p>
          </div>
          <p class="tabular mt-4 text-3xl font-bold text-slate-900 whitespace-nowrap">{{ formatMoneyShort(netWorth.assets) }}</p>
        </div>

        <div class="rounded-3xl bg-white p-7 shadow-sm ring-1 ring-slate-200/70">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-xl bg-rose-50 text-rose-600">↓</span>
            <p class="text-sm font-medium text-slate-500">Liabilities</p>
          </div>
          <p class="tabular mt-4 text-3xl font-bold text-slate-900 whitespace-nowrap">{{ formatMoneyShort(netWorth.liabilities) }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- spending -->
        <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70 lg:col-span-2">
          <div class="mb-5 flex items-baseline justify-between">
            <h2 class="text-base font-semibold text-slate-900">Spending by category</h2>
            <span class="tabular text-sm font-medium text-slate-400">{{ formatMoney(spending.total) }}</span>
          </div>
          <div v-if="!spending.categories.length" class="py-6 text-center text-sm text-slate-400">No spending yet.</div>
          <ul class="space-y-4">
            <li v-for="(c, i) in spending.categories" :key="c.category_id">
              <div class="mb-1.5 flex justify-between text-sm">
                <span class="font-medium text-slate-700">{{ c.category_name }}</span>
                <span class="tabular text-slate-500">{{ formatMoney(c.amount) }}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full bg-slate-100">
                <div class="h-full rounded-full bg-gradient-to-r" :class="barColors[i % barColors.length]" :style="{ width: pct(c.amount.amount, spendingMax) }" />
              </div>
            </li>
          </ul>
        </section>

        <!-- currency exposure -->
        <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
          <h2 class="mb-4 text-base font-semibold text-slate-900">Currency exposure</h2>
          <ul class="space-y-3">
            <li v-for="e in netWorth.by_currency" :key="e.currency" class="flex items-center justify-between">
              <span class="grid h-9 w-9 place-items-center rounded-xl bg-slate-100 text-xs font-bold text-slate-600">{{ e.currency }}</span>
              <div class="text-right">
                <p class="tabular text-sm font-semibold" :class="e.net < 0 ? 'text-rose-600' : 'text-slate-800'">{{ formatMinor(e.net, e.currency) }}</p>
                <p class="tabular text-xs text-slate-400">{{ e.rate_known ? formatMoney(e.net_in_base) : 'no rate' }}</p>
              </div>
            </li>
          </ul>
          <p v-if="netWorth.missing_rates.length" class="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">
            No rate for {{ netWorth.missing_rates.join(', ') }}
          </p>
        </section>
      </div>

      <!-- cash flow -->
      <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
        <div class="mb-5 flex items-center justify-between">
          <h2 class="text-base font-semibold text-slate-900">Cash flow</h2>
          <div class="flex gap-4 text-xs text-slate-500">
            <span class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-emerald-400" /> Income</span>
            <span class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-rose-400" /> Expense</span>
          </div>
        </div>
        <div v-if="!cashFlow.months.length" class="py-6 text-center text-sm text-slate-400">No data yet.</div>
        <div class="space-y-5">
          <div v-for="m in cashFlow.months" :key="m.month">
            <div class="mb-2 flex justify-between text-sm">
              <span class="font-medium text-slate-700">{{ m.month }}</span>
              <span class="tabular font-semibold" :class="m.net.amount < 0 ? 'text-rose-600' : 'text-emerald-600'">{{ formatMoney(m.net) }}</span>
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
    </template>
  </div>
</template>
