<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import type { Account } from '../api/types'
import { formatMinor } from '../lib/format'
import AccountForm from '../components/AccountForm.vue'
import LoanScheduleModal from '../components/LoanScheduleModal.vue'
import ReconciliationPanel from '../components/ReconciliationPanel.vue'

const accounts = ref<Account[]>([])
const base = ref('UZS')
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<Account | null>(null)
const scheduleFor = ref<Account | null>(null)

const assets = computed(() => accounts.value.filter((a) => a.kind === 'asset'))
const liabilities = computed(() => accounts.value.filter((a) => a.kind === 'liability'))

// For a credit card, balance is what you owe; available = limit − owed.
function availableCredit(a: Account): number {
  return (a.credit_limit ?? 0) - a.balance.amount
}

const typeLabel = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
}
const typeIcon = { cash: '💵', debit_card: '💳', deposit: '🏦', credit_card: '💳', loan: '🏠' }

async function load() {
  loading.value = true
  try {
    accounts.value = await accountsApi.list()
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
function openEdit(a: Account) {
  editing.value = a
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(a: Account) {
  if (!confirm(`Delete account "${a.name}"?`)) return
  try {
    await accountsApi.remove(a.id)
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
  <div class="space-y-8">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Accounts</h1>
        <p class="text-sm text-slate-500">Balances are derived from your transactions</p>
      </div>
      <button class="btn btn-primary shrink-0" @click="openNew">+ New<span class="hidden sm:inline"> account</span></button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <section
        v-for="group in [
          { title: 'Assets', items: assets, dot: 'bg-emerald-400', tile: 'bg-emerald-50' },
          { title: 'Liabilities', items: liabilities, dot: 'bg-rose-400', tile: 'bg-rose-50' },
        ]"
        :key="group.title"
      >
        <div class="mb-3 flex items-center gap-2">
          <span class="h-2.5 w-2.5 rounded-full" :class="group.dot" />
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">{{ group.title }}</h2>
        </div>

        <div class="card overflow-hidden">
          <ul class="divide-y divide-slate-100">
            <li v-for="a in group.items" :key="a.id" class="group flex items-center gap-4 px-4 py-4 transition hover:bg-slate-50 sm:px-5">
              <span class="grid h-11 w-11 place-items-center rounded-2xl text-lg" :class="group.tile">{{ typeIcon[a.type] }}</span>
              <div class="min-w-0">
                <p class="truncate font-semibold text-slate-800">{{ a.name }}</p>
                <p class="text-xs text-slate-400">{{ typeLabel[a.type] || a.type }} · {{ a.currency }}</p>
              </div>
              <div class="ml-auto text-right">
                <p class="tabular text-lg font-semibold" :class="a.balance.isNegative() ? 'text-rose-600' : 'text-slate-900'">
                  {{ a.balance.format() }}
                </p>
                <p v-if="a.type === 'credit_card' && a.credit_limit != null" class="tabular text-sm font-medium text-slate-500">
                  {{ formatMinor(availableCredit(a), a.currency) }}
                </p>
              </div>
              <div class="flex gap-1 opacity-100 transition can-hover:opacity-0 can-hover:group-hover:opacity-100">
                <button v-if="a.type === 'loan'" class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-amber-100 hover:text-amber-700" title="Amortization schedule" @click="scheduleFor = a">📅</button>
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-200 hover:text-slate-700" title="Edit" @click="openEdit(a)">✎</button>
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click="remove(a)">🗑</button>
              </div>
            </li>
            <li v-if="!group.items.length" class="px-5 py-6 text-center text-sm text-slate-400">None yet</li>
          </ul>
        </div>
      </section>

      <ReconciliationPanel />
    </template>

    <AccountForm v-if="formOpen" :account="editing" :base="base" @close="formOpen = false" @saved="onSaved" />
    <LoanScheduleModal v-if="scheduleFor" :account="scheduleFor" @close="scheduleFor = null" />
  </div>
</template>
