<script setup>
import { ref, onMounted } from 'vue'
import { transactionsApi } from '../api/transactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { formatMoney, formatDate } from '../lib/format'

const transactions = ref([])
const names = ref({}) // id -> name, for accounts and categories
const loading = ref(true)
const error = ref('')

const typeBadge = {
  expense: 'bg-red-100 text-red-700',
  income: 'bg-emerald-100 text-emerald-700',
  transfer: 'bg-indigo-100 text-indigo-700',
}

function nameOf(id) {
  return id ? names.value[id] || '—' : '—'
}

function counterparty(t) {
  if (t.type === 'expense') return nameOf(t.from_account_id) + ' → ' + nameOf(t.category_id)
  if (t.type === 'income') return nameOf(t.category_id) + ' → ' + nameOf(t.to_account_id)
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
    <h1 class="text-2xl font-semibold">Transactions</h1>

    <p v-if="error" class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <table class="w-full text-sm">
        <thead class="bg-slate-50 text-left text-slate-500">
          <tr>
            <th class="px-4 py-2">Date</th>
            <th class="px-4 py-2">Type</th>
            <th class="px-4 py-2">Details</th>
            <th class="px-4 py-2">Note</th>
            <th class="px-4 py-2 text-right">Amount</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in transactions" :key="t.id" class="border-t border-slate-100">
            <td class="px-4 py-2 whitespace-nowrap text-slate-500">{{ formatDate(t.date) }}</td>
            <td class="px-4 py-2">
              <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="typeBadge[t.type]">{{ t.type }}</span>
            </td>
            <td class="px-4 py-2">{{ counterparty(t) }}</td>
            <td class="px-4 py-2 text-slate-500">{{ t.note || '' }}</td>
            <td class="px-4 py-2 text-right font-medium">{{ formatMoney(t.amount) }}</td>
          </tr>
          <tr v-if="!transactions.length">
            <td colspan="5" class="px-4 py-4 text-center text-slate-400">No transactions yet.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
