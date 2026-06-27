<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { receiptsApi } from '../api/receipts'
import { errMessage } from '../api/client'
import { formatDateTime } from '../lib/format'
import ReceiptDetail from '../components/ReceiptDetail.vue'
import type { Receipt, ReceiptStatus } from '../api/types'

const receipts = ref<Receipt[]>([])
const page = ref(1)
const limit = 20
const hasMore = ref(false)
const loading = ref(true)
const error = ref('')
const viewingId = ref<string | null>(null)

const statusStyle: Record<ReceiptStatus, { label: string; cls: string }> = {
  pending: { label: 'Pending', cls: 'bg-amber-50 text-amber-700 ring-amber-200' },
  html_received: { label: 'Parsing', cls: 'bg-sky-50 text-sky-700 ring-sky-200' },
  success: { label: 'Parsed', cls: 'bg-emerald-50 text-emerald-700 ring-emerald-200' },
  failed: { label: 'Failed', cls: 'bg-rose-50 text-rose-700 ring-rose-200' },
}

async function load(p = 1) {
  loading.value = true
  error.value = ''
  try {
    const rows = await receiptsApi.list(p, limit)
    receipts.value = rows
    page.value = p
    hasMore.value = rows.length === limit
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

function when(r: Receipt): string {
  return formatDateTime(r.received_at ?? r.created_at)
}

onMounted(() => load())
onMounted(() => window.addEventListener('data:refresh', () => load(page.value)))
onUnmounted(() => window.removeEventListener('data:refresh', () => load(page.value)))
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Receipts</h1>
      <p class="text-sm text-slate-500">Scanned fiscal receipts and their linked transactions</p>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>
    <p v-else-if="receipts.length === 0" class="card p-8 text-center text-sm text-slate-500">
      No receipts yet. Long-press the action button to scan one.
    </p>

    <template v-else>
      <div class="card divide-y divide-slate-100">
        <button
          v-for="r in receipts"
          :key="r.id"
          class="flex w-full items-center gap-3 px-4 py-3 text-left transition hover:bg-slate-50"
          @click="viewingId = r.id"
        >
          <span class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-slate-100 text-slate-500">
            <i class="ti ti-receipt text-xl" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium text-slate-800">{{ r.merchant_name || 'Receipt' }}</p>
            <p class="text-xs text-slate-400">{{ when(r) }}</p>
          </div>
          <div class="flex shrink-0 flex-col items-end gap-1">
            <span class="tabular font-semibold text-slate-900">{{ r.total_amount?.format() }}</span>
            <span class="flex items-center gap-1.5">
              <i
                v-if="r.transaction_id"
                class="ti ti-link text-sm text-emerald-600"
                title="Linked to a transaction"
              />
              <span
                class="rounded-full px-2 py-0.5 text-[11px] font-semibold ring-1"
                :class="statusStyle[r.status].cls"
              >{{ statusStyle[r.status].label }}</span>
            </span>
          </div>
        </button>
      </div>

      <div v-if="page > 1 || hasMore" class="flex items-center justify-center gap-3">
        <button class="btn" :disabled="page <= 1" @click="load(page - 1)">Previous</button>
        <span class="text-sm text-slate-500">Page {{ page }}</span>
        <button class="btn" :disabled="!hasMore" @click="load(page + 1)">Next</button>
      </div>
    </template>

    <ReceiptDetail
      v-if="viewingId"
      :id="viewingId"
      @close="viewingId = null"
      @changed="load(page)"
    />
  </div>
</template>
