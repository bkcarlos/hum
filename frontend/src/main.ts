import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { useLlmConfigStore } from '@/stores/llmConfig'
import './styles.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)

// Restore the user's BYOK config (incl. key) from this browser's localStorage
// before first paint, so a returning user is immediately ready (F0 验收 #2).
useLlmConfigStore(pinia).load()

app.mount('#app')
