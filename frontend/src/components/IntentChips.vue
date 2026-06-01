<script setup lang="ts">
import { NTag, NSpace, NText } from 'naive-ui'
import type { Intent } from '@/types'
import { useConversationStore } from '@/stores/conversation'

// F3: render the parsed intent as removable chips. Editing here mutates the
// shared intent so the next search/refine uses the trimmed conditions.
const convo = useConversationStore()

const groups: { key: keyof Intent; label: string }[] = [
  { key: 'moods', label: '情绪' },
  { key: 'genres', label: '风格' },
  { key: 'instruments', label: '乐器' },
  { key: 'keywords', label: '关键词' },
  { key: 'seed_artists', label: '种子歌手' },
]

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
        <n-text depth="3" class="lbl">{{ g.label }}</n-text>
        <n-space size="small" :wrap="true">
          <n-tag
            v-for="(v, i) in list(g.key)"
            :key="v + i"
            size="small"
            closable
            @close="removeAt(g.key, i)"
          >
            {{ v }}
          </n-tag>
        </n-space>
      </div>
    </template>

    <div v-if="convo.intent.tempo" class="group">
      <n-text depth="3" class="lbl">节奏</n-text>
      <n-tag size="small" closable @close="convo.intent.tempo = ''">{{ convo.intent.tempo }}</n-tag>
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
  line-height: 22px;
}
</style>
