<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import { transactionsApi } from '../api/transactions'
import { errMessage } from '../api/client'
import type { Account, Category, CreateTransactionRequest, Transaction, TransactionType } from '../api/types'
import { toMinor, toMajor, toLocalInput } from '../lib/format'
import Modal from './Modal.vue'
import CategoryIcon from './CategoryIcon.vue'

const props = withDefaults(
  defineProps<{
    transaction?: Transaction | null
    accounts?: Account[]
    categories?: Category[]
    base?: string
  }>(),
  { transaction: null, accounts: () => [], categories: () => [], base: 'UZS' },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.transaction)
const error = ref('')
const saving = ref(false)

const t = props.transaction
// Sensible default account: prefer one in the base currency to avoid an
// immediate rate prompt on a brand-new entry.
const defaultAccount = props.accounts.find((a) => a.currency === props.base) ?? props.accounts[0]

const form = reactive({
  type: (t?.type ?? 'expense') as TransactionType,
  date: toLocalInput(t?.date),
  fromId: t?.from_account_id ?? defaultAccount?.id ?? '',
  toId: t?.to_account_id ?? defaultAccount?.id ?? '',
  categoryId: t?.category_id ?? '',
  amount: t ? toMajor(t.amount.amount, t.amount.currency) : ('' as number | string),
  toAmount: t?.to_amount ? toMajor(t.to_amount.amount, t.to_amount.currency) : ('' as number | string),
  rate: t?.rate_to_base ?? '',
  note: t?.note ?? '',
  tags: (t?.tags ?? []).join(', '),
})

const types: { v: TransactionType; label: string }[] = [
  { v: 'expense', label: 'Expense' },
  { v: 'income', label: 'Income' },
  { v: 'transfer', label: 'Transfer' },
]

const acctIcons: Record<string, string> = {
  cash: '💵',
  debit_card: '💳',
  deposit: '🏦',
  credit_card: '💳',
  loan: '🏠',
}
function acctIcon(a: Account): string {
  return acctIcons[a.type] ?? '💳'
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

// Rare fields live behind a disclosure; auto-open it (and keep it open when
// editing) whenever a required advanced field — FX rate or cross-currency
// received amount — actually applies, so it can never be hidden yet mandatory.
const showDetails = ref(!!props.transaction)
watch([isCross, needsRate], ([cross, rate]) => {
  if (cross || rate) showDetails.value = true
})
const detailsHint = computed(() => {
  if (isCross.value || needsRate.value) return 'exchange rate required'
  return ''
})

async function submit() {
  error.value = ''
  saving.value = true
  try {
    const payload: CreateTransactionRequest = {
      type: form.type,
      date: new Date(form.date).toISOString(),
      amount: toMinor(form.amount, primaryCurrency.value ?? props.base),
    }
    if (form.type !== 'income') payload.from_account_id = form.fromId
    if (form.type !== 'expense') payload.to_account_id = form.toId
    if (form.type !== 'transfer') payload.category_id = form.categoryId
    if (isCross.value) payload.to_amount = toMinor(form.toAmount, toAcc.value?.currency ?? props.base)
    if (needsRate.value && form.rate !== '') payload.rate_to_base = String(form.rate)
    if (form.note) payload.note = form.note
    const tags = form.tags.split(',').map((s) => s.trim()).filter(Boolean)
    if (tags.length) payload.tags = tags

    if (props.transaction) await transactionsApi.update(props.transaction.id, payload)
    else await transactionsApi.create(payload)
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal :title="editing ? 'Edit transaction' : 'New transaction'" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="submit">
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
          <input
            v-model="form.amount"
            type="number"
            inputmode="decimal"
            step="any"
            class="field tabular !py-3 !pr-20 text-2xl font-semibold"
            required
            placeholder="0.00"
            autofocus
          />
          <span class="absolute top-1/2 right-3 -translate-y-1/2 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500">
            {{ amountCurrency }}
          </span>
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
            :class="form.categoryId === c.id ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-slate-200 text-slate-600 hover:border-slate-300 hover:bg-slate-50'"
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
            :class="form.fromId === a.id ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'"
            @click="form.fromId = a.id"
          >
            <span>{{ acctIcon(a) }}</span>{{ a.name }}
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
            :class="form.toId === a.id ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'"
            @click="form.toId = a.id"
          >
            <span>{{ acctIcon(a) }}</span>{{ a.name }}
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
          <div>
            <label class="lbl">Date</label>
            <input v-model="form.date" type="datetime-local" class="field" required />
          </div>

          <div v-if="isCross">
            <label class="lbl">Received ({{ toAcc?.currency }})</label>
            <input v-model="form.toAmount" type="number" step="any" class="field" required placeholder="0.00" />
          </div>

          <div v-if="needsRate">
            <label class="lbl">Rate {{ primaryCurrency }} → {{ base }} (1 minor unit)</label>
            <input v-model="form.rate" class="field" required placeholder="e.g. 12500" />
          </div>

          <div>
            <label class="lbl">Note</label>
            <input v-model="form.note" class="field" />
          </div>
          <div>
            <label class="lbl">Tags (comma-separated)</label>
            <input v-model="form.tags" class="field" placeholder="groceries, weekly" />
          </div>
        </div>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex justify-end gap-2 pt-1">
        <button type="button" class="btn btn-soft" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>
