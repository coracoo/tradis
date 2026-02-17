<template>
  <div class="container">
    <div class="settings-header">
      <div class="header-title">系统设置</div>
      <el-button @click="handleRefresh" plain>
        <template #icon><IconEpRefresh /></template>
        刷新
      </el-button>
    </div>

    <div class="settings-grid">
      <div class="settings-column">
        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>外观设置</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
            <el-form-item label="界面风格">
              <el-radio-group v-model="uiTheme" @change="handleThemeChange" class="theme-group">
                <el-radio-button label="modern">SaaS Modern (默认)</el-radio-button>
                <el-radio-button label="retro">复古/终端</el-radio-button>
                <el-radio-button label="clay">拟态粘土</el-radio-button>
              </el-radio-group>
              <div class="help-text">切换不同风格的主题，体验不一样的界面氛围。</div>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>高级选项</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
            <el-form-item label="高级模式（允许编辑 YAML）">
              <div class="switch-row">
                <el-switch v-model="settingsForm.advancedMode" />
                <el-button type="warning" plain @click="handleAdvancedSettingsClick" :loading="urlLoading">
                  {{ settingsForm.advancedMode ? '关闭高级设置' : '开启高级设置' }}
                </el-button>
                <el-button type="primary" @click="saveServerSettings" :loading="urlLoading">保存</el-button>
              </div>
              <div class="help-text">关闭后将禁用高风险的 YAML 编辑与保存入口，减少误操作。</div>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>安全设置</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
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
          </el-form>
        </el-card>

        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>服务配置</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
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
                controls-position="right"
                class="w-full"
              />
              <div class="help-text">控制全局镜像更新检测的时间间隔，默认 120 分钟。</div>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="saveServerSettings" :loading="urlLoading" class="w-full">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>端口管理设置</span>
            </div>
          </template>
          <div class="port-settings">
            <el-form :model="allocSettings" label-position="top">
              <el-form-item label="自动分配范围">
                <div class="range-row">
                  <el-input-number v-model="allocSettings.start" :min="1024" :max="65535" placeholder="起始端口" controls-position="right" class="port-input" />
                  <span class="text-gray-500">-</span>
                  <el-input-number v-model="allocSettings.end" :min="1024" :max="65535" placeholder="结束端口" controls-position="right" class="port-input" />
                  <el-button type="primary" @click="saveAllocSettings" :loading="allocSaving">保存范围</el-button>
                </div>
              </el-form-item>
            </el-form>

            <div class="help-text">
              <p>开启后：应用部署页面将自动为端口参数填充可用端口（来自锁定范围）。</p>
            </div>
          </div>
        </el-card>
      </div>

      <div class="settings-column">
        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>AI 助手</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
            <el-form-item label="启用 AI 自动管理导航">
              <div class="switch-group">
                <el-switch v-model="settingsForm.aiEnabled" />
              </div>
              <div class="help-text">开启后：在现有自动发现基础上，自动剔除非 Web 端口、补全图标与分类，并为 AI 生成项打标记。</div>
            </el-form-item>

            <el-form-item label="Base URL">
              <el-input v-model="settingsForm.aiBaseUrl" placeholder="例如：https://api.openai.com/v1 或 https://xxx/api/paas/v4" />
              <div class="help-text">后端不会自动补全版本路径，只会在 Base URL 后追加 /chat/completions。</div>
              <div class="help-text">最终请求 URL：{{ aiFinalUrl || '-' }}</div>
            </el-form-item>

            <el-form-item label="API Key">
              <el-input
                v-model="settingsForm.aiApiKey"
                type="password"
                placeholder="留空表示不修改"
                show-password
              />
              <div class="help-text">当前状态：{{ settingsForm.aiApiKeySet ? '已配置' : '未配置' }}。出于安全考虑，不会回显已保存的 Key。</div>
            </el-form-item>

            <el-form-item label="Model">
              <el-input v-model="settingsForm.aiModel" placeholder="例如：gpt-4o-mini / qwen2.5 / deepseek-chat" />
            </el-form-item>

            <el-form-item label="Temperature">
              <el-input-number v-model="settingsForm.aiTemperature" :min="0" :max="2" :step="0.1" controls-position="right" class="w-full" />
            </el-form-item>

            <el-form-item label="提示词（Prompt）">
              <el-input v-model="settingsForm.aiPrompt" type="textarea" :rows="5" placeholder="用于导航整理的系统提示词" />
            </el-form-item>

            <el-form-item>
              <div class="ai-actions">
                <el-button type="primary" @click="saveAiSettings" :loading="aiSaving">保存 AI 配置</el-button>
                <el-button @click="testAiConnectivity" :loading="aiTesting" plain>连接性测试</el-button>
                <el-button v-if="settingsForm.aiApiKeySet" @click="clearAiApiKey" plain>清空 Key</el-button>
              </div>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="settings-card">
          <template #header>
            <div class="card-header">
              <span>卷备份（docker-volume-backup）</span>
            </div>
          </template>
          <el-form :model="settingsForm" label-position="top" class="settings-form">
            <el-form-item label="启用卷定时备份">
              <div class="switch-group">
                <el-switch v-model="settingsForm.volumeBackupEnabled" />
              </div>
              <div class="help-text">启用后会创建并托管一个 offen/docker-volume-backup 容器，配置来自下方环境变量。</div>
            </el-form-item>

            <el-form-item label="镜像">
              <el-input v-model="settingsForm.volumeBackupImage" placeholder="offen/docker-volume-backup:latest" />
            </el-form-item>

            <el-form-item label="选择需要备份的卷">
              <el-select
                v-model="settingsForm.volumeBackupVolumes"
                multiple
                filterable
                clearable
                collapse-tags
                class="w-full"
                placeholder="选择卷（可多选）"
              >
                <el-option v-for="v in volumeOptions" :key="v" :label="v" :value="v" />
              </el-select>
              <div class="help-text">会以只读方式挂载到 /backup/&lt;volume&gt; 供 docker-volume-backup 备份。</div>
            </el-form-item>

            <el-form-item label="本地归档目录（可选）">
              <el-input v-model="settingsForm.volumeBackupArchiveDir" placeholder="例如：/data/backups" />
              <div class="help-text">配置后会将该目录挂载到容器 /archive，用于保存本地备份副本。</div>
              <el-alert
                type="warning"
                :closable="false"
                class="archive-dir-alert"
                title="请提前在宿主机创建该目录，否则容器挂载/写入可能失败。"
              />
            </el-form-item>

            <el-form-item label="每日备份（Cron 表达式）">
              <el-input v-model="settingsForm.volumeBackupCronExpression" placeholder="@daily" />
              <div class="help-text">默认 @daily。更多配置参考：https://offen.github.io/docker-volume-backup/reference/</div>
            </el-form-item>

            <el-form-item label="挂载 Docker Socket">
              <div class="switch-group">
                <el-switch v-model="settingsForm.volumeBackupMountDockerSock" />
              </div>
              <div class="help-text">允许备份容器与 Docker 交互（例如 stop-during-backup）。禁用则不挂载 /var/run/docker.sock。</div>
            </el-form-item>

            <el-form-item label="环境变量（按行 KEY=VALUE，可直接粘贴官方 env_file 内容）">
              <el-input
                v-model="settingsForm.volumeBackupEnv"
                type="textarea"
                :rows="6"
                placeholder="例如：&#10;BACKUP_CRON_EXPRESSION=0 3 * * *&#10;BACKUP_FILENAME=backup-%Y-%m-%dT%H-%M-%S.tar.gz&#10;AWS_S3_BUCKET_NAME=xxx"
              />
              <div class="help-text">仅支持 KEY=VALUE 格式；不会在界面日志中回显敏感值。参考：https://offen.github.io/docker-volume-backup/reference/</div>
            </el-form-item>

            <el-form-item>
              <div class="ai-actions">
                <el-button type="primary" @click="saveVolumeBackupSettings" :loading="volumeBackupSaving">保存卷备份配置</el-button>
                <el-button
                  type="warning"
                  plain
                  :disabled="!settingsForm.volumeBackupEnabled"
                  :loading="volumeBackupRebuilding"
                  @click="rebuildVolumeBackup"
                >重建备份容器</el-button>
                <el-button @click="refreshVolumeOptions" plain>刷新卷列表</el-button>
              </div>
            </el-form-item>
          </el-form>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import api from '../api'

const settingsForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
  lanUrl: '',
  wanUrl: '',
  appStoreServerUrl: '',
  advancedMode: false,
  socketProxyEnabled: false,
  imageUpdateIntervalMinutes: 120,
  aiEnabled: false,
  aiBaseUrl: '',
  aiApiKey: '',
  aiApiKeySet: false,
  aiModel: '',
  aiTemperature: 0.7,
  aiPrompt: '',
  volumeBackupEnabled: false,
  volumeBackupImage: 'offen/docker-volume-backup:latest',
  volumeBackupEnv: '',
  volumeBackupCronExpression: '@daily',
  volumeBackupVolumes: [],
  volumeBackupArchiveDir: '',
  volumeBackupMountDockerSock: true
})

const uiTheme = ref(localStorage.getItem('ui-theme') || 'clay')

const syncAdvancedModeLocal = () => {
  localStorage.setItem('advancedMode', settingsForm.value.advancedMode ? '1' : '0')
  window.dispatchEvent(new Event('advanced-mode-change'))
}

const handleAdvancedSettingsClick = async () => {
  if (urlLoading.value) return
  const next = !settingsForm.value.advancedMode
  const title = next ? '开启高级设置' : '关闭高级设置'
  const tip = next
    ? '开启后将允许修改并保存高风险的 YAML 配置入口，建议仅在明确知道修改内容时使用。是否继续？'
    : '关闭后将禁用高风险的 YAML 编辑与保存入口。是否继续？'
  try {
    await ElMessageBox.confirm(tip, title, {
      confirmButtonText: '继续',
      cancelButtonText: '取消',
      type: next ? 'warning' : 'info'
    })
  } catch (e) {
    return
  }
  settingsForm.value.advancedMode = next
  await saveServerSettings()
}

