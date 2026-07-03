<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { transactionsApi, type TransactionFilter } from '../api/transactions'
import { configApi } from '../api/config'
import { errMessage } from '../api/client'
import { useTransactionsQuery, useAccountsQuery, useCategoriesQuery } from '../api/queries'
import { confirm } from '../lib/confirm'
import type { Category, Transaction, TransactionType } from '../api/types'
import { formatDateTime } from '../lib/format'
import TransactionForm from '../components/TransactionForm.vue'
import TransactionDetail from '../components/TransactionDetail.vue'
import ReceiptDetail from '../components/ReceiptDetail.vue'
import CategoryIcon from '../components/CategoryIcon.vue'
import SwipeRow from '../components/SwipeRow.vue'

const accQuery = useAccountsQuery(true)
const catQuery = useCategoriesQuery({ include_archived: true })
// Debounced filter that drives the transactions query key; changing it refetches.
const queryFilter = ref<TransactionFilter>({ limit: 200 })
const txQuery = useTransactionsQuery(queryFilter)

const transactions = computed(() => txQuery.data.value ?? [])
const accounts = computed(() => accQuery.data.value ?? [])
const categories = computed(() => catQuery.data.value ?? [])
const base = ref('UZS')
const noteThreshold = ref(Number.POSITIVE_INFINITY)
const names = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const a of accounts.value) m[a.id] = a.name
  for (const c of categories.value) m[c.id] = c.name
  return m
})
const loading = computed(() => txQuery.isLoading.value)
const error = computed(() => (txQuery.error.value ? errMessage(txQuery.error.value) : ''))
const formOpen = ref(false)
const editing = ref<Transaction | null>(null)
const viewing = ref<Transaction | null>(null)
const receiptViewingId = ref<string | null>(null)

function openDetail(t: Transaction) {
  viewing.value = t
}

// Open the linked receipt from the transaction detail modal (swap modals).
function viewReceipt(id: string) {
  viewing.value = null
  receiptViewingId.value = id
}

const filters = reactive({
  q: '',
  type: '' as '' | TransactionType,
  accountId: '',
  categoryId: '',
  dateFrom: '',
  dateTo: '',
  tag: '',
})

const activeFilterCount = computed(() =>
  [filters.q, filters.type, filters.accountId, filters.categoryId, filters.dateFrom, filters.dateTo, filters.tag].filter(Boolean).length,
)
const hasActiveFilters = computed(() => activeFilterCount.value > 0)

// Filters live behind a disclosure; open it when any filter is already active.
const showFilters = ref(false)
watch(hasActiveFilters, (active) => {
  if (active) showFilters.value = true
})

const meta = {
  expense: { icon: 'arrow-up-right', ring: 'bg-rose-50 text-rose-600', sign: '−', amount: 'text-rose-600' },
  income: { icon: 'arrow-down-left', ring: 'bg-emerald-50 text-emerald-600', sign: '+', amount: 'text-emerald-600' },
  transfer: { icon: 'transfer', ring: 'bg-indigo-50 text-indigo-600', sign: '', amount: 'text-slate-800' },
}

const catMap = computed<Record<string, Category>>(() => {
  const m: Record<string, Category> = {}
  for (const c of categories.value) m[c.id] = c
  return m
})

// Category for a transaction, but only when it carries a renderable icon —
// otherwise the row falls back to the type arrow (transfers have no category).
function txCategory(t: Transaction): Category | undefined {
  const c = t.category_id ? catMap.value[t.category_id] : undefined
  return c && c.icon ? c : undefined
}

// Pair each transaction with its display category up front.
const rows = computed(() => transactions.value.map((t) => ({ t, cat: txCategory(t) })))

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

