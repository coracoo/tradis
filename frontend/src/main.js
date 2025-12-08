import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css' // 引入暗黑模式 CSS 变量
import './assets/styles/dark.css' // 引入自定义暗黑模式样式
import App from './App.vue'
import router from './router'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'  // 添加这行

// 配置 axios 默认值
axios.defaults.baseURL = import.meta.env.PROD ? '' : ''  // 移除 '/api'
axios.defaults.headers.common['Content-Type'] = 'application/json'

// 添加响应拦截器
axios.interceptors.response.use(
  response => response.data,
  error => {
    // 添加更详细的错误日志
    console.error('API Error:', {
      url: error.config?.url,
      status: error.response?.status,
      data: error.response?.data,
      message: error.message
    })
    ElMessage.error(error.response?.data?.message || '请求失败')
    return Promise.reject(error)
  }
)

const app = createApp(App)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(ElementPlus)
app.use(router)
app.mount('#app')
