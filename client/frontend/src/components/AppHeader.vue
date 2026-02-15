<template>
  <header class="topbar">
    <div>
      <!-- 可以在这里添加面包屑或其他导航辅助 -->
      <div v-if="typeof displayTitle === 'object'" class="title-group">
        <div class="title">{{ displayTitle.title }}</div>
        <div class="subtitle">{{ displayTitle.subtitle }}</div>
      </div>
      <div v-else class="title">{{ displayTitle }}</div>
    </div>
    <div class="actions">
      <div class="notification-area">
        <el-tooltip content="消息通知" placement="bottom">
          <div class="notification-wrapper">
            <el-button circle text @click="showNotifications">
              <IconEpBell />
            </el-button>
            <span v-if="hasUnreadNotifications" class="notification-dot"></span>
          </div>
        </el-tooltip>
        <div v-if="notificationPanelVisible" class="notification-panel">
          <div class="notification-header">
            <div class="header-left">
              <span class="header-title">消息中心</span>
              <span v-if="unreadCount" class="header-badge">{{ unreadCount }} 未读</span>
            </div>
            <div class="header-right">
              <div class="poll-status" :class="pollState">
                <span class="poll-dot"></span>
                <span class="poll-text">{{ pollText }}</span>
              </div>
              <el-dropdown trigger="click" @command="handleClearCommand">
                <el-button size="small" plain class="header-clear-btn">清理</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="clearRead">仅清空已读</el-dropdown-item>
                    <el-dropdown-item command="clearAll" divided>清空全部</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
          <div class="notification-controls">
            <el-select v-model="filterType" size="small" class="filter-select" placeholder="类型">
              <el-option label="全部" value="all" />
              <el-option label="提示" value="info" />
              <el-option label="成功" value="success" />
              <el-option label="警告" value="warning" />
              <el-option label="错误" value="error" />
            </el-select>
            <el-button size="small" plain :type="unreadOnly ? 'primary' : 'default'" @click="unreadOnly = !unreadOnly">
              仅未读
            </el-button>
          </div>
          <div v-if="!notifications.length" class="notification-empty">
            暂无消息
          </div>
          <div v-else-if="!filteredNotifications.length" class="notification-empty">
            暂无匹配消息
          </div>
          <div v-else class="notification-list">
            <div
              v-for="(item, index) in filteredNotifications.slice(0, 10)"
              :key="item.id || index"
              class="notification-item"
              :class="{ unread: !item.read }"
            >
              <div class="item-icon" :class="item.type"></div>
              <div class="item-body">
                <div class="item-message" :title="item.message">{{ item.message }}</div>
                <div class="item-meta">
                  <span class="item-time">{{ item.time }}</span>
                  <span class="item-type">{{ typeLabel(item.type) }}</span>
                </div>
              </div>
              <el-tooltip content="删除" placement="left">
                <el-button circle text class="item-delete" @click.stop="handleDeleteNotification(item)">
                  <IconEpDelete />
                </el-button>
              </el-tooltip>
            </div>
          </div>
        </div>
      </div>

      <el-tooltip :content="isDark ? '切换到亮色模式' : '切换到暗色模式'" placement="bottom">
        <el-button circle text @click="toggleTheme">
          <IconEpMoon v-if="isDark" />
          <IconEpSunny v-else />
        </el-button>
      </el-tooltip>

      <el-dropdown trigger="click">
        <div class="user-avatar cursor-pointer">
          <el-avatar :size="32" class="avatar-bg">AD</el-avatar>
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="goProfile">个人中心</el-dropdown-item>
            <el-dropdown-item divided @click="logout">退出登录</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import api from '../api'
const props = defineProps({
  title: { type: String, default: 'Dockpier' }
})

const titleMap = {
  'overview': { title: '仪表盘', subtitle: '综合展示，一目了然' },
  'containers': { title: '容器管理', subtitle: '单独的容器详情页，就是给你看看' },
  'images': { title: '镜像管理', subtitle: '镜像下载有问题，记得更多操作里开启代理或镜像' },
  'volumes': { title: '卷管理', subtitle: '定期备份 volume，防止数据丢失' },
  'networks': { title: '网络管理', subtitle: '玩 NAS 网络很重要，在这里管理' },
  'appstore': { title: '应用商城', subtitle: '从商城里选择你喜欢的项目安装吧~' },
  'app-store': { title: '应用商城', subtitle: '从商城里选择你喜欢的项目安装吧~' },
  'ports': { title: '端口管理', subtitle: '强迫症最爱，用心管理你的安全端口' },
  'projects': { title: '项目编排', subtitle: '太好了，是我最喜欢的项目管理' },
  'compose': { title: '项目编排', subtitle: '太好了，是我最喜欢的项目管理' },
  'settings': { title: '系统设置', subtitle: '配置通用参数，记得改密码宝子' },
  'navigation': { title: '导航页', subtitle: '系统会自动发现 docker 端口链接' }
}

