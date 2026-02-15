<template>
  <div class="overview-view">
    <div class="scroll-container">
      <!-- 统计卡片 -->
      <div class="stats-grid">
        <div v-for="stat in statistics" :key="stat.title" class="stat-card" :class="stat.type" @click="handleStatClick(stat)">
          <div class="stat-content">
            <div class="stat-value">{{ stat.value }}</div>
            <div class="stat-title">{{ stat.title }}</div>
          </div>
          <div class="stat-icon-wrapper">
            <component :is="stat.iconComp" class="stat-icon" />
          </div>
        </div>
      </div>

      <!-- 资源使用 -->
      <div class="resources-section">
        <h3 class="section-title">系统资源</h3>
        <div class="resources-grid">
            <div v-for="(res, key) in resources" :key="key" class="resource-card">
            <div class="resource-header">
            <div class="resource-info">
              <component :is="res.iconComp" class="resource-icon-img" />
              <span class="resource-name">{{ res.name }}</span>
            </div>
              <span class="resource-percent" :class="getUsageColorText(res.percent)">{{ res.percent }}%</span>
            </div>
            <div class="progress-bar-bg">
              <div class="progress-bar-fill" :class="getUsageColorBg(res.percent)" :style="{ width: res.percent + '%' }"></div>
            </div>
            <div class="resource-meta">
              <span class="meta-item">已用: {{ res.used }}</span>
              <span class="meta-item">总量: {{ res.total }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 系统事件日志 -->
      <div class="logs-section">
        <h3 class="section-title">系统事件 <span class="title-note">（日志20条为一页，只显示最近的5页）</span></h3>
        <div class="logs-card">
          <div class="logs-list">
            <div v-for="log in displayedLogs" :key="log.id" class="log-item">
              <div class="log-status">
                <component
                  :is="getEventIcon(log.typeClass || log.type)"
                  :class="['status-icon', getEventClass(log.typeClass || log.type)]"
                  style="font-size: 20px;"
                />
              </div>
              <div class="log-content">
                <div class="log-header">
                  <span :class="['log-type-tag', getEventClass(log.typeClass || log.type)]">
                    {{ getEventLabel(log.typeClass || log.type) }}
                  </span>
                  <span class="log-time">{{ log.time }}</span>
                </div>
                <div class="log-message">{{ log.message }}</div>
              </div>
            </div>
          </div>
          <div class="logs-pagination" v-if="eventLogs.length > pageSize">
            <el-pagination
              v-model:current-page="currentPage"
              :page-size="pageSize"
              :total="Math.min(eventLogs.length, maxEvents)"
              layout="prev, pager, next"
              hide-on-single-page
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import api from '../api'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { formatBytes } from '../utils/format'
import IconEpMonitor from '~icons/ep/monitor'
import IconEpCircleCheckFilled from '~icons/ep/circle-check-filled'
import IconEpCircleCloseFilled from '~icons/ep/circle-close-filled'
import IconEpFiles from '~icons/ep/files'
import IconEpCollection from '~icons/ep/collection'
import IconEpConnection from '~icons/ep/connection'
import IconMdiCpu64Bit from '~icons/mdi/cpu-64-bit'
import IconMdiMemory from '~icons/mdi/memory'
import IconMdiHarddisk from '~icons/mdi/harddisk'
import IconEpWarningFilled from '~icons/ep/warning-filled'
import IconEpInfoFilled from '~icons/ep/info-filled'

const managementMode = (import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toUpperCase()

const statistics = ref([
  { title: '容器总数', value: 0, iconComp: IconEpMonitor, type: 'stat-primary', key: 'containers' },
  { title: '运行中', value: 0, iconComp: IconEpCircleCheckFilled, type: 'stat-success', key: 'running' },
  { title: '已停止', value: 0, iconComp: IconEpCircleCloseFilled, type: 'stat-danger', key: 'stopped' },
  { title: '镜像总数', value: 0, iconComp: IconEpFiles, type: 'stat-warning', key: 'images' },
  { title: '卷总数', value: 0, iconComp: IconEpCollection, type: 'stat-primary', key: 'volumes' },
  { title: '网络总数', value: 0, iconComp: IconEpConnection, type: 'stat-primary', key: 'networks' }
])
const router = useRouter()

// 资源使用数据
const resources = ref({
  cpu: { name: 'CPU', iconComp: IconMdiCpu64Bit, percent: 0, used: '—', total: '—' },
  memory: { name: '内存', iconComp: IconMdiMemory, percent: 0, used: '—', total: '—' },
  disk: { name: '磁盘', iconComp: IconMdiHarddisk, percent: 0, used: '—', total: '—' }
})

// 系统事件日志（预览）
const eventLogs = ref([
  { id: 1, type: 'INFO', typeClass: 'info', time: '12:01:32', message: '系统启动完成' },
  { id: 2, type: 'WARN', typeClass: 'warning', time: '12:15:10', message: '镜像拉取速度较慢' },
  { id: 3, type: 'ERROR', typeClass: 'danger', time: '12:20:03', message: '容器 nginx 重启失败' }
])

const pageSize = 20
const maxEvents = 100 // 最多显示100条（5页）
const currentPage = ref(1)
let versionWarned = false
let minApiFixNotified = false
const DOCKER_API_REMINDER_KEY = 'docker-api-reminder'

try {
  const reminded = localStorage.getItem(DOCKER_API_REMINDER_KEY)
  if (reminded) {
    versionWarned = true
    minApiFixNotified = true
  }
} catch (e) {
  console.warn('读取 Docker API 提示缓存失败', e)
}

// 事件类型映射
const getEventIcon = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return IconEpCircleCheckFilled
    case 'warning':
    case 'warn': return IconEpWarningFilled
    case 'error':
    case 'danger': return IconEpCircleCloseFilled
    default: return IconEpInfoFilled
  }
}

