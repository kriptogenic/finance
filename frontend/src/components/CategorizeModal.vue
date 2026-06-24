<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { transactionsApi } from '../api/transactions'
import { categoryRulesApi } from '../api/categories'
import { ruleBlocksApi } from '../api/ruleBlocks'
import { errMessage } from '../api/client'
import type { Account, Category, CategorySuggestion, Transaction } from '../api/types'
import { formatDateTime } from '../lib/format'
import Modal from './Modal.vue'
import CategoryIcon from './CategoryIcon.vue'
import TransactionForm from './TransactionForm.vue'

const props = withDefaults(
  defineProps<{
    transactions: Transaction[]
    accounts?: Account[]
    categories?: Category[]
    base?: string
  }>(),
  { accounts: () => [], categories: () => [], base: 'UZS' },
)
const emit = defineEmits<{ close: [] }>()

// Local working copy so we can drop items as they're handled.
const queue = ref<Transaction[]>([...props.transactions])
const index = ref(0)
const current = computed<Transaction | undefined>(() => queue.value[index.value])

const suggestions = ref<CategorySuggestion[]>([])
const loadingSuggestions = ref(false)
const error = ref('')
const busy = ref(false)
const fullEdit = ref(false)
const newTxn = ref(false)
const blocked = ref<Set<string>>(new Set())

// After applying a category, offer to remember it as a rule.
const rulePrompt = ref<{ note: string; categoryId: string } | null>(null)

const total = props.transactions.length
const doneCount = computed(() => index.value)

function norm(s: string | null | undefined): string {
  return (s ?? '').trim().toLowerCase()
}

const categoryOptions = computed(() =>
  props.categories.filter(
    (c) => current.value && c.type === categoryType(current.value) && !c.archived && !c.hidden_in_picker,
  ),
)

function categoryType(tx: Transaction): string {
  return tx.type // 'expense' | 'income'; transfers never reach this modal
}

function accountName(tx: Transaction): string {
  const id = tx.from_account_id ?? tx.to_account_id
  return props.accounts.find((a) => a.id === id)?.name ?? '—'
}

async function loadSuggestions() {
  const tx = current.value
  if (!tx) return
  suggestions.value = []
  loadingSuggestions.value = true
  try {
    suggestions.value = await transactionsApi.suggestCategories(tx.id)
  } catch {
    // suggestions are best-effort; the full grid is always available
    suggestions.value = []
  } finally {
    loadingSuggestions.value = false
  }
}

watch(current, (tx) => {
  if (tx && !rulePrompt.value && !fullEdit.value) loadSuggestions()
})

onMounted(async () => {
  try {
    blocked.value = new Set((await ruleBlocksApi.list()).map((b) => norm(b.merchant)))
  } catch {
    blocked.value = new Set()
  }
  loadSuggestions()
})

async function applyCategory(categoryId: string) {
  const tx = current.value
  if (!tx || busy.value) return
  error.value = ''
  busy.value = true
  try {
    await transactionsApi.patchCategory(tx.id, categoryId)
    const note = (tx.note ?? '').trim()
    if (note && !blocked.value.has(norm(note))) {
      rulePrompt.value = { note, categoryId }
    } else {
      advance()
    }
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    busy.value = false
  }
}

async function addRule() {
  if (!rulePrompt.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await categoryRulesApi.create({ pattern: rulePrompt.value.note, category_id: rulePrompt.value.categoryId })
    rulePrompt.value = null
    advance()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    busy.value = false
  }
}

async function neverRule() {
  if (!rulePrompt.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await ruleBlocksApi.create(rulePrompt.value.note)
    blocked.value.add(norm(rulePrompt.value.note))
    rulePrompt.value = null
    advance()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    busy.value = false
  }
}

function skipRule() {
  rulePrompt.value = null
  advance()
}

function onFullEditSaved() {
  fullEdit.value = false
  advance()
}

// A brand-new transaction created mid-flow doesn't belong to the queue; just
// close the sub-form and let the rest of the app refresh.
function onNewTxnSaved() {
  newTxn.value = false
  window.dispatchEvent(new CustomEvent('data:refresh'))
}

