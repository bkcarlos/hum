<script setup lang="ts">
import { NButton, NTag, NTooltip } from 'naive-ui'
import { useAppleStore } from '@/stores/apple'

const apple = useAppleStore()
</script>

<template>
  <div class="apple-connect">
    <template v-if="apple.authorized">
      <n-tag type="success" round size="small">已连接 · {{ apple.storefront.toUpperCase() }}</n-tag>
      <n-button text size="tiny" @click="apple.disconnect()">断开</n-button>
    </template>

    <n-tooltip v-else trigger="hover">
      <template #trigger>
        <n-button type="default" size="small" :loading="apple.connecting" @click="apple.connect()">
          连接 Apple Music
        </n-button>
      </template>
      {{ apple.error || '用于完整播放、把歌单存进资料库；出推荐与 30s 试听无需连接。' }}
    </n-tooltip>
  </div>
</template>

<style scoped>
.apple-connect {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
