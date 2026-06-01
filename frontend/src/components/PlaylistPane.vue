<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NAlert, NButton, NCheckbox, NEmpty, NInput, NScrollbar, NSpace, NText } from 'naive-ui'
import SongRow from './SongRow.vue'
import { usePlaylistStore } from '@/stores/playlist'
import { useAppleStore } from '@/stores/apple'
import { usePreviewPlayer } from '@/composables/usePreviewPlayer'
import { createPlaylist } from '@/api/client'
import type { ApiError } from '@/types'

const playlist = usePlaylistStore()
const apple = useAppleStore()
const { currentId, playing, setQueue, toggle, next, prev } = usePreviewPlayer()

const songsInOrder = computed(() => playlist.items.map((it) => it.song))
watch(songsInOrder, (s) => setQueue(s), { immediate: true })

const allSelected = computed(
  () => playlist.items.length > 0 && playlist.selectedCount === playlist.items.length,
)
function toggleAll(v: boolean) {
  if (v) playlist.selectAll()
  else playlist.clearSelection()
}

function playPause() {
  const cur = songsInOrder.value.find((s) => s.id === currentId.value)
  if (cur) toggle(cur)
  else if (songsInOrder.value[0]) toggle(songsInOrder.value[0])
}

const creating = ref(false)
const created = ref<{ name: string; url: string } | null>(null)
const createErr = ref('')

async function onCreate() {
  created.value = null
  createErr.value = ''
  if (!apple.authorized) {
    createErr.value = '请先连接并授权 Apple Music。'
    return
  }
  if (playlist.selectedCount === 0) {
    createErr.value = '请至少勾选一首歌。'
    return
  }
  creating.value = true
  try {
    const ids = playlist.selectedSongs.map((s) => s.id)
    const res = await createPlaylist(
      apple.userToken,
      playlist.playlistName || '我的 AI 歌单',
      playlist.description,
      ids,
    )
    created.value = { name: res.name, url: res.url }
  } catch (e) {
    createErr.value = (e as ApiError).message
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="pane">
    <header class="pane-head">
      <strong>歌单 · 精确操作</strong>
      <n-space v-if="playlist.hasResult" size="small" align="center">
        <n-button circle size="tiny" title="上一首" @click="prev">⏮</n-button>
        <n-button circle size="small" title="播放/暂停" @click="playPause">{{ playing ? '⏸' : '▶' }}</n-button>
        <n-button circle size="tiny" title="下一首" @click="next">⏭</n-button>
      </n-space>
    </header>

    <!-- F10 state-sync notice -->
    <n-alert
      v-if="playlist.notice"
      type="warning"
      closable
      style="margin-bottom: 8px"
      @close="playlist.dismissNotice()"
    >
      {{ playlist.notice }}
    </n-alert>

    <template v-if="playlist.hasResult">
      <div class="meta-edit">
        <n-input v-model:value="playlist.playlistName" placeholder="歌单名" size="small" />
        <n-input
          v-model:value="playlist.description"
          placeholder="歌单描述"
          size="small"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 3 }"
          style="margin-top: 6px"
        />
      </div>

      <div class="toolbar">
        <n-checkbox :checked="allSelected" @update:checked="toggleAll">全选</n-checkbox>
        <n-text depth="3" style="font-size: 12px">已选 {{ playlist.selectedCount }} / {{ playlist.items.length }}</n-text>
      </div>

      <n-scrollbar class="list">
        <SongRow
          v-for="it in playlist.items"
          :key="it.song.id"
          :item="it"
          :selected="playlist.selected.has(it.song.id)"
          :current="currentId === it.song.id"
          :playing="playing"
          @toggle-select="playlist.toggle(it.song.id)"
          @toggle-play="toggle(it.song)"
        />
      </n-scrollbar>

      <footer class="actions">
        <n-alert v-if="created" type="success" :bordered="false" style="margin-bottom: 8px">
          已在你的 Apple Music 资料库创建「{{ created.name }}」。
          <a :href="created.url" target="_blank" rel="noopener">在 Apple Music 中打开 ↗</a>
        </n-alert>
        <n-alert v-if="createErr" type="error" :bordered="false" style="margin-bottom: 8px">{{ createErr }}</n-alert>

        <n-button type="primary" block :loading="creating" :disabled="playlist.selectedCount === 0" @click="onCreate">
          建成歌单（{{ playlist.selectedCount }} 首）
        </n-button>
        <n-text depth="3" class="disclaimer">
          通过 API 创建的歌单默认私有，且无法经 API 公开/分享；如需公开请在 Apple Music App 中手动调整。
        </n-text>
      </footer>
    </template>

    <div v-else class="empty">
      <n-empty description="还没有候选歌曲">
        <template #extra>
          <n-text depth="3" style="font-size: 13px">在左侧描述你想听的，这里会出现可试听、可勾选的真实歌曲。</n-text>
        </template>
      </n-empty>
    </div>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 10px;
}
.pane-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.meta-edit {
  flex: 0 0 auto;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 2px;
  border-bottom: 1px solid #eee;
}
.list {
  flex: 1 1 auto;
  min-height: 0;
}
.actions {
  flex: 0 0 auto;
}
.disclaimer {
  display: block;
  margin-top: 8px;
  font-size: 11px;
  line-height: 1.5;
}
.empty {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