const getEventLabel = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return '成功'
    case 'warning':
    case 'warn': return '警告'
    case 'error':
    case 'danger': return '错误'
    default: return '信息'
  }
}

const getEventClass = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return 'event-success'
    case 'warning':
    case 'warn': return 'event-warning'
    case 'error':
    case 'danger': return 'event-error'
    default: return 'event-info'
  }
}

const displayedLogs = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  const end = start + pageSize
  return eventLogs.value.slice(start, end)
})

/**
 * 获取系统事件日志
 */
const fetchEventLogs = async () => {
  try {
    const res = await api.system.events()
    const list = Array.isArray(res?.data || res) ? (res.data || res) : []
    eventLogs.value = list.slice(-maxEvents)
    const maxPage = Math.max(1, Math.ceil(eventLogs.value.length / pageSize))
    if (currentPage.value > maxPage) {
      currentPage.value = maxPage
    }
  } catch (error) {
    console.error('获取事件日志失败:', error)
    // 失败时不覆盖，或者显示错误信息
  }
}

/**
 * 获取统计数据（容器/镜像/卷/网络计数）
 */
const fetchStatistics = async () => {
  try {
    const res = await api.system.info()
    const data = res.data || res // 兼容拦截器返回
    
    statistics.value[0].value = data.Containers || 0
    statistics.value[1].value = data.ContainersRunning || 0
    statistics.value[2].value = data.ContainersStopped || 0
    statistics.value[3].value = data.Images || 0
    statistics.value[4].value = data.Volumes || 0
    statistics.value[5].value = data.Networks || 0
    
    // 更新资源使用数据（基础信息）
    // 注意：后端返回的 DiskUsage/MemUsage 单位是字节，需要转换
    // 这里主要使用百分比，如果后端直接返回百分比最好，或者在此计算
    // 后端 /api/system/info 返回了 MemTotal, MemUsage, DiskTotal, DiskUsage, CpuUsage
    
    if (data.CpuUsage !== undefined) {
      resources.value.cpu.percent = parseFloat(data.CpuUsage.toFixed(1))
    }
    
    if (data.MemTotal && data.MemUsage) {
      const memPercent = (data.MemUsage / data.MemTotal) * 100
      resources.value.memory.percent = parseFloat(memPercent.toFixed(1))
      resources.value.memory.used = formatBytes(data.MemUsage)
      resources.value.memory.total = formatBytes(data.MemTotal)
    }
    
    if (data.DiskTotal && data.DiskUsage) {
      const diskPercent = (data.DiskUsage / data.DiskTotal) * 100
      resources.value.disk.percent = parseFloat(diskPercent.toFixed(1))
      resources.value.disk.used = formatBytes(data.DiskUsage)
      resources.value.disk.total = formatBytes(data.DiskTotal)
    }

    if (!minApiFixNotified && data.MinAPIVersionFixNeeded) {
      minApiFixNotified = true
      if (data.MinAPIVersionFixApplied) {
        ElMessage.success(`已为 Docker 写入 min-api-version=${data.MinAPIVersionFixTarget || '1.43'}，请重启 Docker 服务后再试`)
      } else if (data.DaemonMinAPIVersion) {
        ElMessage.info(`检测到 Docker API 版本偏高，当前 daemon.json 的 min-api-version=${data.DaemonMinAPIVersion}`)
      } else {
        ElMessage.warning(`检测到 Docker API 版本偏高，但无法写入 daemon.json（${data.MinAPIVersionFixError || '未知原因'}），建议手动加入 min-api-version=${data.MinAPIVersionFixTarget || '1.43'} 并重启 Docker`)
      }
      try {
        localStorage.setItem(DOCKER_API_REMINDER_KEY, '1')
      } catch (e) {
        console.warn('写入 Docker API 提示缓存失败', e)
      }
    }

    if (!versionWarned && data.DockerAPIVersion) {
      const apiVersionNum = parseFloat(String(data.DockerAPIVersion))
      if (!Number.isNaN(apiVersionNum) && apiVersionNum >= 1.52 && !data.DaemonMinAPIVersion) {
        versionWarned = true
        ElMessage.warning(`检测到 Docker API 版本为 ${data.DockerAPIVersion}，可能导致部分功能异常；建议设置 daemon.json 的 min-api-version 并重启 Docker`)
        try {
          localStorage.setItem(DOCKER_API_REMINDER_KEY, '1')
        } catch (e) {
          console.warn('写入 Docker API 提示缓存失败', e)
        }
      }
    }

  } catch (error) {
    console.error('获取系统信息失败:', error)
    ElMessage.error('获取系统信息失败')
  }
}

