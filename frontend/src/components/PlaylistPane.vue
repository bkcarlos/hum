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
import type { ApiError, Song } from '@/types'

const playlist = usePlaylistStore()
const apple = useAppleStore()
const preview = usePreviewPlayer()
const full = useFullPlayer()
const fullHint = ref('')

// Mobile (tabs layout): rows become swipeable and the transport moves into a
// dedicated now-playing bar at the top of the list, instead of the header.
const isNarrow = useMediaQuery('(max-width: 900px)')

const songsInOrder = computed(() => playlist.items.map((it) => it.song))

// 统一「当前在放」：哪个播放器有 currentId 就以它为准（同一时刻只放一个）。
const activeIsFull = computed(() => full.currentId.value !== '')
const activeCurrentId = computed(() => full.currentId.value || preview.currentId.value)
const activePlaying = computed(() => (activeIsFull.value ? full.playing.value : preview.playing.value))
const currentSong = computed(
  () => songsInOrder.value.find((s) => s.id === activeCurrentId.value) ?? songsInOrder.value[0] ?? null,
)

watch(
  songsInOrder,
  (s) => {
    preview.setQueue(s)
    full.setQueue(s)
  },
  { immediate: true },
)
// 断开 Apple Music → 停掉完整播放（回到 30s 试听模式）。
watch(
  () => apple.authorized,
  (on) => {
    if (!on) full.stop()
  },
)

const allSelected = computed(
  () => playlist.items.length > 0 && playlist.selectedCount === playlist.items.length,
)
function toggleAll(v: boolean) {
  if (v) playlist.selectAll()
  else playlist.clearSelection()
}

// 播放分流（对齐 iOS）：连了 Apple Music → 完整播放该曲（失败/非订阅回退 30s 试听）；
// 没连 → 30s 试听。同一时刻只放一个。
async function onTogglePlay(song: Song) {
  if (apple.authorized) {
    preview.stop()
    if (await full.toggle(song)) {
      fullHint.value = ''
    } else {
      fullHint.value = '完整播放失败：可能未订阅，或当前结果区域与你的 Apple Music 不一致；已用 30s 试听。'
      preview.toggle(song)
    }
  } else {
    preview.toggle(song)
  }
}

function playPause() {
  const cur = songsInOrder.value.find((s) => s.id === activeCurrentId.value) ?? songsInOrder.value[0]
  if (cur) void onTogglePlay(cur)
}
function next() {
  if (activeIsFull.value) void full.next()
  else preview.next()
}
function prev() {
  if (activeIsFull.value) void full.prev()
  else preview.prev()
}

/** 当前在放这首时的标签：full→完整 / preview→试听 / 其它→空。 */
function rowTag(id: string): '' | 'full' | 'preview' {
  if (id !== activeCurrentId.value) return ''
  return activeIsFull.value ? 'full' : 'preview'
}
function playDisabled(song: Song): boolean {
  // 没连 Apple Music → 靠 previewUrl；连了(完整模式)则都可点。
  return !apple.authorized && !song.previewUrl
}

const creating = ref(false)
const created = ref<{ name: string; url: string } | null>(null)
const createErr = ref('')

async function onCreate() {
  created.value = null
  createErr.value = ''
  if (playlist.selectedCount === 0) {
    createErr.value = '请至少勾选一首歌。'
    return
  }
  // 防御：建歌单 UI 只在连上后显示，正常不会进这里；万一断连竞态则按需重连。
  if (!apple.authorized && !(await apple.connect())) {
    createErr.value = apple.error || '需要连接 Apple Music 才能把歌单存进你的资料库。'
    return
  }
  // Apple catalog ids are storefront-specific: if the pool was resolved against a
  // different region than the connected account, the ids won't match → re-search.
  if (playlist.builtStorefront && playlist.builtStorefront !== apple.storefront) {
    createErr.value = `当前结果基于 ${playlist.builtStorefront.toUpperCase()} 区，你的 Apple Music 是 ${apple.storefront.toUpperCase()} 区。请在左侧「用编辑后的条件重搜」后再建歌单，以匹配你的曲库。`
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
        <n-button
          circle
          size="small"
          :title="apple.authorized ? '完整播放/暂停' : '预览播放/暂停（30s）'"
          @click="playPause"
        >{{ activePlaying ? '⏸' : '▶' }}</n-button>
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
          <div class="p-artist">{{ currentSong ? currentSong.artist : '点一首开始播放' }}</div>
        </div>
        <div class="p-controls">
          <n-button circle size="small" title="上一首" @click="prev">⏮</n-button>
          <n-button
            circle
            type="primary"
            :title="apple.authorized ? '完整播放/暂停' : '预览播放/暂停（30s）'"
            @click="playPause"
          >{{ activePlaying ? '⏸' : '▶' }}</n-button>
          <n-button circle size="small" title="下一首" @click="next">⏭</n-button>
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

      <n-alert v-if="fullHint" type="warning" closable class="undo-bar" @close="fullHint = ''">{{ fullHint }}</n-alert>

      <n-scrollbar class="list">
        <SongRow
          v-for="it in playlist.items"
          :key="it.song.id"
          :item="it"
          :selected="playlist.selected.has(it.song.id)"
          :current="activeCurrentId === it.song.id"
          :playing="activePlaying"
          :tag="rowTag(it.song.id)"
          :play-disabled="playDisabled(it.song)"
          :swipe="isNarrow"
          @toggle-select="playlist.toggle(it.song.id)"
          @toggle-play="onTogglePlay(it.song)"
          @remove="playlist.removeItem(it.song.id)"
        />
      </n-scrollbar>

      <footer class="actions">
        <!-- 建歌单需要 Apple Music（后端 CreatePlaylist 要 Music-User-Token）：连上才显示
             「创建歌单」操作，与完整播放是同一道「连上才解锁」的能力门槛；未连给连接 CTA。 -->
        <template v-if="apple.authorized">
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
        </template>

        <div v-else class="connect-cta">
          <n-text depth="3" class="connect-tip">勾选喜欢的歌，连接 Apple Music 后即可一键存成歌单。</n-text>
          <n-button secondary block :loading="apple.connecting" @click="apple.connect()">
            连接 Apple Music 建歌单
          </n-button>
        </div>
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
.connect-cta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.connect-tip {
  font-size: 13px;
  line-height: 1.5;
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
