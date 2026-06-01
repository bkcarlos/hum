<script setup lang="ts">
import { NButton, NCheckbox, NTag } from 'naive-ui'
import type { PlaylistItem } from '@/stores/playlist'

defineProps<{
  item: PlaylistItem
  selected: boolean
  current: boolean
  playing: boolean
}>()
const emit = defineEmits<{ toggleSelect: []; togglePlay: [] }>()
</script>

<template>
  <div class="row" :class="{ current }">
    <n-checkbox :checked="selected" @update:checked="emit('toggleSelect')" />

    <img v-if="item.song.artworkUrl" :src="item.song.artworkUrl" class="art" alt="" loading="lazy" />
    <div v-else class="art placeholder">♫</div>

    <div class="meta">
      <div class="line1">
        <span class="title" :title="item.song.title">{{ item.song.title }}</span>
        <n-tag v-if="item.status === 'new'" size="tiny" type="info" :bordered="false">新</n-tag>
        <n-tag v-else size="tiny" :bordered="false">保留</n-tag>
      </div>
      <div class="artist">{{ item.song.artist }}</div>
      <div v-if="item.reason" class="reason">{{ item.reason }}</div>
    </div>

    <n-button
      circle
      size="small"
      :disabled="!item.song.previewUrl"
      :title="item.song.previewUrl ? '试听' : '暂无预览'"
      @click="emit('togglePlay')"
    >
      {{ current && playing ? '⏸' : '▶' }}
    </n-button>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  border-radius: 10px;
  transition: background 0.15s;
}
.row:hover {
  background: #f4f4f5;
}
.row.current {
  background: #fff0f2;
}
.art {
  width: 44px;
  height: 44px;
  border-radius: 6px;
  object-fit: cover;
  flex: 0 0 auto;
}
.art.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ececef;
  color: #aaa;
  font-size: 20px;
}
.meta {
  flex: 1 1 auto;
  min-width: 0;
}
.line1 {
  display: flex;
  align-items: center;
  gap: 6px;
}
.title {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.artist {
  font-size: 12px;
  color: #888;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.reason {
  font-size: 12px;
  color: #b0344b;
  margin-top: 2px;
}
</style>
