<script setup>
import { ref, computed, onMounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { reportsApi } from '../api/reports'
import { formatMoney } from '../lib/format'
import AccountForm from '../components/AccountForm.vue'

const accounts = ref([])
const base = ref('UZS')
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref(null)

const assets = computed(() => accounts.value.filter((a) => a.kind === 'asset'))
const liabilities = computed(() => accounts.value.filter((a) => a.kind === 'liability'))

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
    error.value = e.response?.data?.error || e.message
  } finally {
    loading.value = false
  }
}

function openNew() {
  editing.value = null
  formOpen.value = true
}
function openEdit(a) {
  editing.value = a
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(a) {
  if (!confirm(`Delete account "${a.name}"?`)) return
  try {
    await accountsApi.remove(a.id)
    load()
  } catch (e) {
    alert(e.response?.data?.error || e.message)
  }
}

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
      <button class="btn btn-primary" @click="openNew">+ New account</button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <section
        v-for="group in [
          { title: 'Assets', items: assets, dot: 'bg-emerald-400' },
          { title: 'Liabilities', items: liabilities, dot: 'bg-rose-400' },
        ]"
        :key="group.title"
      >
        <div class="mb-3 flex items-center gap-2">
          <span class="h-2.5 w-2.5 rounded-full" :class="group.dot" />
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">{{ group.title }}</h2>
        </div>

        <div class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-slate-200/70">
          <ul class="divide-y divide-slate-100">
            <li v-for="a in group.items" :key="a.id" class="group flex items-center gap-4 px-5 py-4 transition hover:bg-slate-50">
              <span class="grid h-11 w-11 place-items-center rounded-2xl bg-slate-100 text-lg">{{ typeIcon[a.type] }}</span>
              <div class="min-w-0">
                <p class="truncate font-semibold text-slate-800">{{ a.name }}</p>
                <p class="text-xs text-slate-400">{{ typeLabel[a.type] || a.type }} · {{ a.currency }}</p>
              </div>
              <p class="tabular ml-auto text-right text-lg font-semibold" :class="a.balance.amount < 0 ? 'text-rose-600' : 'text-slate-900'">
                {{ formatMoney(a.balance) }}
              </p>
              <div class="flex gap-1 opacity-0 transition group-hover:opacity-100">
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-200 hover:text-slate-700" title="Edit" @click="openEdit(a)">✎</button>
                <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click="remove(a)">🗑</button>
              </div>
            </li>
            <li v-if="!group.items.length" class="px-5 py-6 text-center text-sm text-slate-400">None yet</li>
          </ul>
        </div>
      </section>
    </template>

    <AccountForm v-if="formOpen" :account="editing" :base="base" @close="formOpen = false" @saved="onSaved" />
  </div>
</template>
