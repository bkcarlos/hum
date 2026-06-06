<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NSpin } from 'naive-ui'
import { getAdminConfig, getAdminUsage } from '@/api/client'
import { useAdminStore } from '@/stores/admin'
import type { AdminUsage, ApiError, QuotaConfig } from '@/types'
import StatTile from './StatTile.vue'

const emit = defineEmits<{ goto: ['config' | 'usage'] }>()
const admin = useAdminStore()

const loading = ref(true)
const err = ref('')
const cfg = ref<QuotaConfig | null>(null)
const usage = ref<AdminUsage | null>(null)

const bannedCount = computed(() => usage.value?.users.filter((u) => u.banned).length ?? 0)
const activeCount = computed(() => usage.value?.users.filter((u) => u.used > 0).length ?? 0)

async function load() {
  loading.value = true
  err.value = ''
  try {
    const [c, u] = await Promise.all([getAdminConfig(admin.session), getAdminUsage(admin.session)])
    cfg.value = c
    usage.value = u
  } catch (e) {
    err.value = (e as ApiError).message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="sec">
    <header class="sec-head">
      <div>
        <h1>概览</h1>
        <p class="desc">免费档运行快照 · 今日 (UTC)</p>
      </div>
      <n-button tertiary size="small" :loading="loading" @click="load">刷新</n-button>
    </header>

    <div v-if="loading" class="center"><n-spin /></div>
    <div v-else-if="err" class="err">{{ err }}</div>
    <template v-else-if="cfg && usage">
      <div class="tiles">
        <StatTile
          label="今日全站用量"
          :value="usage.globalUsed"
          :hint="usage.globalLimit ? `上限 ${usage.globalLimit}` : '未设全局上限'"
          tone="accent"
        />
        <StatTile label="今日活跃用户" :value="activeCount" :hint="`${usage.users.length} 人有记录`" />
        <StatTile
          label="免费档"
          :value="cfg.enabled ? '已开启' : '已关闭'"
          :tone="cfg.enabled ? 'success' : 'error'"
          :hint="`每人 ${cfg.perUserDailyLimit || '不限'} / 日`"
        />
        <StatTile
          label="当前封禁"
          :value="bannedCount"
          :tone="bannedCount ? 'error' : 'default'"
          :hint="`管理员 ${cfg.admins.length} 人`"
        />
      </div>

      <div class="quick">
        <button class="quick-item" @click="emit('goto', 'config')">
          <span class="qi-title">调整配额 / 默认模型</span>
          <span class="qi-arrow">→</span>
        </button>
        <button class="quick-item" @click="emit('goto', 'usage')">
          <span class="qi-title">查看用量 / 封禁用户</span>
          <span class="qi-arrow">→</span>
        </button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.sec-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 22px;
}
.sec-head h1 {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
  color: #1d1d1f;
}
.desc {
  font-size: 13px;
  color: #86868b;
  margin: 4px 0 0;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.quick {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-top: 16px;
}
.quick-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 16px;
  padding: 18px 20px;
  font-size: 14px;
  color: #1d1d1f;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.05s;
}
.quick-item:hover {
  border-color: #fa2d48;
}
.quick-item:active {
  transform: scale(0.995);
}
.qi-arrow {
  color: #fa2d48;
  font-weight: 700;
}
.center {
  display: flex;
  justify-content: center;
  padding: 60px 0;
}
.err {
  color: #d03050;
  font-size: 13px;
}
@media (max-width: 820px) {
  .tiles {
    grid-template-columns: repeat(2, 1fr);
  }
  .quick {
    grid-template-columns: 1fr;
  }
}
</style>
