<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { categoriesApi } from '../api/categories'
import { reportsApi } from '../api/reports'
import { errMessage } from '../api/client'
import type { Category, CategoryType, SpendingReport } from '../api/types'
import type { Money } from '../api/money'
import { formatMinor } from '../lib/format'
import CategoryForm from '../components/CategoryForm.vue'
import CategoryIcon from '../components/CategoryIcon.vue'
import CategoryDetailSheet from '../components/CategoryDetailSheet.vue'
import CategoryRulesManager from '../components/CategoryRulesManager.vue'
import DonutChart from '../components/DonutChart.vue'

const categories = ref<Category[]>([])
const spending = ref<SpendingReport | null>(null)
const income = ref<Money | null>(null)
const base = ref('UZS')
const loading = ref(true)
const error = ref('')

// A date anchored in the selected month; spending/cash-flow are scoped to it.
const monthDate = ref(new Date())
const monthLabel = computed(() =>
  monthDate.value.toLocaleString(undefined, { month: 'long', year: 'numeric' }),
)
function monthRange(d: Date) {
  const from = new Date(d.getFullYear(), d.getMonth(), 1, 0, 0, 0)
  const to = new Date(d.getFullYear(), d.getMonth() + 1, 0, 23, 59, 59, 999)
  return { date_from: from.toISOString(), date_to: to.toISOString() }
}

// Solid colors for donut arcs (same set as the dashboard).
const palette = ['#6366f1', '#0ea5e9', '#ec4899', '#f59e0b', '#10b981', '#8b5cf6', '#14b8a6', '#f43f5e']

const spendMap = computed(() => {
  const m = new Map<string, Money>()
  for (const c of spending.value?.categories ?? []) m.set(c.category_id, c.amount)
  return m
})
const donutSegments = computed(() =>
  (spending.value?.categories ?? []).map((c, i) => ({
    color: palette[i % palette.length],
    value: c.amount.amount,
    key: c.category_id,
  })),
)

const expenseTops = computed(() => categories.value.filter((c) => c.type === 'expense' && !c.parent_id))
const incomeTops = computed(() => categories.value.filter((c) => c.type === 'income' && !c.parent_id))
function childrenOf(id: string) {
  return categories.value.filter((c) => c.parent_id === id)
}
function spendOf(c: Category): Money | undefined {
  return spendMap.value.get(c.id)
}
function tileAmount(c: Category): string {
  const m = spendOf(c)
  return m ? m.formatShort() : formatMinor(0, base.value)
}
function tint(c: Category) {
  return c.color ? { backgroundColor: c.color + '22' } : undefined
}

// form + detail sheet state
const formOpen = ref(false)
const editing = ref<Category | null>(null)
const formType = ref<CategoryType | null>(null)
const formParentId = ref<string | null>(null)
const detail = ref<Category | null>(null)

const detailSpent = computed(() => {
  const c = detail.value
  if (!c || c.type !== 'expense') return null
  return spendOf(c)?.format() ?? formatMinor(0, base.value)
})

async function loadSpend() {
  const range = monthRange(monthDate.value)
  const [sp, cf] = await Promise.all([reportsApi.spending(range), reportsApi.cashFlow(range)])
  spending.value = sp
  income.value = cf.months[0]?.income ?? null
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [cats, nw] = await Promise.all([categoriesApi.list(), reportsApi.netWorth().catch(() => null)])
    categories.value = cats
    if (nw) base.value = nw.base
    await loadSpend()
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}

function shiftMonth(n: number) {
  const d = new Date(monthDate.value)
  d.setMonth(d.getMonth() + n)
  monthDate.value = d
  loadSpend()
}

function openNew(type: CategoryType | null = null, parentId: string | null = null) {
  editing.value = null
  formType.value = type
  formParentId.value = parentId
  formOpen.value = true
}
function openEdit(c: Category) {
  editing.value = c
  formType.value = null
  formParentId.value = null
  formOpen.value = true
}
function onSaved() {
  formOpen.value = false
  load()
}
async function remove(c: Category) {
  if (!confirm(`Delete category "${c.name}"?`)) return
  try {
    await categoriesApi.remove(c.id)
    if (detail.value && detail.value.id === c.id) detail.value = null
    load()
  } catch (e) {
    alert(errMessage(e))
  }
}

