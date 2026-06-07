<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NCheckbox, NTag } from 'naive-ui'
import type { PlaylistItem } from '@/stores/playlist'

const props = defineProps<{
  item: PlaylistItem
  selected: boolean
  current: boolean
  playing: boolean
  tag?: '' | 'full' | 'preview' // 当前在放这首时的模式：完整 / 试听
  playDisabled?: boolean // 无法播放（未连 Apple Music 且无 30s 预览）
  swipe?: boolean // mobile: swipe right = select, swipe left = delete
}>()
const emit = defineEmits<{ toggleSelect: []; togglePlay: []; remove: [] }>()

// --- Swipe gesture (mobile only) -------------------------------------------
// Pointer events (not touch) so the same drag works with a mouse in the preview.
// Right past THRESHOLD → toggle select; left past −THRESHOLD → delete; else snap back.
const THRESHOLD = 72 // px the row must travel for a release to commit
const MAX = 100 // px max reveal of the action panel
const offset = ref(0)
const dragging = ref(false)
let startX = 0
let startY = 0
let active = false
let decided = false // have we locked the gesture axis yet?
let horizontal = false
let justDragged = false // swallow the click that follows a drag (so ▶ doesn't fire)

function onDown(e: PointerEvent) {
  if (!props.swipe) return
  active = true
  decided = false
  horizontal = false
  startX = e.clientX
  startY = e.clientY
}
function onMove(e: PointerEvent) {
  if (!props.swipe || !active) return
  const dx = e.clientX - startX
  const dy = e.clientY - startY
  if (!decided) {
    if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return // ignore micro-jitter / taps
    decided = true
    horizontal = Math.abs(dx) > Math.abs(dy) // vertical wins → let the list scroll
    if (horizontal) {
      dragging.value = true
      try {
        ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
      } catch {
        /* synthetic / unsupported pointer — capture is best-effort */
      }
    }
  }
  if (!horizontal) return
  e.preventDefault()
  offset.value = Math.max(-MAX, Math.min(MAX, dx))
}
function onUp() {
  if (!props.swipe || !active) return
  active = false
  if (!dragging.value) return
  const o = offset.value
  dragging.value = false
  offset.value = 0
  justDragged = true
  if (o >= THRESHOLD) emit('toggleSelect')
  else if (o <= -THRESHOLD) emit('remove')
}
function onClickCapture(e: MouseEvent) {
  if (justDragged) {
    e.stopPropagation()
    e.preventDefault()
    justDragged = false
  }
}
</script>

<template>
  <div class="row-wrap" :class="{ swipeable: swipe }">
    <template v-if="swipe">
      <div v-show="offset > 0" class="swipe-bg select" :class="{ armed: offset >= THRESHOLD }" aria-hidden="true">
        <span>✓ 选择</span>
      </div>
      <div v-show="offset < 0" class="swipe-bg delete" :class="{ armed: offset <= -THRESHOLD }" aria-hidden="true">
        <span>删除 🗑</span>
      </div>
    </template>

    <div
      class="row"
      :class="{ current, selected: swipe && selected }"
      :style="swipe ? { transform: `translateX(${offset}px)`, transition: dragging ? 'none' : 'transform .2s ease' } : undefined"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @click.capture="onClickCapture"
    >
      <n-checkbox v-if="!swipe" :checked="selected" @update:checked="emit('toggleSelect')" />
      <div v-else class="sel-dot" :class="{ on: selected }">✓</div>

      <img v-if="item.song.artworkUrl" :src="item.song.artworkUrl" class="art" alt="" loading="lazy" />
      <div v-else class="art placeholder">♫</div>

      <div class="meta">
        <div class="line1">
          <span class="title" :title="item.song.title">{{ item.song.title }}</span>
          <n-tag v-if="tag" size="tiny" :type="tag === 'full' ? 'success' : 'default'" :bordered="false">
            {{ tag === 'full' ? '完整' : '试听' }}
          </n-tag>
          <n-tag v-if="item.status === 'new'" size="tiny" type="info" :bordered="false">新</n-tag>
          <n-tag v-else size="tiny" :bordered="false">保留</n-tag>
          <n-tag v-if="item.song.contentRating === 'explicit'" size="tiny" type="warning" :bordered="false">E</n-tag>
          <n-tag v-if="item.song.hasLyrics === false" size="tiny" :bordered="false">纯音乐</n-tag>
        </div>
        <div class="artist">{{ item.song.artist }}</div>
        <div v-if="item.reason" class="reason">{{ item.reason }}</div>
      </div>

      <n-button
        circle
        size="small"
        :disabled="playDisabled"
        :title="playDisabled ? '暂无预览' : tag === 'full' ? '完整播放/暂停' : '播放/暂停'"
        @click="emit('togglePlay')"
      >
        {{ current && playing ? '⏸' : '▶' }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.row-wrap {
  position: relative;
}
/* Swipe mode: clip the revealed action panels and give the foreground a solid
   background so it occludes them while resting. */
.row-wrap.swipeable {
  overflow: hidden;
  border-radius: 10px;
  touch-action: pan-y; /* we own horizontal drags; let the list scroll vertically */
}
.swipe-bg {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  padding: 0 18px;
  color: #fff;
  font-size: 14px;
  font-weight: 600;
}
.swipe-bg.select {
  justify-content: flex-start;
  background: #18a058;
}
.swipe-bg.delete {
  justify-content: flex-end;
  background: #e7564f;
}
.swipe-bg.armed {
  filter: brightness(1.08);
}
.row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  border-radius: 10px;
  transition: background 0.15s;
}
.row-wrap.swipeable .row {
  position: relative;
  background: #fff; /* opaque so it hides the action panel underneath */
}
.row:hover {
  background: #f4f4f5;
}
.row.current {
  background: #fff0f2;
}
.row-wrap.swipeable .row.selected {
  background: #f1faf4;
  box-shadow: inset 3px 0 0 #18a058;
}
.sel-dot {
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 2px solid #d2d2d6;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 700;
  color: transparent;
}
.sel-dot.on {
  background: #18a058;
  border-color: #18a058;
  color: #fff;
}
.art {
  width: 44px;
  height: 44px;
  border-radius: 6px;
  object-fit: cover;
  flex: 0 0 auto;
}
.art.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ececef;
  color: #aaa;
  font-size: 20px;
}
.meta {
  flex: 1 1 auto;
  min-width: 0;
}
.line1 {
  display: flex;
  align-items: center;
  gap: 6px;
}
.title {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.artist {
  font-size: 12px;
  color: #888;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.reason {
  font-size: 12px;
  color: #b0344b;
  margin-top: 2px;
}
</style>