const displayTitle = computed(() => {
  // 处理标题，移除可能存在的路径部分（如 containers/xxx）
  const baseTitle = props.title.split('/')[0]
  return titleMap[baseTitle] || props.title
})

const isDark = ref(false)
const notifications = ref([])
const notificationPanelVisible = ref(false)
let notificationPollTimer = null
let pollStateResetTimer = null

const filterType = ref('all')
const unreadOnly = ref(false)
const pollState = ref('idle')
const lastPollAt = ref(null)

const deletedTempIds = new Set()

const hasNotifications = computed(() => notifications.value.length > 0)
const hasUnreadNotifications = computed(() => notifications.value.some((n) => !n.read))
const unreadCount = computed(() => notifications.value.reduce((acc, n) => acc + (n && !n.read ? 1 : 0), 0))

const filteredNotifications = computed(() => {
  let list = notifications.value
  if (filterType.value !== 'all') {
    list = list.filter((n) => (n?.type || 'info') === filterType.value)
  }
  if (unreadOnly.value) {
    list = list.filter((n) => !n?.read)
  }
  return list
})

const pollText = computed(() => {
  if (pollState.value === 'syncing') return '同步中'
  if (pollState.value === 'error') return '同步失败'
  if (!lastPollAt.value) return '未同步'
  return '已同步'
})

const typeLabel = (type) => {
  if (type === 'success') return '成功'
  if (type === 'error') return '错误'
  if (type === 'warning') return '警告'
  return '提示'
}

const markAllNotificationsRead = async () => {
  if (!notifications.value.length) {
    return
  }
  notifications.value = notifications.value.map((n) => ({ ...n, read: true }))
  try {
    await api.system.markNotificationsRead()
  } catch (e) {
    console.error('标记通知已读失败:', e)
  }
}

const clearNotificationsByFilter = async (predicate) => {
  const current = notifications.value.slice()
  const toClear = current.filter(predicate)
  if (!toClear.length) {
    ElMessage.info('没有可清理的消息')
    return
  }
  notifications.value = current.filter((n) => !predicate(n))
  for (const item of toClear) {
    if (!item.dbId && item.tempId) {
      deletedTempIds.add(item.tempId)
      continue
    }
    const id = item.dbId || item.id
    if (!id) continue
    try {
      await api.system.deleteNotification(id)
    } catch (e) {
      console.error('删除通知失败:', e)
    }
  }
}

const handleClearCommand = async (cmd) => {
  if (cmd === 'clearAll') {
    await clearNotificationsByFilter(() => true)
  } else if (cmd === 'clearRead') {
    await clearNotificationsByFilter((n) => !!n?.read)
  }
}

const handleDeleteNotification = async (item) => {
  const id = item.dbId || item.id
  if (!item.dbId && item.tempId) {
    deletedTempIds.add(item.tempId)
    notifications.value = notifications.value.filter((n) => n !== item)
    return
  }
  notifications.value = notifications.value.filter((n) => n !== item)
  if (!id) {
    return
  }
  try {
    await api.system.deleteNotification(id)
  } catch (e) {
    console.error('删除通知失败:', e)
  }
}

const showNotifications = () => {
  if (!notifications.value.length) {
    ElMessage.info('暂无新消息')
    return
  }
  notificationPanelVisible.value = !notificationPanelVisible.value
  if (notificationPanelVisible.value) {
    markAllNotificationsRead()
  }
}

// 跳转个人中心
const router = useRouter()
const goProfile = () => {
  router.push('/settings')
}

// 退出登录
const logout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  ElMessage.success('已退出登录')
  router.push('/login')
}

