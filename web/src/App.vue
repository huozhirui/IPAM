<template>
  <!-- 登录页：无侧边栏 -->
  <router-view v-if="isLoginPage" />

  <!-- 主布局：左侧导航 + 右侧内容区 -->
  <a-layout v-else style="min-height: 100vh">
    <a-layout-sider v-model:collapsed="collapsed" collapsible theme="dark">
      <div class="logo">IPAM</div>
      <a-menu theme="dark" mode="inline" :selectedKeys="selectedKeys" @click="onMenuClick">
        <a-menu-item key="/">
          <DashboardOutlined /><span>仪表盘</span>
        </a-menu-item>
        <a-menu-item key="/pools">
          <AppstoreOutlined /><span>网段池管理</span>
        </a-menu-item>
        <a-menu-item key="/allocate">
          <ScissorOutlined /><span>分配子网</span>
        </a-menu-item>
        <a-menu-item key="/free-blocks">
          <UnorderedListOutlined /><span>剩余查询</span>
        </a-menu-item>
        <a-menu-item key="/audit">
          <FileTextOutlined /><span>操作日志</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-layout>
      <a-layout-header style="background: #fff; padding: 0 24px; display: flex; justify-content: space-between; align-items: center;">
        <span style="font-size: 18px; font-weight: bold">{{ currentTitle }}</span>
        <span style="display: flex; align-items: center; gap: 12px;">
          <span style="color: #999; font-size: 12px;">租户:</span>
          <a-select
            v-model:value="currentTenant"
            style="width: 160px;"
            size="small"
            @change="handleTenantChange"
          >
            <a-select-option v-for="tid in myTenants" :key="tid" :value="tid">
              {{ tid }}
            </a-select-option>
          </a-select>
          <span>{{ currentUsername }}</span>
          <a-button type="link" @click="handleLogout">退出登录</a-button>
        </span>
      </a-layout-header>
      <a-layout-content style="margin: 16px; padding: 24px; background: #fff; border-radius: 8px;">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  DashboardOutlined,
  AppstoreOutlined,
  ScissorOutlined,
  UnorderedListOutlined,
  FileTextOutlined,
} from '@ant-design/icons-vue'
import { getMyTenants } from './api'

const router = useRouter()
const route = useRoute()
const collapsed = ref(false)

const selectedKeys = computed(() => [route.path])
const currentTitle = computed(() => (route.meta?.title as string) || 'IP 网段规划管理系统')
const isLoginPage = computed(() => route.path === '/login')
const currentUsername = ref(localStorage.getItem('username') || '')
const currentTenant = ref(localStorage.getItem('tenant_id') || 'default')
const myTenants = ref<string[]>([currentTenant.value])

const onMenuClick = ({ key }: { key: string }) => {
  router.push(key)
}

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  localStorage.removeItem('tenant_id')
  router.push('/login')
}

const handleTenantChange = (value: string) => {
  const prev = localStorage.getItem('tenant_id') || 'default'
  if (value === prev) return
  localStorage.setItem('tenant_id', value)
  currentTenant.value = value
  window.location.reload()
}

const fetchMyTenants = async () => {
  if (!localStorage.getItem('token')) return
  try {
    const res = await getMyTenants()
    if (res.data && res.data.length > 0) {
      myTenants.value = res.data
    }
  } catch {
    // 获取失败时保留当前租户
  }
}

// 路由变化时更新用户名和页面标题；进入主布局时加载租户列表
watch(() => route.path, (path) => {
  currentUsername.value = localStorage.getItem('username') || ''
  currentTenant.value = localStorage.getItem('tenant_id') || 'default'
  document.title = `${currentTitle.value} - IPAM`
  if (path !== '/login') {
    fetchMyTenants()
  }
}, { immediate: true })
</script>

<style>
.logo {
  height: 48px;
  line-height: 48px;
  text-align: center;
  color: #fff;
  font-size: 20px;
  font-weight: bold;
  letter-spacing: 4px;
}
body { margin: 0; }
</style>
