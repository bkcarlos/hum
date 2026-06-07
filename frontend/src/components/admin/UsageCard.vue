<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NEmpty, NInput, NSpin, NTag } from 'naive-ui'
import { getAdminUsage, setUserBan } from '@/api/client'
import { useAdminStore } from '@/stores/admin'
import type { AdminUsage, ApiError } from '@/types'
import StatTile from './StatTile.vue'

const admin = useAdminStore()

const loading = ref(true)
const busy = ref('') // sub currently being banned/unbanned
const data = ref<AdminUsage | null>(null)
const err = ref('')
const day = ref(todayUTC())

// Backend buckets quota by UTC day (quota.Day) — default to that so it matches.
function todayUTC(): string {
  return new Date().toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  err.value = ''
  try {
    data.value = await getAdminUsage(admin.session, day.value)
  } catch (e) {
    err.value = (e as ApiError).message
  } finally {
    loading.value = false
  }
}

async function toggleBan(sub: string, banned: boolean) {
  busy.value = sub
  try {
    await setUserBan(admin.session, sub, banned)
    await load()
  } catch (e) {
    err.value = (e as ApiError).message
  } finally {
    busy.value = ''
  }
}

onMounted(load)
</script>

<template>
  <section class="sec">
    <header class="sec-head">
      <div>
        <h1>用量与封禁</h1>
        <p class="desc">按 UTC 日统计 · 封禁立即生效</p>
      </div>
      <div class="picker">
        <n-input v-model:value="day" placeholder="YYYY-MM-DD" style="width: 150px" @keyup.enter="load" />
        <n-button type="primary" ghost :loading="loading" @click="load">查询</n-button>
      </div>
    </header>

    <div v-if="loading" class="center"><n-spin /></div>
    <div v-else-if="err" class="err">{{ err }}</div>
    <template v-else-if="data">
      <div class="tiles">
        <StatTile
          label="全站已用"
          :value="data.globalUsed"
          :hint="data.globalLimit ? `上限 ${data.globalLimit}` : '未设全局上限'"
          tone="accent"
        />
        <StatTile label="有记录用户" :value="data.users.length" :hint="`每人额度 ${data.perUserLimit || '不限'}`" />
        <StatTile
          label="当前封禁"
          :value="data.users.filter((u) => u.banned).length"
          :tone="data.users.some((u) => u.banned) ? 'error' : 'default'"
          :hint="day + ' (UTC)'"
        />
      </div>

      <div class="card">
        <div class="group-label">用户列表</div>
        <n-empty v-if="!data.users.length" description="当日暂无用量记录" style="padding: 28px 0" />
        <table v-else class="users">
          <thead>
            <tr>
              <th>用户</th>
              <th class="num">已用</th>
              <th class="st">状态</th>
              <th class="op">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in data.users" :key="u.sub">
              <td>
                <div v-if="u.email" class="email">{{ u.email }}</div>
                <div class="sub" :class="{ muted: u.email }" :title="u.sub">{{ u.sub }}</div>
              </td>
              <td class="num">{{ u.used }}</td>
              <td class="st">
                <n-tag :type="u.banned ? 'error' : 'success'" size="small" :bordered="false" round>
                  {{ u.banned ? '已封禁' : '正常' }}
                </n-tag>
              </td>
              <td class="op">
                <n-button
                  size="tiny"
                  :type="u.banned ? 'default' : 'error'"
                  :ghost="!u.banned"
                  :loading="busy === u.sub"
                  @click="toggleBan(u.sub, !u.banned)"
                >
                  {{ u.banned ? '解封' : '封禁' }}
                </n-button>
              </td>
            </tr>
          </tbody>
        </table>
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
.picker {
  display: flex;
  gap: 10px;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}
.card {
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 16px;
  padding: 20px 22px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}
.group-label {
  font-size: 13px;
  font-weight: 600;
  color: #1d1d1f;
  margin-bottom: 14px;
}
.users {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.users th {
  text-align: left;
  font-weight: 500;
  color: #86868b;
  font-size: 12px;
  padding: 0 10px 10px;
  border-bottom: 1px solid #f0f0f2;
}
.users td {
  padding: 12px 10px;
  border-bottom: 1px solid #f5f5f7;
  vertical-align: middle;
}
.users tr:last-child td {
  border-bottom: none;
}
.email {
  font-size: 13px;
  color: #1d1d1f;
}
.sub {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #1d1d1f;
  word-break: break-all;
}
.sub.muted {
  font-size: 11px;
  color: #98989d;
}
.num {
  width: 80px;
}
.st {
  width: 92px;
}
.op {
  width: 96px;
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
    grid-template-columns: 1fr;
  }
}
</style>
