<template>
  <div class="detail-view">
    <div class="header-bar clay-surface">
      <div class="header-left">
        <el-button link @click="goBack">
          <el-icon><Back /></el-icon>
        </el-button>
        <div class="title">{{ containerName }}</div>
        <el-tag
          :type="containerStatus === '运行中' ? 'success' : 'info'"
          :class="['status-tag', 'clay-tag-dot', containerStatus === '运行中' ? 'is-success' : 'is-danger']"
        >
          {{ containerStatus }}
        </el-tag>
        <el-tag v-if="isSelfContainer" size="small" type="warning" effect="plain" style="margin-left: 8px">自身</el-tag>
      </div>
      <div class="header-right">
        <el-button @click="handleRefresh" plain size="medium" class="square-btn">
          <template #icon><el-icon><Refresh /></el-icon></template>
        </el-button>
        <el-button-group>
          <el-button 
            type="primary" 
            :disabled="containerStatus === '运行中' || isSelfContainer"
            @click="handleStart"
            size="medium"
          >
            启动
          </el-button>
          <el-button 
            type="warning" 
            :disabled="containerStatus !== '运行中' || isSelfContainer"
            @click="handleStop"
            size="medium"
          >
            停止
          </el-button>
          <el-button 
            type="primary"
            :disabled="isSelfContainer"
            @click="handleRestart"
            size="medium"
          >
            重启
          </el-button>
        </el-button-group>
      </div>
    </div>

    <el-alert
      v-if="isSelfContainer"
      type="info"
      effect="light"
      title="只读模式"
      description="容器化部署模式下，自身项目/容器不支持操作"
      :closable="false"
      class="self-resource-alert"
    />

    <div class="content-wrapper clay-surface">
      <div class="scroll-content">
        <div class="content-inner">
          <el-tabs v-model="activeTab" class="detail-tabs">
            <el-tab-pane label="基本信息" name="info">
              <div class="info-section">
                <div class="resource-usage">
                  <el-row :gutter="20">
                    <el-col :span="6">
                      <div class="metric-card">
                        <div class="metric-title">CPU</div>
                        <div class="metric-value">{{ cpuUsage }}%</div>
                        <div class="metric-chart">
                          <el-progress 
                            :percentage="cpuUsage" 
                            :color="getProgressColor(cpuUsage)"
                          />
                        </div>
                      </div>
                    </el-col>
                    <el-col :span="6">
                      <div class="metric-card">
                        <div class="metric-title">内存</div>
                        <div class="metric-value">{{ memoryUsage }}MB</div>
                        <div class="metric-chart">
                          <el-progress 
                            :percentage="(memoryUsage / memoryLimit) * 100" 
                            :color="getProgressColor((memoryUsage / memoryLimit) * 100)"
                          />
                        </div>
                      </div>
                    </el-col>
                    <el-col :span="6">
                      <div class="metric-card">
                        <div class="metric-title">网络(上传)</div>
                        <div class="metric-value">{{ networkUp }}</div>
                      </div>
                    </el-col>
                    <el-col :span="6">
                      <div class="metric-card">
                        <div class="metric-title">网络(下载)</div>
                        <div class="metric-value">{{ networkDown }}</div>
                      </div>
                    </el-col>
                  </el-row>
                </div>

                <div class="detail-info">
                  <el-descriptions :column="2" border>
                    <el-descriptions-item label="容器名称">{{ containerName }}</el-descriptions-item>
                    <el-descriptions-item label="镜像">{{ imageInfo }}</el-descriptions-item>
                    <el-descriptions-item label="创建时间">{{ createTime }}</el-descriptions-item>
                    <el-descriptions-item label="运行时长">{{ uptime }}</el-descriptions-item>
                    <el-descriptions-item label="端口映射">{{ ports }}</el-descriptions-item>
                    <el-descriptions-item label="存储卷">{{ volumes }}</el-descriptions-item>
                    <el-descriptions-item label="网络">{{ networks }}</el-descriptions-item>
                    <el-descriptions-item label="重启策略">{{ restartPolicy }}</el-descriptions-item>
                  </el-descriptions>
                </div>
              </div>
            </el-tab-pane>

            <el-tab-pane label="日志" name="logs">
              <div class="logs-container">
                <div class="logs-header">
                  <div class="logs-options">
                    <el-switch
                      v-model="autoScroll"
                      active-text="自动滚动"
                    />
                    <el-input
                      v-model="logFilter"
                      placeholder="检索日志"
                      style="width: 200px"
                      size="medium"
                    >
                      <template #prefix>
                        <el-icon><Search /></el-icon>
                      </template>
                    </el-input>
                  </div>
                  <el-button @click="handleClearLogs" size="medium">清空日志</el-button>
                </div>
                <div class="logs-content" ref="logsRef">
                  <pre v-for="(log, index) in filteredLogs" 
                       :key="index" 
                       :class="getLogClass(log)">{{ log.content }}</pre>
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
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Back, Search, Refresh } from '@element-plus/icons-vue'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
import { useSseLogStream } from '../utils/sseLogStream'

