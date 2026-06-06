<script setup lang="ts">
import { ref, watch } from 'vue'
import { NConfigProvider, NButton, NTabs, NTabPane, NBadge, zhCN, dateZhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import AppleConnect from '@/components/AppleConnect.vue'
import LlmConfigDialog from '@/components/LlmConfigDialog.vue'
import ConversationPane from '@/components/ConversationPane.vue'
import PlaylistPane from '@/components/PlaylistPane.vue'
import { useMediaQuery } from '@/composables/useMediaQuery'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { usePlaylistStore } from '@/stores/playlist'

const llm = useLlmConfigStore()
const playlist = usePlaylistStore()

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
          <n-button
            :type="llm.configured ? 'default' : 'primary'"
            size="small"
            @click="showConfig = true"
          >
            {{ llm.configured ? 'LLM 设置' : '配置 LLM Key' }}
          </n-button>
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
  gap: 12px;
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
