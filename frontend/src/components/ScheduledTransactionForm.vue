<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import { scheduledTransactionsApi } from '../api/scheduledTransactions'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type {
  Account,
  Category,
  CreateScheduledTransactionRequest,
  ScheduledTransaction,
  ScheduleFrequency,
  TransactionType,
} from '../api/types'
import { toMinor, toMajor } from '../lib/format'
import Modal from './Modal.vue'
import CategoryIcon from './CategoryIcon.vue'
import MoneyInput from './MoneyInput.vue'

const props = withDefaults(
  defineProps<{
    schedule?: ScheduledTransaction | null
    accounts?: Account[]
    categories?: Category[]
    base?: string
  }>(),
  { schedule: null, accounts: () => [], categories: () => [], base: 'UZS' },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.schedule)
const error = ref('')
const saving = ref(false)
const removing = ref(false)

async function del() {
  if (!props.schedule) return
  if (!(await confirm({ title: 'Delete schedule?', message: 'This scheduled transaction will be deleted.' }))) return
  removing.value = true
  error.value = ''
  try {
    await scheduledTransactionsApi.remove(props.schedule.id)
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    removing.value = false
  }
}

const s = props.schedule
const defaultAccount = props.accounts.find((a) => a.currency === props.base) ?? props.accounts[0]

// start_date defaults to today (YYYY-MM-DD) for a new schedule.
const today = new Date().toISOString().slice(0, 10)

const form = reactive({
  name: s?.name ?? '',
  type: (s?.type ?? 'expense') as TransactionType,
  fromId: s?.from_account_id ?? defaultAccount?.id ?? '',
  toId: s?.to_account_id ?? defaultAccount?.id ?? '',
  categoryId: s?.category_id ?? '',
  amount: s ? toMajor(s.amount.amount, s.amount.currency) : ('' as number | string),
  toAmount: s?.to_amount ? toMajor(s.to_amount.amount, s.to_amount.currency) : ('' as number | string),
  rate: s?.rate_to_base ?? '',
  note: s?.note ?? '',
  tags: (s?.tags ?? []).join(', '),
  frequency: (s?.frequency ?? 'monthly') as ScheduleFrequency,
  interval: s?.interval ?? 1,
  startDate: s?.start_date ?? today,
  endDate: s?.end_date ?? '',
  paused: s?.paused ?? false,
})

const types: { v: TransactionType; label: string }[] = [
  { v: 'expense', label: 'Expense' },
  { v: 'income', label: 'Income' },
  { v: 'transfer', label: 'Transfer' },
]

const frequencies: { v: ScheduleFrequency; label: string }[] = [
  { v: 'daily', label: 'day(s)' },
  { v: 'weekly', label: 'week(s)' },
  { v: 'monthly', label: 'month(s)' },
  { v: 'yearly', label: 'year(s)' },
]

const acctIcons: Record<string, string> = {
  cash: 'cash',
  debit_card: 'credit-card',
  deposit: 'building-bank',
  credit_card: 'credit-card',
  loan: 'home',
}
function acctIcon(a: Account): string {
  return acctIcons[a.type] ?? 'credit-card'
}

const fromAcc = computed(() => props.accounts.find((a) => a.id === form.fromId))
const toAcc = computed(() => props.accounts.find((a) => a.id === form.toId))
const primaryCurrency = computed(() =>
  form.type === 'income' ? toAcc.value?.currency : fromAcc.value?.currency,
)
const amountCurrency = computed(() => primaryCurrency.value ?? props.base)
const isCross = computed(
  () => form.type === 'transfer' && !!fromAcc.value && !!toAcc.value && fromAcc.value.currency !== toAcc.value.currency,
)
const needsRate = computed(() => !!primaryCurrency.value && primaryCurrency.value !== props.base)
const categoryOptions = computed(() => props.categories.filter((c) => c.type === form.type && !c.archived))

const showDetails = ref(!!props.schedule)
watch([isCross, needsRate], ([cross, rate]) => {
  if (cross || rate) showDetails.value = true
})
const detailsHint = computed(() => (isCross.value || needsRate.value ? 'exchange rate required' : ''))

