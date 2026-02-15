<template>
  <div class="detail-view">
    <div class="filter-bar clay-surface">
      <div class="filter-left">
        <el-button link @click="goBack" class="back-btn">
          <IconEpBack />
        </el-button>
        <div class="title-block">
          <div class="title">{{ projectName }}</div>
          <div class="path">{{ displayProjectPath }}</div>
        </div>
        <el-tag v-if="isSelfProject" size="small" type="warning" effect="plain" class="ml-2">自身</el-tag>
      </div>
      <div class="filter-right">
        <el-button-group class="main-actions">
          <el-button @click="handleRefresh" plain size="medium">
            <template #icon><IconEpRefresh /></template>
            刷新
          </el-button>
          <el-button type="primary" :loading="isBuilding" :disabled="isSelfProject" @click="handleBuild" size="medium">
            <template #icon><IconEpRefreshRight /></template>
            重新构建
          </el-button>
          <el-button type="success" :loading="isStarting" :disabled="isRunning || isSelfProject" @click="handleStart" size="medium">
            <template #icon><IconEpVideoPlay /></template>
            启动
          </el-button>
          <el-button type="danger" :loading="isStopping" :disabled="!isRunning || isSelfProject" @click="handleStop" size="medium">
            <template #icon><IconEpVideoPause /></template>
            停止
          </el-button>
          <el-button type="warning" :loading="isRestarting" :disabled="isSelfProject" @click="handleRestart" size="medium">
            <template #icon><IconEpRefresh /></template>
            重启
          </el-button>
        </el-button-group>
      </div>
    </div>

    <el-alert
      v-if="isSelfProject"
      type="info"
      effect="light"
      title="只读模式"
      description="容器化部署模式下，自身项目/容器不支持操作"
      :closable="false"
      class="self-resource-alert"
    />

    <!-- 构建日志弹窗 -->
    <el-dialog
      v-model="buildDialogVisible"
      title="项目构建"
      width="800px"
      :close-on-click-modal="false"
      :before-close="handleCloseBuildDialog"
    >
      <div class="build-options">
        <el-checkbox v-model="pullLatest" :disabled="isBuildingLogs">构建前重新拉取最新镜像</el-checkbox>
        <el-checkbox v-model="startAfterBuild" :disabled="isBuildingLogs" style="margin-left: 20px">构建后立即启动</el-checkbox>
        <el-button type="primary" :loading="isBuildingLogs" @click="startBuild" style="margin-left: 20px">
          {{ isBuildingLogs ? '构建中...' : '开始构建' }}
        </el-button>
      </div>
      <div class="build-logs" ref="buildLogsRef">
        <pre v-for="(log, index) in buildLogs" :key="index">{{ log }}</pre>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-if="isBuildingLogs" type="warning" @click="backgroundBuildDialog">后台运行</el-button>
          <el-button @click="closeBuildDialog">关闭</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 操作日志弹窗 -->
    <el-dialog
      v-model="actionDialogVisible"
      :title="actionDialogTitle"
      width="800px"
      :close-on-click-modal="false"
      :before-close="handleCloseActionDialog"
    >
      <div class="build-logs" ref="actionLogsRef">
        <pre v-for="(log, index) in actionLogs" :key="index">{{ log }}</pre>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-if="isActionRunning" type="warning" @click="backgroundActionDialog">后台运行</el-button>
          <el-button @click="closeActionDialog">关闭</el-button>
        </span>
      </template>
    </el-dialog>

    <div class="content-wrapper clay-surface">
      <div class="scroll-container">
        <el-tabs v-model="activeTab" class="detail-tabs" v-loading="isLoading">
          <el-tab-pane label="YAML配置" name="yaml">
            <div class="yaml-editor">
              <div class="editor-header">
                <span>Compose 配置（自动识别 *.yml / *.yaml）</span>
                <div class="editor-actions">
                  <el-button type="primary" size="small" :loading="isSaving" @click="handleSaveYaml">
                    保存
                  </el-button>
                </div>
              </div>
              <el-input
                v-model="yamlContent"
                type="textarea"
                :rows="20"
                class="yaml-textarea"
                :spellcheck="false"
              />
            </div>
          </el-tab-pane>

          <el-tab-pane label=".env" name="env">
            <div class="yaml-editor">
              <div class="editor-header">
                <span>.env（仅查看）</span>
              </div>
              <el-input
                v-model="envContent"
                type="textarea"
                :rows="20"
                class="yaml-textarea"
                :spellcheck="false"
                readonly
              />
            </div>
          </el-tab-pane>

          <el-tab-pane label="容器" name="containers">
            <el-table :data="containerList" style="width: 100%" class="custom-table">
              <el-table-column type="index" label="序号" width="80" header-align="center" />
              <el-table-column prop="name" label="名称" width="150" header-align="center" />
              <el-table-column prop="image" label="镜像" width="150" header-align="center" />
              <el-table-column prop="status" label="状态" width="100" header-align="center">
                <template #default="scope">
                  <el-tag :type="scope.row.status === 'running' ? 'success' : 'info'">
                    {{ scope.row.status }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="cpu" label="CPU" min-width="100" header-align="center" />
              <el-table-column prop="memory" label="内存" min-width="100" header-align="center" />
              <el-table-column prop="network" label="网络" min-width="120" header-align="center" />
              <el-table-column label="操作" width="150" fixed="right" header-align="center">
                <template #default="scope">
                  <el-button-group>
                    <el-button
                      size="small"
                      type="primary"
                      @click="handleContainerRestart(scope.row)"
                      :disabled="isSelfProject"
                    >
                      重启
                    </el-button>
                    <el-button
                      size="small"
                      type="danger"
                      @click="handleContainerStop(scope.row)"
                      :disabled="isSelfProject"
                    >
                      停止
                    </el-button>
                  </el-button-group>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="日志" name="logs">
            <div class="logs-container">
              <div class="logs-header">
                <div class="logs-options">
                  <el-switch v-model="autoScroll" active-text="自动滚动" />
                  <el-input v-model="logFilter" placeholder="检索日志" style="width: 220px" size="small" />
                </div>
                <el-button @click="handleClearLogs" size="small">清空日志</el-button>
              </div>
              <div class="logs-content" ref="logsRef">
                <pre v-for="(log, index) in filteredLogs" :key="index" :class="log.level"
                  ><template v-if="log.service"
                    ><span class="service" :style="{ color: log.serviceColor }">{{ log.service }}</span
                    ><span class="pipe"> | </span><span class="msg">{{ log.message }}</span></template
                  ><template v-else>{{ log.content }}</template></pre
                >
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { useSseLogStream } from '../utils/sseLogStream'

const route = useRoute()
const router = useRouter()
const projectName = ref(route.params.name || '')
const projectRoot = ref('')
const displayProjectPath = computed(() => {
  const root = String(projectRoot.value || '').replace(/\/$/, '')
  const name = String(projectName.value || '')
  if (!name) return root || ''
  if (root) return `${root}/${name}`
  return `project/${name}`
})
const activeTab = ref('yaml')
const isRunning = ref(true)
const isLoading = ref(true)
const isBuilding = ref(false)
const isStarting = ref(false)
const isStopping = ref(false)
const isRestarting = ref(false)
const isSaving = ref(false)
const autoScroll = ref(true)
const logsRef = ref(null)
const containerList = ref([])  // 修改为空数组，等待从后端获取数据
const yamlContent = ref('')
const envContent = ref('')
const {
  logs,
  logFilter,
  filteredLogs,
  start: startLogStream,
  stop: stopLogStream,
  clear: clearLogs,
  pushLine: pushLogLine
} = useSseLogStream({
  autoScroll,
  scrollElRef: logsRef,
  makeEntry: (line) => {
    const content = String(line || '')
    const level = inferLogLevel(content)
    const { service, message } = parseComposeLogLine(content)
    return {
      level,
      content,
      service,
      message: service ? message : '',
      serviceColor: service ? serviceColorFor(service) : ''
    }
  },
  getSearchText: (l) => `${String(l?.service || '')} ${String(l?.message || '')} ${String(l?.content || '')}`
})

// 构建弹窗相关状态
const buildDialogVisible = ref(false)
const pullLatest = ref(false)
const startAfterBuild = ref(false)
const buildAutoScroll = ref(true)
const isBuildingLogs = ref(false)
const buildLogsRef = ref(null)
const {
  logs: buildLogs,
  start: startBuildStream,
  stop: stopBuildStream,
  clear: clearBuildLogs,
  pushLine: pushBuildLine
} = useSseLogStream({
  autoScroll: buildAutoScroll,
  scrollElRef: buildLogsRef,
  eventNames: ['log'],
  onOpenLine: '已连接到构建服务...',
  onErrorLine: '',
  makeEntry: (payload) => String(payload || ''),
  onMessage: (event, { payload, pushLine, stop }) => {
    const line = String(payload || '')
    if (!line) return
    pushLine(line)
    if (line.includes('success: 构建完成')) {
      isBuildingLogs.value = false
      ElMessage.success('构建完成')
      stop()
      if (startAfterBuild.value) {
        handleStart()
      }
      return
    }
    if (line.includes('error:')) {
      isBuildingLogs.value = false
      stop()
    }
  },
  onError: ({ pushLine, stop }) => {
    if (isBuildingLogs.value === false) {
      stop()
      return
    }
    pushLine('连接错误，正在尝试重连...')
  }
})

const actionDialogVisible = ref(false)
const actionDialogTitle = ref('')
const actionAutoScroll = ref(true)
const isActionRunning = ref(false)
const actionLogsRef = ref(null)
const {
  logs: actionLogs,
  start: startActionStream,
  stop: stopActionStream,
  clear: clearActionLogs
} = useSseLogStream({
  autoScroll: actionAutoScroll,
  scrollElRef: actionLogsRef,
  eventNames: ['log'],
  onOpenLine: '',
  onErrorLine: '',
  makeEntry: (payload) => String(payload || ''),
  onMessage: (event, { payload, pushLine, stop }) => {
    const line = String(payload || '')
    if (!line) return
    pushLine(line)
    if (line.includes('success:') || line.includes('error:')) {
      stop()
      isActionRunning.value = false
      if (line.includes('success:')) {
        setTimeout(fetchContainers, 500)
      }
    }
  },
  onError: ({ pushLine, stop }) => {
    pushLine('error: 连接错误')
    stop()
    isActionRunning.value = false
  }
})
const isSelfProject = ref(false)

const notifyHeader = async (type, message) => {
  const msg = String(message || '').trim()
  if (!msg) return
  try {
    const saved = await api.system.addNotification({ type, message: msg })
    window.dispatchEvent(new CustomEvent('dockpier-notification', { detail: { type, message: msg, dbId: saved?.id, createdAt: saved?.created_at, read: saved?.read } }))
  } catch (e) {
    window.dispatchEvent(new CustomEvent('dockpier-notification', { detail: { type, message: msg } }))
  }
}

// 返回按钮：根据管理模式跳转到对应页面（distributed: /projects，centralized: /compose）
const goBack = () => {
  const mode = ((window.__ENV__ && window.__ENV__.MANAGEMENT_MODE) || import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toLowerCase()
  const isCS = mode === 'centralized' || mode === 'cs'
  router.push(isCS ? '/compose' : '/projects')
}

const handleRefresh = async () => {
  isLoading.value = true
  try {
    await loadProjectRoot()
    await fetchContainers()
    await fetchYamlContent()
    await fetchEnvContent()
    ElMessage.success('刷新成功')
  } catch (error) {
    ElMessage.error('刷新失败')
  } finally {
    isLoading.value = false
  }
}

const loadProjectRoot = async () => {
  try {
    const res = await api.system.info()
    const data = res?.data || res
    if (data && typeof data.ProjectRoot === 'string') {
      projectRoot.value = data.ProjectRoot
    }
  } catch (e) {}
}

// 添加获取容器列表的方法
const fetchContainers = async () => {
  try {
    const response = await api.compose.getStatus(projectName.value)
    if (response && typeof response.isSelf !== 'undefined') {
      isSelfProject.value = !!response.isSelf
    }
    if (response && Array.isArray(response.containers)) {
      containerList.value = response.containers.map(container => ({
        name: container.name,
        image: container.image,
        status: container.state || container.status,
        cpu: container.cpu || '0%',
        memory: container.memory || '0 MB',
        network: `${container.networkRx || '0 B'} / ${container.networkTx || '0 B'}`
      }))
      // 更新项目运行状态
      isRunning.value = containerList.value.some(c => c.status === 'running')
    } else {
      containerList.value = []
      console.warn('返回的容器数据格式不正确:', response)
    }
  } catch (error) {
    console.error('获取容器列表失败:', error.response?.data || error.message)
    // ElMessage.error(`获取容器列表失败: ${error.response?.data?.error || '服务器错误'}`) // 降低打扰，定时刷新时出错不弹窗
    containerList.value = []
  }
}

// 修改容器操作方法
const handleContainerRestart = async (container) => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  try {
    // 暂时没有单独重启容器的 API，先调用项目重启，或者需要后端增加单独重启容器接口
    // 这里保持原有逻辑，但提示可能需要优化
    await api.compose.restart(projectName.value)
    ElMessage.success(`重启容器 ${container.name} 成功`)
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error(`重启容器 ${container.name} 失败`)
  }
}

const handleContainerStop = async (container) => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  try {
    // 暂时没有单独停止容器的 API
    await api.compose.stop(projectName.value)
    ElMessage.success(`停止容器 ${container.name} 成功`)
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error(`停止容器 ${container.name} 失败`)
  }
}

