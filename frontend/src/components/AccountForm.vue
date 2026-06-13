<script setup>
import { ref, reactive, computed } from 'vue'
import { accountsApi } from '../api/accounts'
import { toMinor, toMajor } from '../lib/format'
import Modal from './Modal.vue'

const props = defineProps({
  account: { type: Object, default: null }, // null = create
  base: { type: String, default: 'UZS' },
})
const emit = defineEmits(['close', 'saved'])

const editing = computed(() => !!props.account)
const error = ref('')
const saving = ref(false)

const typesByKind = {
  asset: ['cash', 'debit_card', 'deposit'],
  liability: ['credit_card', 'loan'],
}
const typeLabel = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
}

const form = reactive({
  name: props.account?.name ?? '',
  kind: props.account?.kind ?? 'asset',
  type: props.account?.type ?? 'cash',
  currency: props.account?.currency ?? props.base,
  opening: props.account ? toMajor(props.account.opening_balance) : 0,
  credit_limit: props.account?.credit_limit != null ? toMajor(props.account.credit_limit) : null,
  interest_rate: props.account?.interest_rate ?? null,
  term_months: props.account?.term_months ?? null,
  archived: props.account?.archived ?? false,
})

const availableTypes = computed(() => typesByKind[form.kind])

function onKindChange() {
  if (!availableTypes.value.includes(form.type)) form.type = availableTypes.value[0]
}

async function submit() {
  error.value = ''
  saving.value = true
  try {
    if (editing.value) {
      await accountsApi.update(props.account.id, {
        name: form.name,
        archived: form.archived,
        ...(form.type === 'credit_card' && form.credit_limit != null ? { credit_limit: toMinor(form.credit_limit) } : {}),
        ...(form.type === 'deposit' || form.type === 'loan' ? { interest_rate: numOrUndef(form.interest_rate), term_months: numOrUndef(form.term_months) } : {}),
      })
    } else {
      await accountsApi.create({
        name: form.name,
        kind: form.kind,
        type: form.type,
        currency: form.currency.toUpperCase(),
        opening_balance: toMinor(form.opening),
        ...(form.type === 'credit_card' && form.credit_limit != null ? { credit_limit: toMinor(form.credit_limit) } : {}),
        ...(form.type === 'deposit' || form.type === 'loan' ? { interest_rate: numOrUndef(form.interest_rate), term_months: numOrUndef(form.term_months) } : {}),
      })
    }
    emit('saved')
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
  }
}

function numOrUndef(v) {
  return v === '' || v == null ? undefined : Number(v)
}
</script>

<template>
  <Modal :title="editing ? 'Edit account' : 'New account'" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="submit">
      <div>
        <label class="lbl">Name</label>
        <input v-model="form.name" class="field" required placeholder="e.g. Cash" />
      </div>

      <template v-if="!editing">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Kind</label>
            <select v-model="form.kind" class="field" @change="onKindChange">
              <option value="asset">Asset</option>
              <option value="liability">Liability</option>
            </select>
          </div>
          <div>
            <label class="lbl">Type</label>
            <select v-model="form.type" class="field">
              <option v-for="t in availableTypes" :key="t" :value="t">{{ typeLabel[t] }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Currency</label>
            <input v-model="form.currency" class="field uppercase" maxlength="3" required />
          </div>
          <div>
            <label class="lbl">Opening balance</label>
            <input v-model="form.opening" type="number" step="any" class="field" />
          </div>
        </div>
      </template>

      <div v-else class="flex items-center gap-2">
        <input id="archived" v-model="form.archived" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
        <label for="archived" class="text-sm text-slate-600">Archived</label>
      </div>

      <div v-if="form.type === 'credit_card'">
        <label class="lbl">Credit limit</label>
        <input v-model="form.credit_limit" type="number" step="any" class="field" />
      </div>
      <div v-if="form.type === 'deposit' || form.type === 'loan'" class="grid grid-cols-2 gap-3">
        <div>
          <label class="lbl">Interest rate</label>
          <input v-model="form.interest_rate" type="number" step="any" class="field" placeholder="0.18" />
        </div>
        <div>
          <label class="lbl">Term (months)</label>
          <input v-model="form.term_months" type="number" class="field" />
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