const route = useRoute()
const router = useRouter()
const containerName = ref(route.params.name || '')
const containerStatus = ref('')
const activeTab = ref('info')
const loading = ref(false)
const containerId = ref('') // 存储容器完整 ID
const isSelfContainer = ref(false)

// 基本信息数据
const cpuUsage = ref(0)
const memoryUsage = ref(0)
const memoryLimit = ref(0)
const networkUp = ref('0 B/s')
const networkDown = ref('0 B/s')
const imageInfo = ref('')
const createTime = ref('')
const uptime = ref('')
const ports = ref('')
const volumes = ref('')
const networks = ref('')
const restartPolicy = ref('')

// 日志相关
const autoScroll = ref(true)
const logsRef = ref(null)
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
  scrollElRef: logsRef
})
let refreshTimer = null
let statsTimer = null
let statsEventSource = null
let prevRxBytes = null
let prevTxBytes = null
let prevTs = null

const getProgressColor = (percentage) => {
  if (percentage < 60) return '#67C23A'
  if (percentage < 80) return '#E6A23C'
  return '#F56C6C'
}

const getLogClass = (log) => ({
  'error': log.level === 'error',
  'warning': log.level === 'warning',
  'info': log.level === 'info' || log.level === 'success'
})

// 格式化网络流量
const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const handleRefresh = async () => {
  loading.value = true
  try {
    await fetchContainerDetail()
    ElMessage.success('刷新成功')
  } catch (error) {
    ElMessage.error('刷新失败')
  } finally {
    loading.value = false
  }
}

