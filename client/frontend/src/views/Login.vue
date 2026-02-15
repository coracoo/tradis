<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <h2>TRADIS</h2>
          <h4>最适合新手的 Docker 面板</h4>
        </div>
      </template>
      <el-form :model="loginForm" :rules="rules" ref="loginFormRef" label-width="0px" @keyup.enter="handleLogin">
        <el-form-item prop="username">
          <el-input 
            v-model="loginForm.username" 
            placeholder="用户名" 
            :prefix-icon="IconEpUser" 
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input 
            v-model="loginForm.password" 
            type="password" 
            placeholder="密码" 
            :prefix-icon="IconEpLock" 
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
import api from '../api'

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
        const res = await api.auth.login(loginForm)
        
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
  background-color: var(--clay-bg);
  padding: 18px;
  box-sizing: border-box;
}

.login-card {
  width: 400px;
  max-width: 100%;
  border-radius: var(--radius-5xl);
  border: 1px solid var(--clay-border);
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-float), var(--shadow-clay-inner);
}

.card-header {
  text-align: center;
  padding: 10px 0;
}

.card-header h2 {
  margin: 0;
  color: var(--clay-ink);
  font-weight: 900;
  font-size: 24px;
  letter-spacing: -0.3px;
}

.login-button {
  width: 100%;
  border-radius: var(--el-border-radius-base);
  font-weight: 600;
}

:deep(.el-input__wrapper) {
  border-radius: var(--el-border-radius-base);
}
</style>