const handleThemeChange = (val) => {
  localStorage.setItem('ui-theme', val)
  window.dispatchEvent(new Event('ui-theme-change'))
  ElMessage.success('主题已切换')
}

const loading = ref(false)
const urlLoading = ref(false)
const aiSaving = ref(false)
const aiTesting = ref(false)
const volumeBackupSaving = ref(false)
const volumeBackupRebuilding = ref(false)
const portRange = ref({ start: 0, end: 65535, protocol: 'TCP+UDP' })
const allocSettings = ref({ start: 50000, end: 51000, allowAutoAllocPort: false })
const allocSaving = ref(false)
const volumeOptions = ref([])

const buildAiEndpoint = (baseUrl) => {
  const u = (baseUrl || '').trim().replace(/\/+$/, '')
  if (!u) return ''
  return `${u}/chat/completions`
}

const aiFinalUrl = computed(() => buildAiEndpoint(settingsForm.value.aiBaseUrl))

onMounted(async () => {
  // 加载全局设置
  try {
    const res = await request.get('/settings/global')
    if (res) {
      settingsForm.value.lanUrl = res.lanUrl || ''
      settingsForm.value.wanUrl = res.wanUrl || ''
      settingsForm.value.appStoreServerUrl = res.appStoreServerUrl || ''
      if (typeof res.advancedMode === 'boolean') settingsForm.value.advancedMode = res.advancedMode
      if (typeof res.aiEnabled === 'boolean') settingsForm.value.aiEnabled = res.aiEnabled
      settingsForm.value.aiBaseUrl = res.aiBaseUrl || ''
      settingsForm.value.aiApiKey = ''
      settingsForm.value.aiApiKeySet = !!res.aiApiKeySet
      settingsForm.value.aiModel = res.aiModel || ''
      if (typeof res.aiTemperature === 'number') settingsForm.value.aiTemperature = res.aiTemperature
      settingsForm.value.aiPrompt = res.aiPrompt || ''
      if (typeof res.imageUpdateIntervalMinutes === 'number' && res.imageUpdateIntervalMinutes > 0) {
        settingsForm.value.imageUpdateIntervalMinutes = res.imageUpdateIntervalMinutes
      }
      if (res.allocPortStart) allocSettings.value.start = res.allocPortStart
      if (res.allocPortEnd) allocSettings.value.end = res.allocPortEnd
      if (typeof res.allowAutoAllocPort === 'boolean') allocSettings.value.allowAutoAllocPort = res.allowAutoAllocPort
      if (typeof res.volumeBackupEnabled === 'boolean') settingsForm.value.volumeBackupEnabled = res.volumeBackupEnabled
      settingsForm.value.volumeBackupImage = res.volumeBackupImage || 'offen/docker-volume-backup:latest'
      settingsForm.value.volumeBackupEnv = res.volumeBackupEnv || ''
      settingsForm.value.volumeBackupCronExpression = res.volumeBackupCronExpression || '@daily'
      settingsForm.value.volumeBackupVolumes = Array.isArray(res.volumeBackupVolumes) ? res.volumeBackupVolumes : []
      settingsForm.value.volumeBackupArchiveDir = res.volumeBackupArchiveDir || ''
      if (typeof res.volumeBackupMountDockerSock === 'boolean') settingsForm.value.volumeBackupMountDockerSock = res.volumeBackupMountDockerSock
      syncAdvancedModeLocal()
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

  await refreshVolumeOptions()
})

const handleRefresh = async () => {
  loading.value = true
  try {
    const res = await request.get('/settings/global')
    if (res) {
      settingsForm.value.lanUrl = res.lanUrl || ''
      settingsForm.value.wanUrl = res.wanUrl || ''
      settingsForm.value.appStoreServerUrl = res.appStoreServerUrl || ''
      if (typeof res.advancedMode === 'boolean') settingsForm.value.advancedMode = res.advancedMode
      if (typeof res.aiEnabled === 'boolean') settingsForm.value.aiEnabled = res.aiEnabled
      settingsForm.value.aiBaseUrl = res.aiBaseUrl || ''
      settingsForm.value.aiApiKey = ''
      settingsForm.value.aiApiKeySet = !!res.aiApiKeySet
      settingsForm.value.aiModel = res.aiModel || ''
      if (typeof res.aiTemperature === 'number') settingsForm.value.aiTemperature = res.aiTemperature
      settingsForm.value.aiPrompt = res.aiPrompt || ''
      if (typeof res.imageUpdateIntervalMinutes === 'number' && res.imageUpdateIntervalMinutes > 0) {
        settingsForm.value.imageUpdateIntervalMinutes = res.imageUpdateIntervalMinutes
      }
      if (res.allocPortStart) allocSettings.value.start = res.allocPortStart
      if (res.allocPortEnd) allocSettings.value.end = res.allocPortEnd
      if (typeof res.allowAutoAllocPort === 'boolean') allocSettings.value.allowAutoAllocPort = res.allowAutoAllocPort
      if (typeof res.volumeBackupEnabled === 'boolean') settingsForm.value.volumeBackupEnabled = res.volumeBackupEnabled
      settingsForm.value.volumeBackupImage = res.volumeBackupImage || 'offen/docker-volume-backup:latest'
      settingsForm.value.volumeBackupEnv = res.volumeBackupEnv || ''
      settingsForm.value.volumeBackupCronExpression = res.volumeBackupCronExpression || '@daily'
      settingsForm.value.volumeBackupVolumes = Array.isArray(res.volumeBackupVolumes) ? res.volumeBackupVolumes : []
      settingsForm.value.volumeBackupArchiveDir = res.volumeBackupArchiveDir || ''
      if (typeof res.volumeBackupMountDockerSock === 'boolean') settingsForm.value.volumeBackupMountDockerSock = res.volumeBackupMountDockerSock
      syncAdvancedModeLocal()
    }
    
    const pr = await api.ports.getRange()
    if (pr && typeof pr.start === 'number') {
      // portRange.value = pr
    }
    await refreshVolumeOptions()
    ElMessage.success('刷新成功')
  } catch (error) {
    console.error('Failed to load settings:', error)
    ElMessage.error('刷新失败')
  } finally {
    loading.value = false
  }
}

const refreshVolumeOptions = async () => {
  try {
    const res = await api.volumes.list()
    const list = Array.isArray(res?.Volumes) ? res.Volumes : []
    volumeOptions.value = list.map((v) => v?.Name).filter(Boolean).sort()
  } catch (e) {
    volumeOptions.value = []
  }
}

const saveServerSettings = async () => {
  urlLoading.value = true
  try {
    await request.post('/settings/global', {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      advancedMode: !!settingsForm.value.advancedMode,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      allowAutoAllocPort: !!allocSettings.value.allowAutoAllocPort,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes
    })
    syncAdvancedModeLocal()
    ElMessage.success('配置已保存')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    urlLoading.value = false
  }
}

const saveVolumeBackupSettings = async () => {
  volumeBackupSaving.value = true
  try {
    await request.post('/settings/global', {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      advancedMode: !!settingsForm.value.advancedMode,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      allowAutoAllocPort: !!allocSettings.value.allowAutoAllocPort,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes,
      volumeBackupEnabled: !!settingsForm.value.volumeBackupEnabled,
      volumeBackupImage: settingsForm.value.volumeBackupImage,
      volumeBackupEnv: settingsForm.value.volumeBackupEnv,
      volumeBackupCronExpression: settingsForm.value.volumeBackupCronExpression,
      volumeBackupVolumes: settingsForm.value.volumeBackupVolumes,
      volumeBackupArchiveDir: settingsForm.value.volumeBackupArchiveDir,
      volumeBackupMountDockerSock: !!settingsForm.value.volumeBackupMountDockerSock
    })
    syncAdvancedModeLocal()
    ElMessage.success('卷备份配置已保存')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    volumeBackupSaving.value = false
  }
}

const rebuildVolumeBackup = async () => {
  if (!settingsForm.value.volumeBackupEnabled) {
    ElMessage.warning('请先启用卷备份')
    return
  }
  try {
    await ElMessageBox.confirm('确定要重建卷备份容器吗？', '提示', { type: 'warning' })
  } catch (e) {
    return
  }
  volumeBackupRebuilding.value = true
  try {
    await api.system.volumeBackupRebuild()
    ElMessage.success('已触发重建')
  } catch (error) {
    ElMessage.error('重建失败: ' + (error.response?.data?.error || error.message))
  } finally {
    volumeBackupRebuilding.value = false
  }
}

const saveAiSettings = async () => {
  aiSaving.value = true
  try {
    const payload = {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      advancedMode: !!settingsForm.value.advancedMode,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      allowAutoAllocPort: !!allocSettings.value.allowAutoAllocPort,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes,
      aiEnabled: !!settingsForm.value.aiEnabled,
      aiBaseUrl: settingsForm.value.aiBaseUrl,
      aiModel: settingsForm.value.aiModel,
      aiTemperature: settingsForm.value.aiTemperature,
      aiPrompt: settingsForm.value.aiPrompt
    }
    if (settingsForm.value.aiApiKey !== '') {
      payload.aiApiKey = settingsForm.value.aiApiKey
    }
    await request.post('/settings/global', payload)
    settingsForm.value.aiApiKey = ''
    const res = await request.get('/settings/global')
    settingsForm.value.aiApiKeySet = !!res?.aiApiKeySet
    if (typeof res?.advancedMode === 'boolean') settingsForm.value.advancedMode = res.advancedMode
    syncAdvancedModeLocal()
    ElMessage.success('AI 配置已保存')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    aiSaving.value = false
  }
}

const clearAiApiKey = async () => {
  aiSaving.value = true
  try {
    await request.post('/settings/global', {
      lanUrl: settingsForm.value.lanUrl,
      wanUrl: settingsForm.value.wanUrl,
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      advancedMode: !!settingsForm.value.advancedMode,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      allowAutoAllocPort: !!allocSettings.value.allowAutoAllocPort,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes,
      aiEnabled: !!settingsForm.value.aiEnabled,
      aiBaseUrl: settingsForm.value.aiBaseUrl,
      aiModel: settingsForm.value.aiModel,
      aiTemperature: settingsForm.value.aiTemperature,
      aiPrompt: settingsForm.value.aiPrompt,
      aiApiKey: ''
    })
    settingsForm.value.aiApiKey = ''
    settingsForm.value.aiApiKeySet = false
    syncAdvancedModeLocal()
    ElMessage.success('Key 已清空')
  } catch (error) {
    ElMessage.error('清空失败: ' + (error.response?.data?.error || error.message))
  } finally {
    aiSaving.value = false
  }
}

const testAiConnectivity = async () => {
  aiTesting.value = true
  try {
    const payload = {
      enabled: !!settingsForm.value.aiEnabled,
      baseUrl: settingsForm.value.aiBaseUrl,
      model: settingsForm.value.aiModel,
      temperature: settingsForm.value.aiTemperature
    }
    if (settingsForm.value.aiApiKey !== '') {
      payload.apiKey = settingsForm.value.aiApiKey
    }
    const res = await request.post('/ai/test', payload)
    if (res?.ok) {
      const url = res?.endpoint || aiFinalUrl.value
      if (url) {
        ElMessage.success(`连接正常（${res.latencyMs}ms）：${url}`)
      } else {
        ElMessage.success(`连接正常（${res.latencyMs}ms）`)
      }
    } else {
      ElMessage.warning('连接测试未通过')
    }
  } catch (error) {
    ElMessage.error('连接失败: ' + (error.response?.data?.error || error.message))
  } finally {
    aiTesting.value = false
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
      appStoreServerUrl: settingsForm.value.appStoreServerUrl,
      advancedMode: !!settingsForm.value.advancedMode,
      allocPortStart: allocSettings.value.start,
      allocPortEnd: allocSettings.value.end,
      allowAutoAllocPort: !!allocSettings.value.allowAutoAllocPort,
      imageUpdateIntervalMinutes: settingsForm.value.imageUpdateIntervalMinutes
    })
    syncAdvancedModeLocal()
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
  height: 100%;
  width: 100%;
  padding: 12px 16px;
  overflow-y: auto;
  box-sizing: border-box;
  background: var(--clay-bg);
  scrollbar-gutter: stable;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.header-title {
  font-size: 18px;
  font-weight: 800;
  color: var(--clay-ink);
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.settings-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.switch-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.settings-card {
  margin-bottom: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.settings-form {
  max-width: 100%;
}

.password-group-vertical {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.password-input {
  width: 100%;
}

.w-full {
  width: 100%;
}

.port-input {
  flex: 1;
}

.switch-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.help-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.settings-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.settings-form :deep(.el-divider--horizontal) {
  margin: 12px 0;
}
.port-settings { padding: 0; }
.range-row { display: flex; align-items: center; gap: 8px; margin-bottom: 0; }
.alloc-result { margin-top: 10px; }
.ai-actions { display: flex; gap: 8px; flex-wrap: wrap; }

@media (max-width: 1200px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
