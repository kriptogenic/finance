<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import { transactionsApi } from '../api/transactions'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type { Account, Category, CreateTransactionRequest, Transaction, TransactionType } from '../api/types'
import { toMinor, toMajor, toLocalInput, formatMinor } from '../lib/format'
import Modal from './Modal.vue'
import CategoryIcon from './CategoryIcon.vue'
import MoneyInput from './MoneyInput.vue'

// 'split' is a UI-only type: it is saved as an expense plus a split breakdown.
type FormType = TransactionType | 'split'

interface Prefill {
  type?: FormType
  fromId?: string
  toId?: string
  amount?: number | string
}

const props = withDefaults(
  defineProps<{
    transaction?: Transaction | null
    accounts?: Account[]
    categories?: Category[]
    base?: string
    prefill?: Prefill | null
    noteThreshold?: number
  }>(),
  {
    transaction: null,
    accounts: () => [],
    categories: () => [],
    base: 'UZS',
    prefill: null,
    noteThreshold: Number.POSITIVE_INFINITY,
  },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.transaction)
const error = ref('')
const saving = ref(false)
const removing = ref(false)

async function del() {
  if (!props.transaction) return
  if (!(await confirm({ title: 'Delete transaction?', message: 'This transaction will be deleted.' }))) return
  removing.value = true
  error.value = ''
  try {
    await transactionsApi.remove(props.transaction.id)
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    removing.value = false
  }
}

const t = props.transaction

// Archived accounts are hidden unless this entry already points at one.
const pinnedAccountIds = new Set(
  [t?.from_account_id, t?.to_account_id, props.prefill?.fromId, props.prefill?.toId].filter(Boolean) as string[],
)
const accountOptions = computed(() => props.accounts.filter((a) => !a.archived || pinnedAccountIds.has(a.id)))

// Sensible default account: prefer one in the base currency to avoid an
// immediate rate prompt on a brand-new entry.
const defaultAccount = accountOptions.value.find((a) => a.currency === props.base) ?? accountOptions.value[0]

// New entries default to Transfer; an existing split expense opens on Split.
const initialType: FormType = t ? (t.split_group_id ? 'split' : t.type) : (props.prefill?.type ?? 'transfer')

const form = reactive({
  type: initialType,
  date: toLocalInput(t?.date),
  fromId: t?.from_account_id ?? props.prefill?.fromId ?? defaultAccount?.id ?? '',
  toId: t?.to_account_id ?? props.prefill?.toId ?? defaultAccount?.id ?? '',
  categoryId: t?.category_id ?? '',
  amount: t ? toMajor(t.amount.amount, t.amount.currency) : (props.prefill?.amount ?? ('' as number | string)),
  toAmount: t?.to_amount ? toMajor(t.to_amount.amount, t.to_amount.currency) : ('' as number | string),
  rate: t?.rate_to_base ?? '',
  note: t?.note ?? '',
  tags: (t?.tags ?? []).join(', '),
})

const types: { v: FormType; label: string }[] = [
  { v: 'expense', label: 'Expense' },
  { v: 'income', label: 'Income' },
  { v: 'transfer', label: 'Transfer' },
  { v: 'split', label: 'Split' },
]

// the underlying ledger type a split maps to
const baseType = computed<TransactionType>(() => (form.type === 'split' ? 'expense' : form.type))

const acctIcons: Record<string, string> = {
  cash: 'cash',
  debit_card: 'credit-card',
  deposit: 'building-bank',
  credit_card: 'credit-card',
  loan: 'home',
  receivable: 'users',
}
function acctIcon(a: Account): string {
  return acctIcons[a.type] ?? 'credit-card'
}

const fromAcc = computed(() => props.accounts.find((a) => a.id === form.fromId))
const toAcc = computed(() => props.accounts.find((a) => a.id === form.toId))
const primaryCurrency = computed(() => (form.type === 'income' ? toAcc.value?.currency : fromAcc.value?.currency))
const amountCurrency = computed(() => primaryCurrency.value ?? props.base)
const isCross = computed(
  () => form.type === 'transfer' && !!fromAcc.value && !!toAcc.value && fromAcc.value.currency !== toAcc.value.currency,
)
const needsRate = computed(() => !!primaryCurrency.value && primaryCurrency.value !== props.base)
const categoryOptions = computed(() => props.categories.filter((c) => c.type === baseType.value && !c.archived))

// Rare fields live behind a disclosure; auto-open it (and keep it open when
// editing) whenever a required advanced field — FX rate or cross-currency
// received amount — actually applies, so it can never be hidden yet mandatory.
// large amounts must carry a note (threshold comes from server config)
const submitAmountMinor = computed(() => (form.amount === '' ? 0 : toMinor(form.amount, amountCurrency.value)))
const noteRequired = computed(() => submitAmountMinor.value > props.noteThreshold)
const noteMissing = computed(() => noteRequired.value && form.note.trim() === '')

