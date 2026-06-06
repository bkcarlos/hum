<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NEmpty, NInput, NSpace, NSpin, NTable, NTag, NText, useMessage } from 'naive-ui'
import { getAdminUsage, setUserBan } from '@/api/client'
import { useAdminStore } from '@/stores/admin'
import type { AdminUsage, ApiError } from '@/types'

const admin = useAdminStore()
const message = useMessage()

const loading = ref(true)
const busy = ref('') // sub currently being banned/unbanned
const data = ref<AdminUsage | null>(null)
const day = ref(todayUTC())

// Backend buckets quota by UTC day (quota.Day) — match it so the default matches.
function todayUTC(): string {
  return new Date().toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    data.value = await getAdminUsage(admin.session, day.value)
  } catch (e) {
    message.error((e as ApiError).message)
  } finally {
    loading.value = false
  }
}

async function toggleBan(sub: string, banned: boolean) {
  busy.value = sub
  try {
    await setUserBan(admin.session, sub, banned)
    message.success(banned ? '已封禁。' : '已解封。')
    await load()
  } catch (e) {
    message.error((e as ApiError).message)
  } finally {
    busy.value = ''
  }
}

onMounted(load)
</script>

<template>
  <n-card title="用量与封禁">
    <template #header-extra>
      <n-space :size="8" align="center">
        <n-input v-model:value="day" placeholder="YYYY-MM-DD" style="width: 140px" @keyup.enter="load" />
        <n-button size="small" :loading="loading" @click="load">查询</n-button>
      </n-space>
    </template>

    <div v-if="loading" style="padding: 24px; text-align: center"><n-spin /></div>
    <template v-else-if="data">
      <n-space :size="24" align="center" style="margin-bottom: 14px">
        <n-text>日期 <b>{{ data.day }}</b> (UTC)</n-text>
        <n-text>
          全站已用 <b>{{ data.globalUsed }}</b>
          <span v-if="data.globalLimit"> / {{ data.globalLimit }}</span>
          <span v-else> （不限）</span>
        </n-text>
        <n-text depth="3" style="font-size: 12px">每人额度 {{ data.perUserLimit || '不限' }}</n-text>
      </n-space>

      <n-empty v-if="!data.users.length" description="当日暂无用量记录" style="padding: 24px 0" />
      <n-table v-else :bordered="false" :single-line="false" size="small">
        <thead>
          <tr>
            <th>Apple sub</th>
            <th style="width: 80px">已用</th>
            <th style="width: 90px">状态</th>
            <th style="width: 100px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in data.users" :key="u.sub">
            <td style="font-family: monospace; font-size: 12px; word-break: break-all">{{ u.sub }}</td>
            <td>{{ u.used }}</td>
            <td>
              <n-tag :type="u.banned ? 'error' : 'success'" size="small" :bordered="false">
                {{ u.banned ? '已封禁' : '正常' }}
              </n-tag>
            </td>
            <td>
              <n-button
                size="tiny"
                :type="u.banned ? 'default' : 'error'"
                :loading="busy === u.sub"
                @click="toggleBan(u.sub, !u.banned)"
              >
                {{ u.banned ? '解封' : '封禁' }}
              </n-button>
            </td>
          </tr>
        </tbody>
      </n-table>
    </template>
  </n-card>
</template>
