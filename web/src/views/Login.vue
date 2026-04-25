<template>
  <div class="login-wrapper">
    <a-card title="IP 网段规划管理系统" style="width: 400px">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="租户">
          <a-select
            v-model:value="form.tenant"
            placeholder="选择租户"
            :loading="tenantsLoading"
            show-search
          >
            <a-select-option v-for="t in tenantList" :key="t.slug" :value="t.slug">
              {{ t.name }} ({{ t.slug }})
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="用户名">
          <a-input v-model:value="form.username" placeholder="请输入用户名" @pressEnter="handleLogin" />
        </a-form-item>
        <a-form-item label="密码">
          <a-input-password v-model:value="form.password" placeholder="请输入密码" @pressEnter="handleLogin" />
        </a-form-item>
        <a-form-item :wrapper-col="{ offset: 6, span: 16 }">
          <a-button type="primary" block :loading="loading" @click="handleLogin">登录</a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { login, getTenants } from '../api'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const tenantsLoading = ref(false)
const tenantList = ref<{ id: number; name: string; slug: string }[]>([])

const form = ref({
  tenant: 'default',
  username: '',
  password: '',
})

onMounted(async () => {
  if (route.query.tenant) {
    form.value.tenant = route.query.tenant as string
  }
  tenantsLoading.value = true
  try {
    const res = await getTenants()
    tenantList.value = res.data
  } catch {
    // 获取租户列表失败时保留手动输入的默认值
  } finally {
    tenantsLoading.value = false
  }
})

const handleLogin = async () => {
  if (!form.value.username || !form.value.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const res = await login(form.value)
    localStorage.setItem('token', res.data.token)
    localStorage.setItem('username', res.data.user.username)
    localStorage.setItem('role', res.data.user.role)
    localStorage.setItem('tenant_id', res.data.user.tenant_id || 'default')
    message.success('登录成功')
    router.push('/')
  } catch (e: any) {
    message.error(e.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f0f2f5;
}
</style>
