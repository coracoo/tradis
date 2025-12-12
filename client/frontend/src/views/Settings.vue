<template>
  <div class="container">
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <span>系统设置</span>
        </div>
      </template>
      
  <el-form :model="settingsForm" label-width="180px" class="settings-form">
        <div class="section-title">安全设置</div>
        <el-form-item label="修改管理员密码">
          <div class="password-group-vertical">
            <el-input 
              v-model="settingsForm.oldPassword" 
              type="password" 
              placeholder="当前密码"
              show-password 
              class="password-input"
            />
            <el-input 
              v-model="settingsForm.newPassword" 
              type="password" 
              placeholder="新密码"
              show-password 
              class="password-input"
            />
             <el-input 
              v-model="settingsForm.confirmPassword" 
              type="password" 
              placeholder="确认新密码"
              show-password 
              class="password-input"
            />
            <el-button type="primary" @click="updatePassword" :loading="loading">更新密码</el-button>
          </div>
        </el-form-item>

        <el-divider />

        <div class="section-title">服务配置</div>
        <el-form-item label="内网服务器地址">
          <el-input 
            v-model="settingsForm.lanUrl" 
            placeholder="http://192.168.1.100" 
          />
          <div class="help-text">用于自动生成内网访问的容器导航链接。</div>
        </el-form-item>
        
        <el-form-item label="外网服务器地址">
          <el-input 
            v-model="settingsForm.wanUrl" 
            placeholder="https://example.com" 
          />
          <div class="help-text">用于自动生成外网访问的容器导航链接。</div>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="saveServerSettings" :loading="urlLoading">保存配置</el-button>
        </el-form-item>

        <el-form-item label="Docker Socket Proxy">
          <div class="switch-group">
            <el-switch v-model="settingsForm.socketProxyEnabled" />
            <span class="status-text">{{ settingsForm.socketProxyEnabled ? '已开启' : '已关闭' }}</span>
          </div>
          <div class="help-text">开启后允许通过 TCP 端口访问 Docker Socket，请谨慎操作。</div>
        </el-form-item>

        <el-divider />
        
        <div class="section-title">界面设置</div>
        <el-form-item label="主题模式">
           <el-radio-group v-model="theme" @change="handleThemeChange">
             <el-radio-button label="light">浅色</el-radio-button>
             <el-radio-button label="dark">深色</el-radio-button>
             <el-radio-button label="auto">跟随系统</el-radio-button>
           </el-radio-group>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

const settingsForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
  theme: 'light',
  lanUrl: '',
  wanUrl: '',
  socketProxyEnabled: false
})
const loading = ref(false)
const urlLoading = ref(false)

onMounted(async () => {
  // 加载主题设置
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme) {
    settingsForm.value.theme = savedTheme
  }
  
  // 加载全局设置
  try {
    const res = await request.get('/settings/global')
    if (res) {
      settingsForm.value.lanUrl = res.lanUrl || ''
      settingsForm.value.wanUrl = res.wanUrl || ''
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  }
})

const saveServerSettings = async () => {
  if (!settingsForm.value.lanUrl && !settingsForm.value.wanUrl) {
    ElMessage.warning('请至少填写一个地址')
    return
  }
  
  // 简单的格式校验
  if (settingsForm.value.lanUrl && !/^https?:\/\//.test(settingsForm.value.lanUrl)) {
    ElMessage.warning('内网地址必须以 http:// 或 https:// 开头')
    return
  }
  if (settingsForm.value.wanUrl && !/^https?:\/\//.test(settingsForm.value.wanUrl)) {
    ElMessage.warning('外网地址必须以 http:// 或 https:// 开头')
    return
  }

  urlLoading.value = true
  try {
    await request.post('/settings/global', { 
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl
    })
    ElMessage.success('配置已保存')
  } catch (error) {
    console.error('Failed to save settings:', error)
    ElMessage.error('保存失败')
  } finally {
    urlLoading.value = false
  }
}

const updatePassword = async () => {
  if (!settingsForm.value.oldPassword) {
    ElMessage.warning('请输入当前密码')
    return
  }
  if (!settingsForm.value.newPassword) {
    ElMessage.warning('请输入新密码')
    return
  }
  if (settingsForm.value.newPassword !== settingsForm.value.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }

  loading.value = true
  try {
    await request.post('/auth/change-password', {
      oldPassword: settingsForm.value.oldPassword,
      newPassword: settingsForm.value.newPassword
    })
    ElMessage.success('密码修改成功')
    settingsForm.value.oldPassword = ''
    settingsForm.value.newPassword = ''
    settingsForm.value.confirmPassword = ''
  } catch (error) {
    console.error('修改密码失败', error)
    if (error.response && error.response.data && error.response.data.error) {
       ElMessage.error(error.response.data.error)
    } else {
       ElMessage.error('修改密码失败，请重试')
    }
  } finally {
    loading.value = false
  }
}

const handleThemeChange = (val) => {
  // 具体的切换逻辑将在 App.vue 或 layout 中统一处理，这里只保存设置
  localStorage.setItem('theme', val)
  // 触发自定义事件通知
  window.dispatchEvent(new Event('theme-change'))
}
</script>

<style scoped>
.container {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 20px;
  color: var(--el-text-color-primary);
  border-left: 4px solid var(--el-color-primary);
  padding-left: 10px;
}

.password-group-vertical {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 400px;
  width: 100%;
}

.password-input {
  width: 100%;
}

.switch-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.help-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 5px;
}
</style>
