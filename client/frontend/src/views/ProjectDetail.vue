<template>
  <div class="detail-view">
    <div class="header-bar">
      <div class="header-left">
        <el-button link @click="goBack">
          <el-icon><Back /></el-icon>
        </el-button>
        <div class="title">{{ projectName }}</div>
      </div>
      <div class="header-right">
        <el-button @click="handleRefresh" plain size="medium" class="square-btn">
          <template #icon><el-icon><Refresh /></el-icon></template>
        </el-button>
        <el-button-group>
          <el-button type="primary" :loading="isBuilding" @click="handleBuild" size="medium">
            重新构建
          </el-button>
          <el-button type="success" :loading="isStarting" :disabled="isRunning" @click="handleStart" size="medium">
            启动
          </el-button>
          <el-button type="danger" :loading="isStopping" :disabled="!isRunning" @click="handleStop" size="medium">
            停止
          </el-button>
          <el-button type="warning" :loading="isRestarting" @click="handleRestart" size="medium">
            重启
          </el-button>
        </el-button-group>
      </div>
    </div>

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
    </el-dialog>

    <div class="content-wrapper">
      <div class="scroll-content">
        <div class="content-inner">
          <el-tabs v-model="activeTab" class="detail-tabs" v-loading="isLoading">
            <el-tab-pane label="YAML配置" name="yaml">
              <div class="yaml-editor">
                <div class="editor-header">
                  <span>docker-compose.yml</span>
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
                      >
                        重启
                      </el-button>
                      <el-button 
                        size="small" 
                        type="danger"
                        @click="handleContainerStop(scope.row)"
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
                  <el-switch
                    v-model="autoScroll"
                    active-text="自动滚动"
                  />
                  <el-button @click="handleClearLogs" size="small">
                    清空日志
                  </el-button>
                </div>
                <div class="logs-content" ref="logsRef">
                  <pre v-for="(log, index) in logs" :key="index" :class="log.type">{{ log.content }}</pre>
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Back, CircleClose, Refresh } from '@element-plus/icons-vue'
import api from '../api'

const route = useRoute()
const router = useRouter()
const projectName = ref(route.params.name || '')
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
const logs = ref([])  // 添加日志数组
const logWebSocket = ref(null)  // 移动到这里统一声明
const yamlContent = ref('')

// 构建弹窗相关状态
const buildDialogVisible = ref(false)
const pullLatest = ref(false)
const startAfterBuild = ref(false)
const buildLogs = ref([])
const isBuildingLogs = ref(false)
const buildEventSource = ref(null)
const buildLogsRef = ref(null)