const showDetails = ref(!!props.transaction)
watch([isCross, needsRate, noteRequired], ([cross, rate, note]) => {
  if (cross || rate || note) showDetails.value = true
})
const detailsHint = computed(() => {
  if (noteMissing.value) return 'note required'
  if (isCross.value || needsRate.value) return 'exchange rate required'
  return ''
})

// ---- split breakdown (type === 'split') ----
const splitReady = ref(false)
const myShare = ref<number | string>('')
const friends = reactive<{ name: string; amount: number | string }[]>([])

function minor(v: number | string): number {
  return v === '' || v == null ? 0 : toMinor(v, amountCurrency.value)
}
const billMinor = computed(() => minor(form.amount))
const assignedMinor = computed(() => minor(myShare.value) + friends.reduce((s, f) => s + minor(f.amount), 0))
const remainingMinor = computed(() => billMinor.value - assignedMinor.value)
const peopleCount = computed(() => 1 + friends.length)
const splitValid = computed(
  () =>
    minor(myShare.value) > 0 &&
    remainingMinor.value === 0 &&
    friends.every((f) => minor(f.amount) > 0 && f.name.trim() !== ''),
)

// Load the existing breakdown (edit) or seed a fresh one (your share = the bill).
async function ensureSplit() {
  if (splitReady.value) return
  if (props.transaction?.split_group_id) {
    try {
      const res = await transactionsApi.getSplit(props.transaction.id)
      friends.splice(
        0,
        friends.length,
        ...res.participants.map((p) => ({ name: p.name, amount: toMajor(p.amount.amount, p.amount.currency) })),
      )
      const sum = res.participants.reduce((s, p) => s + p.amount.amount, 0)
      form.amount = toMajor(res.my_share.amount + sum, res.my_share.currency)
      myShare.value = toMajor(res.my_share.amount, res.my_share.currency)
    } catch (e) {
      error.value = errMessage(e)
    }
  } else {
    friends.splice(0, friends.length)
    myShare.value = form.amount
  }
  splitReady.value = true
}

// Divide the bill across `n` people (you + friends); the remainder lands on
// your share so the parts always re-sum to the total.
function setCount(n: number) {
  const count = Math.max(1, Math.floor(n || 1))
  const friendsCount = count - 1
  const total = billMinor.value
  const per = Math.floor(total / count)
  const next = Array.from({ length: friendsCount }, (_, i) => ({
    name: friends[i]?.name || `Friend ${i + 1}`,
    amount: toMajor(per, amountCurrency.value),
  }))
  friends.splice(0, friends.length, ...next)
  myShare.value = toMajor(total - per * friendsCount, amountCurrency.value)
}

function addFriend() {
  friends.push({ name: `Friend ${friends.length + 1}`, amount: '' })
}
function removeFriend(i: number) {
  friends.splice(i, 1)
}
function giveRemainderToMe() {
  myShare.value = toMajor(minor(myShare.value) + remainingMinor.value, amountCurrency.value)
}

watch(
  () => form.type,
  (ty) => {
    if (ty === 'split') ensureSplit()
  },
  { immediate: true },
)

function commonExtras(payload: CreateTransactionRequest) {
  if (needsRate.value && form.rate !== '') payload.rate_to_base = String(form.rate)
  if (form.note) payload.note = form.note
  const tags = form.tags.split(',').map((s) => s.trim()).filter(Boolean)
  if (tags.length) payload.tags = tags
}

