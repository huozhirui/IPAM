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
        <span>
          {{ currentUsername }}
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

const router = useRouter()
const route = useRoute()
const collapsed = ref(false)

const selectedKeys = computed(() => [route.path])
const currentTitle = computed(() => (route.meta?.title as string) || 'IP 网段规划管理系统')
const isLoginPage = computed(() => route.path === '/login')
const currentUsername = ref(localStorage.getItem('username') || '')

const onMenuClick = ({ key }: { key: string }) => {
  router.push(key)
}

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  router.push('/login')
}

// 路由变化时更新用户名和页面标题
watch(() => route.path, () => {
  currentUsername.value = localStorage.getItem('username') || ''
  document.title = `${currentTitle.value} - IPAM`
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
