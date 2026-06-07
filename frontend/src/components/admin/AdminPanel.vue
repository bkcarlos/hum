<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NSpin } from 'naive-ui'
import { useAdminStore } from '@/stores/admin'
import { adminMe } from '@/api/client'
import AdminLogin from './AdminLogin.vue'
import OverviewSection from './OverviewSection.vue'
import QuotaConfigCard from './QuotaConfigCard.vue'
import UsageCard from './UsageCard.vue'

const admin = useAdminStore()

type Phase = 'checking' | 'login' | 'ready'
const phase = ref<Phase>('checking')

type Section = 'overview' | 'config' | 'usage'
const section = ref<Section>('overview')
const nav: { key: Section; label: string; icon: string }[] = [
  { key: 'overview', label: '概览', icon: '◎' },
  { key: 'config', label: '配额配置', icon: '⚙' },
  { key: 'usage', label: '用量与封禁', icon: '☷' },
]

// Verify the stored session against the admin gate on first load.
async function verify() {
  phase.value = 'checking'
  try {
    const { sub, email } = await adminMe(admin.session)
    admin.sub = sub
    admin.email = email
    phase.value = 'ready'
  } catch {
    admin.clear()
    phase.value = 'login'
  }
}

function onLogout() {
  admin.clear()
  phase.value = 'login'
}

onMounted(() => {
  if (admin.hasSession) verify()
  else phase.value = 'login'
})
</script>

<template>
  <div v-if="phase === 'checking'" class="full-center"><n-spin size="large" /></div>

  <AdminLogin v-else-if="phase === 'login'" @authed="phase = 'ready'" />

  <div v-else class="shell">
    <aside class="sidebar">
      <div class="brand">
        <img class="logo" src="/logo.svg" alt="Hum" width="32" height="32" />
        <div class="brand-text">
          <div class="bn">Hum</div>
          <div class="bt">管理后台</div>
        </div>
      </div>

      <nav class="nav">
        <button
          v-for="n in nav"
          :key="n.key"
          class="nav-item"
          :class="{ active: section === n.key }"
          @click="section = n.key"
        >
          <span class="nav-icon">{{ n.icon }}</span>
          <span>{{ n.label }}</span>
        </button>
      </nav>

      <div class="spacer" />

      <div class="who">
        <div class="who-label">已登录</div>
        <div class="who-sub" :title="admin.email || admin.sub">{{ admin.email || admin.sub }}</div>
        <button class="logout" @click="onLogout">退出登录</button>
      </div>
    </aside>

    <main class="content">
      <OverviewSection v-if="section === 'overview'" @goto="section = $event" />
      <QuotaConfigCard v-else-if="section === 'config'" />
      <UsageCard v-else />
    </main>
  </div>
</template>

<style scoped>
.full-center {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f5f7;
}
.shell {
  display: grid;
  grid-template-columns: 232px 1fr;
  min-height: 100vh;
  background: #f5f5f7;
}
.sidebar {
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #ececef;
  padding: 22px 16px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 4px 8px 22px;
}
.logo {
  border-radius: 8px;
}
.bn {
  font-size: 17px;
  font-weight: 700;
  color: #1d1d1f;
  line-height: 1.1;
}
.bt {
  font-size: 12px;
  color: #86868b;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 10px 12px;
  border: none;
  background: none;
  border-radius: 10px;
  font-size: 14px;
  color: #424245;
  cursor: pointer;
  text-align: left;
  transition: background 0.12s, color 0.12s;
}
.nav-item:hover {
  background: #f2f2f4;
}
.nav-item.active {
  background: #fa2d48;
  color: #fff;
}
.nav-icon {
  width: 18px;
  text-align: center;
  font-size: 14px;
}
.spacer {
  flex: 1;
}
.who {
  border-top: 1px solid #ececef;
  padding: 14px 10px 4px;
}
.who-label {
  font-size: 11px;
  color: #98989d;
}
.who-sub {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #1d1d1f;
  margin: 3px 0 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.logout {
  width: 100%;
  padding: 7px;
  border: 1px solid #e0e0e3;
  background: #fff;
  border-radius: 9px;
  font-size: 13px;
  color: #424245;
  cursor: pointer;
}
.logout:hover {
  border-color: #d03050;
  color: #d03050;
}
.content {
  padding: 30px 34px;
  overflow: auto;
  max-width: 1080px;
}

@media (max-width: 720px) {
  .shell {
    grid-template-columns: 1fr;
  }
  .sidebar {
    flex-direction: row;
    align-items: center;
    border-right: none;
    border-bottom: 1px solid #ececef;
    padding: 12px 16px;
    gap: 12px;
  }
  .brand {
    padding: 0;
  }
  .nav {
    flex-direction: row;
  }
  .spacer {
    flex: 1;
  }
  .who {
    border-top: none;
    padding: 0;
  }
  .who-label,
  .who-sub {
    display: none;
  }
  .content {
    padding: 20px 16px;
  }
}
</style>
