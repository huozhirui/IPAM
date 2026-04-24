<template>
  <!-- 仪表盘：统计卡片 + 网段池概览 + 最近分配 -->
  <div>
    <!-- 统计卡片 -->
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic title="网段池总数" :value="dashboard.pool_count" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="已分配子网" :value="dashboard.allocation_count" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="总 IP 数" :value="dashboard.total_ips" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic title="总使用率" :value="dashboard.usage_rate" suffix="%" :precision="1" />
        </a-card>
      </a-col>
    </a-row>

    <!-- 网段池概览 -->
    <a-card title="网段池概览" style="margin-bottom: 24px">
      <a-table :dataSource="pools" :columns="poolColumns" rowKey="id" :pagination="false" size="small">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'usage'">
            <a-progress :percent="Number(record.usage_rate.toFixed(1))" :strokeWidth="12" />
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 最近分配记录 -->
    <a-card title="最近分配记录">
      <a-list :dataSource="dashboard.recent_allocs" size="small">
        <template #renderItem="{ item }">
          <a-list-item>
            <a-list-item-meta>
              <template #title>
                <code>{{ item.cidr }}</code> &rarr; {{ item.purpose }}
              </template>
              <template #description>
                {{ item.actual_count }} IPs &middot; {{ formatTime(item.allocated_at) }}
              </template>
            </a-list-item-meta>
          </a-list-item>
        </template>
      </a-list>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getDashboard, getPools } from '../api'

const dashboard = ref<any>({
  pool_count: 0,
  allocation_count: 0,
  total_ips: 0,
  used_ips: 0,
  usage_rate: 0,
  recent_allocs: [],
})
const pools = ref<any[]>([])

const formatTime = (val: string) => {
  if (!val) return ''
  const d = new Date(val)
  if (isNaN(d.getTime())) return val
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

const poolColumns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: 'CIDR', dataIndex: 'cidr', key: 'cidr' },
  { title: '总 IP', dataIndex: 'total_ips', key: 'total_ips' },
  { title: '已用', dataIndex: 'used_ips', key: 'used_ips' },
  { title: '使用率', key: 'usage' },
]

onMounted(async () => {
  const [dashRes, poolRes] = await Promise.all([getDashboard(), getPools()])
  const d = dashRes.data
  d.recent_allocs = d.recent_allocs || []
  dashboard.value = d
  pools.value = poolRes.data || []
})
</script>
