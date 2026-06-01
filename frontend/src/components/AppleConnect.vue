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

    <n-tooltip v-else trigger="hover" :disabled="!apple.error">
      <template #trigger>
        <n-button type="primary" size="small" :loading="apple.connecting" @click="apple.connect()">
          连接 Apple Music
        </n-button>
      </template>
      {{ apple.error }}
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