// 修改项目操作方法
const handleBuild = () => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  buildDialogVisible.value = true
  clearBuildLogs()
  // 不重置 pullLatest，保留用户上次选择
}

const startBuild = () => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  if (isBuildingLogs.value) return
  
  isBuildingLogs.value = true
  clearBuildLogs()
  pushBuildLine('开始构建请求...')
  
  const token = localStorage.getItem('token')
  const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
  const url = `/api/compose/${projectName.value}/build/events?pull=${pullLatest.value}${tokenParam}`
  
  startBuildStream(url, { reset: false })
}

const backgroundBuildDialog = () => {
  isBuildingLogs.value = false
  buildDialogVisible.value = false
  notifyHeader('info', projectName.value ? `构建任务已后台运行：${projectName.value}` : '构建任务已后台运行')
}

const closeBuildDialog = () => {
  buildDialogVisible.value = false
}

const handleCloseBuildDialog = (done) => {
  if (isBuildingLogs.value) {
    ElMessageBox.confirm('构建正在进行中，关闭窗口不会停止后台构建，确定关闭吗？')
      .then(() => {
        stopBuildStream()
        isBuildingLogs.value = false
        done()
      })
      .catch(() => {})
  } else {
    stopBuildStream()
    done()
  }
}

const handleStart = async () => {
  await runProjectAction('start')
}

