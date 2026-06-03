<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { NButton, NInput, NScrollbar, NSpin, NText } from 'naive-ui'
import IntentChips from './IntentChips.vue'
import * as api from '@/api/client'
import { useConversationStore } from '@/stores/conversation'
import { usePlaylistStore } from '@/stores/playlist'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { useRecommendation } from '@/composables/useRecommendation'
import { nowContext, pickExamples, topTastes } from '@/data/personalize'

const convo = useConversationStore()
const playlist = usePlaylistStore()
const llm = useLlmConfigStore()
const rec = useRecommendation()

const input = ref('')
const seeds = ref('')

// Empty-state inspiration: a pool of natural-language prompts (not atomic tags)
// that show what to type and play to Option A's strength. We surface a few —
// without an LLM key, picked & ranked by local context + taste history
// (pickExamples, no model); with a key, the LLM writes fresh phrasings instead.
// Clicking one fills the composer so the user can edit before sending.
const EXAMPLE_POOL = [
  '适合雨天加班的慵懒爵士，别太吵',
  '深夜一个人开车，放空的氛围电子',
  '周末早晨做早餐，轻快有活力',
  '健身冲刺，强节奏的快歌',
  '失恋后一个人，温柔的抒情慢歌',
  '咖啡馆看书，慵懒的 bossa nova',
  '通勤路上提神，干净的独立流行',
  '睡前放松，纯钢琴轻音乐',
  '派对热场，复古 disco / funk',
  '专注写代码，无人声 lo-fi',
]
const examples = ref<string[]>(pickExamples(EXAMPLE_POOL, 4))
const exLoading = ref(false)
const SESSION_KEY = 'hum.examples.session'

// 千人千面: when an LLM key is configured, generate examples personalized to the
// current context + recent local tastes; otherwise (or on any failure) fall back
// to a random draw from the static pool. Cached per browser session so we don't
// re-call on every reload; "换一批" forces a fresh generation.
async function refreshExamples(force = false) {
  if (!llm.configured) {
    examples.value = pickExamples(EXAMPLE_POOL, 4)
    return
  }
  if (!force) {
    try {
      const cached = JSON.parse(sessionStorage.getItem(SESSION_KEY) || 'null')
      if (Array.isArray(cached) && cached.length) {
        examples.value = cached
        return
      }
    } catch {
      /* ignore bad cache */
    }
  }
  exLoading.value = true
  try {
    const { examples: ex } = await api.genExamples(llm.body, llm.apiKey, nowContext(), topTastes(8), 4)
    if (ex && ex.length) {
      examples.value = ex
      sessionStorage.setItem(SESSION_KEY, JSON.stringify(ex))
    }
  } catch {
    /* keep current / pool fallback — generation is best-effort */
  } finally {
    exLoading.value = false
  }
}
function shuffleExamples() {
  if (llm.configured) refreshExamples(true)
  else examples.value = pickExamples(EXAMPLE_POOL, 4)
}
function useExample(t: string) {
  input.value = t
}

onMounted(() => refreshExamples())
watch(
  () => llm.configured,
  (ok) => {
    if (ok) refreshExamples()
  },
)

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
  if (e.key !== 'Enter') return
  if (e.isComposing) return // mid-IME composition (Chinese input) — Enter confirms a candidate, never sends
  if (e.shiftKey) return // Shift+Enter = newline
  e.preventDefault()
  send()
}
</script>

<template>
  <div class="pane">
    <header class="pane-head">
      <strong>对话 · 懂你 &amp; 微调</strong>
      <n-text depth="3" style="font-size: 12px">左边“指挥”，右边“验收”</n-text>
    </header>

    <n-scrollbar class="stream">
      <!-- Empty state: hero + tappable example prompts + how-it-works (F2) -->
      <div v-if="!convo.messages.length" class="welcome">
        <div class="hero-title">用一句话，描述你想听的</div>
        <p class="hero-sub">Hum 从 Apple Music 真实曲库帮你挑歌、试听、一键建成歌单。</p>

        <div class="examples">
          <div class="ex-head">
            <n-text depth="3" class="ex-label">试试这样说</n-text>
            <button type="button" class="shuffle" :disabled="exLoading" @click="shuffleExamples">
              {{ exLoading ? '生成中…' : '换一批 ⟳' }}
            </button>
          </div>
          <button v-for="ex in examples" :key="ex" type="button" class="ex" @click="useExample(ex)">
            {{ ex }}
          </button>
        </div>

        <ol class="steps">
          <li>描述心情 / 场景</li>
          <li>AI 选歌，你试听、勾选</li>
          <li>一键建成歌单</li>
        </ol>
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
        placeholder="描述心情/场景，或追加微调（如“去掉有歌词的”）。Enter 发送，Shift+Enter 换行"
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
  padding: 12px 2px 4px;
}
.hero-title {
  font-size: 18px;
  font-weight: 700;
  color: #1f2225;
}
.hero-sub {
  margin: 6px 0 0;
  font-size: 13px;
  color: #8a8a8a;
  line-height: 1.6;
}
.examples {
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.ex-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.ex-label {
  font-size: 12px;
}
.shuffle {
  font: inherit;
  font-size: 12px;
  color: #b0344b;
  background: none;
  border: none;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 6px;
}
.shuffle:hover {
  background: #fff0f2;
}
.shuffle:disabled {
  opacity: 0.5;
  cursor: default;
}
.ex {
  display: block;
  width: 100%;
  text-align: left;
  font: inherit;
  font-size: 13px;
  color: #4a4a4a;
  background: #fafafa;
  border: 1px solid #ececec;
  border-radius: 10px;
  padding: 9px 12px;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.ex:hover {
  border-color: var(--brand);
  color: var(--brand);
  background: #fff5f6;
}
.steps {
  margin: 20px 0 0;
  padding-left: 18px;
  color: #9a9a9a;
  font-size: 12px;
  line-height: 1.9;
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
