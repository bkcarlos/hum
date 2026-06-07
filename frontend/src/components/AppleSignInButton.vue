<script setup lang="ts">
// Apple 官方风格「通过 Apple 登录」按钮（按 HIG：黑底、白 Apple 标志 + 文案、圆角）。
// 点击仅 emit('click')，登录逻辑由父组件接（复用现有 appleSignIn 弹窗流程）。
defineProps<{ loading?: boolean }>()
const emit = defineEmits<{ (e: 'click'): void }>()
</script>

<template>
  <button type="button" class="apple-btn" :disabled="loading" @click="emit('click')">
    <svg class="apple-logo" viewBox="0 0 384 512" aria-hidden="true">
      <path
        fill="currentColor"
        d="M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z"
      />
    </svg>
    <span>{{ loading ? '登录中…' : '通过 Apple 登录' }}</span>
  </button>
</template>

<style scoped>
.apple-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 34px;
  padding: 0 16px;
  border: none;
  border-radius: 8px;
  background: #000;
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  line-height: 1;
  cursor: pointer;
  white-space: nowrap;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', sans-serif;
}
.apple-btn:hover {
  background: #1a1a1a;
}
.apple-btn:disabled {
  opacity: 0.55;
  cursor: default;
}
.apple-logo {
  width: 15px;
  height: 18px;
  margin-top: -2px; /* 视觉居中 */
}
</style>
