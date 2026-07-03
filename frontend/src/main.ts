import { registerSW } from 'virtual:pwa-register'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './assets/tabler/tabler-icons.css'
import './style.css'

// autoUpdate: reload the page once the new SW takes control, so a deploy shows
// up on the next load instead of after several manual refreshes.
registerSW({ immediate: true })

// staleTime: 0 => every mount/navigation refetches in the background while the
// cached (old) data renders immediately. gcTime (default 5m) keeps that cache
// warm between visits so revisiting a page never shows a blank/spinner.
const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 0, refetchOnWindowFocus: false, retry: 1 } },
})

// Bridge the existing global refresh event (FAB quick-add, categorize modal) to
// vue-query: invalidate everything so all active queries refetch.
window.addEventListener('data:refresh', () => queryClient.invalidateQueries())

createApp(App).use(VueQueryPlugin, { queryClient }).use(router).mount('#app')