async function submit() {
  error.value = ''
  if (form.type !== 'transfer' && !form.categoryId) {
    error.value = 'Please choose a category.'
    return
  }
  saving.value = true
  try {
    const payload: CreateScheduledTransactionRequest = {
      type: form.type,
      amount: toMinor(form.amount, primaryCurrency.value ?? props.base),
      frequency: form.frequency,
      interval: Number(form.interval) || 1,
      start_date: form.startDate,
      paused: form.paused,
    }
    if (form.name) payload.name = form.name
    if (form.type !== 'income') payload.from_account_id = form.fromId
    if (form.type !== 'expense') payload.to_account_id = form.toId
    if (form.type !== 'transfer') payload.category_id = form.categoryId
    if (isCross.value) payload.to_amount = toMinor(form.toAmount, toAcc.value?.currency ?? props.base)
    if (needsRate.value && form.rate !== '') payload.rate_to_base = String(form.rate)
    if (form.note) payload.note = form.note
    if (form.endDate) payload.end_date = form.endDate
    const tags = form.tags.split(',').map((x) => x.trim()).filter(Boolean)
    if (tags.length) payload.tags = tags

    if (props.schedule) await scheduledTransactionsApi.update(props.schedule.id, payload)
    else await scheduledTransactionsApi.create(payload)
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal :title="editing ? 'Edit schedule' : 'New schedule'" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="submit">
      <!-- name -->
      <div>
        <label class="lbl">Name (optional)</label>
        <input v-model="form.name" class="field" placeholder="e.g. Rent" />
      </div>

      <!-- type -->
      <div class="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1">
        <button
          v-for="opt in types"
          :key="opt.v"
          type="button"
          class="rounded-lg py-1.5 text-sm font-medium transition"
          :class="form.type === opt.v ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'"
          @click="form.type = opt.v"
        >
          {{ opt.label }}
        </button>
      </div>

      <!-- amount -->
      <div>
        <label class="lbl">Amount</label>
        <div class="relative">
          <MoneyInput
            v-model="form.amount"
            :currency="amountCurrency"
            class="field tabular !py-3 !pr-20 text-2xl font-semibold"
            required
            placeholder="0,00"
          />
          <span class="absolute top-1/2 right-3 -translate-y-1/2 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500">
            {{ amountCurrency }}
          </span>
        </div>
      </div>

      <!-- recurrence -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="lbl">Repeat every</label>
          <div class="flex gap-2">
            <input v-model.number="form.interval" type="number" min="1" class="field w-20" required />
            <select v-model="form.frequency" class="field">
              <option v-for="f in frequencies" :key="f.v" :value="f.v">{{ f.label }}</option>
            </select>
          </div>
        </div>
        <div>
          <label class="lbl">Starts</label>
          <input v-model="form.startDate" type="date" class="field" required />
        </div>
      </div>

      <!-- category (expense / income) -->
      <div v-if="form.type !== 'transfer'">
        <label class="lbl">Category</label>
        <p v-if="!categoryOptions.length" class="text-sm text-slate-400">No {{ form.type }} categories yet.</p>
        <div v-else class="grid grid-cols-4 gap-2 sm:grid-cols-5">
          <button
            v-for="c in categoryOptions"
            :key="c.id"
            type="button"
            class="flex flex-col items-center gap-1 rounded-xl border p-2 text-center transition"
            :class="form.categoryId === c.id ? 'border-amber-400 bg-amber-50 text-amber-800' : 'border-slate-200 text-slate-600 hover:border-slate-300 hover:bg-slate-50'"
            @click="form.categoryId = c.id"
          >
            <CategoryIcon :icon="c.icon" :color="c.color" :size="22" />
            <span class="w-full truncate text-[11px] leading-tight">{{ c.name }}</span>
          </button>
        </div>
      </div>

      <!-- from account (expense / transfer) -->
      <div v-if="form.type !== 'income'">
        <label class="lbl">From account</label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="a in accounts"
            :key="a.id"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm font-medium transition"
            :class="form.fromId === a.id ? 'border-amber-400 bg-amber-50 text-amber-800' : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'"
            @click="form.fromId = a.id"
          >
            <i :class="`ti ti-${acctIcon(a)}`" />{{ a.name }}
            <span class="text-xs opacity-60">{{ a.currency }}</span>
          </button>
        </div>
      </div>

      <!-- to account (income / transfer) -->
      <div v-if="form.type !== 'expense'">
        <label class="lbl">To account</label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="a in accounts"
            :key="a.id"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm font-medium transition"
            :class="form.toId === a.id ? 'border-amber-400 bg-amber-50 text-amber-800' : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'"
            @click="form.toId = a.id"
          >
            <i :class="`ti ti-${acctIcon(a)}`" />{{ a.name }}
            <span class="text-xs opacity-60">{{ a.currency }}</span>
          </button>
        </div>
      </div>

      <!-- more details -->
      <div class="border-t border-slate-100 pt-3">
        <button
          type="button"
          class="flex w-full items-center gap-1.5 text-sm font-medium text-slate-500 transition hover:text-slate-700"
          @click="showDetails = !showDetails"
        >
          <svg
            class="h-4 w-4 transition-transform"
            :class="showDetails ? 'rotate-90' : ''"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
          </svg>
          More details
          <span v-if="detailsHint" class="ml-1 rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-600">{{ detailsHint }}</span>
        </button>

        <div v-show="showDetails" class="mt-3 space-y-3">
          <div v-if="isCross">
            <label class="lbl">Received ({{ toAcc?.currency }})</label>
            <MoneyInput v-model="form.toAmount" :currency="toAcc?.currency" class="field" required placeholder="0,00" />
          </div>

          <div v-if="needsRate">
            <label class="lbl">Rate {{ primaryCurrency }} → {{ base }} (1 minor unit)</label>
            <input v-model="form.rate" class="field" required placeholder="e.g. 12500" />
          </div>

          <div>
            <label class="lbl">End date (optional)</label>
            <input v-model="form.endDate" type="date" class="field" />
          </div>
          <div>
            <label class="lbl">Note</label>
            <input v-model="form.note" class="field" />
          </div>
          <div>
            <label class="lbl">Tags (comma-separated)</label>
            <input v-model="form.tags" class="field" placeholder="rent, monthly" />
          </div>
          <label class="flex items-center gap-2">
            <input v-model="form.paused" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
            <span class="text-sm text-slate-600">Paused (excluded from forecast)</span>
          </label>
        </div>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex items-center gap-2 pt-1">
        <button v-if="editing" type="button" class="btn btn-danger" :disabled="saving || removing" @click="del">Delete</button>
        <button type="button" class="btn btn-soft ml-auto" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || removing">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>
