<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { accountsApi } from '../api/accounts'
import { errMessage } from '../api/client'
import type { Account, AccountKind, AccountType, CreateAccountRequest, UpdateAccountRequest } from '../api/types'
import { toMinor, toMajor } from '../lib/format'
import Modal from './Modal.vue'
import MoneyInput from './MoneyInput.vue'

const props = withDefaults(
  defineProps<{ account?: Account | null; base?: string }>(),
  { account: null, base: 'UZS' },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.account)
const error = ref('')
const saving = ref(false)

const typesByKind: Record<AccountKind, AccountType[]> = {
  asset: ['cash', 'debit_card', 'deposit'],
  liability: ['credit_card', 'loan'],
}
const typeLabel: Record<AccountType, string> = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
}

type Num = number | string | null

interface FormState {
  name: string
  kind: AccountKind
  type: AccountType
  currency: string
  opening: Num
  credit_limit: Num
  interest_rate: Num
  term_months: Num
  card_last4: string
  archived: boolean
}

const form = reactive<FormState>({
  name: props.account?.name ?? '',
  kind: props.account?.kind ?? 'asset',
  type: props.account?.type ?? 'debit_card',
  currency: props.account?.currency ?? props.base,
  opening: props.account ? toMajor(props.account.opening_balance, props.account.currency) : 0,
  credit_limit: props.account?.credit_limit != null ? toMajor(props.account.credit_limit, props.account.currency) : null,
  interest_rate: props.account?.interest_rate ?? null,
  term_months: props.account?.term_months ?? null,
  card_last4: props.account?.card_last4 ?? '',
  archived: props.account?.archived ?? false,
})

const availableTypes = computed(() => typesByKind[form.kind])

// card_last4 only applies to card accounts (it routes bank-notification ingest).
const isCard = computed(() => form.type === 'debit_card' || form.type === 'credit_card')

function onKindChange() {
  if (!availableTypes.value.includes(form.type)) form.type = availableTypes.value[0]
}

function numOrUndef(v: Num): number | undefined {
  return v === '' || v == null ? undefined : Number(v)
}

async function submit() {
  error.value = ''
  saving.value = true
  try {
    const cur = props.account ? props.account.currency : form.currency.toUpperCase()
    if (props.account) {
      const body: UpdateAccountRequest = {
        name: form.name,
        archived: form.archived,
        ...(isCard.value && form.card_last4 ? { card_last4: form.card_last4 } : {}),
        ...(form.type === 'credit_card' && form.credit_limit != null ? { credit_limit: toMinor(form.credit_limit, cur) } : {}),
        ...(form.type === 'deposit' || form.type === 'loan'
          ? { interest_rate: numOrUndef(form.interest_rate), term_months: numOrUndef(form.term_months) }
          : {}),
      }
      await accountsApi.update(props.account.id, body)
    } else {
      const body: CreateAccountRequest = {
        name: form.name,
        kind: form.kind,
        type: form.type,
        currency: cur,
        opening_balance: toMinor(form.opening, cur),
        ...(isCard.value && form.card_last4 ? { card_last4: form.card_last4 } : {}),
        ...(form.type === 'credit_card' && form.credit_limit != null ? { credit_limit: toMinor(form.credit_limit, cur) } : {}),
        ...(form.type === 'deposit' || form.type === 'loan'
          ? { interest_rate: numOrUndef(form.interest_rate), term_months: numOrUndef(form.term_months) }
          : {}),
      }
      await accountsApi.create(body)
    }
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
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
            <MoneyInput v-model="form.opening" :currency="form.currency || base" allow-negative class="field" />
          </div>
        </div>
      </template>

      <div v-else class="flex items-center gap-2">
        <input id="archived" v-model="form.archived" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
        <label for="archived" class="text-sm text-slate-600">Archived</label>
      </div>

      <div v-if="isCard">
        <label class="lbl">Card last 4 (optional)</label>
        <input v-model="form.card_last4" class="field" maxlength="4" placeholder="4853" />
        <p class="mt-1 text-xs text-slate-400">Routes external bank-notification ingest to this account.</p>
      </div>

      <div v-if="form.type === 'credit_card'">
        <label class="lbl">Credit limit</label>
        <MoneyInput v-model="form.credit_limit" :currency="form.currency || base" class="field" />
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
