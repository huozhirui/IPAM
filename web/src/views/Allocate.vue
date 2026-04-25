<template>
  <div>
    <!-- 已分配记录列表 -->
    <a-card title="分配记录">
      <template #extra>
        <a-space>
          <a-button type="primary" @click="openAllocModal">分配子网</a-button>
          <a-button size="small" @click="handleExport('csv')">导出 CSV</a-button>
          <a-button size="small" @click="handleExport('json')">导出 JSON</a-button>
        </a-space>
      </template>
      <a-table :dataSource="allocations" :columns="allocColumns" rowKey="id" size="small">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            <a-space>
              <a-button type="link" @click="openEdit(record)">编辑</a-button>
              <a-popconfirm title="确认回收该子网？" @confirm="handleReclaim(record.id)">
                <a-button type="link" danger>回收</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 分配子网弹窗 -->
    <a-modal v-model:open="allocVisible" title="分配子网" :footer="null" width="640px" @afterClose="resetAllocForm">
      <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 18 }">
        <a-form-item label="网段池" required>
          <a-select v-model:value="form.pool_id" placeholder="选择目标网段池" @change="onPoolChange">
            <a-select-option v-for="p in pools" :key="p.id" :value="p.id">
              {{ p.name }} ({{ p.cidr }}) — 可用 {{ p.total_ips - p.used_ips }} / {{ p.total_ips }} IPs
            </a-select-option>
          </a-select>
          <div v-if="selectedPool" style="margin-top: 8px; color: #666; font-size: 13px">
            总量 {{ selectedPool.total_ips }} IPs，已用 {{ selectedPool.used_ips }} IPs，可用
            <span :style="{ color: selectedPool.total_ips - selectedPool.used_ips > 0 ? '#52c41a' : '#ff4d4f', fontWeight: 'bold' }">
              {{ selectedPool.total_ips - selectedPool.used_ips }}
            </span>
            IPs（使用率 {{ selectedPool.usage_rate.toFixed(1) }}%）
          </div>
        </a-form-item>
        <a-form-item label="IP 数量" required>
          <a-input-number v-model:value="form.ip_count" :min="1" :max="65536" style="width: 200px"
            @change="onCalc" />
          <span v-if="suggest" style="margin-left: 16px; color: #1890ff">
            推荐: /{{ suggest.prefix_len }} ({{ suggest.actual_count }} IPs) &rarr; {{ suggest.suggest_cidr }}
          </span>
        </a-form-item>
        <a-form-item label="用途" required>
          <a-input v-model:value="form.purpose" placeholder="如：VPC-Prod-A" />
        </a-form-item>
        <a-form-item label="负责人">
          <a-input v-model:value="form.allocated_by" placeholder="可选，默认当前登录用户" />
        </a-form-item>
        <a-form-item :wrapper-col="{ offset: 4 }">
          <a-space>
            <a-button type="primary" @click="handleAllocate" :loading="loading">确认分配</a-button>
            <a-button @click="showBatch = !showBatch">{{ showBatch ? '关闭批量' : '批量模式' }}</a-button>
          </a-space>
        </a-form-item>
      </a-form>

      <!-- 批量分配区域 -->
      <div v-if="showBatch" style="border-top: 1px solid #f0f0f0; padding-top: 16px; margin-top: 8px">
        <h4 style="margin-bottom: 12px">批量分配</h4>
        <div v-for="(item, idx) in batchItems" :key="idx" style="margin-bottom: 8px">
          <a-space>
            <span>#{{ idx + 1 }}</span>
            <a-input-number v-model:value="item.ip_count" :min="1" placeholder="IP 数" />
            <a-input v-model:value="item.purpose" placeholder="用途" style="width: 200px" />
            <a-input v-model:value="item.allocated_by" placeholder="负责人（可选）" style="width: 150px" />
            <a-button type="link" danger @click="batchItems.splice(idx, 1)">删除</a-button>
          </a-space>
        </div>
        <a-space style="margin-top: 8px">
          <a-button @click="batchItems.push({ ip_count: 0, purpose: '', allocated_by: '' })">+ 添加</a-button>
          <a-button type="primary" @click="handleBatch" :loading="loading">统一提交</a-button>
        </a-space>
      </div>
    </a-modal>

    <!-- 编辑弹窗 -->
    <a-modal v-model:open="editVisible" title="编辑分配记录" @ok="handleEdit" :confirmLoading="editLoading">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 17 }">
        <a-form-item label="CIDR">
          <span>{{ editForm.cidr }}</span>
        </a-form-item>
        <a-form-item label="用途">
          <a-input v-model:value="editForm.purpose" />
        </a-form-item>
        <a-form-item label="负责人">
          <a-input v-model:value="editForm.allocated_by" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import {
  getPools, getAllocations, allocateSubnet, batchAllocate,
  reclaimAllocation, calculateSubnet, updateAllocation, exportAllocations
} from '../api'

const pools = ref<any[]>([])
const allocations = ref<any[]>([])
const loading = ref(false)
const showBatch = ref(false)
const suggest = ref<any>(null)
const allocVisible = ref(false)

const form = ref({
  pool_id: undefined as number | undefined,
  ip_count: undefined as number | undefined,
  purpose: '',
  allocated_by: '',
})

