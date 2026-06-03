<script setup lang="ts">
import { NTag } from 'naive-ui'
import type { Intent } from '@/types'
import { useConversationStore } from '@/stores/conversation'

// F3: render the parsed intent as removable chips, color-coded by category so the
// conditions are scannable. Editing here mutates the shared intent so the next
// search/refine uses the trimmed conditions.
const convo = useConversationStore()

interface Group {
  key: keyof Intent
  label: string
  color: { color: string; textColor: string }
}
const groups: Group[] = [
  { key: 'moods', label: '情绪', color: { color: '#fff0f2', textColor: '#c0395a' } },
  { key: 'genres', label: '风格', color: { color: '#eef2ff', textColor: '#3a5fb0' } },
  { key: 'instruments', label: '乐器', color: { color: '#fff6e8', textColor: '#a9761c' } },
  { key: 'keywords', label: '关键词', color: { color: '#f2f3f5', textColor: '#5a5f66' } },
  { key: 'seed_artists', label: '种子歌手', color: { color: '#eafaf0', textColor: '#2f8a4e' } },
]
const tempoText: Record<string, string> = { slow: '慢节奏', medium: '中速', fast: '快节奏' }
const tempoColor = { color: '#f4f0ff', textColor: '#6a4bb0' }

function list(key: keyof Intent): string[] {
  return convo.intent[key] as string[]
}
function removeAt(key: keyof Intent, i: number) {
  list(key).splice(i, 1)
}
</script>

<template>
  <div v-if="convo.hasIntent" class="intent">
    <template v-for="g in groups" :key="g.key">
      <div v-if="list(g.key).length" class="group">
        <span class="lbl">{{ g.label }}</span>
        <div class="chips">
          <n-tag
            v-for="(v, i) in list(g.key)"
            :key="v + i"
            size="small"
            round
            :bordered="false"
            closable
            :color="g.color"
            @close="removeAt(g.key, i)"
          >
            {{ v }}
          </n-tag>
        </div>
      </div>
    </template>

    <div v-if="convo.intent.tempo" class="group">
      <span class="lbl">节奏</span>
      <div class="chips">
        <n-tag size="small" round :bordered="false" closable :color="tempoColor" @close="convo.intent.tempo = ''">
          {{ tempoText[convo.intent.tempo] ?? convo.intent.tempo }}
        </n-tag>
      </div>
    </div>
  </div>
</template>

<style scoped>
.intent {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 10px;
}
.group {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.lbl {
  flex: 0 0 52px;
  font-size: 12px;
  line-height: 24px;
  color: #9a9a9a;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