const exporting = ref(false)
async function exportXlsx() {
  if (exporting.value) return
  exporting.value = true
  try {
    const { limit, ...filter } = buildFilter()
    void limit
    await transactionsApi.exportXlsx(filter)
  } catch (e) {
    alert(errMessage(e))
  } finally {
    exporting.value = false
  }
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

function loadTransactions() {
  txQuery.refetch()
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
async function remove(t: Transaction): Promise<boolean> {
  if (!(await confirm({ title: 'Delete transaction?', message: 'This transaction will be deleted.' }))) return false
  try {
    await transactionsApi.remove(t.id)
    loadTransactions()
    return true
  } catch (e) {
    alert(errMessage(e))
    return false
  }
}

function detailEdit() {
  const t = viewing.value
  viewing.value = null
  if (t) openEdit(t)
}
async function detailRemove() {
  const t = viewing.value
  if (t && (await remove(t))) viewing.value = null
}

let timer: number | undefined
watch(
  filters,
  () => {
    clearTimeout(timer)
    timer = window.setTimeout(() => {
      queryFilter.value = buildFilter()
    }, 250)
  },
  { deep: true },
)

onMounted(async () => {
  try {
    const cfg = await configApi.get()
    base.value = cfg.base_currency
    noteThreshold.value = cfg.note_required_above
  } catch {
    /* keep defaults */
  }
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Transactions</h1>
        <p class="text-sm text-slate-500">{{ transactions.length }} result{{ transactions.length === 1 ? '' : 's' }}</p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button class="btn btn-soft" :disabled="exporting" title="Export to Excel" @click="exportXlsx">
          <i class="ti ti-file-spreadsheet" />
          <span class="hidden sm:inline"> {{ exporting ? 'Exporting…' : 'Export' }}</span>
        </button>
        <button class="btn btn-primary" @click="openNew">+ New<span class="hidden sm:inline"> transaction</span></button>
      </div>
    </div>

    <!-- filter bar -->
    <div class="card p-4">
      <button
        type="button"
        class="flex w-full items-center gap-1.5 text-sm font-medium text-slate-500 transition hover:text-slate-700"
        @click="showFilters = !showFilters"
      >
        <svg class="h-4 w-4 transition-transform" :class="showFilters ? 'rotate-90' : ''" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
        </svg>
        Search
        <span v-if="activeFilterCount" class="ml-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">{{ activeFilterCount }} active</span>
      </button>

      <div v-show="showFilters" class="mt-3">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="relative lg:col-span-2">
            <i class="ti ti-search pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-slate-400" />
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
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>

    <div class="card overflow-hidden">
      <ul class="divide-y divide-slate-100">
        <li v-for="{ t, cat } in rows" :key="t.id">
          <!-- mobile: swipe right = edit, swipe left = delete -->
          <SwipeRow @swipe-right="openEdit(t)" @swipe-left="remove(t)">
            <div class="group flex cursor-pointer items-center gap-3 px-4 py-3.5 transition hover:bg-slate-50 sm:gap-4 sm:px-5" @click="openDetail(t)">
              <span
                v-if="cat"
                class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-lg"
                :class="cat.color ? '' : 'bg-slate-100'"
                :style="cat.color ? { backgroundColor: cat.color + '22' } : undefined"
              >
                <CategoryIcon :icon="cat.icon" :color="cat.color" :size="20" />
              </span>
              <span v-else class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-lg" :class="meta[t.type].ring"><i :class="`ti ti-${meta[t.type].icon}`" /></span>
              <div class="min-w-0">
                <p class="flex items-center gap-1.5 font-medium text-slate-800">
                  <span class="truncate">{{ title(t) }}</span>
                  <i v-if="t.receipt_id" class="ti ti-qrcode shrink-0 text-sm text-emerald-500" title="Has a linked receipt" />
                </p>
                <p class="truncate text-xs text-slate-400">{{ subtitle(t) }}<span v-if="t.note"> · {{ t.note }}</span></p>
              </div>
              <div class="ml-auto shrink-0 text-right">
                <p class="tabular font-semibold text-sm sm:text-base" :class="meta[t.type].amount">{{ meta[t.type].sign }}{{ t.amount.format() }}</p>
                <p class="text-xs text-slate-400">{{ formatDateTime(t.date) }}</p>
              </div>
              <!-- desktop only: hover reveals buttons; mobile uses swipe -->
              <div class="hidden shrink-0 can-hover:flex can-hover:gap-1 can-hover:opacity-0 can-hover:transition can-hover:group-hover:opacity-100">
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-200 hover:text-slate-700" title="Edit" @click.stop="openEdit(t)"><i class="ti ti-pencil text-base" /></button>
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click.stop="remove(t)"><i class="ti ti-trash text-base" /></button>
              </div>
            </div>
          </SwipeRow>
        </li>
        <li v-if="loading" class="px-5 py-8 text-center text-sm text-slate-400">Loading…</li>
        <li v-else-if="!transactions.length" class="px-5 py-8 text-center text-sm text-slate-400">
          {{ hasActiveFilters ? 'No transactions match these filters.' : 'No transactions yet.' }}
        </li>
      </ul>
    </div>

    <TransactionDetail
      v-if="viewing"
      :transaction="viewing"
      :names="names"
      :base="base"
      @close="viewing = null"
      @edit="detailEdit"
      @remove="detailRemove"
      @view-receipt="viewReceipt"
    />

    <ReceiptDetail
      v-if="receiptViewingId"
      :id="receiptViewingId"
      @close="receiptViewingId = null"
      @changed="loadTransactions"
    />

    <TransactionForm
      v-if="formOpen"
      :transaction="editing"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      :note-threshold="noteThreshold"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
