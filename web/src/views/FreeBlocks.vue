<template>
  <!-- 剩余网段查询：选择网段池后展示可用空闲段 -->
  <div>
    <a-form layout="inline" style="margin-bottom: 24px">
      <a-form-item label="选择网段池">
        <a-select v-model:value="selectedPool" style="width: 300px" placeholder="请选择" @change="fetchBlocks">
          <a-select-option v-for="p in pools" :key="p.id" :value="p.id">
            {{ p.name }} ({{ p.cidr }}) - 使用率 {{ p.usage_rate.toFixed(1) }}%
          </a-select-option>
        </a-select>
      </a-form-item>
    </a-form>

    <!-- 空闲段列表 -->
    <a-table
      :dataSource="blocks"
      :columns="columns"
      rowKey="cidr"
      :pagination="false"
      :locale="{ emptyText: selectedPool ? '该网段池已满' : '请先选择网段池' }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'capacity'">
          <a-tag color="green">可容纳 {{ record.ip_count }} 个 IP</a-tag>
        </template>
      </template>
    </a-table>

    <!-- 汇总信息 -->
    <div v-if="blocks.length > 0" style="margin-top: 16px; color: #666">
      共 {{ blocks.length }} 个空闲段，合计可用
      {{ blocks.reduce((s: number, b: any) => s + b.ip_count, 0) }} 个 IP
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getPools, getFreeBlocks } from '../api'

const pools = ref<any[]>([])
const blocks = ref<any[]>([])
const selectedPool = ref<number | undefined>(undefined)

const columns = [
  { title: 'CIDR', dataIndex: 'cidr', key: 'cidr' },
  { title: '起始 IP', dataIndex: 'start_ip', key: 'start_ip' },
  { title: '结束 IP', dataIndex: 'end_ip', key: 'end_ip' },
  { title: '可用容量', key: 'capacity' },
]

const fetchBlocks = async () => {
  if (!selectedPool.value) return
  const res = await getFreeBlocks(selectedPool.value)
  blocks.value = res.data || []
}

onMounted(async () => {
  const res = await getPools()
  pools.value = res.data || []
})
</script>
