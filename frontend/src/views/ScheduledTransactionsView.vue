<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { scheduledTransactionsApi } from '../api/scheduledTransactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type { Account, Category, ScheduledTransaction } from '../api/types'
import { formatDate } from '../lib/format'
import ScheduledTransactionForm from '../components/ScheduledTransactionForm.vue'

const schedules = ref<ScheduledTransaction[]>([])
const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const base = ref('UZS')
const loading = ref(true)
const error = ref('')
const formOpen = ref(false)
const editing = ref<ScheduledTransaction | null>(null)
const running = ref<string | null>(null)

const accountName = computed(() => Object.fromEntries(accounts.value.map((a) => [a.id, a.name])))
const categoryName = computed(() => Object.fromEntries(categories.value.map((c) => [c.id, c.name])))

const unit: Record<string, string> = { daily: 'day', weekly: 'week', monthly: 'month', yearly: 'year' }

function cadence(s: ScheduledTransaction): string {
  const u = unit[s.frequency] ?? s.frequency
  return s.interval === 1 ? `Every ${u}` : `Every ${s.interval} ${u}s`
}

function subtitle(s: ScheduledTransaction): string {
  if (s.type === 'transfer') {
    return `${accountName.value[s.from_account_id ?? ''] ?? '—'} → ${accountName.value[s.to_account_id ?? ''] ?? '—'}`
  }
  const acc = accountName.value[(s.from_account_id ?? s.to_account_id) ?? ''] ?? '—'
  const cat = categoryName.value[s.category_id ?? ''] ?? '—'
  return `${cat} · ${acc}`
}

const typeChip: Record<string, string> = {
  expense: 'bg-rose-50 text-rose-600',
  income: 'bg-emerald-50 text-emerald-600',
  transfer: 'bg-sky-50 text-sky-600',
}

async function load() {
  loading.value = true
  try {
    const [scs, accs, cats] = await Promise.all([
      scheduledTransactionsApi.list(),
      accountsApi.list(),
      categoriesApi.list({}),
    ])
    schedules.value = scs
    accounts.value = accs
    categories.value = cats
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
function openEdit(s: ScheduledTransaction) {
  editing.value = s
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function runNow(s: ScheduledTransaction) {
  running.value = s.id
  try {
    await scheduledTransactionsApi.run(s.id)
    window.dispatchEvent(new Event('data:refresh'))
    await load()
  } catch (e) {
    alert(errMessage(e))
  } finally {
    running.value = null
  }
}
async function remove(s: ScheduledTransaction) {
  if (!(await confirm({ title: 'Delete schedule?', message: 'This scheduled transaction will be deleted.' }))) return
  try {
    await scheduledTransactionsApi.remove(s.id)
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
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Scheduled</h1>
        <p class="text-sm text-slate-500">Recurring transactions, posted automatically</p>
      </div>
      <button class="btn btn-primary shrink-0" @click="openNew">+ New<span class="hidden sm:inline"> schedule</span></button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <div v-else-if="!schedules.length" class="card p-10 text-center text-sm text-slate-400">
      No schedules yet. Create one to post a recurring transaction automatically.
    </div>

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div v-for="s in schedules" :key="s.id" class="card group p-5">
        <div class="flex items-start justify-between">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <p class="truncate font-semibold text-slate-800">{{ s.name || categoryName[s.category_id ?? ''] || cadence(s) }}</p>
              <span class="rounded-full px-2 py-0.5 text-[11px] font-semibold capitalize" :class="typeChip[s.type]">{{ s.type }}</span>
              <span v-if="s.paused" class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-500">Paused</span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-400">{{ subtitle(s) }}</p>
          </div>
          <div class="flex gap-1 opacity-100 transition can-hover:opacity-0 can-hover:group-hover:opacity-100">
            <button class="text-sm text-slate-400 hover:text-slate-700" title="Edit" @click="openEdit(s)"><i class="ti ti-pencil" /></button>
            <button class="text-sm text-slate-400 hover:text-rose-600" title="Delete" @click="remove(s)"><i class="ti ti-trash" /></button>
          </div>
        </div>

        <div class="mt-4 flex items-baseline justify-between">
          <span class="tabular text-lg font-semibold text-slate-800">{{ s.amount.format() }}</span>
          <span class="text-xs text-slate-400">{{ cadence(s) }}</span>
        </div>

        <div class="mt-3 flex items-center justify-between border-t border-slate-100 pt-3">
          <p class="text-xs text-slate-500">Next: <span class="font-medium text-slate-700">{{ formatDate(s.next_run) }}</span></p>
          <button class="btn btn-soft !py-1 !text-xs" :disabled="running === s.id" @click="runNow(s)">
            {{ running === s.id ? 'Running…' : 'Run now' }}
          </button>
        </div>
      </div>
    </div>

    <ScheduledTransactionForm
      v-if="formOpen"
      :schedule="editing"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
