// API 请求封装，所有后端接口统一通过此模块调用
import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

// 请求拦截器：自动附加 JWT Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：401 时清除登录态并跳转登录页
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      localStorage.removeItem('role')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// ---------- 认证 ----------

export const login = (data: { username: string; password: string }) =>
  api.post('/login', data)

export const register = (data: { username: string; password: string }) =>
  api.post('/register', data)

export const getMe = () => api.get('/me')

// ---------- 仪表盘 ----------

export const getDashboard = () => api.get('/dashboard')

// ---------- 网段池 ----------

export const getPools = () => api.get('/pools')

// 创建网段池：支持 CIDR 模式 或 IP 范围模式
export interface CreatePoolReq {
  name: string
  description?: string
  cidr?: string      // CIDR 模式
  start_ip?: string  // 范围模式
  end_ip?: string    // 范围模式
}

export const createPool = (data: CreatePoolReq) =>
  api.post('/pools', data)

export const deletePool = (id: number) => api.delete(`/pools/${id}`)

// ---------- 子网分配 ----------

export interface AllocateReq {
  pool_id: number
  ip_count: number
  purpose: string
  allocated_by?: string
}

export const getAllocations = (poolId?: number) =>
  api.get('/allocations', { params: poolId ? { pool_id: poolId } : {} })

export const allocateSubnet = (data: AllocateReq) =>
  api.post('/allocations', data)

export const batchAllocate = (items: AllocateReq[]) =>
  api.post('/allocations/batch', { items })

export const reclaimAllocation = (id: number) => api.delete(`/allocations/${id}`)

export const updateAllocation = (id: number, data: { purpose?: string; allocated_by?: string }) =>
  api.put(`/allocations/${id}`, data)

// ---------- 剩余网段 ----------

export const getFreeBlocks = (poolId: number) =>
  api.get(`/pools/${poolId}/free-blocks`)

// ---------- 预计算 ----------

export const calculateSubnet = (data: { pool_id: number; ip_count: number }) =>
  api.post('/calculate', data)

// ---------- 审计日志 ----------

export const getAuditLogs = (params: { action?: string; page?: number; page_size?: number }) =>
  api.get('/audit', { params })

// ---------- 导出 ----------

// 带 token 下载文件（解决 window.open 不携带 Authorization header 的问题）
export const downloadFile = async (url: string, filename: string) => {
  const res = await api.get(url, { responseType: 'blob' })
  const blob = new Blob([res.data])
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

export const exportAllocations = (format: 'csv' | 'json' = 'csv') =>
  downloadFile(`/export?format=${format}`, `allocations.${format}`)

export const exportAuditLogs = (format: 'csv' | 'json' = 'csv') =>
  downloadFile(`/export?format=${format}&type=audit`, `audit_logs.${format}`)