const handleStatClick = (stat) => {
  if (stat.key === 'containers' || stat.key === 'running' || stat.key === 'stopped') {
    const containersPath = managementMode === 'DS' ? '/containers' : '/compose'
    const status =
      stat.key === 'running'
        ? 'running'
        : stat.key === 'stopped'
        ? 'stopped'
        : ''
    router.push({ path: containersPath, query: { status } })
    return
  }
  if (stat.key === 'images') {
    router.push({ path: '/images' })
    return
  }
  if (stat.key === 'volumes') {
    router.push({ path: '/volumes' })
    return
  }
  if (stat.key === 'networks') {
    router.push({ path: '/networks' })
    return
  }
}

// 获取颜色类名
const getUsageColorText = (percent) => {
  if (percent >= 90) return 'text-danger'
  if (percent >= 75) return 'text-warning'
  return 'text-success'
}

const getUsageColorBg = (percent) => {
  if (percent >= 90) return 'bg-danger'
  if (percent >= 75) return 'bg-warning'
  return 'bg-success'
}

let timer = null
const refreshInterval = 5000

const stopRefresh = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

const startRefresh = () => {
  stopRefresh()
  fetchStatistics()
  fetchEventLogs()
  timer = setInterval(() => {
    if (document.visibilityState !== 'visible') return
    fetchStatistics()
    fetchEventLogs()
  }, refreshInterval)
}

const handleVisibilityChange = () => {
  if (document.visibilityState === 'visible') {
    startRefresh()
  } else {
    stopRefresh()
  }
}

