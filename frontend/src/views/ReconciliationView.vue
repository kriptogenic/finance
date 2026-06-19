<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { reconciliationApi } from '../api/reconciliation'
import { errMessage } from '../api/client'
import type { ReconciliationReport } from '../api/types'
import { formatDate } from '../lib/format'

const report = ref<ReconciliationReport | null>(null)
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    report.value = await reconciliationApi.list()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-8">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Reconciliation</h1>
      <p class="text-sm text-slate-500">Bank-reported card balances vs. your tracked balances</p>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else-if="report">
      <div
        v-if="!report.rows.length"
        class="rounded-3xl bg-white p-10 text-center text-sm text-slate-400 shadow-sm ring-1 ring-slate-200/70"
      >
        No reported balances yet. Once the bot posts a balance snapshot, matched cards appear here.
      </div>

      <section v-else class="overflow-hidden rounded-3xl bg-white shadow-sm ring-1 ring-slate-200/70">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-100 text-left text-xs font-semibold tracking-wide text-slate-400 uppercase">
              <th class="px-5 py-3">Card</th>
              <th class="px-5 py-3 text-right">Reported</th>
              <th class="px-5 py-3 text-right">Tracked</th>
              <th class="px-5 py-3 text-right">Difference</th>
              <th class="px-5 py-3 text-right">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-50">
            <tr v-for="row in report.rows" :key="row.card_last4" class="hover:bg-slate-50/60">
              <td class="px-5 py-4">
                <p class="font-medium text-slate-800">{{ row.account_name }}</p>
                <p class="text-xs text-slate-400">
                  <span v-if="row.bank">{{ row.bank }} · </span>••{{ row.card_last4 }} · {{ formatDate(row.reported_at) }}
                </p>
              </td>
              <td class="tabular px-5 py-4 text-right text-slate-700">{{ row.reported.format() }}</td>
              <td class="tabular px-5 py-4 text-right text-slate-700">{{ row.derived.format() }}</td>
              <td
                class="tabular px-5 py-4 text-right font-medium"
                :class="row.delta && !row.delta.isZero() ? 'text-rose-600' : 'text-slate-400'"
              >
                {{ row.delta ? row.delta.format() : '—' }}
              </td>
              <td class="px-5 py-4 text-right">
                <span
                  v-if="row.currency_mismatch"
                  class="inline-flex rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 ring-1 ring-amber-100"
                >
                  Currency mismatch
                </span>
                <span
                  v-else-if="row.in_sync"
                  class="inline-flex rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-emerald-100"
                >
                  In sync
                </span>
                <span
                  v-else
                  class="inline-flex rounded-full bg-rose-50 px-2.5 py-1 text-xs font-medium text-rose-700 ring-1 ring-rose-100"
                >
                  Off
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </div>
</template>
