<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NInput, NScrollbar, NSpin, NText } from 'naive-ui'
import IntentChips from './IntentChips.vue'
import { useConversationStore } from '@/stores/conversation'
import { usePlaylistStore } from '@/stores/playlist'
import { useRecommendation } from '@/composables/useRecommendation'

const convo = useConversationStore()
const playlist = usePlaylistStore()
const rec = useRecommendation()

const input = ref('')
const seeds = ref('')

function send() {
  const text = input.value.trim()
  if (!text || rec.loading.value) return
  const seedList = seeds.value.split(/[,，、]/).map((s) => s.trim()).filter(Boolean)

  // First turn → full pipeline; later turns → F10 refinement.
  if (playlist.hasResult) rec.refine(text)
  else rec.submit(text, seedList)

  input.value = ''
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    send()
  }
}
</script>

<template>
  <div class="pane">
    <header class="pane-head">
      <strong>对话 · 懂你 &amp; 微调</strong>
      <n-text depth="3" style="font-size: 12px">左边“指挥”，右边“验收”</n-text>
    </header>

    <n-scrollbar class="stream">
      <!-- Empty state: a short example to break the blank page (F2) -->
      <div v-if="!convo.messages.length" class="welcome">
        <p>用自然语言描述你想听的，比如“适合雨天加班的慵懒爵士，别太吵”。</p>
      </div>

      <div v-for="m in convo.messages" :key="m.id" class="msg" :class="m.role">
        <div class="bubble">{{ m.text }}</div>
      </div>

      <div v-if="rec.loading.value" class="msg assistant">
        <div class="bubble loading">
          <n-spin :size="14" /> <span>{{ rec.stage.value || '思考中…' }}</span>
        </div>
      </div>
    </n-scrollbar>

    <IntentChips />

    <div v-if="convo.hasIntent" class="intent-actions">
      <n-button size="tiny" tertiary :disabled="rec.loading.value" @click="rec.researchFromIntent()">
        用编辑后的条件重搜
      </n-button>
    </div>

    <div v-if="rec.lastError.value && !rec.loading.value" class="retry-bar">
      <n-text depth="3" style="font-size: 12px">上一步失败</n-text>
      <n-button size="tiny" type="primary" tertiary @click="rec.retry()">重试</n-button>
    </div>

    <footer class="composer">
      <n-input
        v-model:value="seeds"
        size="small"
        placeholder="可选：种子歌手/歌曲（用逗号分隔）"
        style="margin-bottom: 8px"
      />
      <n-input
        v-model:value="input"
        type="textarea"
        :autosize="{ minRows: 2, maxRows: 5 }"
        placeholder="描述心情/场景，或追加微调（如“去掉有歌词的”）。⌘/Ctrl+Enter 发送"
        @keydown="onKeydown"
      />
      <div class="send-row">
        <n-text depth="3" style="font-size: 12px">
          {{ playlist.hasResult ? '将作为微调，更新右侧列表' : '将生成一批候选' }}
        </n-text>
        <n-button type="primary" :loading="rec.loading.value" :disabled="!input.trim()" @click="send">
          发送
        </n-button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 12px;
}
.pane-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.stream {
  flex: 1 1 auto;
  min-height: 0;
}
.welcome {
  color: #555;
  font-size: 14px;
}
.welcome p {
  margin-top: 4px;
}
.msg {
  display: flex;
  margin: 8px 0;
}
.msg.user {
  justify-content: flex-end;
}
.bubble {
  max-width: 86%;
  padding: 8px 12px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}
.msg.user .bubble {
  background: var(--brand);
  color: #fff;
}
.msg.assistant .bubble {
  background: #fff;
  border: 1px solid #eee;
}
.bubble.loading {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #888;
}
.intent-actions {
  display: flex;
  justify-content: flex-end;
}
.retry-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: #fff7f8;
  border: 1px solid #ffe0e4;
  border-radius: 8px;
}
.composer {
  flex: 0 0 auto;
}
.send-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
}
</style>