async function submitSplit() {
  if (!form.categoryId) {
    error.value = 'Please choose a category.'
    return
  }
  if (minor(myShare.value) <= 0) {
    error.value = 'Your share must be positive.'
    return
  }
  if (remainingMinor.value !== 0) {
    error.value = 'Assign the full bill across everyone.'
    return
  }
  if (noteMissing.value) {
    error.value = `A note is required for amounts over ${formatMinor(props.noteThreshold, amountCurrency.value)}.`
    return
  }
  saving.value = true
  try {
    const expense: CreateTransactionRequest = {
      type: 'expense',
      date: new Date(form.date).toISOString(),
      amount: billMinor.value,
      from_account_id: form.fromId,
      category_id: form.categoryId,
    }
    commonExtras(expense)

    let id = props.transaction?.id
    if (id) await transactionsApi.update(id, expense)
    else id = (await transactionsApi.create(expense)).id

    const participants = friends.map((f) => ({ name: f.name.trim() || 'Friend', amount: minor(f.amount) }))
    await transactionsApi.setSplit(id, { my_share: minor(myShare.value), participants })
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}

async function submit() {
  error.value = ''
  if (form.type === 'split') return submitSplit()

  if (form.type !== 'transfer' && !form.categoryId) {
    error.value = 'Please choose a category.'
    return
  }
  if (noteMissing.value) {
    error.value = `A note is required for amounts over ${formatMinor(props.noteThreshold, amountCurrency.value)}.`
    return
  }
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
    commonExtras(payload)

    // dropping a split: clear its legs while the row is still an expense
    if (props.transaction?.split_group_id) {
      await transactionsApi.setSplit(props.transaction.id, {
        my_share: Math.max(1, toMinor(form.amount, amountCurrency.value)),
        participants: [],
      })
    }

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
      <div class="grid grid-cols-4 gap-1 rounded-xl bg-slate-100 p-1">
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
        <label class="lbl">{{ form.type === 'split' ? 'Total bill' : 'Amount' }}</label>
        <div class="relative">
          <MoneyInput
            v-model="form.amount"
            :currency="amountCurrency"
            class="field tabular !py-3 !pr-20 text-2xl font-semibold"
            required
            placeholder="0,00"
            autofocus
          />
          <span class="absolute top-1/2 right-3 -translate-y-1/2 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500">
            {{ amountCurrency }}
          </span>
        </div>
      </div>

      <!-- category (expense / income / split) -->
      <div v-if="form.type !== 'transfer'">
        <label class="lbl">Category</label>
        <p v-if="!categoryOptions.length" class="text-sm text-slate-400">No {{ baseType }} categories yet.</p>
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

      <!-- from account (expense / transfer / split) -->
      <div v-if="form.type !== 'income'">
        <label class="lbl">{{ form.type === 'split' ? 'Paid from' : 'From account' }}</label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="a in accountOptions"
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

      <!-- split breakdown -->
      <div v-if="form.type === 'split'" class="space-y-3 rounded-xl bg-slate-50 p-3">
        <p class="text-xs text-slate-500">
          Your share stays the expense; each friend's share becomes money they owe you, tracked in a person account
          that disappears once they pay you back.
        </p>

        <div>
          <label class="lbl">People (including you)</label>
          <input
            type="number"
            min="1"
            :value="peopleCount"
            class="field w-24"
            @input="setCount(+($event.target as HTMLInputElement).value)"
          />
          <span v-if="billMinor % peopleCount !== 0" class="ml-2 text-xs text-amber-600">
            doesn't divide evenly — remainder on your share
          </span>
        </div>

        <div>
          <label class="lbl">Your share</label>
          <MoneyInput v-model="myShare" :currency="amountCurrency" class="field" placeholder="0,00" />
        </div>

        <div v-if="friends.length" class="space-y-2">
          <label class="lbl">Friends</label>
          <div v-for="(f, i) in friends" :key="i" class="flex items-center gap-2">
            <input v-model="f.name" class="field flex-1" placeholder="Name" />
            <MoneyInput v-model="f.amount" :currency="amountCurrency" class="field w-32" placeholder="0,00" />
            <button type="button" class="text-slate-400 hover:text-rose-500" @click="removeFriend(i)">
              <i class="ti ti-trash" />
            </button>
          </div>
        </div>

        <button type="button" class="text-sm font-medium text-amber-600 hover:text-amber-700" @click="addFriend">
          + Add person
        </button>

        <div
          class="flex items-center justify-between rounded-lg px-3 py-2 text-sm"
          :class="remainingMinor === 0 ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'"
        >
          <span>{{ remainingMinor === 0 ? 'Fully assigned' : 'Left to assign' }}</span>
          <span class="tabular font-semibold">
            {{ formatMinor(remainingMinor, amountCurrency) }}
            <button v-if="remainingMinor !== 0" type="button" class="ml-2 underline" @click="giveRemainderToMe">
              give to me
            </button>
          </span>
        </div>
      </div>

      <!-- to account (income / transfer) -->
      <div v-if="form.type === 'income' || form.type === 'transfer'">
        <label class="lbl">To account</label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="a in accountOptions"
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
          <div>
            <label class="lbl">Date</label>
            <input v-model="form.date" type="datetime-local" class="field" required />
          </div>

          <div v-if="isCross">
            <label class="lbl">Received ({{ toAcc?.currency }})</label>
            <MoneyInput v-model="form.toAmount" :currency="toAcc?.currency" class="field" required placeholder="0,00" />
          </div>

          <div v-if="needsRate">
            <label class="lbl">Rate {{ primaryCurrency }} → {{ base }} (1 minor unit)</label>
            <input v-model="form.rate" class="field" required placeholder="e.g. 12500" />
          </div>

          <div>
            <label class="lbl">Note <span v-if="noteRequired" class="text-rose-500">* required for large amounts</span></label>
            <input v-model="form.note" class="field" :class="noteMissing ? 'ring-1 ring-rose-300' : ''" :required="noteRequired" />
          </div>
          <div>
            <label class="lbl">Tags (comma-separated)</label>
            <input v-model="form.tags" class="field" placeholder="groceries, weekly" />
          </div>
        </div>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex items-center gap-2 pt-1">
        <button v-if="editing" type="button" class="btn btn-danger" :disabled="saving || removing" @click="del">Delete</button>
        <button type="button" class="btn btn-soft ml-auto" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || removing || (form.type === 'split' && !splitValid)">
          {{ saving ? 'Saving…' : 'Save' }}
        </button>
      </div>
    </form>
  </Modal>
</template>
