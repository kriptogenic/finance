<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { configApi } from '../api/config'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type { Account, Category } from '../api/types'
import { Money } from '../api/money'
import { toMajor } from '../lib/format'
import SwipeRow from '../components/SwipeRow.vue'
import AccountForm from '../components/AccountForm.vue'
import TransactionForm from '../components/TransactionForm.vue'
import LoanScheduleModal from '../components/LoanScheduleModal.vue'
import ReconciliationPanel from '../components/ReconciliationPanel.vue'

const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const base = ref('UZS')
const noteThreshold = ref(Number.POSITIVE_INFINITY)
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<Account | null>(null)
const scheduleFor = ref<Account | null>(null)
const charging = ref<Account | null>(null)

// receivables (people who owe you) live in their own group, separate from
// your real assets.
const assets = computed(() => accounts.value.filter((a) => a.kind === 'asset' && a.type !== 'receivable'))
const liabilities = computed(() => accounts.value.filter((a) => a.kind === 'liability'))
const people = computed(() => accounts.value.filter((a) => a.type === 'receivable'))

// prefill the transaction form to collect a friend's full outstanding balance
const chargePrefill = computed(() =>
  charging.value
    ? { type: 'transfer' as const, fromId: charging.value.id, amount: toMajor(charging.value.balance.amount, charging.value.currency) }
    : null,
)

// For a credit card, balance is what you owe; available = limit − owed.
function availableCredit(a: Account): Money {
  return (a.credit_limit ?? Money.of(0, a.currency)).sub(a.balance)
}

const typeLabel = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
  receivable: 'Person',
}
const typeIcon = { cash: 'cash', debit_card: 'credit-card', deposit: 'building-bank', credit_card: 'credit-card', loan: 'home', receivable: 'users' }

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

function onCharged() {
  charging.value = null
  load()
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
  if (!(await confirm({ title: 'Delete account?', message: `"${a.name}" will be deleted.` }))) return
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
    const cfg = await configApi.get()
    base.value = cfg.base_currency
    noteThreshold.value = cfg.note_required_above
  } catch {
    /* keep defaults */
  }
  try {
    categories.value = await categoriesApi.list()
  } catch {
    /* full-charge is a transfer; categories are optional */
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
            <li v-for="a in group.items" :key="a.id">
              <!-- mobile: tap = edit, swipe right = edit, swipe left = delete -->
              <SwipeRow @swipe-right="openEdit(a)" @swipe-left="remove(a)">
                <div class="group flex cursor-pointer items-center gap-4 px-4 py-4 transition hover:bg-slate-50 sm:px-5" @click="openEdit(a)">
                  <span class="grid h-11 w-11 place-items-center rounded-2xl text-xal text-slate-600" :class="group.tile"><i :class="`ti ti-${typeIcon[a.type]}`" /></span>
                  <div class="min-w-0">
                    <p class="truncate font-semibold text-slate-800">{{ a.name }}</p>
                    <p class="text-xs text-slate-400">{{ typeLabel[a.type] || a.type }}</p>
                  </div>
                  <div class="ml-auto text-right">
                    <p class="tabular text-lg font-semibold" :class="a.balance.isNegative() ? 'text-rose-600' : 'text-slate-900'">
                      {{ a.balance.format() }}
                    </p>
                    <p v-if="a.type === 'credit_card' && a.credit_limit != null" class="tabular text-sm font-medium text-slate-500">
                      {{ availableCredit(a).format() }}
                    </p>
                  </div>
                  <div class="flex shrink-0 items-center gap-1">
                    <!-- schedule has no gesture, so it stays reachable on every size -->
                    <button v-if="a.type === 'loan'" class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-amber-100 hover:text-amber-700" title="Payment schedule" @click.stop="scheduleFor = a"><i class="ti ti-calendar text-base" /></button>
                    <!-- desktop only: edit/delete (mobile uses tap + swipe) -->
                    <button class="hidden h-8 w-8 place-items-center rounded-lg text-slate-400 can-hover:grid can-hover:opacity-0 can-hover:transition can-hover:group-hover:opacity-100 hover:bg-slate-200 hover:text-slate-700" title="Edit" @click.stop="openEdit(a)"><i class="ti ti-pencil text-base" /></button>
                    <button class="hidden h-8 w-8 place-items-center rounded-lg text-slate-400 can-hover:grid can-hover:opacity-0 can-hover:transition can-hover:group-hover:opacity-100 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click.stop="remove(a)"><i class="ti ti-trash text-base" /></button>
                  </div>
                </div>
              </SwipeRow>
            </li>
            <li v-if="!group.items.length" class="px-5 py-6 text-center text-sm text-slate-400">None yet</li>
          </ul>
        </div>
      </section>

      <!-- people who owe you (temporary split accounts) -->
      <section v-if="people.length">
        <div class="mb-3 flex items-center gap-2">
          <span class="h-2.5 w-2.5 rounded-full bg-sky-400" />
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">Owed to you</h2>
        </div>

        <div class="card overflow-hidden">
          <ul class="divide-y divide-slate-100">
            <li v-for="a in people" :key="a.id" class="flex items-center gap-4 px-4 py-4 sm:px-5">
              <span class="grid h-11 w-11 place-items-center rounded-2xl bg-sky-50 text-slate-600"><i class="ti ti-users" /></span>
              <div class="min-w-0">
                <p class="truncate font-semibold text-slate-800">{{ a.name }}</p>
                <p class="text-xs text-slate-400">owes you</p>
              </div>
              <p class="ml-auto tabular text-lg font-semibold text-slate-900">{{ a.balance.format() }}</p>
              <button class="btn btn-soft shrink-0" title="Record full repayment" @click="charging = a">Full charge</button>
            </li>
          </ul>
        </div>
      </section>

      <ReconciliationPanel />
    </template>

    <AccountForm v-if="formOpen" :account="editing" :base="base" @close="formOpen = false" @saved="onSaved" />
    <TransactionForm
      v-if="charging"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      :note-threshold="noteThreshold"
      :prefill="chargePrefill"
      @close="charging = null"
      @saved="onCharged"
    />
    <LoanScheduleModal v-if="scheduleFor" :account="scheduleFor" @close="scheduleFor = null" />
  </div>
</template>