onMounted(() => {
  startRefresh()
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onUnmounted(() => {
  stopRefresh()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.overview-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--clay-bg);
  box-sizing: border-box;
  overflow: hidden;
  gap: 12px;
}


/* 滚动容器 */
.scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.overview-hero {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 16px;
  align-items: stretch;
  padding: 16px 20px;
  border-radius: var(--radius-5xl);
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.hero-text {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  min-width: 0;
}

.hero-title {
  font-size: 24px;
  font-weight: 900;
  letter-spacing: -0.4px;
  color: var(--el-text-color-primary);
}

.hero-subtitle {
  font-size: 14px;
  font-weight: 700;
  color: var(--clay-text-secondary);
  line-height: 1.5;
}

.hero-visual {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
}

.section-title {
  font-size: 16px;
  font-weight: 900;
  color: var(--el-text-color-primary);
  margin: 0 0 12px 0;
  display: flex;
  align-items: center;
}

.title-note {
  font-size: 12px;
  font-weight: normal;
  color: var(--el-text-color-secondary);
  margin-left: 8px;
}

/* 统计卡片区域 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  flex-shrink: 0;
}

.stat-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  padding: 16px 20px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-clay-float), var(--shadow-clay-inner);
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-value {
  font-size: 24px;
  font-weight: 900;
  color: var(--el-text-color-primary);
  line-height: 1.2;
}

.stat-title {
  font-size: 13px;
  color: var(--clay-text-secondary);
  font-weight: 700;
}

.stat-icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
  box-shadow: var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.stat-icon {
  font-size: 24px;
  color: var(--stat-icon-color);
}

/* 统计卡片颜色 */
.stat-primary .stat-icon-wrapper { background: var(--stat-bg-primary); }
.stat-success .stat-icon-wrapper { background: var(--stat-bg-success); }
.stat-warning .stat-icon-wrapper { background: var(--stat-bg-warning); }
.stat-danger .stat-icon-wrapper { background: var(--stat-bg-danger); }

/* 资源使用区域 */
.resources-section {
  flex-shrink: 0;
}

.resources-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.resource-card {
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  padding: 16px 20px;
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  transition: all 0.3s;
}

.resource-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-clay-float), var(--shadow-clay-inner);
}

.resource-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.resource-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.resource-icon-img {
  width: 22px;
  height: 22px;
  border-radius: 8px;
  object-fit: cover;
  box-shadow: var(--shadow-clay-float);
}

.resource-name {
  font-weight: 900;
  font-size: 15px;
  color: var(--el-text-color-primary);
}

.resource-percent {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 600;
  font-size: 16px;
}

.progress-bar-bg {
  width: 100%;
  height: 10px;
  background-color: var(--clay-card);
  border-radius: 999px;
  overflow: hidden;
  margin-bottom: 16px;
  box-shadow: var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.progress-bar-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.resource-meta {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 颜色类 */
.text-success { color: var(--el-color-success); }
.text-warning { color: var(--el-color-warning); }
.text-danger { color: var(--el-color-danger); }
.bg-success { background-color: var(--el-color-success); }
.bg-warning { background-color: var(--el-color-warning); }
.bg-danger { background-color: var(--el-color-danger); }

/* 日志卡片 */
.logs-section {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.logs-card {
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 1;
  min-height: 0;
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.logs-list {
  flex: 1;
  overflow-y: auto;
  padding: 0;
}

.logs-pagination {
  padding: 12px 24px;
  border-top: 1px solid var(--clay-border);
  display: flex;
  justify-content: flex-end;
  background-color: transparent;
}

.log-item {
  display: flex;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--clay-border);
  align-items: flex-start;
  transition: background-color 0.2s;
}

.log-item:last-child {
  border-bottom: none;
}

.log-item:hover {
  background-color: var(--clay-card);
}

.log-status {
  padding-top: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.log-content {
  flex: 1;
  min-width: 0;
}

.log-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}

.log-type-tag {
  font-size: 12px;
  font-weight: 900;
  padding: 6px 10px;
  border-radius: 999px;
  box-shadow: var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.log-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: 'JetBrains Mono', monospace;
}

.log-message {
  font-size: 14px;
  color: var(--el-text-color-primary);
  line-height: 1.5;
  word-break: break-all;
}

/* 日志类型样式 */
.event-success { color: var(--el-color-success); }
.event-warning { color: var(--el-color-warning); }
.event-error { color: var(--el-color-danger); }
.event-info { color: var(--el-color-info); }

.log-type-tag.event-success { background: var(--tag-bg-success); }
.log-type-tag.event-warning { background: var(--tag-bg-warning); }
.log-type-tag.event-error { background: var(--tag-bg-danger); }
.log-type-tag.event-info { background: var(--tag-bg-info); }

/* 响应式调整 */
@media (max-width: 768px) {
  .scroll-container {
    overflow-y: auto;
    padding: 14px;
  }

  .stats-grid {
    grid-template-columns: 1fr;
    flex-shrink: 0;
  }
  
  .resources-grid {
    grid-template-columns: 1fr;
  }
  
  .logs-section {
    flex: none;
    height: 500px;
  }

  .filter-bar {
    padding: 0 16px;
  }

  .overview-hero {
    grid-template-columns: 1fr;
    padding: 18px 16px;
    gap: 16px;
  }
}
</style>
