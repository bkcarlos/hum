<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useAppleStore } from '@/stores/apple'

const apple = useAppleStore()
</script>

<template>
  <!-- Only the connected status is shown here. Connecting happens during free-tier
       login (auto-connect) or on demand when the user hits 完整播放 / 建成歌单, so
       there's no separate "连接 Apple Music" button competing in the topbar. -->
  <div v-if="apple.authorized" class="apple-connect">
    <n-tag type="success" round size="small">已连接 · {{ apple.storefront.toUpperCase() }}</n-tag>
    <n-button text size="tiny" @click="apple.disconnect()">断开</n-button>
  </div>
</template>

<style scoped>
.apple-connect {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
