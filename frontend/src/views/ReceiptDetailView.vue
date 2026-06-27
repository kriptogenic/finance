<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { receiptsApi } from '../api/receipts'
import { transactionsApi } from '../api/transactions'
import { categoriesApi } from '../api/categories'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import { formatDateTime } from '../lib/format'
import type { Receipt, ReceiptStatus, Transaction, Category } from '../api/types'

const route = useRoute()
const id = route.params.id as string

const receipt = ref<Receipt | null>(null)
const linked = ref<Transaction | null>(null)
const categories = ref<Category[]>([])
const loading = ref(true)
const error = ref('')

const statusStyle: Record<ReceiptStatus, { label: string; cls: string }> = {
  pending: { label: 'Pending', cls: 'bg-amber-50 text-amber-700 ring-amber-200' },
  html_received: { label: 'Parsing', cls: 'bg-sky-50 text-sky-700 ring-sky-200' },
  success: { label: 'Parsed', cls: 'bg-emerald-50 text-emerald-700 ring-emerald-200' },
  failed: { label: 'Failed', cls: 'bg-rose-50 text-rose-700 ring-rose-200' },
}

const catName = (cid?: string | null) => categories.value.find((c) => c.id === cid)?.name ?? 'Uncategorized'

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [rec, cats] = await Promise.all([receiptsApi.get(id), categoriesApi.list().catch(() => [])])
    receipt.value = rec
    categories.value = cats
    linked.value = rec.transaction_id ? await transactionsApi.get(rec.transaction_id).catch(() => null) : null
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

// ── link picker ────────────────────────────────────────────────────────────
const picking = ref(false)
const candidates = ref<Transaction[]>([])
const search = ref('')
const busy = ref(false)

async function openPicker() {
  picking.value = true
  search.value = ''
  try {
    const txns = await transactionsApi.list({ type: 'expense', limit: 100 })
    const total = receipt.value?.total_amount?.amount
    const center = new Date(receipt.value?.received_at ?? receipt.value?.created_at ?? Date.now()).getTime()
    // Surface exact-amount matches first, then nearest-in-time.
    candidates.value = txns.sort((a, b) => {
      const am = total != null && a.amount.amount === total ? 0 : 1
      const bm = total != null && b.amount.amount === total ? 0 : 1
      if (am !== bm) return am - bm
      return Math.abs(new Date(a.date).getTime() - center) - Math.abs(new Date(b.date).getTime() - center)
    })
  } catch (e) {
    error.value = errMessage(e)
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return candidates.value
  return candidates.value.filter(
    (t) => (t.note ?? '').toLowerCase().includes(q) || catName(t.category_id).toLowerCase().includes(q),
  )
})

const isMatch = (t: Transaction) => receipt.value?.total_amount?.amount === t.amount.amount

async function link(t: Transaction) {
  busy.value = true
  try {
    receipt.value = await receiptsApi.linkTransaction(id, t.id)
    linked.value = t
    picking.value = false
  } catch (e) {
    alert(errMessage(e))
  } finally {
    busy.value = false
  }
}

