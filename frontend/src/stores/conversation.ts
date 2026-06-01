import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ChatMessage, Intent } from '@/types'
import { emptyIntent } from '@/types'

// Left column state (F2/F3/F10): the chat transcript plus the current,
// user-editable parsed Intent.
export const useConversationStore = defineStore('conversation', () => {
  const messages = ref<ChatMessage[]>([])
  const intent = ref<Intent>(emptyIntent())
  const hasIntent = ref(false)
  let seq = 0

  function addUser(text: string) {
    messages.value.push({ id: ++seq, role: 'user', text })
  }
  function addAssistant(text: string) {
    messages.value.push({ id: ++seq, role: 'assistant', text })
  }
  function setIntent(i: Intent) {
    intent.value = i
    hasIntent.value = true
  }
  function reset() {
    messages.value = []
    intent.value = emptyIntent()
    hasIntent.value = false
    seq = 0
  }

  return { messages, intent, hasIntent, addUser, addAssistant, setIntent, reset }
})
