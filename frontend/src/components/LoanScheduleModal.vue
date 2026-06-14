<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { errMessage } from '../api/client'
import type { Account, AmortizationSchedule } from '../api/types'
import { formatDate } from '../lib/format'
import Modal from './Modal.vue'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ close: [] }>()

const schedule = ref<AmortizationSchedule | null>(null)
const loading = ref(true)
const error = ref('')

const payoffDate = computed(() => {
  const rows = schedule.value?.rows
  return rows?.length ? rows[rows.length - 1].date : ''
})

onMounted(async () => {
  try {
    schedule.value = await accountsApi.amortization(props.account.id)
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <Modal :title="`Amortization · ${account.name}`" size="xl" @close="emit('close')">
    <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>
    <p v-else-if="loading" class="py-6 text-center text-sm text-slate-400">Loading…</p>

    <div v-else-if="schedule" class="space-y-5">
      <div class="grid grid-cols-3 gap-3">
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

      <div class="max-h-[55vh] overflow-y-auto rounded-xl ring-1 ring-slate-200">
        <table class="w-full text-sm">
          <thead class="sticky top-0 bg-slate-50 text-left text-xs tracking-wide text-slate-400 uppercase">
            <tr>
              <th class="px-3 py-2">#</th>
              <th class="px-3 py-2">Date</th>
              <th class="px-3 py-2 text-right">Payment</th>
              <th class="px-3 py-2 text-right">Principal</th>
              <th class="px-3 py-2 text-right">Interest</th>
              <th class="px-3 py-2 text-right">Balance</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="r in schedule.rows" :key="r.period" class="hover:bg-slate-50">
              <td class="px-3 py-1.5 text-slate-400">{{ r.period }}</td>
              <td class="px-3 py-1.5 whitespace-nowrap text-slate-600">{{ formatDate(r.date) }}</td>
              <td class="tabular px-3 py-1.5 text-right">{{ r.payment.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right text-slate-600">{{ r.principal.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right text-rose-600">{{ r.interest.format() }}</td>
              <td class="tabular px-3 py-1.5 text-right font-medium">{{ r.balance.format() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </Modal>
</template>
