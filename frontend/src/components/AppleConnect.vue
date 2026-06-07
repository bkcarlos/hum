<script setup lang="ts">
import { computed } from 'vue'
import { NTooltip, NSpin } from 'naive-ui'
import { useAppleStore } from '@/stores/apple'

// Apple Music 连接：一个音符图标开关。状态靠图标(已连=品牌色实心+绿点 / 未连=灰描边)，
// 点一下直接切换连/断；状态文案放 tooltip，不在顶栏写「已连接」字样。
// 连接仅用于完整播放 + 建歌单；出推荐 / 30s 试听不需要它。
const apple = useAppleStore()

const tip = computed(() =>
  apple.connecting
    ? '连接中…'
    : apple.authorized
      ? `已连接 Apple Music · ${apple.storefront.toUpperCase()}（点击断开）`
      : '连接 Apple Music（完整播放 / 建歌单用）',
)

async function toggle() {
  if (apple.connecting) return
  if (apple.authorized) await apple.disconnect()
  else await apple.connect()
}
</script>

<template>
  <n-tooltip>
    <template #trigger>
      <button
        type="button"
        class="am-toggle"
        :class="{ on: apple.authorized }"
        :disabled="apple.connecting"
        :aria-label="tip"
        :aria-pressed="apple.authorized"
        @click="toggle"
      >
        <n-spin v-if="apple.connecting" :size="14" />
        <template v-else>
          <svg class="am-note" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="currentColor"
              d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z"
            />
          </svg>
          <span v-if="apple.authorized" class="dot" aria-hidden="true" />
        </template>
      </button>
    </template>
    {{ tip }}
  </n-tooltip>
</template>

<style scoped>
.am-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 1px solid #e3e3e6;
  background: #fff;
  color: #b3b3b8; /* 未连：灰 */
  cursor: pointer;
  transition: all 0.15s ease;
}
.am-toggle:hover {
  background: #fafafa;
}
.am-toggle.on {
  color: #fa2d48; /* 已连：品牌色 */
  border-color: rgba(250, 45, 72, 0.35);
  background: rgba(250, 45, 72, 0.08);
}
.am-toggle:disabled {
  opacity: 0.6;
  cursor: default;
}
.am-note {
  width: 17px;
  height: 17px;
}
.dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #34c759; /* 绿：已连接 */
  border: 1.5px solid #fff;
}
</style>
