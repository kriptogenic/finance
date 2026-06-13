<script setup>
import { ref, onMounted } from 'vue'
import { transactionsApi } from '../api/transactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { formatMoney, formatDate } from '../lib/format'

const transactions = ref([])
const names = ref({})
const loading = ref(true)
const error = ref('')

const meta = {
  expense: { icon: '↗', ring: 'bg-rose-50 text-rose-600', sign: '−', amount: 'text-rose-600' },
  income: { icon: '↙', ring: 'bg-emerald-50 text-emerald-600', sign: '+', amount: 'text-emerald-600' },
  transfer: { icon: '⇄', ring: 'bg-indigo-50 text-indigo-600', sign: '', amount: 'text-slate-800' },
}

function nameOf(id) {
  return id ? names.value[id] || '—' : '—'
}
function title(t) {
  if (t.type === 'expense') return nameOf(t.category_id)
  if (t.type === 'income') return nameOf(t.category_id)
  return 'Transfer'
}
function subtitle(t) {
  if (t.type === 'expense') return 'from ' + nameOf(t.from_account_id)
  if (t.type === 'income') return 'to ' + nameOf(t.to_account_id)
  return nameOf(t.from_account_id) + ' → ' + nameOf(t.to_account_id)
}

onMounted(async () => {
  try {
    const [txns, accounts, categories] = await Promise.all([
      transactionsApi.list({ limit: 100 }),
      accountsApi.list(true),
      categoriesApi.list({ include_archived: true }),
    ])
    const map = {}
    for (const a of accounts) map[a.id] = a.name
    for (const c of categories) map[c.id] = c.name
    names.value = map
    transactions.value = txns
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
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Transactions</h1>
      <p class="text-sm text-slate-500">{{ transactions.length }} recent</p>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-slate-200/70">
      <ul class="divide-y divide-slate-100">
        <li v-for="t in transactions" :key="t.id" class="flex items-center gap-4 px-5 py-3.5 transition hover:bg-slate-50">
          <span class="grid h-10 w-10 place-items-center rounded-full text-lg font-semibold" :class="meta[t.type].ring">{{ meta[t.type].icon }}</span>
          <div class="min-w-0">
            <p class="truncate font-medium text-slate-800">{{ title(t) }}</p>
            <p class="truncate text-xs text-slate-400">{{ subtitle(t) }}<span v-if="t.note"> · {{ t.note }}</span></p>
          </div>
          <div class="ml-auto text-right">
            <p class="tabular font-semibold" :class="meta[t.type].amount">{{ meta[t.type].sign }}{{ formatMoney(t.amount) }}</p>
            <p class="text-xs text-slate-400">{{ formatDate(t.date) }}</p>
          </div>
        </li>
        <li v-if="!transactions.length" class="px-5 py-8 text-center text-sm text-slate-400">No transactions yet.</li>
      </ul>
    </div>
  </div>
</template>