const batchItems = ref<{ ip_count: number; purpose: string; allocated_by: string }[]>([])

// 编辑相关
const editVisible = ref(false)
const editLoading = ref(false)
const editForm = ref({ id: 0, cidr: '', purpose: '', allocated_by: '' })

const formatTime = (val: string) => {
  if (!val) return ''
  const d = new Date(val)
  if (isNaN(d.getTime())) return val
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

const poolNameMap = computed(() => {
  const map: Record<number, string> = {}
  pools.value.forEach((p: any) => { map[p.id] = `${p.name} (${p.cidr})` })
  return map
})

const allocColumns = [
  { title: 'CIDR', dataIndex: 'cidr', key: 'cidr' },
  { title: '所属网段池', dataIndex: 'pool_id', key: 'pool_id', customRender: ({ text }: any) => poolNameMap.value[text] || text },
  { title: '申请 IP', dataIndex: 'ip_count', key: 'ip_count' },
  { title: '实际 IP', dataIndex: 'actual_count', key: 'actual_count' },
  { title: '用途', dataIndex: 'purpose', key: 'purpose' },
  { title: '负责人', dataIndex: 'allocated_by', key: 'allocated_by' },
  { title: '分配时间', dataIndex: 'allocated_at', key: 'allocated_at', customRender: ({ text }: any) => formatTime(text) },
  { title: '操作', key: 'action' },
]

const fetchPools = async () => {
  const res = await getPools()
  pools.value = res.data || []
}

const selectedPool = computed(() => {
  return pools.value.find((p: any) => p.id === form.value.pool_id) || null
})

const fetchAllocations = async () => {
  const res = await getAllocations()
  allocations.value = res.data || []
}

// 切换网段池时刷新预计算
const onPoolChange = () => {
  onCalc()
}

const openAllocModal = () => {
  allocVisible.value = true
}

const resetAllocForm = () => {
  form.value = { pool_id: undefined, ip_count: undefined, purpose: '', allocated_by: '' }
  suggest.value = null
  showBatch.value = false
  batchItems.value = []
}

// 输入 IP 数量后实时预计算
const onCalc = async () => {
  suggest.value = null
  if (!form.value.pool_id || !form.value.ip_count || form.value.ip_count <= 0) return
  try {
    const res = await calculateSubnet({ pool_id: form.value.pool_id, ip_count: form.value.ip_count })
    suggest.value = res.data
  } catch (e: any) {
    message.error(e.response?.data?.error || '预计算失败')
  }
}

// 单条分配
const handleAllocate = async () => {
  if (!form.value.pool_id || !form.value.ip_count || !form.value.purpose) {
    message.warning('请填写完整信息')
    return
  }
  loading.value = true
  try {
    await allocateSubnet(form.value as any)
    message.success('分配成功')
    allocVisible.value = false
    await fetchPools()
    await fetchAllocations()
  } catch (e: any) {
    message.error(e.response?.data?.error || '分配失败')
  } finally {
    loading.value = false
  }
}

// 批量分配
const handleBatch = async () => {
  if (!form.value.pool_id) {
    message.warning('请先选择网段池')
    return
  }
  const valid = batchItems.value.filter(i => i.ip_count > 0 && i.purpose)
  if (valid.length === 0) {
    message.warning('请至少填写一条有效记录')
    return
  }
  loading.value = true
  try {
    const items = valid.map(i => ({
      pool_id: form.value.pool_id!,
      ip_count: i.ip_count,
      purpose: i.purpose,
      allocated_by: i.allocated_by || form.value.allocated_by,
    }))
    await batchAllocate(items)
    message.success(`成功分配 ${items.length} 条子网`)
    allocVisible.value = false
    await fetchPools()
    await fetchAllocations()
  } catch (e: any) {
    message.error(e.response?.data?.error || '批量分配失败')
  } finally {
    loading.value = false
  }
}

// 打开编辑弹窗
const openEdit = (record: any) => {
  editForm.value = {
    id: record.id,
    cidr: record.cidr,
    purpose: record.purpose || '',
    allocated_by: record.allocated_by || '',
  }
  editVisible.value = true
}

// 提交编辑
const handleEdit = async () => {
  editLoading.value = true
  try {
    await updateAllocation(editForm.value.id, {
      purpose: editForm.value.purpose,
      allocated_by: editForm.value.allocated_by,
    })
    message.success('修改成功')
    editVisible.value = false
    await fetchAllocations()
  } catch (e: any) {
    message.error(e.response?.data?.error || '修改失败')
  } finally {
    editLoading.value = false
  }
}

// 导出分配记录
const handleExport = async (format: 'csv' | 'json') => {
  try {
    await exportAllocations(format)
  } catch (e: any) {
    message.error('导出失败')
  }
}

// 回收子网
const handleReclaim = async (id: number) => {
  try {
    await reclaimAllocation(id)
    message.success('回收成功')
    await fetchPools()
    await fetchAllocations()
  } catch (e: any) {
    message.error(e.response?.data?.error || '回收失败')
  }
}

onMounted(async () => {
  await fetchPools()
  await fetchAllocations()
})
</script>
