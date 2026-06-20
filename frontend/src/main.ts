import { registerSW } from 'virtual:pwa-register'
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './assets/tabler/tabler-icons.css'
import './style.css'

// autoUpdate: reload the page once the new SW takes control, so a deploy shows
// up on the next load instead of after several manual refreshes.
registerSW({ immediate: true })

createApp(App).use(router).mount('#app')
