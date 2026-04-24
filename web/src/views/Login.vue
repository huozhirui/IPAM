<template>
  <div class="login-wrapper">
    <a-card title="IP 网段规划管理系统" style="width: 400px">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
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
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { login } from '../api'

const router = useRouter()
const loading = ref(false)

const form = ref({
  username: '',
  password: '',
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
