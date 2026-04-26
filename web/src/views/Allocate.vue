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

      <!-- 搜索栏 -->
      <div style="margin-bottom: 16px; display: flex; gap: 12px; flex-wrap: wrap; align-items: center">
        <a-select v-model:value="search.pool_id" placeholder="所属网段池" allowClear style="width: 220px"
          @change="handleSearch">
          <a-select-option v-for="p in pools" :key="p.id" :value="p.id">
            {{ p.name }} ({{ p.cidr }})
          </a-select-option>
        </a-select>
        <a-input v-model:value="search.cidr" placeholder="网段（模糊搜索）" allowClear style="width: 180px"
          @pressEnter="handleSearch" @change="onSearchInputChange" />
        <a-input v-model:value="search.purpose" placeholder="用途（模糊搜索）" allowClear style="width: 180px"
          @pressEnter="handleSearch" @change="onSearchInputChange" />
        <a-input v-model:value="search.allocated_by" placeholder="负责人" allowClear style="width: 150px"
          @pressEnter="handleSearch" @change="onSearchInputChange" />
        <a-button type="primary" @click="handleSearch">搜索</a-button>
        <a-button @click="handleResetSearch">重置</a-button>
      </div>

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
    <a-modal v-model:open="allocVisible" title="分配子网" :footer="null" width="880px" @afterClose="resetAllocForm">

      <!-- 第一步：选择操作模式 -->
      <div style="margin-bottom: 20px">
        <a-radio-group v-model:value="opMode" button-style="solid" size="large">
          <a-radio-button value="single">单个分配</a-radio-button>
          <a-radio-button value="batch">批量分配</a-radio-button>
        </a-radio-group>
      </div>

      <!-- 公共：选择网段池 -->
      <a-form :label-col="{ span: 3 }" :wrapper-col="{ span: 20 }">
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
      </a-form>

      <!-- 单个分配面板 -->
      <div v-if="opMode === 'single'">
        <a-form :label-col="{ span: 3 }" :wrapper-col="{ span: 20 }">
          <a-form-item label="分配方式" required>
            <a-radio-group v-model:value="allocMode" @change="onAllocModeChange">
              <a-radio-button value="count">按数量</a-radio-button>
              <a-radio-button value="cidr">按 CIDR</a-radio-button>
            </a-radio-group>
          </a-form-item>
          <a-form-item v-if="allocMode === 'count'" label="IP 数量" required>
            <a-input-number v-model:value="form.ip_count" :min="1" :max="65536" style="width: 200px"
              @change="onCalc" />
            <span v-if="suggest" style="margin-left: 16px; color: #1890ff">
              推荐: /{{ suggest.prefix_len }} ({{ suggest.actual_count }} IPs) &rarr; {{ suggest.suggest_cidr }}
            </span>
          </a-form-item>
          <a-form-item v-if="allocMode === 'cidr'" label="CIDR 地址" required>
            <a-input v-model:value="form.cidr" placeholder="如：10.0.1.0/24" style="width: 320px" />
            <span style="margin-left: 12px; color: #888; font-size: 12px">输入要分配的网段</span>
          </a-form-item>
          <a-form-item label="用途" required>
            <a-input v-model:value="form.purpose" placeholder="如：VPC-Prod-A" />
          </a-form-item>
          <a-form-item label="负责人">
            <a-input v-model:value="form.allocated_by" placeholder="可选，默认当前登录用户" />
          </a-form-item>
          <a-form-item :wrapper-col="{ offset: 3 }">
            <a-button type="primary" @click="handleAllocate" :loading="loading">确认分配</a-button>
          </a-form-item>
        </a-form>
      </div>

      <!-- 批量分配面板 -->
      <div v-if="opMode === 'batch'">
        <!-- 表头 -->
        <div class="batch-row batch-header">
          <span class="batch-col-seq">#</span>
          <span class="batch-col-status">状态</span>
          <span class="batch-col-mode">方式</span>
          <span class="batch-col-value">数量 / CIDR</span>
          <span class="batch-col-purpose">用途</span>
          <span class="batch-col-owner">负责人</span>
          <span class="batch-col-action">操作</span>
        </div>
        <!-- 数据行 -->
        <div v-for="(item, idx) in batchItems" :key="idx">
          <div class="batch-row" :class="{ 'batch-row-success': item.status === 'success', 'batch-row-error': item.status === 'error' }">
            <span class="batch-col-seq">{{ idx + 1 }}</span>
            <span class="batch-col-status">
              <a-tag v-if="item.status === 'success'" color="success">成功</a-tag>
              <a-tag v-else-if="item.status === 'error'" color="error">失败</a-tag>
              <a-tag v-else color="default">待提交</a-tag>
            </span>
            <span class="batch-col-mode">
              <a-radio-group v-model:value="item.mode" size="small" :disabled="item.status === 'success'" @change="onBatchItemModeChange(item)">
                <a-radio-button value="count">数量</a-radio-button>
                <a-radio-button value="cidr">CIDR</a-radio-button>
              </a-radio-group>
            </span>
            <span class="batch-col-value">
              <a-input-number v-if="item.mode === 'count'" v-model:value="item.ip_count" :min="1" :disabled="item.status === 'success'" placeholder="IP 数" style="width: 100%" />
              <a-input v-if="item.mode === 'cidr'" v-model:value="item.cidr" :disabled="item.status === 'success'" placeholder="10.0.1.0/24" />
            </span>
            <span class="batch-col-purpose">
              <a-input v-model:value="item.purpose" :disabled="item.status === 'success'" placeholder="用途" />
            </span>
            <span class="batch-col-owner">
              <a-input v-model:value="item.allocated_by" :disabled="item.status === 'success'" placeholder="负责人（可选）" />
            </span>
            <span class="batch-col-action">
              <a-button type="link" danger size="small" @click="batchItems.splice(idx, 1)">删除</a-button>
            </span>
          </div>
          <!-- 错误信息 -->
          <div v-if="item.status === 'error' && item.error" class="batch-error-msg">
            {{ item.error }}
          </div>
        </div>
        <!-- 操作按钮 -->
        <div style="margin-top: 12px; display: flex; gap: 8px; align-items: center">
          <a-button @click="addBatchItem">+ 添加一行</a-button>
          <a-button type="primary" @click="handleBatch" :loading="loading" :disabled="pendingCount === 0">
            {{ hasSubmitted ? `重新提交（${pendingCount} 条）` : '统一提交' }}
          </a-button>
          <span v-if="hasSubmitted" style="font-size: 13px; color: #666">
            共 {{ batchItems.length }} 条，成功
            <span style="color: #52c41a; font-weight: 600">{{ successCount }}</span> 条，失败
            <span style="color: #ff4d4f; font-weight: 600">{{ failCount }}</span> 条
          </span>
        </div>
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

