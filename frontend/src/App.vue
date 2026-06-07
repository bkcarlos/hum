<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NConfigProvider, NButton, NTabs, NTabPane, NBadge, zhCN, dateZhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import AppleConnect from '@/components/AppleConnect.vue'
import LlmConfigDialog from '@/components/LlmConfigDialog.vue'
import ConversationPane from '@/components/ConversationPane.vue'
import PlaylistPane from '@/components/PlaylistPane.vue'
import { useMediaQuery } from '@/composables/useMediaQuery'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { useSessionStore } from '@/stores/session'
import { usePlaylistStore } from '@/stores/playlist'
import { useAppleLogin } from '@/composables/useAppleLogin'

const llm = useLlmConfigStore()
const session = useSessionStore()
const playlist = usePlaylistStore()

// Topbar access: ONE primary CTA for the common path (免费 Apple 登录), with BYOK
// demoted to a small secondary link. The primary reflects the current setup.
const { loading: appleLoading, available: appleAvailable, login: doAppleLogin, ensureConfig } = useAppleLogin()
onMounted(ensureConfig)

const primaryLabel = computed(() => {
  if (session.signedIn) return session.displayName ? `免费额度 · ${session.displayName}` : '免费额度 ✓'
  if (session.mode === 'byok') return llm.configured ? '自带 Key ✓' : '配置自带 Key'
  return '用 Apple 登录 · 免费开始'
})
const primaryType = computed<'primary' | 'default'>(() => {
  if (session.signedIn) return 'default'
  if (session.mode === 'byok') return llm.configured ? 'default' : 'primary'
  return 'primary'
})
// The non-active path, shown as a subtle link.
const secondaryLabel = computed(() => (session.mode === 'byok' ? '用免费额度' : '自带 Key'))

// Responsive degradation (must-do): narrow screens drop the side-by-side layout
// for tabs (对话 / 歌单) — never two panes squeezed on mobile.
const isNarrow = useMediaQuery('(max-width: 900px)')
const activeTab = ref<'chat' | 'list'>('chat')

// On narrow screens the result lands in the (unfocused) 歌单 tab — surface it the
// first time candidates appear so the user doesn't think nothing happened. Only on
// the false→true transition; later refines feed back via the chat bubble.
watch(
  () => playlist.hasResult,
  (has, prev) => {
    if (has && !prev && isNarrow.value) activeTab.value = 'list'
  },
)

const showConfig = ref(false)

async function onPrimary() {
  if (session.signedIn || session.mode === 'byok') {
    showConfig.value = true // already set up → open settings to manage
    return
  }
  // Fresh + free: log in directly (one click); fall back to the dialog if web
  // Apple login isn't configured server-side.
  session.setMode('free')
  if (appleAvailable.value) await doAppleLogin()
  else showConfig.value = true
}
function onSecondary() {
  session.setMode(session.mode === 'byok' ? 'free' : 'byok')
  showConfig.value = true
}

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
  <n-config-provider :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <div class="app">
      <header class="topbar">
        <div class="brand">
          <img class="logo" src="/logo.svg" alt="Hum" width="36" height="36" />
          <div>
            <div class="name">Hum</div>
            <div class="tagline">哼一首 · 用自然语言，从 Apple Music 挑歌建单</div>
          </div>
        </div>
        <div class="actions">
          <AppleConnect />
          <n-button :type="primaryType" size="small" :loading="appleLoading" @click="onPrimary">
            <span class="acc-label">{{ primaryLabel }}</span>
          </n-button>
          <n-button text size="small" class="alt-link" @click="onSecondary">{{ secondaryLabel }}</n-button>
        </div>
      </header>

      <!-- Wide: dual pane. Narrow: tabs. -->
      <main v-if="!isNarrow" class="dual">
        <section class="col left"><ConversationPane /></section>
        <section class="col right"><PlaylistPane /></section>
      </main>

      <main v-else class="tabs">
        <n-tabs v-model:value="activeTab" type="line" justify-content="space-evenly" animated>
          <n-tab-pane name="chat" tab="对话">
            <div class="tab-body"><ConversationPane /></div>
          </n-tab-pane>
          <n-tab-pane name="list">
            <template #tab>
              <n-badge :value="playlist.selectedCount" :max="99" :show="playlist.selectedCount > 0">
                <span style="padding-right: 4px">歌单</span>
              </n-badge>
            </template>
            <div class="tab-body"><PlaylistPane /></div>
          </n-tab-pane>
        </n-tabs>
      </main>

      <LlmConfigDialog v-model:show="showConfig" />
    </div>
  </n-config-provider>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  /* Pin to the viewport directly: NConfigProvider renders a block wrapper with
     auto height, which breaks a height:100% chain and leaves the lower part of
     the page empty on tall/full-screen viewports. */
  height: 100vh;
  height: 100dvh;
}
.topbar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #fff;
  border-bottom: 1px solid #ececec;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.logo {
  width: 36px;
  height: 36px;
  display: block;
  border-radius: 9px;
}
.name {
  font-weight: 700;
  font-size: 16px;
}
.tagline {
  font-size: 12px;
  color: #999;
}
.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
/* Keep the signed-in email from blowing up the button width. */
.acc-label {
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
/* BYOK is the demoted, secondary path — a subtle link next to the primary CTA. */
.alt-link {
  font-size: 12px;
}
.alt-link :deep(.n-button__content) {
  color: #8a8a8e;
}

.dual {
  flex: 1 1 auto;
  min-height: 0;
  display: grid;
  /* The list (right) is where the work happens — previewing, selecting, creating.
     Give it ~65% and keep the command pane (left) just wide enough for the composer,
     instead of an even split that over-weights the mostly-idle chat side. */
  grid-template-columns: minmax(320px, 0.7fr) minmax(440px, 1.3fr);
  gap: 16px;
  padding: 16px;
}
.col {
  background: #fff;
  border: 1px solid #ececec;
  border-radius: 14px;
  padding: 14px;
  min-height: 0;
}

.tabs {
  flex: 1 1 auto;
  min-height: 0;
  padding: 8px 12px 12px;
}
.tab-body {
  height: calc(100dvh - 140px);
  background: #fff;
  border: 1px solid #ececec;
  border-radius: 14px;
  padding: 14px;
}

/* Responsive: compact header on small screens — hide the decorative tagline and
   let the action buttons wrap to a second row instead of overflowing. */
@media (max-width: 900px) {
  .tagline {
    display: none;
  }
}
@media (max-width: 560px) {
  .topbar {
    flex-wrap: wrap;
    row-gap: 8px;
  }
  .actions {
    margin-left: auto;
  }
}
</style>
