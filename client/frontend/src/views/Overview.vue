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
            <el-icon class="stat-icon"><component :is="stat.icon" /></el-icon>
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
                <el-icon class="resource-icon"><component :is="res.icon" /></el-icon>
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
                <el-icon :class="['status-icon', getEventClass(log.type)]" :size="20">
                  <component :is="getEventIcon(log.type)" />
                </el-icon>
              </div>
              <div class="log-content">
                <div class="log-header">
                  <span :class="['log-type-tag', getEventClass(log.type)]">
                    {{ getEventLabel(log.type) }}
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
import {
  Monitor,
  Cpu,
  Connection,
  Files,
  Collection,
  InfoFilled,
  WarningFilled,
  CircleCheckFilled,
  CircleCloseFilled,
  Refresh,
  Platform,
  Odometer,
  Histogram,
  Aim,
  DataLine,
  Box
} from '@element-plus/icons-vue'
import api from '../api'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const managementMode = (import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toUpperCase()

const statistics = ref([
  { title: '容器总数', value: 0, icon: 'Monitor', type: 'stat-primary', key: 'containers' },
  { title: '运行中', value: 0, icon: 'CircleCheckFilled', type: 'stat-success', key: 'running' },
  { title: '已停止', value: 0, icon: 'CircleCloseFilled', type: 'stat-danger', key: 'stopped' },
  { title: '镜像总数', value: 0, icon: 'Files', type: 'stat-warning', key: 'images' },
  { title: '卷总数', value: 0, icon: 'Collection', type: 'stat-primary', key: 'volumes' },
  { title: '网络总数', value: 0, icon: 'Connection', type: 'stat-primary', key: 'networks' }
])
const router = useRouter()

// 资源使用数据
const resources = ref({
  cpu: { name: 'CPU', icon: 'Aim', percent: 0, used: '—', total: '—' },
  memory: { name: '内存', icon: 'DataLine', percent: 0, used: '—', total: '—' },
  disk: { name: '磁盘', icon: 'Box', percent: 0, used: '—', total: '—' }
})

// 系统事件日志（预览）
const eventLogs = ref([
  { id: 1, type: 'INFO', typeClass: 'status-info', time: '12:01:32', message: '系统启动完成' },
  { id: 2, type: 'WARN', typeClass: 'status-warn', time: '12:15:10', message: '镜像拉取速度较慢' },
  { id: 3, type: 'ERROR', typeClass: 'status-error', time: '12:20:03', message: '容器 nginx 重启失败' }
])

const pageSize = 20
const maxEvents = 100 // 最多显示100条（5页）
const currentPage = ref(1)
let versionWarned = false
let minApiFixNotified = false

// 事件类型映射
const getEventIcon = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return 'CircleCheckFilled'
    case 'warning': return 'WarningFilled'
    case 'error': return 'CircleCloseFilled'
    default: return 'InfoFilled'
  }
}

const getEventLabel = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return 'info'
    case 'warning': return 'warm'
    case 'error': return 'danger'
    default: return 'info'
  }
}

const getEventClass = (type) => {
  switch (type?.toLowerCase()) {
    case 'success': return 'event-success'
    case 'warning': return 'event-warning'
    case 'error': return 'event-error'
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
    eventLogs.value = res.data || res
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
    }

    if (!versionWarned && data.DockerAPIVersion) {
      const apiVersionNum = parseFloat(String(data.DockerAPIVersion))
      if (!Number.isNaN(apiVersionNum) && apiVersionNum >= 1.52 && !data.DaemonMinAPIVersion) {
        versionWarned = true
        ElMessage.warning(`检测到 Docker API 版本为 ${data.DockerAPIVersion}，可能导致部分功能异常；建议设置 daemon.json 的 min-api-version 并重启 Docker`)
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

// 格式化字节大小
const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
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

onMounted(() => {
  fetchStatistics()
  fetchEventLogs() // 初始加载事件日志
  // 每5秒刷新一次
  timer = setInterval(() => {
    fetchStatistics()
    fetchEventLogs()
  }, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.overview-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--el-bg-color-page);
}

/* 顶部操作栏 - 统一风格 */
.filter-bar {
  height: 60px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.filter-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 滚动容器 */
.scroll-container {
  flex: 1;
  overflow: hidden;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0 0 16px 0;
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
  gap: 20px;
  flex-shrink: 0;
}

.stat-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  padding: 24px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.1);
  border-color: var(--el-border-color-darker);
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  line-height: 1.2;
}

.stat-title {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.stat-icon-wrapper {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
}

.stat-icon {
  font-size: 28px;
  color: #fff;
}

/* 统计卡片颜色 */
.stat-primary .stat-icon-wrapper { background: linear-gradient(135deg, #409EFF, #337ecc); box-shadow: 0 4px 12px rgba(64, 158, 255, 0.3); }
.stat-success .stat-icon-wrapper { background: linear-gradient(135deg, #67C23A, #529b2e); box-shadow: 0 4px 12px rgba(103, 194, 58, 0.3); }
.stat-warning .stat-icon-wrapper { background: linear-gradient(135deg, #E6A23C, #b88230); box-shadow: 0 4px 12px rgba(230, 162, 60, 0.3); }
.stat-danger .stat-icon-wrapper { background: linear-gradient(135deg, #F56C6C, #c45656); box-shadow: 0 4px 12px rgba(245, 108, 108, 0.3); }

/* 资源使用区域 */
.resources-section {
  flex-shrink: 0;
}

.resources-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
}

.resource-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  transition: all 0.3s;
}

.resource-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.resource-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.resource-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.resource-icon {
  font-size: 20px;
  color: var(--el-text-color-regular);
}

.resource-name {
  font-weight: 600;
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
  background-color: var(--el-fill-color);
  border-radius: 5px;
  overflow: hidden;
  margin-bottom: 16px;
}

.progress-bar-fill {
  height: 100%;
  border-radius: 5px;
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
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 1;
  min-height: 0;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.logs-list {
  flex: 1;
  overflow-y: auto;
  padding: 0;
}

.logs-pagination {
  padding: 12px 24px;
  border-top: 1px solid var(--el-border-color-lighter);
  display: flex;
  justify-content: flex-end;
  background-color: var(--el-bg-color-page);
}

.log-item {
  display: flex;
  gap: 16px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  align-items: flex-start;
  transition: background-color 0.2s;
}

.log-item:last-child {
  border-bottom: none;
}

.log-item:hover {
  background-color: var(--el-fill-color-light);
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
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
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

.log-type-tag.event-success { background-color: var(--el-color-success-light-9); }
.log-type-tag.event-warning { background-color: var(--el-color-warning-light-9); }
.log-type-tag.event-error { background-color: var(--el-color-danger-light-9); }
.log-type-tag.event-info { background-color: var(--el-color-info-light-9); }

/* 响应式调整 */
@media (max-width: 768px) {
  .scroll-container {
    overflow-y: auto;
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
}
</style>
