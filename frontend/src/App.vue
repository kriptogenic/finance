<script setup lang="ts">
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
    to: '/categories',
    label: 'Categories',
    icon: 'M6 6.88V6a2.25 2.25 0 0 1 2.25-2.25h7.5A2.25 2.25 0 0 1 18 6v.88m-12 0c.24-.08.49-.13.75-.13h10.5c.26 0 .51.05.75.13m-12 0A2.25 2.25 0 0 0 4.5 9v.88m15-3A2.25 2.25 0 0 1 21 12v6a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 18v-6c0-.98.63-1.81 1.5-2.12',
  },
]
</script>

<template>
  <div class="flex min-h-screen bg-slate-100 text-slate-800">
    <!-- sidebar -->
    <aside class="sticky top-0 hidden h-screen w-64 shrink-0 flex-col bg-slate-900 px-4 py-6 text-slate-300 md:flex">
      <div class="mb-8 flex items-center gap-2 px-2">
        <span class="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 text-lg">💰</span>
        <span class="text-lg font-semibold tracking-tight text-white">Finance</span>
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

      <div class="mt-auto px-3 text-xs text-slate-600">Phase 1 · MVP</div>
    </aside>

    <!-- content -->
    <div class="flex-1">
      <!-- mobile top bar -->
      <header class="flex items-center gap-4 border-b border-slate-200 bg-white px-5 py-3 md:hidden">
        <span class="text-lg font-semibold">💰 Finance</span>
        <nav class="flex gap-1 overflow-x-auto text-sm">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            class="rounded-lg px-3 py-1.5 font-medium text-slate-500"
            exact-active-class="bg-slate-900 text-white"
          >
            {{ item.label }}
          </RouterLink>
        </nav>
      </header>

      <main class="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <RouterView />
      </main>
    </div>
  </div>
</template>
