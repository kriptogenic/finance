<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { reconciliationApi } from '../api/reconciliation'
import { errMessage } from '../api/client'
import type { ReconciliationReport } from '../api/types'
import { formatDateTime } from '../lib/format'

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
  <section>
    <div class="mb-3 flex items-center gap-2">
      <span class="h-2.5 w-2.5 rounded-full bg-amber-400" />
      <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">Reconciliation</h2>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-sm text-slate-400">Loading…</p>

    <template v-else-if="report">
      <div
        v-if="!report.rows.length"
        class="rounded-2xl bg-white p-6 text-center text-sm text-slate-400 shadow-sm ring-1 ring-slate-200/70"
      >
        No reported balances yet. Once the bot posts a balance snapshot, matched cards appear here.
      </div>

      <ul v-else class="space-y-3">
        <li
          v-for="row in report.rows"
          :key="row.card_last4"
          class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200/70"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate font-semibold text-slate-800">{{ row.account_name }}</p>
              <p class="mt-0.5 text-xs text-slate-400">
                <span v-if="row.bank">{{ row.bank }} · </span>••{{ row.card_last4 }} · {{ formatDateTime(row.reported_at) }}
              </p>
            </div>
            <span
              v-if="row.currency_mismatch"
              class="shrink-0 rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 ring-1 ring-amber-100"
            >
              Currency mismatch
            </span>
            <span
              v-else-if="row.in_sync"
              class="shrink-0 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-emerald-100"
            >
              In sync
            </span>
            <span
              v-else
              class="shrink-0 rounded-full bg-rose-50 px-2.5 py-1 text-xs font-medium text-rose-700 ring-1 ring-rose-100"
            >
              Off
            </span>
          </div>

          <div class="mt-3 grid grid-cols-3 gap-2">
            <div>
              <p class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Reported</p>
              <p class="tabular mt-0.5 text-sm font-medium text-slate-700">{{ row.reported.format() }}</p>
            </div>
            <div>
              <p class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Tracked</p>
              <p class="tabular mt-0.5 text-sm font-medium text-slate-700">{{ row.derived.format() }}</p>
            </div>
            <div>
              <p class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Difference</p>
              <p
                class="tabular mt-0.5 text-sm font-medium"
                :class="row.delta && !row.delta.isZero() ? 'text-rose-600' : 'text-slate-400'"
              >
                {{ row.delta ? row.delta.format() : '—' }}
              </p>
            </div>
          </div>
        </li>
      </ul>
    </template>
  </section>
</template>
