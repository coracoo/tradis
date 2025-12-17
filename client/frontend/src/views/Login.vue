<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <h2>Docker Manager</h2>
        </div>
      </template>
      <el-form :model="loginForm" :rules="rules" ref="loginFormRef" label-width="0px" @keyup.enter="handleLogin">
        <el-form-item prop="username">
          <el-input 
            v-model="loginForm.username" 
            placeholder="用户名" 
            :prefix-icon="User" 
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input 
            v-model="loginForm.password" 
            type="password" 
            placeholder="密码" 
            :prefix-icon="Lock" 
            show-password 
            size="large"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" class="login-button" :loading="loading" @click="handleLogin" size="large">
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import request from '../utils/request' // 修改为导入 axios 实例或者直接使用 index.js 导出的 api 对象

const router = useRouter()
const route = useRoute()
const loginFormRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  await loginFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        // 直接使用 axios 实例进行请求，或者使用 api 对象中定义的方法（如果定义了 auth.login）
        // 由于 api/index.js 默认导出的对象中没有 auth 模块，且也没有直接暴露 axios 实例的方法（如 .post）
        // 所以这里应该导入 utils/request.js 或者直接在 api/index.js 中添加 auth 模块
        
        // 方案 1：使用 utils/request.js (如果它是一个 axios 实例)
        const res = await request.post('/auth/login', loginForm)
        
        // 兼容不同返回结构
        const token = res.token || res.data?.token
        if (token) {
            localStorage.setItem('token', token)
            localStorage.setItem('username', loginForm.username)
            ElMessage.success('登录成功')
            
            // 跳转到重定向页面或首页
            const redirect = route.query.redirect || '/'
            router.push(redirect)
        } else {
             ElMessage.error('登录失败: 未获取到 token')
        }
      } catch (error) {
        // 错误处理通常在 request 拦截器中统一处理，这里可额外处理
        console.error(error)
      } finally {
        loading.value = false
      }
    }
  })
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background-color: var(--el-bg-color-page);
}

.login-card {
  width: 400px;
  border-radius: 12px;
  border: none;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
}

.card-header {
  text-align: center;
  padding: 10px 0;
}

.card-header h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-weight: 600;
  font-size: 24px;
}

.login-button {
  width: 100%;
  border-radius: 8px;
  font-weight: 600;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
}
</style>
