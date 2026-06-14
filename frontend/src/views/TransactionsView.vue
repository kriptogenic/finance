<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { transactionsApi, type TransactionFilter } from '../api/transactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import type { Account, Category, Transaction, TransactionType } from '../api/types'
import { formatDate } from '../lib/format'
import TransactionForm from '../components/TransactionForm.vue'

const transactions = ref<Transaction[]>([])
const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const base = ref('UZS')
const names = ref<Record<string, string>>({})
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<Transaction | null>(null)

const filters = reactive({
  q: '',
  type: '' as '' | TransactionType,
  accountId: '',
  categoryId: '',
  dateFrom: '',
  dateTo: '',
  tag: '',
})

const hasActiveFilters = computed(() =>
  !!(filters.q || filters.type || filters.accountId || filters.categoryId || filters.dateFrom || filters.dateTo || filters.tag),
)

const meta = {
  expense: { icon: '↗', ring: 'bg-rose-50 text-rose-600', sign: '−', amount: 'text-rose-600' },
  income: { icon: '↙', ring: 'bg-emerald-50 text-emerald-600', sign: '+', amount: 'text-emerald-600' },
  transfer: { icon: '⇄', ring: 'bg-indigo-50 text-indigo-600', sign: '', amount: 'text-slate-800' },
}

function nameOf(id: string | null | undefined): string {
  return id ? names.value[id] || '—' : '—'
}
function title(t: Transaction): string {
  return t.type === 'transfer' ? 'Transfer' : nameOf(t.category_id)
}
function subtitle(t: Transaction): string {
  if (t.type === 'expense') return 'from ' + nameOf(t.from_account_id)
  if (t.type === 'income') return 'to ' + nameOf(t.to_account_id)
  return nameOf(t.from_account_id) + ' → ' + nameOf(t.to_account_id)
}

function buildFilter(): TransactionFilter {
  const f: TransactionFilter = { limit: 200 }
  if (filters.q) f.q = filters.q
  if (filters.type) f.type = filters.type
  if (filters.accountId) f.account_id = filters.accountId
  if (filters.categoryId) f.category_id = filters.categoryId
  if (filters.dateFrom) f.date_from = new Date(filters.dateFrom + 'T00:00:00').toISOString()
  if (filters.dateTo) f.date_to = new Date(filters.dateTo + 'T23:59:59.999').toISOString()
  if (filters.tag) f.tag = filters.tag
  return f
}

function clearFilters() {
  filters.q = ''
  filters.type = ''
  filters.accountId = ''
  filters.categoryId = ''
  filters.dateFrom = ''
  filters.dateTo = ''
  filters.tag = ''
}

async function loadMeta() {
  const [accs, cats] = await Promise.all([accountsApi.list(true), categoriesApi.list({ include_archived: true })])
  const map: Record<string, string> = {}
  for (const a of accs) map[a.id] = a.name
  for (const c of cats) map[c.id] = c.name
  names.value = map
  accounts.value = accs
  categories.value = cats
}

async function loadTransactions() {
  loading.value = true
  error.value = ''
  try {
    transactions.value = await transactionsApi.list(buildFilter())
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
function openEdit(t: Transaction) {
  editing.value = t
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  loadTransactions()
}
async function remove(t: Transaction) {
  if (!confirm('Delete this transaction?')) return
  try {
    await transactionsApi.remove(t.id)
    loadTransactions()
  } catch (e) {
    alert(errMessage(e))
  }
}

let timer: number | undefined
watch(
  filters,
  () => {
    clearTimeout(timer)
    timer = window.setTimeout(loadTransactions, 250)
  },
  { deep: true },
)

onMounted(async () => {
  try {
    base.value = (await reportsApi.netWorth()).base
  } catch {
    /* keep default */
  }
  await loadMeta()
  await loadTransactions()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Transactions</h1>
        <p class="text-sm text-slate-500">{{ transactions.length }} result{{ transactions.length === 1 ? '' : 's' }}</p>
      </div>
      <button class="btn btn-primary" @click="openNew">+ New transaction</button>
    </div>

    <!-- filter bar -->
    <div class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200/70">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="relative lg:col-span-2">
          <span class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-slate-400">🔍</span>
          <input v-model="filters.q" class="field pl-9" placeholder="Search notes…" />
        </div>
        <select v-model="filters.type" class="field">
          <option value="">All types</option>
          <option value="expense">Expense</option>
          <option value="income">Income</option>
          <option value="transfer">Transfer</option>
        </select>
        <select v-model="filters.accountId" class="field">
          <option value="">All accounts</option>
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</option>
        </select>
        <select v-model="filters.categoryId" class="field">
          <option value="">All categories</option>
          <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.parent_id ? '— ' : '' }}{{ c.name }}</option>
        </select>
        <input v-model="filters.tag" class="field" placeholder="Tag" />
        <input v-model="filters.dateFrom" type="date" class="field" title="From date" />
        <input v-model="filters.dateTo" type="date" class="field" title="To date" />
      </div>
      <div v-if="hasActiveFilters" class="mt-3 flex justify-end">
        <button class="btn btn-soft" @click="clearFilters">Clear filters</button>
      </div>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>

    <div class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-slate-200/70">
      <ul class="divide-y divide-slate-100">
        <li v-for="t in transactions" :key="t.id" class="group flex items-center gap-4 px-5 py-3.5 transition hover:bg-slate-50">
          <span class="grid h-10 w-10 place-items-center rounded-full text-lg font-semibold" :class="meta[t.type].ring">{{ meta[t.type].icon }}</span>
          <div class="min-w-0">
            <p class="truncate font-medium text-slate-800">{{ title(t) }}</p>
            <p class="truncate text-xs text-slate-400">{{ subtitle(t) }}<span v-if="t.note"> · {{ t.note }}</span></p>
          </div>
          <div class="ml-auto text-right">
            <p class="tabular font-semibold" :class="meta[t.type].amount">{{ meta[t.type].sign }}{{ t.amount.format() }}</p>
            <p class="text-xs text-slate-400">{{ formatDate(t.date) }}</p>
          </div>
          <div class="flex gap-1 opacity-0 transition group-hover:opacity-100">
            <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-200 hover:text-slate-700" title="Edit" @click="openEdit(t)">✎</button>
            <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click="remove(t)">🗑</button>
          </div>
        </li>
        <li v-if="loading" class="px-5 py-8 text-center text-sm text-slate-400">Loading…</li>
        <li v-else-if="!transactions.length" class="px-5 py-8 text-center text-sm text-slate-400">
          {{ hasActiveFilters ? 'No transactions match these filters.' : 'No transactions yet.' }}
        </li>
      </ul>
    </div>

    <TransactionForm
      v-if="formOpen"
      :transaction="editing"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
