<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { errMessage } from '../api/client'
import type { Account, LoanScheduleResponse } from '../api/types'
import { formatDate } from '../lib/format'
import Modal from './Modal.vue'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ close: [] }>()

const schedule = ref<LoanScheduleResponse | null>(null)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const editingPeriod = ref<number | null>(null)
const editDate = ref('')

const payoffDate = computed(() => {
  const rows = schedule.value?.rows
  return rows?.length ? rows[rows.length - 1].due_date : ''
})

async function load() {
  loading.value = true
  try {
    schedule.value = await accountsApi.loanSchedule(props.account.id)
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

// mutate wraps a row/regenerate call: it toggles busy, swaps in the recomputed
// schedule, and surfaces any error.
async function mutate(fn: () => Promise<LoanScheduleResponse>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    schedule.value = await fn()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    busy.value = false
  }
}

function togglePaid(period: number, paid: boolean) {
  return mutate(() => accountsApi.updateScheduleRow(props.account.id, period, { paid }))
}

function startEdit(period: number, date: string) {
  editingPeriod.value = period
  editDate.value = date.slice(0, 10)
}

async function saveOverride(period: number) {
  await mutate(() =>
    accountsApi.updateScheduleRow(props.account.id, period, { date_override: editDate.value }),
  )
  editingPeriod.value = null
}

async function clearOverride(period: number) {
  await mutate(() =>
    accountsApi.updateScheduleRow(props.account.id, period, { date_override: null }),
  )
  editingPeriod.value = null
}

function regenerate() {
  return mutate(() => accountsApi.regenerateSchedule(props.account.id))
}

onMounted(load)
</script>

<template>
  <Modal :title="`Payment schedule · ${account.name}`" size="xl" @close="emit('close')">
    <p v-if="error" class="mb-3 rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>
    <p v-if="loading" class="py-6 text-center text-sm text-slate-400">Loading…</p>

    <div v-else-if="schedule" class="space-y-5">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div class="rounded-xl bg-slate-50 p-4">
          <p class="text-xs text-slate-500">Monthly payment</p>
          <p class="tabular mt-1 text-lg font-bold text-slate-900">{{ schedule.monthly_payment.format() }}</p>
        </div>
        <div class="rounded-xl bg-rose-50 p-4">
          <p class="text-xs text-rose-500">Total interest</p>
          <p class="tabular mt-1 text-lg font-bold text-rose-600">{{ schedule.total_interest.formatShort() }}</p>
        </div>
        <div class="rounded-xl bg-slate-50 p-4">
          <p class="text-xs text-slate-500">Payoff</p>
          <p class="mt-1 text-lg font-bold text-slate-900">{{ formatDate(payoffDate) }}</p>
        </div>
      </div>

      <div class="max-h-[55vh] overflow-auto rounded-xl ring-1 ring-slate-200">
        <table class="w-full min-w-[40rem] text-sm">
          <thead class="sticky top-0 bg-slate-50 text-left text-xs tracking-wide text-slate-400 uppercase">
            <tr>
              <th class="px-3 py-2">#</th>
              <th class="px-3 py-2">Date</th>
              <th class="px-3 py-2 text-right">Payment</th>
              <th class="px-3 py-2 text-right">Principal</th>
              <th class="px-3 py-2 text-right">Interest</th>
              <th class="px-3 py-2 text-right">Balance</th>
              <th class="px-3 py-2 text-center">Paid</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="r in schedule.rows" :key="r.period" class="hover:bg-slate-50" :class="{ 'bg-emerald-50/40': r.paid }">
              <td class="px-3 py-1.5 text-slate-400">{{ r.period }}</td>

              <td class="px-3 py-1.5 whitespace-nowrap">
                <div v-if="editingPeriod === r.period" class="flex items-center gap-1">
                  <input v-model="editDate" type="date" class="rounded-md border border-slate-300 px-1.5 py-0.5 text-xs" />
                  <button class="grid h-6 w-6 place-items-center rounded text-emerald-600 hover:bg-emerald-100" title="Save" :disabled="busy" @click="saveOverride(r.period)"><i class="ti ti-check" /></button>
                  <button v-if="r.date_override" class="grid h-6 w-6 place-items-center rounded text-slate-400 hover:bg-slate-100" title="Clear override" :disabled="busy" @click="clearOverride(r.period)"><i class="ti ti-rotate" /></button>
                  <button class="grid h-6 w-6 place-items-center rounded text-slate-400 hover:bg-slate-100" title="Cancel" @click="editingPeriod = null"><i class="ti ti-x" /></button>
                </div>
                <button v-else class="group flex items-center gap-1 text-slate-600 hover:text-amber-700" title="Override date" @click="startEdit(r.period, r.due_date)">
                  {{ formatDate(r.due_date) }}
                  <i v-if="r.date_override" class="ti ti-pencil text-amber-500" title="Manually overridden" />
                  <i v-else class="ti ti-pencil text-slate-300 opacity-0 group-hover:opacity-100" />
                </button>
              </td>

              <td class="tabular px-3 py-1.5 text-right">{{ r.payment.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right text-slate-600">{{ r.principal.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right text-rose-600">{{ r.interest.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right font-medium">{{ r.balance.format() }}</td>
              <td class="px-3 py-1.5 text-center">
                <input type="checkbox" class="h-4 w-4 rounded accent-emerald-600" :checked="r.paid" :disabled="busy" @change="togglePaid(r.period, ($event.target as HTMLInputElement).checked)" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex items-center justify-between">
        <p class="text-xs text-slate-400">Dates roll off weekends &amp; holidays; interest is actual/365 on the real gap.</p>
        <button class="rounded-lg px-3 py-1.5 text-sm font-medium text-amber-700 ring-1 ring-amber-200 hover:bg-amber-50 disabled:opacity-50" :disabled="busy" @click="regenerate">Regenerate</button>
      </div>
    </div>
  </Modal>
</template>
