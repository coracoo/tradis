import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css' // 引入暗黑模式 CSS 变量
import './assets/css/layout.css'
import App from './App.vue'
import router from './router'
import '@mdi/font/css/materialdesignicons.min.css'

const app = createApp(App)

app.use(ElementPlus)
app.use(router)
app.mount('#app')