interface BatchItem {
  mode: 'count' | 'cidr'
  ip_count: number
  cidr: string
  purpose: string
  allocated_by: string
  status: 'pending' | 'success' | 'error'
  error: string
}

const pools = ref<any[]>([])
const allocations = ref<any[]>([])
const loading = ref(false)
const suggest = ref<any>(null)
const allocVisible = ref(false)
const opMode = ref<'single' | 'batch'>('single')
const allocMode = ref<'count' | 'cidr'>('count')
const hasSubmitted = ref(false)

// 搜索条件
const search = ref({
  pool_id: undefined as number | undefined,
  cidr: '',
  purpose: '',
  allocated_by: '',
})

const form = ref({
  pool_id: undefined as number | undefined,
  ip_count: undefined as number | undefined,
  cidr: '',
  purpose: '',
  allocated_by: '',
})

const batchItems = ref<BatchItem[]>([])

const successCount = computed(() => batchItems.value.filter(i => i.status === 'success').length)
const failCount = computed(() => batchItems.value.filter(i => i.status === 'error').length)
const pendingCount = computed(() => batchItems.value.filter(i => i.status !== 'success').length)

const addBatchItem = () => {
  batchItems.value.push({ mode: 'count', ip_count: 0, cidr: '', purpose: '', allocated_by: '', status: 'pending', error: '' })
}

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
  const params: any = {}
  if (search.value.pool_id) params.pool_id = search.value.pool_id
  if (search.value.cidr) params.cidr = search.value.cidr
  if (search.value.purpose) params.purpose = search.value.purpose
  if (search.value.allocated_by) params.allocated_by = search.value.allocated_by
  const res = await getAllocations(Object.keys(params).length > 0 ? params : undefined)
  allocations.value = res.data || []
}

const handleSearch = () => {
  fetchAllocations()
}

const handleResetSearch = () => {
  search.value = { pool_id: undefined, cidr: '', purpose: '', allocated_by: '' }
  fetchAllocations()
}

// 输入框清空时自动刷新
const onSearchInputChange = (e: any) => {
  const val = e?.target?.value ?? e
  if (val === '' || val === undefined || val === null) {
    fetchAllocations()
  }
}

// 切换网段池时刷新预计算
const onPoolChange = () => {
  onCalc()
}

const openAllocModal = () => {
  allocVisible.value = true
}

const resetAllocForm = () => {
  form.value = { pool_id: undefined, ip_count: undefined, cidr: '', purpose: '', allocated_by: '' }
  suggest.value = null
  opMode.value = 'single'
  allocMode.value = 'count'
  batchItems.value = []
  hasSubmitted.value = false
}