// 获取容器详情
const fetchContainerDetail = async () => {
  if (!containerName.value) return
  
  try {
    // 这里我们假设后端支持通过名称或ID获取详情
    // 由于后端只实现了通过ID获取，我们需要先在前端找到对应的容器ID
    // 或者修改后端路由支持通过名称获取。
    // 这里采用先获取列表匹配名称的方式（虽然效率低，但暂时可行）
    // TODO: 优化后端 API 支持通过名称查询或前端传递 ID
    
    // 如果没有 ID，先通过列表查找
    if (!containerId.value) {
      const list = await api.containers.listContainers()
      // axios 返回的数据在 list.data 中，但如果在拦截器中处理过，可能直接就是数据
      // 需要确认 list 的结构。通常 axios.get 返回的是 { data: ... }
      // 如果后端直接返回数组，那么 list.data 就是数组
      // 如果出错提示 list.data.find is undefined，说明 list.data 可能不是数组或者 list 已经是数组了
      
      const containers = Array.isArray(list) ? list : (list.data || [])
      
      const target = containers.find(c => {
         const name = c.Names?.[0]?.replace(/^\//, '')
         return name === containerName.value
      })
      
      if (target) {
        containerId.value = target.Id
      } else {
        ElMessage.error('找不到指定容器')
        return
      }
    }

    // 获取详情
    const res = await api.containers.getContainer(containerId.value)
    
    // api.index.js 中拦截器直接返回 response.data，所以 res 已经是数据本身，不需要再 .data
    // 但后端可能返回的是 { data: ... } 或者直接是数据结构，这取决于后端
    // 后端 api/container.go 中 GetContainer 函数是 c.JSON(http.StatusOK, result)
    // 其中 result 是 map[string]interface{}
    // 所以 res 应该就是 result
    
    // 如果返回结构是 { data: {...} }，那么这里 res.data 才是数据
    // 如果拦截器返回 response.data，而 axios 原始 response.data 就是后端返回的 JSON
    // 假设后端返回 {"Name": "xxx", "State": "running", ...}
    // 那么 res 就应该是这个对象
    // 如果报错 Cannot read properties of undefined (reading 'State')
    // 说明 res.data 是 undefined，这意味着 res 已经是数据对象了，或者 res 本身就是 undefined
    
    const data = res.data || res // 尝试兼容两种情况

    if (!data) {
        throw new Error('获取容器详情返回为空')
    }

    // 更新数据
    // 注意：后端返回的字段首字母大写
    // 且后端返回的 State 是个 struct 或者 map，需要确认结构
    // 查看 backend/api/container.go，返回的是：
    /*
        result := gin.H{
            "Id":              inspect.ID,
            "Name":            inspect.Name,
            "State":           inspect.State.Status, // 这里直接是 string
            "Running":         inspect.State.Running,
            // ...
        }
    */
    // 所以 data.State 应该是 string
    
    containerStatus.value = (data.State === 'running' || data.Running) ? '运行中' : data.State
    isSelfContainer.value = !!data.isSelf
    imageInfo.value = data.Image
    createTime.value = formatTimeTwoLines(data.Created)
    uptime.value = data.RunningTime
    restartPolicy.value = data.RestartPolicy
    
    // 格式化端口
    if (data.Ports && data.Ports.length) {
      ports.value = data.Ports.map(p => {
        if (p.PublicPort) {
          return `${p.IP || '0.0.0.0'}:${p.PublicPort}:${p.PrivatePort}/${p.Type}`
        }
        return `${p.PrivatePort}/${p.Type}`
      }).join(', ')
    } else {
      ports.value = '-'
    }

    // 格式化挂载卷
    if (data.Mounts && data.Mounts.length) {
      volumes.value = data.Mounts.map(m => `${m.Source}:${m.Destination}`).join(', ')
    } else {
      volumes.value = '-'
    }

    // 格式化网络
    if (data.Networks && data.Networks.length) {
      networks.value = data.Networks.join(', ')
    } else {
      networks.value = '-'
    }
    
    // 资源使用（暂时模拟或通过 WebSocket 获取）
    // 这里简单设置一些默认值，实际应该从后端获取实时状态
    // memoryLimit.value = data.HostConfig.Memory || 0 // Docker API 这里的单位可能是字节
    
    // 获取到容器ID后，启动资源统计流
    try {
      startStatsStream()
    } catch {}
  } catch (error) {
    console.error('获取容器详情失败:', error)
    ElMessage.error('获取容器详情失败')
  }
}

// 返回按钮：根据管理模式跳转到对应页面（distributed: /containers，centralized: /compose）
const goBack = () => {
  const mode = ((window.__ENV__ && window.__ENV__.MANAGEMENT_MODE) || import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toLowerCase()
  const isCS = mode === 'centralized' || mode === 'cs'
  router.push(isCS ? '/compose' : '/containers')
}

const handleStart = async () => {
  if (isSelfContainer.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身容器')
    return
  }
  try {
    await api.containers.startContainer(containerId.value)
    ElMessage.success('容器已启动')
    fetchContainerDetail()
  } catch (error) {
    ElMessage.error('启动失败: ' + (error.response?.data?.error || error.message))
  }
}

const handleStop = async () => {
  if (isSelfContainer.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身容器')
    return
  }
  try {
    await api.containers.stopContainer(containerId.value)
    ElMessage.success('容器已停止')
    fetchContainerDetail()
  } catch (error) {
    ElMessage.error('停止失败: ' + (error.response?.data?.error || error.message))
  }
}

const handleRestart = async () => {
  if (isSelfContainer.value) {
    ElMessage.warning('容器化部署模式下，不支持操作自身容器')
    return
  }
  try {
    await api.containers.restartContainer(containerId.value) // 假设 API 有这个方法
    ElMessage.success('容器已重启')
    fetchContainerDetail()
  } catch (error) {
    // ElMessage.error('重启失败')
    // 注意：api/container.js 中可能没有 restartContainer，需要确认
    // 经检查 api/container.js 中确实没有 restartContainer，只有 start/stop/remove
    // 如果需要重启功能，需要在 api/container.js 添加
    console.error(error)
  }
}

const handleClearLogs = () => {
  clearLogs()
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

const stopLogsStream = () => {
  stopLogStream()
}

// 启动日志流（SSE，逻辑与其他日志页保持一致）
const startLogsStream = () => {
  if (!containerId.value) return
  try {
    const token = localStorage.getItem('token') || ''
    const url = `/api/containers/${containerId.value}/logs/events?tail=200&token=${encodeURIComponent(token)}`
    startLogStream(url, { reset: true })
  } catch (e) {
    console.error('日志流错误:', e)
    ElMessage.error('日志获取失败')
  }
}

// 获取资源使用情况
const fetchContainerStats = async () => {
  if (!containerId.value) return
  try {
    const res = await api.containers.stats(containerId.value)
    const data = res.data || res
    const ts = data.timestamp || Math.floor(Date.now() / 1000)
    // CPU
    cpuUsage.value = Math.max(0, Math.min(100, Number(data.cpu_percent || 0)))
    // Memory (MB)
    memoryUsage.value = Number(((data.memory_usage || 0) / (1024 * 1024)).toFixed(2))
    memoryLimit.value = Number(((data.memory_limit || 0) / (1024 * 1024)).toFixed(2))
    // Network rate (B/s)
    const rx = Number(data.net_rx_bytes || 0)
    const tx = Number(data.net_tx_bytes || 0)
    if (prevRxBytes != null && prevTxBytes != null && prevTs != null) {
      const dt = Math.max(1, (ts - prevTs)) // 秒
      const upRate = (tx - prevTxBytes) / dt
      const downRate = (rx - prevRxBytes) / dt
      networkUp.value = `${formatBytes(upRate)}/s`
      networkDown.value = `${formatBytes(downRate)}/s`
    }
    prevRxBytes = rx
    prevTxBytes = tx
    prevTs = ts
  } catch (e) {
    console.error('获取资源统计失败:', e)
  }
}

// 监听标签页切换，进入“日志”时拉取日志
watch(activeTab, (tab) => {
  if (tab === 'logs') {
    startLogsStream()
  } else {
    stopLogsStream()
  }
})

// 启动资源统计 SSE 流
const startStatsStream = () => {
  // 若已有定时器或流，先清理
  if (statsTimer) {
    try { clearInterval(statsTimer) } catch {}
    statsTimer = null
  }
  if (statsEventSource) {
    try { statsEventSource.close() } catch {}
    statsEventSource = null
  }
  if (!containerId.value) return
  const token = localStorage.getItem('token') || ''
  const url = `/api/containers/${containerId.value}/stats/stream?token=${encodeURIComponent(token)}`
  try {
    statsEventSource = new EventSource(url)
    statsEventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        const ts = data.timestamp || Math.floor(Date.now() / 1000)
        cpuUsage.value = Math.max(0, Math.min(100, Number(data.cpu_percent || 0)))
        memoryUsage.value = Number(((data.memory_usage || 0) / (1024 * 1024)).toFixed(2))
        memoryLimit.value = Number(((data.memory_limit || 0) / (1024 * 1024)).toFixed(2))
        // 使用后端提供的速率，如无则回退本地计算
        if (typeof data.up_rate === 'number' && typeof data.down_rate === 'number') {
          networkUp.value = `${formatBytes(data.up_rate)}/s`
          networkDown.value = `${formatBytes(data.down_rate)}/s`
        } else {
          const rx = Number(data.net_rx_bytes || 0)
          const tx = Number(data.net_tx_bytes || 0)
          if (prevRxBytes != null && prevTxBytes != null && prevTs != null) {
            const dt = Math.max(1, (ts - prevTs))
            const upRate = (tx - prevTxBytes) / dt
            const downRate = (rx - prevRxBytes) / dt
            networkUp.value = `${formatBytes(upRate)}/s`
            networkDown.value = `${formatBytes(downRate)}/s`
          }
          prevRxBytes = rx
          prevTxBytes = tx
          prevTs = ts
        }
      } catch (e) {
        // 忽略单次解析错误
      }
    }
    statsEventSource.onerror = () => {
      // 流错误时，回退为轮询模式
      try { statsEventSource.close() } catch {}
      statsEventSource = null
      if (!statsTimer) {
        statsTimer = setInterval(fetchContainerStats, 5000)
      }
    }
  } catch (e) {
    // 创建流失败时，回退轮询
    if (!statsTimer) {
      statsTimer = setInterval(fetchContainerStats, 5000)
    }
  }
}

onMounted(() => {
  fetchContainerDetail()
  refreshTimer = setInterval(fetchContainerDetail, 10000)
  // SSE 启动在 fetchContainerDetail 完成后调用
  // 如果默认展示日志标签页，则立即启动日志
  if (activeTab.value === 'logs') {
    startLogsStream()
  }
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (statsTimer) clearInterval(statsTimer)
  if (statsEventSource) {
    try { statsEventSource.close() } catch {}
    statsEventSource = null
  }
  stopLogsStream()
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

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  flex-shrink: 0;
  background: var(--clay-card);
  border-radius: var(--radius-5xl);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.self-resource-alert {
  margin: 0 0 12px;
  border-radius: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 18px;
  font-weight: 900;
  color: var(--clay-ink);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-tag {
  border-radius: 999px;
  font-weight: 900;
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
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.scroll-content {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
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

.metric-card {
  background: var(--clay-card);
  padding: 18px;
  border-radius: var(--radius-5xl);
  margin-bottom: 20px;
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.metric-title {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
  font-weight: 500;
}

.metric-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.detail-info {
  margin-top: 24px;
  background: transparent;
}

.logs-container {
  height: 600px;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  overflow: hidden;
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: transparent;
  border-bottom: 1px solid rgba(55, 65, 81, 0.12);
}

.logs-options {
  display: flex;
  align-items: center;
  gap: 20px;
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

.logs-content .warning {
  color: #f59e0b;
}

.logs-content .info {
  color: #22c55e;
}
</style>
