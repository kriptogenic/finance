<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearCredentials } from './api/auth'

const route = useRoute()
const router = useRouter()

// The login screen renders standalone, without the app chrome.
const showChrome = computed(() => route.name !== 'login')

function logout() {
  clearCredentials()
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
</script>

<template>
  <RouterView v-if="!showChrome" />

  <div v-else class="flex min-h-screen bg-slate-100 text-slate-800">
    <!-- sidebar -->
    <aside class="sticky top-0 hidden h-screen w-64 shrink-0 flex-col bg-slate-900 px-4 py-6 text-slate-300 md:flex">
      <div class="mb-8 flex items-center gap-2 px-2">
        <img src="/favicon.svg" alt="" class="h-9 w-9 rounded-xl bg-white p-1" />
        <span class="text-lg font-semibold tracking-tight text-white">Mullajiring</span>
      </div>

      <nav class="flex flex-col gap-1">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="group flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition hover:bg-white/5 hover:text-white"
          exact-active-class="!bg-white/10 !text-white"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
          </svg>
          {{ item.label }}
        </RouterLink>
      </nav>

      <button
        class="mt-auto flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-400 transition hover:bg-white/5 hover:text-white"
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
    <div class="flex-1">
      <!-- mobile top bar -->
      <header class="sticky top-0 z-30 flex items-center justify-between border-b border-slate-200 bg-white/90 px-4 py-3 backdrop-blur md:hidden">
        <span class="flex items-center gap-2 text-lg font-semibold">
          <img src="/favicon.svg" alt="" class="h-7 w-7" />
          Mullajiring
        </span>
        <button
          class="grid h-9 w-9 place-items-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-slate-700"
          aria-label="Sign out"
          @click="logout"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
          </svg>
        </button>
      </header>

      <main class="mx-auto max-w-6xl px-4 py-6 pb-24 sm:px-6 md:py-8 md:pb-8 lg:px-10">
        <RouterView />
      </main>
    </div>

    <!-- mobile bottom nav -->
    <nav
      class="fixed inset-x-0 bottom-0 z-30 flex border-t border-slate-200 bg-white/95 backdrop-blur md:hidden"
      style="padding-bottom: env(safe-area-inset-bottom)"
    >
      <RouterLink
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="flex flex-1 flex-col items-center gap-1 px-1 py-2 text-[10px] leading-tight font-medium text-slate-400 transition"
        exact-active-class="!text-indigo-600"
      >
        <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
        </svg>
        {{ item.label }}
      </RouterLink>
    </nav>
  </div>
</template>