const handleStop = async () => {
  await runProjectAction('stop')
}

const handleRestart = async () => {
  await runProjectAction('restart')
}

const closeActionEventSource = () => {
  stopActionStream()
  isActionRunning.value = false
}

const handleCloseActionDialog = (done) => {
  if (isActionRunning.value) {
    ElMessageBox.confirm('操作正在进行中，关闭窗口不会停止后台执行，确定关闭吗？')
      .then(() => {
        closeActionEventSource()
        done()
      })
      .catch(() => {})
  } else {
    closeActionEventSource()
    done()
  }
}

const inferLogLevel = (line) => {
  const raw = String(line || '')
  const lower = raw.toLowerCase()
  if (lower.startsWith('error:')) return 'error'
  if (lower.startsWith('warning:')) return 'warning'
  if (lower.startsWith('success:')) return 'success'
  if (lower.startsWith('info:')) return 'info'
  if (lower.includes('error') || lower.includes('err')) return 'error'
  if (lower.includes('warn')) return 'warning'
  return 'info'
}

const hashString = (s) => {
  let h = 0
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h) + s.charCodeAt(i)
    h |= 0
  }
  return h
}

const serviceColorFor = (service) => {
  const palette = [
    '--el-color-primary',
    '--el-color-success',
    '--el-color-warning',
    '--el-color-danger',
    '--el-color-info'
  ]
  const idx = Math.abs(hashString(service || '')) % palette.length
  return `var(${palette[idx]})`
}