function advance() {
  if (index.value >= queue.value.length - 1) {
    emit('close')
    return
  }
  index.value += 1
}
</script>

<template>
  <TransactionForm
    v-if="fullEdit && current"
    :transaction="current"
    :accounts="accounts"
    :categories="categories"
    :base="base"
    @close="fullEdit = false"
    @saved="onFullEditSaved"
  />

  <TransactionForm
    v-else-if="newTxn"
    :accounts="accounts"
    :categories="categories"
    :base="base"
    @close="newTxn = false"
    @saved="onNewTxnSaved"
  />

  <Modal v-else title="Categorize" size="lg" @close="emit('close')">
    <div v-if="!current" class="flex flex-col items-center gap-2 py-8 text-center text-sm text-slate-500">
      <i class="ti ti-confetti text-2xl text-emerald-500" />
      All done
    </div>

    <div v-else class="space-y-5">
      <p class="text-xs font-medium text-slate-400">{{ doneCount + 1 }} of {{ total }}</p>

      <!-- transaction summary -->
      <div class="rounded-2xl bg-slate-50 p-4">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="truncate font-medium text-slate-800">{{ current.note || 'No description' }}</p>
            <p class="mt-0.5 text-xs text-slate-400">{{ accountName(current) }} · {{ formatDateTime(current.date) }}</p>
          </div>
          <span class="tabular shrink-0 font-semibold" :class="current.type === 'income' ? 'text-emerald-600' : 'text-slate-800'">
            {{ current.amount.format() }}
          </span>
        </div>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <!-- rule suggestion (after a category is chosen) -->
      <div v-if="rulePrompt" class="space-y-3 rounded-2xl border border-amber-200 bg-amber-50/60 p-4">
        <p class="text-sm text-slate-700">
          Always categorize <span class="font-semibold">“{{ rulePrompt.note }}”</span> this way?
        </p>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-primary" :disabled="busy" @click="addRule">Add rule</button>
          <button class="btn btn-soft" :disabled="busy" @click="skipRule">Not now</button>
          <button class="btn btn-soft !text-rose-600" :disabled="busy" @click="neverRule">Never for this merchant</button>
        </div>
      </div>

      <!-- categorize -->
      <template v-else>
        <!-- suggested -->
        <div v-if="loadingSuggestions" class="text-sm text-slate-400">Finding suggestions…</div>
        <div v-else-if="suggestions.length">
          <p class="lbl">Suggested</p>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="s in suggestions"
              :key="s.category_id"
              type="button"
              class="inline-flex items-center gap-1.5 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-sm font-medium text-amber-800 transition hover:bg-amber-50 disabled:opacity-50"
              :disabled="busy"
              @click="applyCategory(s.category_id)"
            >
              {{ s.category_name }}
              <span class="rounded-full bg-slate-100 px-1.5 text-[10px] font-semibold tracking-wide text-slate-500 uppercase">{{ s.source }}</span>
            </button>
          </div>
        </div>

        <!-- all categories -->
        <div>
          <p class="lbl">All categories</p>
          <p v-if="!categoryOptions.length" class="text-sm text-slate-400">No {{ current.type }} categories yet.</p>
          <div v-else class="grid grid-cols-4 gap-2 sm:grid-cols-5">
            <button
              v-for="c in categoryOptions"
              :key="c.id"
              type="button"
              class="flex flex-col items-center gap-1 rounded-xl border border-slate-200 p-2 text-center text-slate-600 transition hover:border-slate-300 hover:bg-slate-50 disabled:opacity-50"
              :disabled="busy"
              @click="applyCategory(c.id)"
            >
              <CategoryIcon :icon="c.icon" :color="c.color" :size="22" />
              <span class="w-full truncate text-[11px] leading-tight">{{ c.name }}</span>
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between gap-2 border-t border-slate-100 pt-4">
          <button type="button" class="btn btn-soft" :disabled="busy" @click="fullEdit = true">Full editI </button>
          <button type="button" class="btn btn-soft" :disabled="busy" @click="newTxn = true">
            <i class="ti ti-plus" /> New
          </button>
          <button type="button" class="btn btn-soft" :disabled="busy" @click="advance">Skip</button>
        </div>
      </template>
    </div>
  </Modal>
</template>