const initTheme = () => {
  const theme = localStorage.getItem('theme') || 'auto'
  isDark.value = theme === 'dark' || (theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
}

const toggleTheme = () => {
  const newTheme = isDark.value ? 'light' : 'dark'
  localStorage.setItem('theme', newTheme)
  isDark.value = !isDark.value
  
  // 触发全局主题变更事件
  window.dispatchEvent(new Event('theme-change'))
}

const handleNotification = (event) => {
  const detail = event.detail || {}
  const type = detail.type || 'info'
  const message = detail.message || ''
  if (!message) {
    return
  }
  if (detail.tempId && detail.dbId) {
    const idx = notifications.value.findIndex((n) => n.tempId === detail.tempId || n.id === detail.tempId)
    if (idx >= 0) {
      const existing = notifications.value[idx]
      const next = {
        ...existing,
        id: detail.dbId,
        dbId: detail.dbId,
        tempId: detail.tempId,
        time: detail.createdAt || existing.time || new Date().toLocaleTimeString(),
        read: typeof detail.read === 'boolean' ? detail.read : existing.read
      }
      notifications.value.splice(idx, 1, next)
      if (deletedTempIds.has(detail.tempId)) {
        deletedTempIds.delete(detail.tempId)
        notifications.value = notifications.value.filter((n) => n !== next)
        api.system.deleteNotification(detail.dbId).catch((e) => {
          console.error('删除通知失败:', e)
        })
      }
      return
    }
  }

  const time = detail.createdAt || detail.time || new Date().toLocaleTimeString()
  const dbId = detail.dbId || (typeof detail.id === 'number' ? detail.id : null)
  const tempId = detail.tempId || (!dbId ? (typeof detail.id === 'string' ? detail.id : null) : null)
  notifications.value.unshift({
    id: dbId || tempId || Date.now(),
    dbId,
    tempId,
    type,
    message,
    time,
    read: !!detail.read
  })
  if (notifications.value.length > 50) {
    notifications.value.pop()
  }
  if (type === 'success') {
    ElMessage.success(message)
  } else if (type === 'error') {
    ElMessage.error(message)
  } else if (type === 'warning') {
    ElMessage.warning(message)
  } else {
    ElMessage.info(message)
  }
}

const loadNotifications = async () => {
  pollState.value = 'syncing'
  try {
    const list = await api.system.getNotifications({ limit: 50 })
    if (Array.isArray(list)) {
      notifications.value = list.map((item) => ({
        id: item.id,
        dbId: item.id,
        tempId: null,
        type: item.type || 'info',
        message: item.message,
        time: item.created_at || item.time || '',
        read: !!item.read
      }))
    }
    pollState.value = 'ok'
    lastPollAt.value = Date.now()
  } catch (e) {
    pollState.value = 'error'
    console.error('加载通知失败:', e)
  } finally {
    if (pollStateResetTimer) {
      clearTimeout(pollStateResetTimer)
      pollStateResetTimer = null
    }
    pollStateResetTimer = setTimeout(() => {
      pollState.value = 'idle'
    }, 1200)
  }
}

const handleDocumentClick = (event) => {
  const target = event.target
  if (!(target instanceof HTMLElement)) {
    return
  }
  if (!target.closest('.notification-area')) {
    notificationPanelVisible.value = false
  }
}

const sendHeaderNotification = async (type, message) => {
  try {
    const saved = await api.system.addNotification({ type, message })
    handleNotification({
      detail: {
        type,
        message,
        dbId: saved?.id,
        createdAt: saved?.created_at,
        read: saved?.read
      }
    })
  } catch (e) {
    console.error('保存通知失败:', e)
    handleNotification({ detail: { type, message } })
  }
}

onMounted(() => {
  initTheme()
  window.addEventListener('theme-change', initTheme)
  window.addEventListener('dockpier-notification', handleNotification)
  document.addEventListener('click', handleDocumentClick)
  loadNotifications()
  notificationPollTimer = setInterval(() => {
    loadNotifications()
  }, 15000)
})

onUnmounted(() => {
  window.removeEventListener('theme-change', initTheme)
  window.removeEventListener('dockpier-notification', handleNotification)
  document.removeEventListener('click', handleDocumentClick)
  if (notificationPollTimer) {
    clearInterval(notificationPollTimer)
    notificationPollTimer = null
  }
  if (pollStateResetTimer) {
    clearTimeout(pollStateResetTimer)
    pollStateResetTimer = null
  }
})
</script>

<style scoped>
.topbar {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 16px;
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  transition: all 0.3s ease;
}

.left-panel {
  display: flex;
  align-items: center;
  gap: 16px;
}

.left-panel .title {
  display: flex;
  align-items: center;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.5px;
  background: linear-gradient(120deg, var(--el-color-primary), var(--el-color-primary-light-3));
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  transition: opacity 0.3s;
}

.title-group {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.title-group .title {
  font-size: 20px;
  font-weight: 900;
  color: var(--el-text-color-primary);
  letter-spacing: -0.5px;
}

.title-group .subtitle {
  font-size: 13px;
  color: var(--clay-text-secondary);
  font-weight: 700;
}

.actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.notification-area {
  position: relative;
}

.notification-wrapper {
  position: relative;
  display: inline-flex;
}

.notification-dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, #fda4af, var(--clay-coral));
  box-shadow: 2px 2px 6px rgba(0, 0, 0, 0.1), inset 1px 1px 2px rgba(255, 255, 255, 0.6);
  animation: clay-pulse 1.6s ease-in-out infinite;
}

.notification-panel {
  position: absolute;
  right: 0;
  top: 40px;
  width: 372px;
  max-height: 360px;
  background: var(--clay-card);
  border-radius: var(--radius-5xl);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-float), var(--shadow-clay-inner);
  padding: 12px 14px 14px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
}

.notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 6px 10px;
  margin-bottom: 6px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-title {
  font-size: 15px;
  font-weight: 950;
  color: var(--el-text-color-primary);
}

.header-badge {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 900;
  color: var(--clay-text);
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-inner);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-tip {
  font-size: 12px;
  font-weight: 700;
  color: var(--clay-text-secondary);
}

.poll-status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-inner);
  color: var(--clay-text);
  font-size: 12px;
  font-weight: 800;
}

.poll-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 62%),
    linear-gradient(135deg, #cbd5e1, #94a3b8);
  box-shadow: 2px 2px 6px rgba(0, 0, 0, 0.08), inset 1px 1px 2px rgba(255, 255, 255, 0.55);
}

.poll-status.syncing .poll-dot {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 62%),
    linear-gradient(135deg, var(--clay-sky), #60a5fa);
  animation: clay-pulse 1.1s ease-in-out infinite;
}

.poll-status.ok .poll-dot {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 62%),
    linear-gradient(135deg, var(--clay-mint), var(--clay-mint-2));
}

.poll-status.error .poll-dot {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 62%),
    linear-gradient(135deg, #fb7185, #ef4444);
  animation: clay-pulse 1.1s ease-in-out infinite;
}

.poll-text {
  white-space: nowrap;
}

.header-clear-btn {
  height: 28px;
  padding: 0 10px;
  font-weight: 800;
}

.notification-controls {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 6px 10px;
}

.filter-select {
  width: 120px;
}

.notification-empty {
  padding: 18px 0 14px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.notification-list {
  padding: 2px 4px 0;
  max-height: 286px;
  overflow-y: auto;
}

.notification-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 10px;
  border-radius: 18px;
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-inner);
  margin-bottom: 10px;
  transition: transform 0.15s ease, filter 0.15s ease;
}

.notification-item:last-child {
  margin-bottom: 0;
}

.notification-item:hover {
  transform: translateY(-1px);
  filter: saturate(1.04);
}

.notification-item.unread {
  background:
    radial-gradient(120% 160% at 20% 0%, rgba(255, 255, 255, 0.92), rgba(255, 255, 255, 0.55) 58%, rgba(255, 255, 255, 0.45) 100%),
    linear-gradient(135deg, rgba(251, 113, 133, 0.12), rgba(244, 114, 182, 0.10));
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-inner), var(--shadow-clay-card);
}

.item-icon {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 6px;
  flex-shrink: 0;
  box-shadow: 2px 2px 6px rgba(0, 0, 0, 0.08), inset 1px 1px 2px rgba(255, 255, 255, 0.65);
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, #93c5fd, #60a5fa);
}

.item-icon.success {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, var(--clay-mint), var(--clay-mint-2));
}

.item-icon.warning {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, #fbbf24, #fb7185);
}

.item-icon.error {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, #fb7185, #ef4444);
}

.item-body {
  flex: 1;
  min-width: 0;
}

.item-message {
  font-size: 13px;
  font-weight: 750;
  color: var(--clay-text);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.item-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  color: var(--clay-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.item-type {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--clay-card);
  border: 1px solid var(--clay-border);
}

.item-delete {
  margin-top: -2px;
  flex-shrink: 0;
  color: rgba(239, 68, 68, 0.92);
}

.user-avatar {
  margin-left: 12px;
  display: flex;
  align-items: center;
  cursor: pointer;
  transition: transform 0.2s;
}

.user-avatar:hover {
  transform: scale(1.05);
}

.avatar-bg {
  background: linear-gradient(135deg, var(--el-color-primary), var(--el-color-primary-light-3));
  color: white;
  font-weight: 600;
  font-size: 14px;
  border: 2px solid rgba(255, 255, 255, 0.75);
  box-shadow: var(--shadow-clay-btn), inset 0 0 0 1px rgba(255, 255, 255, 0.35);
}

:deep(.el-button.is-circle) {
  width: 36px;
  height: 36px;
  font-size: 18px;
  border: 1px solid var(--clay-border);
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-btn);
  color: var(--el-text-color-primary);
  transition: transform 0.15s ease, box-shadow 0.15s ease, filter 0.15s ease;
}

:deep(.el-button.is-circle:hover) {
  background: var(--clay-card-solid);
  transform: translateY(-2px);
  filter: saturate(1.04);
}

:deep(.el-button.is-circle:active) {
  transform: scale(0.98);
  box-shadow: var(--shadow-clay-inner);
}

@keyframes clay-pulse {
  0% { transform: scale(1); filter: saturate(1); opacity: 0.9; }
  50% { transform: scale(1.12); filter: saturate(1.08); opacity: 1; }
  100% { transform: scale(1); filter: saturate(1); opacity: 0.9; }
}

/* Dark mode specific overrides handled by CSS variables, 
   but we can add some specific tweaks if needed */
html.dark .topbar {
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}
</style>
