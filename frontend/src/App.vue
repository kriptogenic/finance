<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearCredentials } from './api/auth'
import { hideMinorUnits } from './lib/settings'
import { accountsApi } from './api/accounts'
import { categoriesApi } from './api/categories'
import { reportsApi } from './api/reports'
import { transactionsApi } from './api/transactions'
import type { Account, Category, Transaction } from './api/types'
import TransactionForm from './components/TransactionForm.vue'
import CategorizeModal from './components/CategorizeModal.vue'

const route = useRoute()
const router = useRouter()

// The login screen renders standalone, without the app chrome.
const showChrome = computed(() => route.name !== 'login')

function logout() {
  clearCredentials()
  moreOpen.value = false
  router.push({ name: 'login' })
}

// A 401 from any request boots the user back to the login screen.
function onUnauthorized() {
  if (route.name !== 'login') router.push({ name: 'login' })
}
onMounted(() => window.addEventListener('auth:unauthorized', onUnauthorized))
onUnmounted(() => window.removeEventListener('auth:unauthorized', onUnauthorized))

const nav = [
  {
    to: '/',
    label: 'Dashboard',
    icon: 'M2.25 12 11.2 3.05c.44-.44 1.15-.44 1.59 0L21.75 12M4.5 9.75v10.13c0 .62.5 1.12 1.13 1.12H9.75v-4.88c0-.62.5-1.12 1.13-1.12h2.25c.62 0 1.12.5 1.12 1.12V21h4.13c.62 0 1.12-.5 1.12-1.12V9.75',
  },
  {
    to: '/accounts',
    label: 'Accounts',
    icon: 'M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Z',
  },
  {
    to: '/transactions',
    label: 'Transactions',
    icon: 'M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5',
  },
  {
    to: '/budgets',
    label: 'Budgets',
    icon: 'M3 13.13C3 12.5 3.5 12 4.13 12h2.25c.62 0 1.12.5 1.12 1.13v6.75c0 .62-.5 1.12-1.13 1.12H4.13c-.63 0-1.13-.5-1.13-1.13v-6.75Zm6.75-4.5c0-.63.5-1.13 1.13-1.13h2.25c.62 0 1.12.5 1.12 1.13v11.25c0 .62-.5 1.12-1.13 1.12h-2.25c-.62 0-1.12-.5-1.12-1.13V8.63ZM16.5 4.13c0-.63.5-1.13 1.13-1.13h2.25c.62 0 1.12.5 1.12 1.13v15.75c0 .62-.5 1.12-1.13 1.12h-2.25c-.62 0-1.12-.5-1.12-1.13V4.13Z',
  },
  {
    to: '/categories',
    label: 'Categories',
    icon: 'M6 6.88V6a2.25 2.25 0 0 1 2.25-2.25h7.5A2.25 2.25 0 0 1 18 6v.88m-12 0c.24-.08.49-.13.75-.13h10.5c.26 0 .51.05.75.13m-12 0A2.25 2.25 0 0 0 4.5 9v.88m15-3A2.25 2.25 0 0 1 21 12v6a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 18v-6c0-.98.63-1.81 1.5-2.12',
  },
]

// Mobile bottom bar shows 4 tabs + a "More" sheet; these two live behind "More".
const mobilePrimary = computed(() => nav.filter((n) => n.to === '/' || n.to === '/accounts'))
const mobileSecondary = computed(() => nav.filter((n) => n.to === '/transactions'))
const moreRoutes = ['/budgets', '/categories']
const moreActive = computed(() => moreRoutes.includes(route.path))
const moreItems = computed(() => nav.filter((n) => moreRoutes.includes(n.to)))

const moreOpen = ref(false)

// ── Global action button (FAB) ────────────────────────────────────────────
// The FAB doubles as a categorize prompt: when uncategorized transactions
// exist it changes look and opens the categorize flow; otherwise it adds a new
// transaction.
const accounts = ref<Account[]>([])
const categories = ref<Category[]>([])
const base = ref('UZS')
const pending = ref<Transaction[]>([])
const quickOpen = ref(false)
const categorizeOpen = ref(false)
const fabBusy = ref(false)

const hasUncategorized = computed(() => pending.value.length > 0)

async function loadPending() {
  try {
    pending.value = await transactionsApi.list({ uncategorized: true, limit: 100 })
  } catch {
    pending.value = []
  }
}

async function loadMeta() {
  const [accs, cats, nw] = await Promise.all([
    accountsApi.list(),
    categoriesApi.list(),
    reportsApi.netWorth().catch(() => null),
  ])
  accounts.value = accs
  categories.value = cats
  if (nw) base.value = nw.base
}

