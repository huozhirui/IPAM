// 前端路由配置
import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { title: '登录', public: true },
  },
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue'),
    meta: { title: '仪表盘' },
  },
  {
    path: '/pools',
    name: 'PoolManage',
    component: () => import('../views/PoolManage.vue'),
    meta: { title: '网段池管理' },
  },
  {
    path: '/allocate',
    name: 'Allocate',
    component: () => import('../views/Allocate.vue'),
    meta: { title: '分配子网' },
  },
  {
    path: '/free-blocks',
    name: 'FreeBlocks',
    component: () => import('../views/FreeBlocks.vue'),
    meta: { title: '剩余查询' },
  },
  {
    path: '/audit',
    name: 'AuditLog',
    component: () => import('../views/AuditLog.vue'),
    meta: { title: '操作日志' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 路由守卫：未登录时跳转登录页
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (!token && !to.meta.public) {
    next('/login')
  } else if (token && to.path === '/login') {
    next('/')
  } else {
    next()
  }
})

export default router
