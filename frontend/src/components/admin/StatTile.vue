<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  hint?: string
  tone?: 'default' | 'success' | 'error' | 'accent'
}>()
</script>

<template>
  <div class="tile" :class="`tone-${tone ?? 'default'}`">
    <div class="label">{{ label }}</div>
    <div class="value">{{ value }}</div>
    <div v-if="hint" class="hint">{{ hint }}</div>
  </div>
</template>

<style scoped>
.tile {
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 16px;
  padding: 18px 20px;
  min-width: 0;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}
.label {
  font-size: 12px;
  color: #86868b;
  margin-bottom: 10px;
  letter-spacing: 0.02em;
}
.value {
  font-size: 30px;
  font-weight: 700;
  line-height: 1.05;
  color: #1d1d1f;
  letter-spacing: -0.01em;
}
.hint {
  font-size: 12px;
  color: #98989d;
  margin-top: 8px;
}
.tone-accent .value {
  color: #fa2d48;
}
.tone-success .value {
  color: #18a058;
}
.tone-error .value {
  color: #d03050;
}
</style>
