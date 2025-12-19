<template>
  <div class="container">
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <span>系统设置</span>
          <el-button @click="handleRefresh" plain>
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
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

        <el-form-item label="镜像更新检查间隔（分钟）">
          <el-input-number
            v-model="settingsForm.imageUpdateIntervalMinutes"
            :min="5"
            :max="720"
          />
          <div class="help-text">控制全局镜像更新检测的时间间隔，默认 30 分钟。</div>
        </el-form-item>

        <!--<el-form-item label="应用商城服务器地址">
          <el-input 
            v-model="settingsForm.appStoreServerUrl" 
            placeholder="https://template.cgakki.top:33333" 
          />
          <div class="help-text">用于从应用商城获取模板列表与详情。</div>
        </el-form-item>-->

        <el-form-item>
          <el-button type="primary" @click="saveServerSettings" :loading="urlLoading">保存配置</el-button>
        </el-form-item>

        <!--<el-form-item label="Docker Socket Proxy">
          <div class="switch-group">
            <el-switch v-model="settingsForm.socketProxyEnabled" />
            <span class="status-text">{{ settingsForm.socketProxyEnabled ? '已开启' : '已关闭' }}</span>
          </div>
          <div class="help-text">（开发中）开启后允许通过 TCP 端口访问 Docker Socket，请谨慎操作。</div>
        </el-form-item>-->

        <el-divider />
        
      </el-form>
    </el-card>

    <el-card class="settings-card" style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>端口管理设置</span>
        </div>
      </template>
      <div class="port-settings">
        <el-form :model="allocSettings" label-width="140px">
          <el-form-item label="自动分配范围">
            <div class="range-row">
              <el-input-number v-model="allocSettings.start" :min="1024" :max="65535" placeholder="起始端口" />
              <span class="text-gray-500">-</span>
              <el-input-number v-model="allocSettings.end" :min="1024" :max="65535" placeholder="结束端口" />
              <el-button type="primary" @click="saveAllocSettings" :loading="allocSaving">保存范围</el-button>
            </div>
          </el-form-item>
        </el-form>
        
        <div class="help-text">
          <p>自动分配范围：用于应用部署时自动填充端口。</p>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'
import api from '../api'

const settingsForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
  lanUrl: '',
  wanUrl: '',
  appStoreServerUrl: '',
  socketProxyEnabled: false,
  imageUpdateIntervalMinutes: 30
})
const loading = ref(false)
const urlLoading = ref(false)
// const portRange = ref({ start: 0, end: 65535, protocol: 'TCP+UDP' })
const allocSettings = ref({ start: 50000, end: 51000 })
const allocSaving = ref(false)

onMounted(async () => {
  // 加载全局设置
  try {
    const res = await request.get('/settings/global')
    if (res) {
      settingsForm.value.lanUrl = res.lanUrl || ''
      settingsForm.value.wanUrl = res.wanUrl || ''
      settingsForm.value.appStoreServerUrl = res.appStoreServerUrl || ''
      if (typeof res.imageUpdateIntervalMinutes === 'number' && res.imageUpdateIntervalMinutes > 0) {
        settingsForm.value.imageUpdateIntervalMinutes = res.imageUpdateIntervalMinutes
      }
      if (res.allocPortStart) allocSettings.value.start = res.allocPortStart
      if (res.allocPortEnd) allocSettings.value.end = res.allocPortEnd
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  }

  try {
    const pr = await api.ports.getRange()
    if (pr && typeof pr.start === 'number') {
      portRange.value = pr
    }
  } catch (e) {}
})

const handleRefresh = async () => {
  loading.value = true
  try {
    const res = await request.get('/settings/global')
    if (res) {
      settingsForm.value.lanUrl = res.lanUrl || ''
      settingsForm.value.wanUrl = res.wanUrl || ''
      settingsForm.value.appStoreServerUrl = res.appStoreServerUrl || ''
      if (typeof res.imageUpdateIntervalMinutes === 'number' && res.imageUpdateIntervalMinutes > 0) {
        settingsForm.value.imageUpdateIntervalMinutes = res.imageUpdateIntervalMinutes
      }
      if (res.allocPortStart) allocSettings.value.start = res.allocPortStart
      if (res.allocPortEnd) allocSettings.value.end = res.allocPortEnd
    }
    
    const pr = await api.ports.getRange()
    if (pr && typeof pr.start === 'number') {
      // portRange.value = pr
    }
    ElMessage.success('刷新成功')
  } catch (error) {
    console.error('Failed to load settings:', error)
    ElMessage.error('刷新失败')
  } finally {
    loading.value = false
  }
}

const saveServerSettings = async () => {
  urlLoading.value = true
  try {
    await request.post('/settings/global', {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes
    })
    ElMessage.success('配置已保存')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
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

const saveAllocSettings = async () => {
  allocSaving.value = true
  try {
    if (!allocSettings.value.start || !allocSettings.value.end) {
      ElMessage.warning('请输入端口范围')
      return
    }
    if (allocSettings.value.end <= allocSettings.value.start) {
      ElMessage.warning('结束端口必须大于起始端口')
      return
    }
    await request.post('/settings/global', {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes
    })
    ElMessage.success('端口分配范围已保存')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    allocSaving.value = false
  }
}

/*
const testAllocate = async () => {
  allocating.value = true
  allocResult.value = []
  try {
    const res = await api.ports.allocate({ count: allocCount.value, protocol: 'tcp', type: 'host', useAllocRange: false })
    if (res && res.length > 0) {
      allocResult.value = res[0]
      ElMessage.success(`成功分配 ${res[0].length} 个端口`)
    }
  } catch (e) {
    ElMessage.error('分配失败: ' + e.message)
  } finally {
    allocating.value = false
  }
}
*/
</script>

<style scoped>
.container {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.settings-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.settings-form {
  max-width: 800px;
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
.port-settings { padding: 10px; }
.range-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.alloc-result { margin-top: 10px; }
</style>