// detail-sheet handlers
function onDetailEdit(c: Category) {
  detail.value = null
  openEdit(c)
}
function onDetailAddSub(parent: Category) {
  detail.value = null
  openNew(parent.type, parent.id)
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900">Categories</h1>
        <p class="text-sm text-slate-500">Spending by category</p>
      </div>
      <button class="btn btn-primary shrink-0" @click="openNew()">+ New<span class="hidden sm:inline"> category</span></button>
    </div>

    <!-- month switcher -->
    <div class="flex items-center justify-center gap-3">
      <button class="grid h-9 w-9 place-items-center rounded-full text-slate-500 transition hover:bg-slate-100" @click="shiftMonth(-1)">
        <i class="ti ti-chevron-left text-lg" />
      </button>
      <span class="min-w-40 text-center font-semibold text-slate-800">{{ monthLabel }}</span>
      <button class="grid h-9 w-9 place-items-center rounded-full text-slate-500 transition hover:bg-slate-100" @click="shiftMonth(1)">
        <i class="ti ti-chevron-right text-lg" />
      </button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>
    <p v-else-if="loading" class="text-slate-500">Loading…</p>

    <template v-else>
      <!-- expenses: donut + grid -->
      <section class="card p-6">
        <div class="flex flex-col items-center gap-6 sm:flex-row sm:items-center">
          <DonutChart :segments="donutSegments" :size="180">
            <span class="text-xs font-medium text-slate-400">Expenses</span>
            <span class="tabular text-base font-bold text-rose-600">{{ spending?.total.formatShort() }}</span>
            <span v-if="income" class="tabular text-xs font-semibold text-emerald-600">{{ income.formatShort() }}</span>
          </DonutChart>

          <div class="grid w-full grid-cols-3 gap-2 sm:grid-cols-4">
            <button
              v-for="c in expenseTops"
              :key="c.id"
              class="flex flex-col items-center gap-1.5 rounded-2xl p-2 transition hover:bg-slate-50"
              @click="detail = c"
            >
              <span class="grid h-12 w-12 place-items-center rounded-full" :class="c.color ? '' : 'bg-slate-100'" :style="tint(c)">
                <CategoryIcon :icon="c.icon" :color="c.color" :size="24" />
              </span>
              <span class="w-full truncate text-center text-xs font-medium text-slate-700">{{ c.name }}</span>
              <span class="tabular text-[11px] text-slate-400">{{ tileAmount(c) }}</span>
            </button>

            <button
              class="flex flex-col items-center justify-center gap-1.5 rounded-2xl border-2 border-dashed border-slate-200 p-2 text-slate-400 transition hover:border-slate-300 hover:text-slate-500"
              @click="openNew('expense')"
            >
              <span class="grid h-12 w-12 place-items-center rounded-full"><i class="ti ti-plus text-xl" /></span>
              <span class="text-xs">Add</span>
            </button>
          </div>
        </div>
      </section>

      <!-- income -->
      <section class="card p-6">
        <div class="mb-4 flex items-center gap-2">
          <span class="h-2.5 w-2.5 rounded-full bg-emerald-400" />
          <h2 class="text-sm font-semibold tracking-wide text-slate-500 uppercase">Income</h2>
        </div>
        <div class="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6">
          <button
            v-for="c in incomeTops"
            :key="c.id"
            class="flex flex-col items-center gap-1.5 rounded-2xl p-2 transition hover:bg-slate-50"
            @click="detail = c"
          >
            <span class="grid h-12 w-12 place-items-center rounded-full" :class="c.color ? '' : 'bg-slate-100'" :style="tint(c)">
              <CategoryIcon :icon="c.icon" :color="c.color" :size="24" />
            </span>
            <span class="w-full truncate text-center text-xs font-medium text-slate-700">{{ c.name }}</span>
          </button>

          <button
            class="flex flex-col items-center justify-center gap-1.5 rounded-2xl border-2 border-dashed border-slate-200 p-2 text-slate-400 transition hover:border-slate-300 hover:text-slate-500"
            @click="openNew('income')"
          >
            <span class="grid h-12 w-12 place-items-center rounded-full"><i class="ti ti-plus text-xl" /></span>
            <span class="text-xs">Add</span>
          </button>
        </div>
      </section>

      <CategoryRulesManager :categories="categories" />
    </template>

    <CategoryDetailSheet
      v-if="detail"
      :category="detail"
      :subcategories="childrenOf(detail.id)"
      :spent="detailSpent"
      @edit="onDetailEdit"
      @remove="remove"
      @add-sub="onDetailAddSub"
      @close="detail = null"
    />

    <CategoryForm
      v-if="formOpen"
      :category="editing"
      :categories="categories"
      :preset-type="formType"
      :preset-parent-id="formParentId"
      @close="formOpen = false"
      @saved="onSaved"
    />
  </div>
</template>
