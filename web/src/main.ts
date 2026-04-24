// 应用入口：注册 Ant Design Vue 组件库和路由
import { createApp } from 'vue'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'
import App from './App.vue'
import router from './router'

const app = createApp(App)
app.use(Antd)
app.use(router)
app.mount('#app')
