<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { transactionsApi } from '../api/transactions'
import { errMessage } from '../api/client'
import type { Account, Category, CreateTransactionRequest, Transaction, TransactionType } from '../api/types'
import { toMinor, toMajor, toLocalInput } from '../lib/format'
import Modal from './Modal.vue'

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
const form = reactive({
  type: (t?.type ?? 'expense') as TransactionType,
  date: toLocalInput(t?.date),
  fromId: t?.from_account_id ?? '',
  toId: t?.to_account_id ?? '',
  categoryId: t?.category_id ?? '',
  amount: t ? toMajor(t.amount.amount) : ('' as number | string),
  toAmount: t?.to_amount ? toMajor(t.to_amount.amount) : ('' as number | string),
  rate: t?.rate_to_base ?? '',
  note: t?.note ?? '',
  tags: (t?.tags ?? []).join(', '),
})

const types: { v: TransactionType; label: string }[] = [
  { v: 'expense', label: 'Expense' },
  { v: 'income', label: 'Income' },
  { v: 'transfer', label: 'Transfer' },
]

const fromAcc = computed(() => props.accounts.find((a) => a.id === form.fromId))
const toAcc = computed(() => props.accounts.find((a) => a.id === form.toId))
const primaryCurrency = computed(() =>
  form.type === 'income' ? toAcc.value?.currency : fromAcc.value?.currency,
)
const isCross = computed(
  () => form.type === 'transfer' && !!fromAcc.value && !!toAcc.value && fromAcc.value.currency !== toAcc.value.currency,
)
const needsRate = computed(() => !!primaryCurrency.value && primaryCurrency.value !== props.base)
const categoryOptions = computed(() => props.categories.filter((c) => c.type === form.type && !c.archived))

function accLabel(a: Account): string {
  return `${a.name} (${a.currency})`
}

async function submit() {
  error.value = ''
  saving.value = true
  try {
    const payload: CreateTransactionRequest = {
      type: form.type,
      date: new Date(form.date).toISOString(),
      amount: toMinor(form.amount),
    }
    if (form.type !== 'income') payload.from_account_id = form.fromId
    if (form.type !== 'expense') payload.to_account_id = form.toId
    if (form.type !== 'transfer') payload.category_id = form.categoryId
    if (isCross.value) payload.to_amount = toMinor(form.toAmount)
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

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="lbl">Amount</label>
          <input v-model="form.amount" type="number" step="any" class="field" required placeholder="0.00" />
        </div>
        <div>
          <label class="lbl">Date</label>
          <input v-model="form.date" type="datetime-local" class="field" required />
        </div>
      </div>

      <div v-if="form.type !== 'income'">
        <label class="lbl">From account</label>
        <select v-model="form.fromId" class="field" required>
          <option value="" disabled>Select…</option>
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accLabel(a) }}</option>
        </select>
      </div>

      <div v-if="form.type !== 'expense'">
        <label class="lbl">To account</label>
        <select v-model="form.toId" class="field" required>
          <option value="" disabled>Select…</option>
          <option v-for="a in accounts" :key="a.id" :value="a.id">{{ accLabel(a) }}</option>
        </select>
      </div>

      <div v-if="form.type !== 'transfer'">
        <label class="lbl">Category</label>
        <select v-model="form.categoryId" class="field" required>
          <option value="" disabled>Select…</option>
          <option v-for="c in categoryOptions" :key="c.id" :value="c.id">
            {{ c.parent_id ? '— ' : '' }}{{ c.name }}
          </option>
        </select>
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
        <label class="lbl">Note (optional)</label>
        <input v-model="form.note" class="field" />
      </div>
      <div>
        <label class="lbl">Tags (comma-separated)</label>
        <input v-model="form.tags" class="field" placeholder="groceries, weekly" />
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex justify-end gap-2 pt-1">
        <button type="button" class="btn btn-soft" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>