async function onFab() {
  if (fabBusy.value) return
  moreOpen.value = false
  fabBusy.value = true
  try {
    await Promise.all([loadMeta(), loadPending()])
    if (hasUncategorized.value) categorizeOpen.value = true
    else quickOpen.value = true
  } finally {
    fabBusy.value = false
  }
}

// Let every visible view (and the FAB itself) reload after a change.
function broadcastRefresh() {
  window.dispatchEvent(new CustomEvent('data:refresh'))
}
function onQuickSaved() {
  quickOpen.value = false
  broadcastRefresh()
}
function onCategorizeClose() {
  categorizeOpen.value = false
  broadcastRefresh()
}

onMounted(() => window.addEventListener('data:refresh', loadPending))
onUnmounted(() => window.removeEventListener('data:refresh', loadPending))
onMounted(loadPending)
</script>

<template>
  <RouterView v-if="!showChrome" />

  <div v-else class="flex min-h-screen bg-slate-50 text-slate-800">
    <!-- sidebar (desktop) -->
    <aside class="sticky top-0 hidden h-screen w-64 shrink-0 flex-col bg-emerald-950 px-4 py-6 text-slate-300 md:flex">
      <div class="mb-7 flex items-center gap-2 px-2">
        <img src="/favicon.svg" alt="" class="h-9 w-9 rounded-xl bg-amber-400 p-1" />
        <span class="text-lg font-semibold tracking-tight text-white">Mullajiring</span>
      </div>

      <button
        class="btn mb-6 w-full"
        :class="hasUncategorized ? 'bg-emerald-800 text-white ring-1 ring-amber-400/40 hover:bg-emerald-700' : 'btn-primary'"
        :disabled="fabBusy"
        @click="onFab"
      >
        <template v-if="hasUncategorized">
          <i class="ti ti-tag text-base text-amber-400" />
          Categorize
          <span class="ml-0.5 rounded-full bg-amber-400 px-1.5 text-xs font-semibold text-slate-900">{{ pending.length }}</span>
        </template>
        <template v-else>
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2.2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          New transaction
        </template>
      </button>

      <nav class="flex flex-col gap-1">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="group flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition hover:bg-white/5 hover:text-white"
          exact-active-class="!bg-amber-400 !text-slate-900 hover:!bg-amber-400"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
          </svg>
          {{ item.label }}
        </RouterLink>
      </nav>

      <label class="mt-auto flex cursor-pointer items-center justify-between gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition hover:bg-white/5 hover:text-white">
        <span>Hide cents</span>
        <span class="relative inline-flex h-5 w-9 shrink-0">
          <input v-model="hideMinorUnits" type="checkbox" class="peer sr-only" />
          <span class="absolute inset-0 rounded-full bg-slate-600 transition peer-checked:bg-amber-400" />
          <span class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white transition peer-checked:translate-x-4" />
        </span>
      </label>

      <button
        class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition hover:bg-white/5 hover:text-white"
        @click="logout"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
        </svg>
        Sign out
      </button>
      <div class="mt-4 px-3 text-xs text-slate-600">Phase 1 · MVP</div>
    </aside>

    <!-- content -->
    <div class="min-w-0 flex-1">
      <!-- mobile top bar -->
      <header class="sticky top-0 z-30 flex items-center justify-between border-b border-slate-200/70 bg-slate-50/90 px-4 py-3 backdrop-blur md:hidden">
        <span class="flex items-center gap-2 text-lg font-semibold">
          <img src="/favicon.svg" alt="" class="h-7 w-7 rounded-lg bg-amber-400 p-0.5" />
          Mullajiring
        </span>
        <label class="flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-500">
          <span>Hide cents</span>
          <span class="relative inline-flex h-5 w-9 shrink-0">
            <input v-model="hideMinorUnits" type="checkbox" class="peer sr-only" />
            <span class="absolute inset-0 rounded-full bg-slate-300 transition peer-checked:bg-amber-400" />
            <span class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white shadow transition peer-checked:translate-x-4" />
          </span>
        </label>
      </header>

      <main class="mx-auto max-w-6xl px-4 py-6 pb-28 sm:px-6 md:py-8 md:pb-8 lg:px-10">
        <RouterView />
      </main>
    </div>

    <!-- mobile bottom nav: Dashboard · Accounts · (+) · Transactions · More -->
    <nav
      class="fixed inset-x-0 bottom-0 z-30 flex items-stretch border-t border-slate-200 bg-white/95 backdrop-blur md:hidden"
      style="padding-bottom: env(safe-area-inset-bottom)"
    >
      <RouterLink
        v-for="item in mobilePrimary"
        :key="item.to"
        :to="item.to"
        class="flex flex-1 flex-col items-center gap-1 px-1 py-2.5 text-[10px] leading-tight font-medium text-slate-400 transition"
        exact-active-class="!text-slate-900"
      >
        <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
        </svg>
        {{ item.label }}
      </RouterLink>

      <!-- center FAB: doubles as the categorize prompt when items are pending -->
      <div class="flex w-16 shrink-0 justify-center">
        <button
          class="relative -mt-5 grid h-14 w-14 place-items-center rounded-full ring-4 ring-slate-50 transition active:scale-95"
          :class="hasUncategorized ? 'bg-emerald-950 text-amber-400 shadow-lg shadow-emerald-950/40' : 'bg-amber-400 text-slate-900 shadow-lg shadow-amber-400/40'"
          :disabled="fabBusy"
          :aria-label="hasUncategorized ? 'Categorize transactions' : 'New transaction'"
          @click="onFab"
        >
          <i v-if="hasUncategorized" class="ti ti-tag text-2xl" />
          <svg v-else class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke-width="2.4" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          <span
            v-if="hasUncategorized"
            class="absolute -top-1 -right-1 grid h-5 min-w-5 place-items-center rounded-full bg-amber-400 px-1 text-xs font-bold text-slate-900 ring-2 ring-white"
          >{{ pending.length }}</span>
        </button>
      </div>

      <RouterLink
        v-for="item in mobileSecondary"
        :key="item.to"
        :to="item.to"
        class="flex flex-1 flex-col items-center gap-1 px-1 py-2.5 text-[10px] leading-tight font-medium text-slate-400 transition"
        exact-active-class="!text-slate-900"
      >
        <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
        </svg>
        {{ item.label }}
      </RouterLink>

      <button
        class="flex flex-1 flex-col items-center gap-1 px-1 py-2.5 text-[10px] leading-tight font-medium transition"
        :class="moreActive ? 'text-slate-900' : 'text-slate-400'"
        @click="moreOpen = true"
      >
        <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
        </svg>
        More
      </button>
    </nav>

    <!-- "More" bottom sheet (mobile) -->
    <Teleport to="body">
      <div v-if="moreOpen" class="fixed inset-0 z-40 md:hidden">
        <div class="absolute inset-0 bg-slate-900/40 backdrop-blur-sm" style="animation: fade-in 0.15s ease-out" @click="moreOpen = false" />
        <div
          class="absolute inset-x-0 bottom-0 rounded-t-3xl bg-white p-4 pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl"
          style="animation: sheet-up 0.22s cubic-bezier(0.16, 1, 0.3, 1)"
        >
          <div class="mx-auto mb-4 h-1.5 w-10 rounded-full bg-slate-200" />
          <nav class="grid grid-cols-2 gap-3">
            <RouterLink
              v-for="item in moreItems"
              :key="item.to"
              :to="item.to"
              class="flex flex-col items-start gap-3 rounded-2xl bg-slate-50 p-4 text-sm font-semibold text-slate-700 ring-1 ring-slate-200/70 transition active:scale-[0.98]"
              @click="moreOpen = false"
            >
              <span class="grid h-10 w-10 place-items-center rounded-xl bg-amber-400 text-slate-900">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
                </svg>
              </span>
              {{ item.label }}
            </RouterLink>
          </nav>

          <div class="mt-4 space-y-1">
            <label class="flex cursor-pointer items-center justify-between rounded-xl px-2 py-3 text-sm font-medium text-slate-600">
              <span>Hide cents</span>
              <span class="relative inline-flex h-5 w-9 shrink-0">
                <input v-model="hideMinorUnits" type="checkbox" class="peer sr-only" />
                <span class="absolute inset-0 rounded-full bg-slate-300 transition peer-checked:bg-amber-400" />
                <span class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white shadow transition peer-checked:translate-x-4" />
              </span>
            </label>
            <button class="flex w-full items-center gap-2 rounded-xl px-2 py-3 text-sm font-medium text-rose-600" @click="logout">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
              </svg>
              Sign out
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- global quick-add -->
    <TransactionForm
      v-if="quickOpen"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="quickOpen = false"
      @saved="onQuickSaved"
    />

    <!-- global categorize flow (opened from the FAB when items are pending) -->
    <CategorizeModal
      v-if="categorizeOpen"
      :transactions="pending"
      :accounts="accounts"
      :categories="categories"
      :base="base"
      @close="onCategorizeClose"
    />
  </div>
</template>
