<script setup>
import { ref, computed, onMounted } from 'vue'
import { accountsApi } from '../api/accounts'
import { formatMoney } from '../lib/format'

const accounts = ref([])
const loading = ref(true)
const error = ref('')

const assets = computed(() => accounts.value.filter((a) => a.kind === 'asset'))
const liabilities = computed(() => accounts.value.filter((a) => a.kind === 'liability'))

const typeLabel = {
  cash: 'Cash',
  debit_card: 'Debit card',
  deposit: 'Deposit',
  credit_card: 'Credit card',
  loan: 'Loan',
}

onMounted(async () => {
  try {
    accounts.value = await accountsApi.list()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-8">
    <h1 class="text-2xl font-semibold">Accounts</h1>

    <p v-if="error" class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <section v-for="group in [{ title: 'Assets', items: assets }, { title: 'Liabilities', items: liabilities }]" :key="group.title">
        <h2 class="mb-2 text-lg font-medium">{{ group.title }}</h2>
        <div class="overflow-hidden rounded-xl border border-slate-200 bg-white">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 text-left text-slate-500">
              <tr>
                <th class="px-4 py-2">Name</th>
                <th class="px-4 py-2">Type</th>
                <th class="px-4 py-2 text-right">Balance</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="a in group.items" :key="a.id" class="border-t border-slate-100">
                <td class="px-4 py-2 font-medium">{{ a.name }}</td>
                <td class="px-4 py-2 text-slate-500">{{ typeLabel[a.type] || a.type }}</td>
                <td class="px-4 py-2 text-right font-medium">{{ formatMoney(a.balance) }}</td>
              </tr>
              <tr v-if="!group.items.length">
                <td colspan="3" class="px-4 py-3 text-center text-slate-400">None</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>
