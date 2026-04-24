<template>
  <!-- 网段池管理：列表 + 新增弹窗（支持 CIDR / IP 范围两种模式） -->
  <div>
    <a-button v-if="isAdmin" type="primary" @click="openCreate" style="margin-bottom: 16px">新增网段池</a-button>

    <a-table :dataSource="pools" :columns="columns" rowKey="id" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'usage'">
          <a-progress :percent="Number(record.usage_rate.toFixed(1))" :strokeWidth="10" size="small" />
        </template>
        <template v-if="column.key === 'action'">
          <template v-if="isAdmin">
            <a-popconfirm title="确认删除该网段池？" @confirm="handleDelete(record.id)">
              <a-button type="link" danger :disabled="record.alloc_count > 0">删除</a-button>
            </a-popconfirm>
            <span v-if="record.alloc_count > 0" style="color: #999; font-size: 12px">
              (有 {{ record.alloc_count }} 条分配)
            </span>
          </template>
          <span v-else style="color: #999">—</span>
        </template>
      </template>
    </a-table>

    <!-- 新增弹窗 -->
    <a-modal v-model:open="showModal" title="新增网段池" @ok="handleCreate" :confirmLoading="loading">
      <a-form :label-col="{ span: 5 }">
        <a-form-item label="名称" required>
          <a-input v-model:value="form.name" placeholder="如：生产环境" />
        </a-form-item>

        <!-- 输入模式切换 -->
        <a-form-item label="模式">
          <a-radio-group v-model:value="inputMode">
            <a-radio value="cidr">CIDR</a-radio>
            <a-radio value="range">IP 范围</a-radio>
          </a-radio-group>
        </a-form-item>

        <!-- CIDR 模式 -->
        <a-form-item v-if="inputMode === 'cidr'" label="CIDR" required>
          <a-input v-model:value="form.cidr" placeholder="如：10.0.0.0/16" />
        </a-form-item>

        <!-- 范围模式 -->
        <template v-if="inputMode === 'range'">
          <a-form-item label="起始 IP" required>
            <a-input v-model:value="form.start_ip" placeholder="如：10.0.0.0" />
          </a-form-item>
          <a-form-item label="结束 IP" required>
            <a-input v-model:value="form.end_ip" placeholder="如：10.0.255.255" />
          </a-form-item>
        </template>

        <a-form-item label="备注">
          <a-textarea v-model:value="form.description" :rows="2" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import { getPools, createPool, deletePool } from '../api'

const isAdmin = computed(() => localStorage.getItem('role') === 'admin')

const pools = ref<any[]>([])
const showModal = ref(false)
const loading = ref(false)
const inputMode = ref<'cidr' | 'range'>('cidr')

const form = ref({
  name: '',
  cidr: '',
  start_ip: '',
  end_ip: '',
  description: '',
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '网段', dataIndex: 'cidr', key: 'cidr' },
  { title: '起始 IP', dataIndex: 'start_ip', key: 'start_ip' },
  { title: '结束 IP', dataIndex: 'end_ip', key: 'end_ip' },
  { title: '总 IP', dataIndex: 'total_ips', key: 'total_ips' },
  { title: '已用 IP', dataIndex: 'used_ips', key: 'used_ips' },
  { title: '使用率', key: 'usage', width: 200 },
  { title: '操作', key: 'action' },
]

const fetchPools = async () => {
  const res = await getPools()
  pools.value = res.data || []
}

const openCreate = () => {
  form.value = { name: '', cidr: '', start_ip: '', end_ip: '', description: '' }
  inputMode.value = 'cidr'
  showModal.value = true
}

const handleCreate = async () => {
  if (!form.value.name) {
    message.warning('名称为必填项')
    return
  }
  if (inputMode.value === 'cidr' && !form.value.cidr) {
    message.warning('请填写 CIDR')
    return
  }
  if (inputMode.value === 'range' && (!form.value.start_ip || !form.value.end_ip)) {
    message.warning('请填写起始 IP 和结束 IP')
    return
  }

  // 根据模式构造请求参数
  const data: any = { name: form.value.name, description: form.value.description }
  if (inputMode.value === 'cidr') {
    data.cidr = form.value.cidr
  } else {
    data.start_ip = form.value.start_ip
    data.end_ip = form.value.end_ip
  }

  loading.value = true
  try {
    await createPool(data)
    message.success('创建成功')
    showModal.value = false
    await fetchPools()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建失败')
  } finally {
    loading.value = false
  }
}

const handleDelete = async (id: number) => {
  try {
    await deletePool(id)
    message.success('删除成功')
    await fetchPools()
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除失败')
  }
}

onMounted(fetchPools)
</script>