async function unlink() {
  if (!(await confirm({ title: 'Unlink transaction?', message: 'This receipt will no longer be tied to a transaction.' }))) return
  busy.value = true
  try {
    receipt.value = await receiptsApi.unlinkTransaction(id)
    linked.value = null
  } catch (e) {
    alert(errMessage(e))
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <RouterLink to="/receipts" class="inline-flex items-center gap-1 text-sm font-medium text-slate-500 hover:text-slate-700">
      <i class="ti ti-chevron-left" /> Receipts
    </RouterLink>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else-if="receipt">
      <!-- header -->
      <div class="flex items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-bold tracking-tight text-slate-900">{{ receipt.merchant_name || 'Receipt' }}</h1>
          <p class="text-sm text-slate-500">{{ formatDateTime(receipt.received_at ?? receipt.created_at) }}</p>
          <p v-if="receipt.merchant_address" class="text-xs text-slate-400">{{ receipt.merchant_address }}</p>
        </div>
        <span
          class="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ring-1"
          :class="statusStyle[receipt.status].cls"
        >{{ statusStyle[receipt.status].label }}</span>
      </div>

      <p v-if="receipt.status === 'failed' && receipt.error" class="rounded-xl bg-rose-50 px-4 py-3 text-sm text-rose-700 ring-1 ring-rose-100">
        {{ receipt.error }}
      </p>

      <!-- totals -->
      <section class="card grid grid-cols-2 gap-4 p-5 sm:grid-cols-4">
        <div>
          <p class="text-xs text-slate-400">Total</p>
          <p class="tabular text-lg font-bold text-slate-900">{{ receipt.total_amount?.format() }}</p>
        </div>
        <div>
          <p class="text-xs text-slate-400">Paid by card</p>
          <p class="tabular font-semibold text-slate-700">{{ receipt.paid_card?.format() }}</p>
        </div>
        <div>
          <p class="text-xs text-slate-400">Paid cash</p>
          <p class="tabular font-semibold text-slate-700">{{ receipt.paid_cash?.format() }}</p>
        </div>
        <div>
          <p class="text-xs text-slate-400">VAT</p>
          <p class="tabular font-semibold text-slate-700">{{ receipt.total_vat?.format() }}</p>
        </div>
      </section>

      <!-- linked transaction -->
      <section class="card p-5">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">Linked transaction</h2>
          <button v-if="!linked && !picking" class="btn btn-primary" @click="openPicker">Link</button>
          <button v-if="picking" class="btn" @click="picking = false">Cancel</button>
        </div>

        <!-- linked state -->
        <div v-if="linked" class="flex items-center justify-between gap-3">
          <RouterLink to="/transactions" class="min-w-0">
            <p class="font-medium text-slate-800">{{ catName(linked.category_id) }}</p>
            <p class="truncate text-sm text-slate-500">{{ linked.note || formatDateTime(linked.date) }}</p>
          </RouterLink>
          <div class="flex shrink-0 items-center gap-3">
            <span class="tabular font-semibold text-slate-900">{{ linked.amount.format() }}</span>
            <button class="btn" :disabled="busy" @click="unlink">Unlink</button>
          </div>
        </div>

        <!-- picker -->
        <div v-else-if="picking" class="space-y-3">
          <input v-model="search" type="text" placeholder="Search by note or category…" class="field text-sm" />
          <div class="max-h-80 divide-y divide-slate-100 overflow-y-auto rounded-xl ring-1 ring-slate-100">
            <button
              v-for="t in filtered"
              :key="t.id"
              class="flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left transition hover:bg-slate-50 disabled:opacity-50"
              :disabled="busy"
              @click="link(t)"
            >
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-slate-800">{{ catName(t.category_id) }}</p>
                <p class="truncate text-xs text-slate-400">{{ t.note || formatDateTime(t.date) }}</p>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <span v-if="isMatch(t)" class="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 ring-1 ring-emerald-200">match</span>
                <span class="tabular text-sm font-semibold text-slate-700">{{ t.amount.format() }}</span>
              </div>
            </button>
            <p v-if="filtered.length === 0" class="px-3 py-4 text-center text-sm text-slate-400">No expenses found.</p>
          </div>
        </div>

        <!-- empty -->
        <p v-else class="text-sm text-slate-500">Not linked to any transaction yet.</p>
      </section>

      <!-- items -->
      <section v-if="receipt.items.length" class="card overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-slate-50 text-left text-xs text-slate-400">
            <tr>
              <th class="px-4 py-2 font-medium">Item</th>
              <th class="px-4 py-2 text-right font-medium">Qty</th>
              <th class="px-4 py-2 text-right font-medium">Price</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="(it, i) in receipt.items" :key="i">
              <td class="px-4 py-2.5 text-slate-700">{{ it.name }}</td>
              <td class="px-4 py-2.5 text-right text-slate-500">{{ it.quantity }}</td>
              <td class="tabular px-4 py-2.5 text-right font-medium text-slate-800">{{ it.price.format() }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </div>
</template>
