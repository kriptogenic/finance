<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Account } from '../api/types'
import { Money } from '../api/money'
import Modal from './Modal.vue'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ edit: [Account]; remove: [Account]; close: [] }>()

const router = useRouter()
// Navigate first, then close: the modal's history sentinel is only safe to drop
// once the new entry is on the stack.
async function viewTransactions() {
  await router.push({ name: 'transactions', query: { account: props.account.id } })
  emit('close')
}

const typeLabel: Record<string, string> = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
  receivable: 'Person',
}
const typeIcon: Record<string, string> = {
  cash: 'cash',
  debit_card: 'credit-card',
  deposit: 'building-bank',
  credit_card: 'credit-card',
  loan: 'home',
  receivable: 'users',
}

// For a credit card, available = limit − owed.
const available = computed<Money | null>(() =>
  props.account.type === 'credit_card' && props.account.credit_limit != null
    ? props.account.credit_limit.sub(props.account.balance)
    : null,
)
</script>

<template>
  <Modal :title="account.name" @close="emit('close')">
    <div class="space-y-5">
      <!-- header -->
      <div class="flex flex-col items-center gap-2 text-center">
        <span class="grid h-16 w-16 place-items-center rounded-2xl bg-slate-100 text-slate-600">
          <i :class="`ti ti-${typeIcon[account.type] || 'wallet'} text-2xl`" />
        </span>
        <p class="tabular text-2xl font-bold" :class="account.balance.isNegative() ? 'text-rose-600' : 'text-slate-900'">
          {{ account.balance.format() }}
        </p>
        <p class="text-xs text-slate-400">{{ typeLabel[account.type] || account.type }}</p>
        <p v-if="available" class="tabular text-sm font-medium text-slate-500">{{ available.format() }} available</p>
      </div>

      <button class="btn btn-soft w-full" @click="viewTransactions">
        <i class="ti ti-list-details" /> View transactions
      </button>

      <div class="flex gap-2 border-t border-slate-100 pt-4">
        <button class="btn btn-danger flex-1" @click="emit('remove', account)">Delete</button>
        <button class="btn btn-primary flex-1" @click="emit('edit', account)">Edit</button>
      </div>
    </div>
  </Modal>
</template>