const parseComposeLogLine = (line) => {
  const raw = String(line || '')
  const split = raw.split(' | ')
  if (split.length >= 2) {
    const service = split[0].trim()
    const message = split.slice(1).join(' | ')
    if (service) {
      return { service, message }
    }
  }
  return { service: '', message: '' }
}

const runProjectAction = async (action) => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  const titleMap = { start: '项目启动', stop: '项目停止', restart: '项目重启' }
  actionDialogTitle.value = titleMap[action] || '项目操作'
  actionDialogVisible.value = true
  clearActionLogs()
  isActionRunning.value = true

  if (action === 'start') isStarting.value = true
  if (action === 'stop') isStopping.value = true
  if (action === 'restart') isRestarting.value = true

  const token = localStorage.getItem('token') || ''
  const url = `/api/compose/${projectName.value}/${action}/events?token=${encodeURIComponent(token)}`
  startActionStream(url, { reset: false })

  const finalize = () => {
    if (action === 'start') isStarting.value = false
    if (action === 'stop') isStopping.value = false
    if (action === 'restart') isRestarting.value = false
  }

  const stopWatcher = watch(isActionRunning, (running) => {
    if (!running) {
      finalize()
      stopWatcher()
    }
  }, { immediate: true })
}

const backgroundActionDialog = () => {
  isActionRunning.value = false
  actionDialogVisible.value = false
  const act = String(actionDialogTitle.value || '').trim()
  const name = String(projectName.value || '').trim()
  notifyHeader('info', act && name ? `${act}已后台运行：${name}` : '操作已后台运行')
}

