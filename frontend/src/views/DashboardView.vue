<script setup>
import { ref, computed, onMounted } from 'vue'
import { reportsApi } from '../api/reports'
import { formatMoney, formatMinor } from '../lib/format'

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
    <h1 class="text-2xl font-semibold">Dashboard</h1>

    <p v-if="error" class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <!-- net worth cards -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="rounded-xl border border-slate-200 bg-white p-5">
          <p class="text-sm text-slate-500">Net worth</p>
          <p class="mt-1 text-2xl font-semibold" :class="netWorth.net_worth.amount < 0 ? 'text-red-600' : 'text-slate-900'">
            {{ formatMoney(netWorth.net_worth) }}
          </p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-white p-5">
          <p class="text-sm text-slate-500">Assets</p>
          <p class="mt-1 text-2xl font-semibold text-emerald-600">{{ formatMoney(netWorth.assets) }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-white p-5">
          <p class="text-sm text-slate-500">Liabilities</p>
          <p class="mt-1 text-2xl font-semibold text-red-600">{{ formatMoney(netWorth.liabilities) }}</p>
        </div>
      </div>

      <!-- currency exposure -->
      <section class="rounded-xl border border-slate-200 bg-white p-5">
        <h2 class="mb-3 text-lg font-medium">Currency exposure</h2>
        <table class="w-full text-sm">
          <thead class="text-left text-slate-500">
            <tr>
              <th class="py-1">Currency</th>
              <th class="py-1 text-right">Net (own)</th>
              <th class="py-1 text-right">Net (base)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in netWorth.by_currency" :key="e.currency" class="border-t border-slate-100">
              <td class="py-1.5 font-medium">{{ e.currency }}</td>
              <td class="py-1.5 text-right">{{ formatMinor(e.net, e.currency) }}</td>
              <td class="py-1.5 text-right text-slate-600">
                {{ e.rate_known ? formatMoney(e.net_in_base) : '—' }}
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="netWorth.missing_rates.length" class="mt-2 text-xs text-amber-600">
          No rate for: {{ netWorth.missing_rates.join(', ') }} (excluded from base totals)
        </p>
      </section>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <!-- spending -->
        <section class="rounded-xl border border-slate-200 bg-white p-5">
          <h2 class="mb-1 text-lg font-medium">Spending by category</h2>
          <p class="mb-3 text-sm text-slate-500">Total {{ formatMoney(spending.total) }}</p>
          <div v-if="!spending.categories.length" class="text-sm text-slate-400">No spending yet.</div>
          <ul class="space-y-3">
            <li v-for="c in spending.categories" :key="c.category_id">
              <div class="flex justify-between text-sm">
                <span>{{ c.category_name }}</span>
                <span class="font-medium">{{ formatMoney(c.amount) }}</span>
              </div>
              <div class="mt-1 h-2 rounded-full bg-slate-100">
                <div class="h-2 rounded-full bg-indigo-500" :style="{ width: (c.amount.amount / spendingMax) * 100 + '%' }" />
              </div>
            </li>
          </ul>
        </section>

        <!-- cash flow -->
        <section class="rounded-xl border border-slate-200 bg-white p-5">
          <h2 class="mb-3 text-lg font-medium">Cash flow</h2>
          <div v-if="!cashFlow.months.length" class="text-sm text-slate-400">No data yet.</div>
          <div class="space-y-4">
            <div v-for="m in cashFlow.months" :key="m.month">
              <div class="flex justify-between text-sm">
                <span class="font-medium">{{ m.month }}</span>
                <span :class="m.net.amount < 0 ? 'text-red-600' : 'text-emerald-600'">{{ formatMoney(m.net) }}</span>
              </div>
              <div class="mt-1 flex gap-1">
                <div class="h-2 rounded-full bg-emerald-400" :style="{ width: (m.income.amount / flowMax) * 100 + '%' }" />
              </div>
              <div class="mt-1 flex gap-1">
                <div class="h-2 rounded-full bg-red-400" :style="{ width: (m.expense.amount / flowMax) * 100 + '%' }" />
              </div>
            </div>
          </div>
          <div class="mt-3 flex gap-4 text-xs text-slate-500">
            <span><span class="inline-block h-2 w-2 rounded-full bg-emerald-400" /> income</span>
            <span><span class="inline-block h-2 w-2 rounded-full bg-red-400" /> expense</span>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>
