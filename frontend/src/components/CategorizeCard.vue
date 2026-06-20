<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { transactionsApi } from '../api/transactions'
import { accountsApi } from '../api/accounts'
import { categoriesApi } from '../api/categories'
import { errMessage } from '../api/client'
import type { Account, Category, Transaction } from '../api/types'
import CategorizeModal from './CategorizeModal.vue'

withDefaults(defineProps<{ base?: string }>(), { base: 'UZS' })

const pending = ref<Transaction[]>([])
const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const loading = ref(true)
const error = ref('')
const showModal = ref(false)

async function load() {
  loading.value = true
  try {
    const [txns, accs, cats] = await Promise.all([
      transactionsApi.list({ uncategorized: true, limit: 100 }),
      accountsApi.list(),
      categoriesApi.list(),
    ])
    pending.value = txns
    accounts.value = accs
    categories.value = cats
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

function onClose() {
  showModal.value = false
  load()
}

onMounted(load)
</script>

<template>
  <section class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-slate-200/70">
    <p v-if="error" class="text-sm text-rose-600">{{ error }}</p>
    <p v-else-if="loading" class="text-sm text-slate-400">Loading…</p>

    <div v-else-if="!pending.length" class="flex items-center gap-3">
      <span class="grid h-10 w-10 place-items-center rounded-xl bg-emerald-50 text-xl text-emerald-600"><i class="ti ti-check" /></span>
      <div>
        <p class="font-semibold text-slate-900">All transactions are categorized</p>
        <p class="text-sm text-slate-500">Nothing to review — you're all caught up.</p>
      </div>
    </div>

    <div v-else class="flex flex-wrap items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <span class="grid h-10 w-10 place-items-center rounded-xl bg-amber-50 text-xl text-amber-600"><i class="ti ti-alert-triangle" /></span>
        <div>
          <p class="font-semibold text-slate-900">
            {{ pending.length }} transaction{{ pending.length === 1 ? '' : 's' }} need a category
          </p>
          <p class="text-sm text-slate-500">Review and assign categories with a few taps.</p>
        </div>
      </div>
      <button class="btn btn-primary" @click="showModal = true">Categorize</button>
    </div>

    <CategorizeModal
      v-if="showModal"
      :transactions="pending"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="onClose"
    />
  </section>
</template>