const closeActionDialog = () => {
  actionDialogVisible.value = false
}

// 修改获取 YAML 配置的方法
const fetchYamlContent = async () => {
  try {
    if (isSelfProject.value) {
      yamlContent.value = '容器化部署模式下，不支持查看自身项目配置'
      return
    }
    console.log('api.compose:', api.compose); // 添加调试日志
    const response = await api.compose.getYaml(projectName.value)
    console.log('YAML Response:', response); // 添加调试日志
    
    if (response && response.content) {
      yamlContent.value = response.content
    } else {
      console.warn('YAML内容为空')
      yamlContent.value = ''
    }
  } catch (error) {
    console.error('获取YAML配置失败:', error)
    if (error.response && error.response.status === 400 && error.response.data && error.response.data.error) {
      ElMessage.warning(error.response.data.error)
    } else {
      ElMessage.error('获取YAML配置失败')
    }
  }
}

const fetchEnvContent = async () => {
  try {
    if (isSelfProject.value) {
      envContent.value = '容器化部署模式下，不支持查看自身项目配置'
      return
    }
    const response = await api.compose.getEnv(projectName.value)
    if (response && typeof response.content === 'string') {
      envContent.value = response.content
    } else {
      envContent.value = ''
    }
  } catch (error) {
    console.error('获取 .env 失败:', error)
    envContent.value = ''
  }
}

