import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import AdminApp from './AdminApp.vue'
import { useLlmConfigStore } from '@/stores/llmConfig'
import './styles.css'

// No vue-router: the protected admin backstage (C4) is a separate root mounted
// only on /admin (served via the SPA fallback). On /admin we mount AdminApp and
// skip restoring the BYOK key store entirely — the admin page never touches it.
const isAdmin = window.location.pathname.startsWith('/admin')

const pinia = createPinia()
const app = createApp(isAdmin ? AdminApp : App)
app.use(pinia)

if (!isAdmin) {
  // Restore the user's BYOK config (incl. key) from this browser's localStorage
  // before first paint, so a returning user is immediately ready (F0 验收 #2).
  useLlmConfigStore(pinia).load()
}

app.mount('#app')
