<script setup lang="ts">
import { NConfigProvider, NMessageProvider, zhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import AdminPanel from '@/components/admin/AdminPanel.vue'

// Match the main app's Apple-Music-red accent.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#fa2d48',
    primaryColorHover: '#ff4d63',
    primaryColorPressed: '#d61f38',
    borderRadius: '10px',
  },
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides" :locale="zhCN">
    <n-message-provider>
      <AdminPanel />
    </n-message-provider>
  </n-config-provider>
</template>
