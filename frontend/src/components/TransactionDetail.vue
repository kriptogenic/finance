<script setup lang="ts">
import type { Transaction } from '../api/types'
import Modal from './Modal.vue'

const props = withDefaults(
  defineProps<{ transaction: Transaction; names?: Record<string, string>; base?: string }>(),
  { names: () => ({}), base: 'UZS' },
)
const emit = defineEmits<{ close: []; edit: []; remove: []; 'view-receipt': [id: string] }>()

const meta = {
  expense: { icon: 'arrow-up-right', ring: 'bg-rose-50 text-rose-600', sign: '−', amount: 'text-rose-600', label: 'Expense' },
  income: { icon: 'arrow-down-left', ring: 'bg-emerald-50 text-emerald-600', sign: '+', amount: 'text-emerald-600', label: 'Income' },
  transfer: { icon: 'transfer', ring: 'bg-indigo-50 text-indigo-600', sign: '', amount: 'text-slate-800', label: 'Transfer' },
}

function nameOf(id: string | null | undefined): string {
  return id ? props.names[id] || '—' : '—'
}
function formatDateTime(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : ''
}

const t = props.transaction
const m = meta[t.type]
// Show the base-currency figure only when it differs from the transaction's own.
const showBase = !!t.base_amount && t.base_amount.currency !== t.amount.currency
</script>

<template>
  <Modal title="Transaction" @close="emit('close')">
    <div class="space-y-5">
      <!-- amount header -->
      <div class="flex flex-col items-center gap-2 text-center">
        <span class="grid h-12 w-12 place-items-center rounded-full text-2xl" :class="m.ring"><i :class="`ti ti-${m.icon}`" /></span>
        <p class="tabular text-2xl font-bold" :class="m.amount">{{ m.sign }}{{ t.amount.format() }}</p>
        <span class="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-500">{{ m.label }}</span>
      </div>

      <dl class="divide-y divide-slate-100 text-sm">
        <div v-if="t.type !== 'transfer'" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Category</dt>
          <dd class="text-right font-medium text-slate-800">{{ nameOf(t.category_id) }}</dd>
        </div>
        <div v-if="t.receipt_id" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Receipt</dt>
          <dd class="text-right">
            <button
              class="inline-flex items-center gap-1.5 font-medium text-emerald-600 hover:text-emerald-700"
              @click="emit('view-receipt', t.receipt_id!)"
            >
              <i class="ti ti-qrcode" /> View receipt
            </button>
          </dd>
        </div>
        <div v-if="t.from_account_id" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">From</dt>
          <dd class="text-right font-medium text-slate-800">{{ nameOf(t.from_account_id) }}</dd>
        </div>
        <div v-if="t.to_account_id" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">To</dt>
          <dd class="text-right font-medium text-slate-800">{{ nameOf(t.to_account_id) }}</dd>
        </div>
        <div v-if="t.to_amount" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Received</dt>
          <dd class="tabular text-right font-medium text-slate-800">{{ t.to_amount.format() }}</dd>
        </div>
        <div v-if="t.rate_to_base" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Rate to {{ base }}</dt>
          <dd class="tabular text-right font-medium text-slate-800">{{ t.rate_to_base }}</dd>
        </div>
        <div v-if="showBase" class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">In {{ base }}</dt>
          <dd class="tabular text-right font-medium text-slate-800">{{ t.base_amount!.format() }}</dd>
        </div>
        <div class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Date</dt>
          <dd class="text-right font-medium text-slate-800">{{ formatDateTime(t.date) }}</dd>
        </div>
        <div v-if="t.note" class="py-2.5">
          <dt class="mb-1 text-slate-400">Note</dt>
          <dd class="break-words whitespace-pre-wrap text-slate-800">{{ t.note }}</dd>
        </div>
        <div v-if="t.tags.length" class="py-2.5">
          <dt class="mb-1.5 text-slate-400">Tags</dt>
          <dd class="flex flex-wrap gap-1.5">
            <span v-for="tag in t.tags" :key="tag" class="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">{{ tag }}</span>
          </dd>
        </div>
        <div class="flex justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-slate-400">Added</dt>
          <dd class="text-right text-slate-500">{{ formatDateTime(t.created_at) }}</dd>
        </div>
      </dl>

      <div class="flex justify-end gap-2 pt-1">
        <button class="btn btn-danger" @click="emit('remove')">Delete</button>
        <button class="btn btn-primary" @click="emit('edit')">Edit</button>
      </div>
    </div>
  </Modal>
</template>
