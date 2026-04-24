<template>
  <!-- 操作审计日志：分页表格 + 筛选 + 导出 -->
  <div>
    <a-form layout="inline" style="margin-bottom: 16px">
      <a-form-item label="操作类型">
        <a-select v-model:value="filter.action" style="width: 180px" allowClear placeholder="全部" @change="fetchLogs">
          <a-select-option value="ALLOCATE">分配</a-select-option>
          <a-select-option value="RECLAIM">回收</a-select-option>
          <a-select-option value="CREATE_POOL">创建网段池</a-select-option>
          <a-select-option value="DELETE_POOL">删除网段池</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item>
        <a-space>
          <a-button @click="exportData('csv')">导出 CSV</a-button>
          <a-button @click="exportData('json')">导出 JSON</a-button>
        </a-space>
      </a-form-item>
    </a-form>

    <a-table
      :dataSource="logs"
      :columns="columns"
      rowKey="id"
      :pagination="pagination"
      @change="onTableChange"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <a-tag :color="actionColor(record.action)">{{ actionLabel(record.action) }}</a-tag>
        </template>
        <template v-if="column.key === 'detail'">
          <span style="font-size: 12px; word-break: break-all">{{ formatDetail(record.detail) }}</span>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { getAuditLogs, exportAuditLogs } from '../api'

const logs = ref<any[]>([])
const filter = reactive({ action: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const formatTime = (val: string) => {
  if (!val) return ''
  const d = new Date(val)
  if (isNaN(d.getTime())) return val
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: '操作类型', key: 'action', width: 120 },
  { title: '详情', key: 'detail' },
  { title: '操作人', dataIndex: 'operator', key: 'operator', width: 100 },
  { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: any) => formatTime(text) },
]

const actionColor = (action: string) => {
  const map: Record<string, string> = {
    ALLOCATE: 'blue', RECLAIM: 'orange', CREATE_POOL: 'green', DELETE_POOL: 'red',
  }
  return map[action] || 'default'
}

const actionLabel = (action: string) => {
  const map: Record<string, string> = {
    ALLOCATE: '分配', RECLAIM: '回收', CREATE_POOL: '创建网段池', DELETE_POOL: '删除网段池',
  }
  return map[action] || action
}

// 格式化 JSON 详情为简短摘要
const formatDetail = (detail: string) => {
  try {
    const obj = JSON.parse(detail)
    if (obj.cidr) return `${obj.cidr} ${obj.purpose || obj.name || ''}`
    return detail
  } catch {
    return detail
  }
}

const fetchLogs = async () => {
  const res = await getAuditLogs({
    action: filter.action,
    page: pagination.current,
    page_size: pagination.pageSize,
  })
  logs.value = res.data.items || []
  pagination.total = res.data.total
}

const onTableChange = (pag: any) => {
  pagination.current = pag.current
  fetchLogs()
}

const exportData = async (format: 'csv' | 'json') => {
  try {
    await exportAuditLogs(format)
  } catch (e: any) {
    console.error('导出失败', e)
  }
}

onMounted(fetchLogs)
</script>
