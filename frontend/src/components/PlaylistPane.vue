<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NAlert, NButton, NCheckbox, NEmpty, NInput, NScrollbar, NSpace, NText } from 'naive-ui'
import SongRow from './SongRow.vue'
import { usePlaylistStore } from '@/stores/playlist'
import { useAppleStore } from '@/stores/apple'
import { usePreviewPlayer } from '@/composables/usePreviewPlayer'
import { useFullPlayer } from '@/composables/useFullPlayer'
import { useMediaQuery } from '@/composables/useMediaQuery'
import { createPlaylist } from '@/api/client'
import type { ApiError } from '@/types'

const playlist = usePlaylistStore()
const apple = useAppleStore()
const { currentId, playing, setQueue, toggle, next, prev } = usePreviewPlayer()
const { playingFull, setFullQueue, playPauseFull } = useFullPlayer()

// Mobile (tabs layout): rows become swipeable and the transport moves into a
// dedicated now-playing bar at the top of the list, instead of the header.
const isNarrow = useMediaQuery('(max-width: 900px)')

const songsInOrder = computed(() => playlist.items.map((it) => it.song))
const currentSong = computed(
  () => songsInOrder.value.find((s) => s.id === currentId.value) ?? songsInOrder.value[0] ?? null,
)
watch(
  songsInOrder,
  (s) => {
    setQueue(s)
    setFullQueue(s.map((x) => x.id))
  },
  { immediate: true },
)

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
      <n-space v-if="playlist.hasResult && !isNarrow" size="small" align="center">
        <n-button circle size="tiny" title="上一首" @click="prev">⏮</n-button>
        <n-button circle size="small" title="预览播放/暂停（30s）" @click="playPause">{{ playing ? '⏸' : '▶' }}</n-button>
        <n-button circle size="tiny" title="下一首" @click="next">⏭</n-button>
        <n-button
          v-if="apple.authorized"
          circle
          size="small"
          title="完整播放/暂停（需 Apple Music 订阅）"
          @click="playPauseFull"
        >{{ playingFull ? '⏸' : '♪' }}</n-button>
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
      <n-button text type="primary" size="tiny" style="margin-left: 8px" @click="playlist.keepDropped()">
        保留这些歌
      </n-button>
    </n-alert>

    <template v-if="playlist.hasResult">
      <!-- Mobile now-playing bar: transport lives here (not the cramped header). -->
      <div v-if="isNarrow" class="player">
        <img v-if="currentSong?.artworkUrl" :src="currentSong.artworkUrl" class="p-art" alt="" />
        <div v-else class="p-art placeholder">♫</div>
        <div class="p-meta">
          <div class="p-title">{{ currentSong?.title ?? '—' }}</div>
          <div class="p-artist">{{ currentSong ? currentSong.artist : '点一首开始试听' }}</div>
        </div>
        <div class="p-controls">
          <n-button circle size="small" title="上一首" @click="prev">⏮</n-button>
          <n-button circle type="primary" title="预览播放/暂停（30s）" @click="playPause">{{ playing ? '⏸' : '▶' }}</n-button>
          <n-button circle size="small" title="下一首" @click="next">⏭</n-button>
          <n-button
            v-if="apple.authorized"
            circle
            size="small"
            title="完整播放/暂停（需 Apple Music 订阅）"
            @click="playPauseFull"
          >{{ playingFull ? '⏸' : '♪' }}</n-button>
        </div>
      </div>

      <div class="toolbar">
        <n-checkbox :checked="allSelected" @update:checked="toggleAll">全选</n-checkbox>
        <n-text depth="3" style="font-size: 12px">已选 {{ playlist.selectedCount }} / {{ playlist.items.length }}</n-text>
      </div>

      <p v-if="isNarrow" class="swipe-hint">← 左滑删除 · 右滑选择 →</p>

      <n-alert
        v-if="playlist.lastRemoved"
        type="default"
        closable
        class="undo-bar"
        @close="playlist.clearRemoved()"
      >
        已删除「{{ playlist.lastRemoved.item.song.title }}」
        <n-button text type="primary" size="tiny" style="margin-left: 8px" @click="playlist.undoRemove()">
          撤销
        </n-button>
      </n-alert>

      <n-scrollbar class="list">
        <SongRow
          v-for="it in playlist.items"
          :key="it.song.id"
          :item="it"
          :selected="playlist.selected.has(it.song.id)"
          :current="currentId === it.song.id"
          :playing="playing"
          :swipe="isNarrow"
          @toggle-select="playlist.toggle(it.song.id)"
          @toggle-play="toggle(it.song)"
          @remove="playlist.removeItem(it.song.id)"
        />
      </n-scrollbar>

      <footer class="actions">
        <!-- Name/description sit by the create action, not above the list: you name
             the playlist after reviewing and selecting, right before creating it. -->
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
  margin-bottom: 8px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 2px;
  border-bottom: 1px solid #eee;
}
.player {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  background: #fafafa;
  border: 1px solid #ececec;
  border-radius: 12px;
}
.p-art {
  width: 40px;
  height: 40px;
  border-radius: 6px;
  object-fit: cover;
  flex: 0 0 auto;
}
.p-art.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ececef;
  color: #aaa;
  font-size: 18px;
}
.p-meta {
  flex: 1 1 auto;
  min-width: 0;
}
.p-title {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.p-artist {
  font-size: 12px;
  color: #888;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.p-controls {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 6px;
}
.swipe-hint {
  margin: 6px 2px 0;
  font-size: 11px;
  color: #b0b0b0;
  text-align: center;
}
.undo-bar {
  margin-top: 8px;
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