// 修改保存 YAML 的方法
const handleSaveYaml = async () => {
  if (isSelfProject.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  isSaving.value = true
  ElMessage.info('正在保存配置...')
  try {
    await api.compose.saveYaml(projectName.value, yamlContent.value)
    ElMessage.success('保存成功')
  } catch (error) {
    console.error('保存YAML失败:', error)
    ElMessage.error('保存失败')
  } finally {
    isSaving.value = false
  }
}

onMounted(async () => {
  isLoading.value = true
  try {
    // 并行获取项目信息和YAML配置
    await loadProjectRoot()
    await fetchContainers()
    await fetchYamlContent()
    await fetchEnvContent()

    // 设置定时刷新
    refreshTimer = setInterval(fetchContainers, 5000)
  } catch (error) {
    console.error('初始化失败:', error)
  } finally {
    isLoading.value = false
  }
})

// 添加定时刷新
let refreshTimer = null

const stopLogsStream = () => {
  stopLogStream()
}

const startLogsStream = () => {
  const token = localStorage.getItem('token')
  const tokenParam = token ? `?token=${encodeURIComponent(token)}` : ''
  startLogStream(`/api/compose/${projectName.value}/logs${tokenParam}`, { reset: true })
}

watch(activeTab, (tab) => {
  if (tab === 'logs') {
    startLogsStream()
  } else if (tab === 'env') {
    fetchEnvContent()
  } else {
    stopLogsStream()
  }
})

// 添加清理日志的方法
const handleClearLogs = () => {
  clearLogs()
}

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  stopLogsStream()
  stopBuildStream()
  closeActionEventSource()
  clearLogs()
  containerList.value = []
  yamlContent.value = ''
})
</script>

<style scoped>
.detail-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 16px;
  background-color: var(--clay-bg);
  gap: 12px;
}

.self-resource-alert {
  margin: 0 0 12px;
  border-radius: 12px;
}

.scroll-container {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px;
}

.title-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.title {
  font-size: 18px;
  font-weight: 900;
  color: var(--clay-ink);
}

.path {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.2;
}

/* 覆盖 el-tabs 样式 */
:deep(.el-tabs__header) {
  margin-bottom: 20px;
}

:deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background-color: var(--el-border-color-lighter);
}

:deep(.el-tabs__item) {
  font-size: 15px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}

:deep(.el-tabs__item.is-active) {
  color: var(--el-color-primary);
  font-weight: 600;
}

:deep(.el-tabs__active-bar) {
  background-color: var(--el-color-primary);
  height: 2px;
}

.yaml-editor {
  border: 1px solid var(--clay-border);
  border-radius: 18px;
  overflow: hidden;
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-inner);
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background-color: var(--port-header-bg);
  border-bottom: 1px solid var(--clay-border);
  font-weight: 500;
  color: var(--el-text-color-secondary);
}

.yaml-textarea {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

:deep(.el-textarea__inner) {
  border: none;
  border-radius: 0;
  padding: 16px;
  background-color: transparent;
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-primary);
}

.logs-container {
  height: 600px;
  display: flex;
  flex-direction: column;
  border: var(--log-border);
  border-radius: 18px;
  overflow: hidden;
  background: var(--log-bg);
  box-shadow: var(--shadow-clay-inner);
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--port-header-bg);
  border-bottom: var(--log-border);
}

.logs-options {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logs-content {
  flex: 1;
  overflow-y: auto;
  background: var(--log-bg);
  color: var(--log-text);
  padding: 16px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.5;
}

.logs-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.logs-content .service {
  font-weight: 600;
}

.logs-content .pipe {
  color: var(--log-text);
  opacity: 0.6;
}

.logs-content .error {
  color: var(--log-error);
}

.logs-content .success {
  color: var(--log-info);
}

.logs-content .warning {
  color: var(--log-warning);
}

.logs-content .info {
  color: var(--log-text);
  opacity: 0.8;
}

.build-options {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
}

.build-logs {
  height: 400px;
  overflow-y: auto;
  background: var(--log-bg);
  color: var(--log-text);
  padding: 16px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  border: var(--log-border);
  border-radius: 18px;
  font-size: 13px;
  line-height: 1.5;
  box-shadow: var(--shadow-clay-inner);
}

.build-logs pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

</style>