// 返回按钮：根据管理模式跳转到对应页面（distributed: /projects，centralized: /compose）
const goBack = () => {
  const mode = ((window.__ENV__ && window.__ENV__.MANAGEMENT_MODE) || import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toLowerCase()
  const isCS = mode === 'centralized' || mode === 'cs'
  router.push(isCS ? '/compose' : '/projects')
}

const handleRefresh = async () => {
  isLoading.value = true
  try {
    await Promise.all([
      fetchContainers(),
      fetchYamlContent()
    ])
    ElMessage.success('刷新成功')
  } catch (error) {
    ElMessage.error('刷新失败')
  } finally {
    isLoading.value = false
  }
}

// 添加获取容器列表的方法
const fetchContainers = async () => {
  try {
    const response = await api.compose.getStatus(projectName.value)
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
  buildDialogVisible.value = true
  buildLogs.value = []
  // 不重置 pullLatest，保留用户上次选择
}

const startBuild = () => {
  if (isBuildingLogs.value) return
  
  isBuildingLogs.value = true
  buildLogs.value = []
  buildLogs.value.push("开始构建请求...")
  
  const token = localStorage.getItem('token')
  const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
  const url = `/api/compose/${projectName.value}/build/events?pull=${pullLatest.value}${tokenParam}`
  
  if (buildEventSource.value) {
    buildEventSource.value.close()
  }

  const es = new EventSource(url)
  buildEventSource.value = es
  
  es.onopen = () => {
    buildLogs.value.push("已连接到构建服务...")
  }

  es.addEventListener('log', (event) => {
    buildLogs.value.push(event.data)
    scrollToBuildBottom()
    
    if (event.data.includes('success: 构建完成')) {
      isBuildingLogs.value = false
      ElMessage.success('构建完成')
      es.close()
      
      if (startAfterBuild.value) {
        handleStart()
      }
    } else if (event.data.includes('error:')) {
      isBuildingLogs.value = false
      // ElMessage.error('构建出错') // 日志中已有错误信息，不再弹窗
      es.close()
    }
  })
  
  es.onerror = (e) => {
    console.error('SSE Build Error:', e)
    if (es.readyState === EventSource.CLOSED) {
        buildLogs.value.push("连接已关闭")
    } else {
        buildLogs.value.push("连接错误，正在尝试重连...")
    }
    // 通常 error 后需要手动关闭，防止无限重连，除非后端支持断线重连
    if (es.readyState === EventSource.CLOSED || isBuildingLogs.value === false) {
       es.close()
       isBuildingLogs.value = false
    }
  }
}

const handleCloseBuildDialog = (done) => {
  if (isBuildingLogs.value) {
    ElMessageBox.confirm('构建正在进行中，关闭窗口不会停止后台构建，确定关闭吗？')
      .then(() => {
        if (buildEventSource.value) {
          buildEventSource.value.close()
          buildEventSource.value = null
        }
        isBuildingLogs.value = false
        done()
      })
      .catch(() => {})
  } else {
    if (buildEventSource.value) {
      buildEventSource.value.close()
      buildEventSource.value = null
    }
    done()
  }
}

const scrollToBuildBottom = () => {
  if (buildLogsRef.value) {
    nextTick(() => {
      buildLogsRef.value.scrollTop = buildLogsRef.value.scrollHeight
    })
  }
}

const handleStart = async () => {
  isStarting.value = true
  ElMessage.info('正在发送启动指令...')
  try {
    await api.compose.start(projectName.value)
    ElMessage.success('启动指令已发送，正在后台处理')
    // 异步操作，无需立即设置为 true，等待 fetchContainers 更新
    setTimeout(fetchContainers, 1000)
    setTimeout(fetchContainers, 3000)
    setTimeout(fetchContainers, 5000)
  } catch (error) {
    ElMessage.error('启动请求失败: ' + (error.response?.data?.error || error.message))
  } finally {
    isStarting.value = false
  }
}

const handleStop = async () => {
  isStopping.value = true
  ElMessage.info('正在发送停止指令...')
  try {
    await api.compose.stop(projectName.value)
    ElMessage.success('停止指令已发送，正在后台处理')
    setTimeout(fetchContainers, 1000)
    setTimeout(fetchContainers, 3000)
    setTimeout(fetchContainers, 5000)
  } catch (error) {
    ElMessage.error('停止请求失败: ' + (error.response?.data?.error || error.message))
  } finally {
    isStopping.value = false
  }
}

const handleRestart = async () => {
  isRestarting.value = true
  ElMessage.info('正在发送重启指令...')
  try {
    await api.compose.restart(projectName.value)
    ElMessage.success('重启指令已发送，正在后台处理')
    setTimeout(fetchContainers, 1000)
    setTimeout(fetchContainers, 3000)
    setTimeout(fetchContainers, 5000)
  } catch (error) {
    ElMessage.error('重启请求失败: ' + (error.response?.data?.error || error.message))
  } finally {
    isRestarting.value = false
  }
}

// 修改获取 YAML 配置的方法
const fetchYamlContent = async () => {
  try {
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

// 修改保存 YAML 的方法
const handleSaveYaml = async () => {
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
    await Promise.all([
      fetchContainers(),
      fetchYamlContent()
    ])

    // 设置定时刷新
    refreshTimer = setInterval(fetchContainers, 5000)
    
    // 设置WebSocket连接
    setupWebSocket()
  } catch (error) {
    console.error('初始化失败:', error)
  } finally {
    isLoading.value = false
  }
})

// 添加定时刷新
let refreshTimer = null

onUnmounted(() => {
  // 清理定时器
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  
  // 清理 EventSource
  if (logWebSocket.value) {
    logWebSocket.value.close()
    logWebSocket.value = null
  }
  
  // 清理数据
  logs.value = []
  containerList.value = []
  yamlContent.value = ''
})

// 替换 setupWebSocket 函数
const setupWebSocket = () => {
  if (logWebSocket.value) {
    logWebSocket.value.close()
  }

  const token = localStorage.getItem('token')
  const tokenParam = token ? `?token=${encodeURIComponent(token)}` : ''
  const eventSource = new EventSource(`/api/compose/${projectName.value}/logs${tokenParam}`)
  
  eventSource.onopen = () => {
    console.log('SSE connection established')
    logs.value.push({
      type: 'info',
      content: '已连接到日志服务'
    })
  }
  
  eventSource.onmessage = (event) => {
    const data = event.data
    if (data.startsWith('error:')) {
      logs.value.push({
        type: 'error',
        content: data.substring(6)
      })
    } else {
      logs.value.push({
        type: 'info',
        content: data
      })
    }
    
    if (autoScroll.value) {
      scrollToBottom()
    }
  }
  
  eventSource.onerror = (error) => {
    console.error('SSE error:', error)
    logs.value.push({
      type: 'error',
      content: '日志连接错误'
    })
    eventSource.close()
  }

  // 保存 EventSource 实例以便后续清理
  logWebSocket.value = eventSource
}

// 修改 onUnmounted 钩子中的清理代码
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (logWebSocket.value) {
    logWebSocket.value.close()
  }
})

// 添加清理日志的方法
const handleClearLogs = () => {
  logs.value = []
}

// 添加自动滚动相关代码
const scrollToBottom = () => {
  if (logsRef.value) {
    nextTick(() => {
      logsRef.value.scrollTop = logsRef.value.scrollHeight
    })
  }
}

// 监听日志变化
watch(logs, () => {
  if (autoScroll.value) {
    scrollToBottom()
  }
})
</script>

<style scoped>
.detail-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 24px;
}

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  background: var(--el-bg-color);
  padding: 12px 20px;
  border-radius: 12px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.square-btn {
  width: 36px;
  height: 36px;
  padding: 0;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.content-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.scroll-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.content-inner {
  max-width: 1200px;
  margin: 0 auto;
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
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-lighter);
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
  background-color: var(--el-bg-color);
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-primary);
}

.logs-container {
  height: 600px;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.logs-content {
  flex: 1;
  overflow-y: auto;
  background: #1e1e1e;
  color: #e2e8f0;
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

.logs-content .error {
  color: #ef4444;
}

.logs-content .success {
  color: #22c55e;
}

.logs-content .info {
  color: #94a3b8;
}

.build-options {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
}

.build-logs {
  height: 400px;
  overflow-y: auto;
  background: #1e1e1e;
  color: #e2e8f0;
  padding: 16px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
}

.build-logs pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}
</style>