// 切换分配方式时清除另一种模式的数据
const onAllocModeChange = () => {
  suggest.value = null
  if (allocMode.value === 'count') {
    form.value.cidr = ''
  } else {
    form.value.ip_count = undefined
  }
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
  if (!form.value.pool_id || !form.value.purpose) {
    message.warning('请填写完整信息')
    return
  }
  if (allocMode.value === 'count' && !form.value.ip_count) {
    message.warning('请输入 IP 数量')
    return
  }
  if (allocMode.value === 'cidr' && !form.value.cidr) {
    message.warning('请输入 CIDR 地址')
    return
  }
  loading.value = true
  try {
    const data: any = {
      pool_id: form.value.pool_id,
      purpose: form.value.purpose,
      allocated_by: form.value.allocated_by,
    }
    if (allocMode.value === 'count') {
      data.ip_count = form.value.ip_count
    } else {
      data.cidr = form.value.cidr
    }
    await allocateSubnet(data)
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

// 批量项切换模式时清除另一种的值
const onBatchItemModeChange = (item: BatchItem) => {
  if (item.mode === 'count') {
    item.cidr = ''
  } else {
    item.ip_count = 0
  }
  // 修改后重置该行状态
  if (item.status === 'error') {
    item.status = 'pending'
    item.error = ''
  }
}

// 批量分配
const handleBatch = async () => {
  if (!form.value.pool_id) {
    message.warning('请先选择网段池')
    return
  }

  // 只提交未成功的行
  const pendingItems = batchItems.value.filter(i => i.status !== 'success')
  const validItems = pendingItems.filter(i => (i.mode === 'count' ? i.ip_count > 0 : !!i.cidr) && i.purpose)

  if (validItems.length === 0) {
    message.warning('没有可提交的有效记录')
    return
  }

  // 构建提交数据，记录原始行索引映射
  const submitItems: any[] = []
  const indexMap: number[] = [] // submitItems[j] 对应 batchItems 中的索引

  for (let i = 0; i < batchItems.value.length; i++) {
    const item = batchItems.value[i]
    if (item.status === 'success') continue
    const isValid = (item.mode === 'count' ? item.ip_count > 0 : !!item.cidr) && item.purpose
    if (!isValid) {
      // 标记无效行
      item.status = 'error'
      item.error = '请填写完整信息（数量/CIDR + 用途）'
      continue
    }
    const base: any = {
      pool_id: form.value.pool_id!,
      purpose: item.purpose,
      allocated_by: item.allocated_by || form.value.allocated_by,
    }
    if (item.mode === 'cidr') {
      base.cidr = item.cidr
    } else {
      base.ip_count = item.ip_count
    }
    submitItems.push(base)
    indexMap.push(i)
  }

  if (submitItems.length === 0) {
    return
  }

  loading.value = true
  try {
    const res = await batchAllocate(submitItems)
    const data = res.data
    const results: any[] = data.results || []

    // 将后端返回的每条结果映射回原始行
    for (let j = 0; j < results.length; j++) {
      const r = results[j]
      const origIdx = indexMap[j]
      if (origIdx === undefined) continue
      const item = batchItems.value[origIdx]
      if (r.success) {
        item.status = 'success'
        item.error = ''
      } else {
        item.status = 'error'
        item.error = r.error || '分配失败'
      }
    }

    hasSubmitted.value = true
    const sc = data.success_count || 0
    const fc = data.fail_count || 0

    if (fc === 0) {
      message.success(`全部 ${sc} 条分配成功`)
      allocVisible.value = false
    } else if (sc > 0) {
      message.warning(`${sc} 条成功，${fc} 条失败，请修改后重试`)
    } else {
      message.error(`全部 ${fc} 条分配失败，请检查后重试`)
    }

    await fetchPools()
    await fetchAllocations()
  } catch (e: any) {
    message.error(e.response?.data?.error || '批量分配请求失败')
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

<style scoped>
.batch-row {
  display: grid;
  grid-template-columns: 32px 64px 120px 1fr 1fr 1fr 60px;
  gap: 8px;
  align-items: center;
  padding: 6px 0;
}
.batch-header {
  font-weight: 600;
  color: #666;
  font-size: 13px;
  border-bottom: 1px solid #f0f0f0;
  padding-bottom: 8px;
  margin-bottom: 4px;
}
.batch-row-success {
  background: #f6ffed;
  border-radius: 4px;
  padding-left: 4px;
  padding-right: 4px;
}
.batch-row-error {
  background: #fff2f0;
  border-radius: 4px;
  padding-left: 4px;
  padding-right: 4px;
}
.batch-error-msg {
  color: #ff4d4f;
  font-size: 12px;
  padding: 0 0 4px 104px;
  line-height: 1.4;
}
.batch-col-seq { text-align: center; }
.batch-col-status { text-align: center; }
.batch-col-action { text-align: center; }
</style>
