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
      <el-tooltip content="刷新" placement="bottom">
        <el-button circle text @click="$emit('refresh')">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </el-tooltip>

      <div class="notification-area">
        <el-tooltip content="消息通知" placement="bottom">
          <div class="notification-wrapper">
            <el-button circle text @click="showNotifications">
              <el-icon><Bell /></el-icon>
            </el-button>
            <span v-if="hasUnreadNotifications" class="notification-dot"></span>
          </div>
        </el-tooltip>
        <div v-if="notificationPanelVisible" class="notification-panel">
          <div class="notification-header">
            <span class="header-title">消息中心</span>
          </div>
          <div v-if="!notifications.length" class="notification-empty">
            暂无消息
          </div>
          <div v-else class="notification-list">
            <div
              v-for="(item, index) in notifications.slice(0, 10)"
              :key="item.id || index"
              class="notification-item"
            >
              <div class="item-time">{{ item.time }}</div>
              <div class="item-message">{{ item.message }}</div>
              <el-button
                text
                type="danger"
                size="small"
                @click.stop="handleDeleteNotification(item)"
              >
                删除
              </el-button>
            </div>
          </div>
        </div>
      </div>

      <el-tooltip :content="isDark ? '切换到亮色模式' : '切换到暗色模式'" placement="bottom">
        <el-button circle text @click="toggleTheme">
          <el-icon v-if="isDark"><Moon /></el-icon>
          <el-icon v-else><Sunny /></el-icon>
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
import { Refresh, Bell, Moon, Sunny } from '@element-plus/icons-vue'
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

const deletedTempIds = new Set()

const hasNotifications = computed(() => notifications.value.length > 0)
const hasUnreadNotifications = computed(() => notifications.value.some((n) => !n.read))

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
  } catch (e) {
    console.error('加载通知失败:', e)
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
})

onUnmounted(() => {
  window.removeEventListener('theme-change', initTheme)
  window.removeEventListener('dockpier-notification', handleNotification)
  document.removeEventListener('click', handleDocumentClick)
})
</script>

<style scoped>
.topbar {
  height: 60px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
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
  font-weight: 700;
  color: #1e293b;
  letter-spacing: -0.5px;
}

.title-group .subtitle {
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
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
  background-color: var(--el-color-danger);
}

.notification-panel {
  position: absolute;
  right: 0;
  top: 40px;
  width: 320px;
  max-height: 360px;
  background: var(--el-bg-color);
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.18);
  padding: 12px 16px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
}

.notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.header-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.notification-empty {
  padding: 12px 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.notification-list {
  margin-top: 4px;
  max-height: 280px;
  overflow-y: auto;
}

.notification-item {
  padding: 6px 0;
  border-bottom: 1px dashed var(--el-border-color-lighter);
}

.notification-item:last-child {
  border-bottom: none;
}

.item-time {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-bottom: 2px;
}

.item-message {
  font-size: 13px;
  color: var(--el-text-color-primary);
  line-height: 1.4;
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
  border: 2px solid var(--el-bg-color);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

:deep(.el-button.is-circle) {
  width: 36px;
  height: 36px;
  font-size: 18px;
  border: none;
  background-color: transparent;
  color: var(--el-text-color-regular);
  transition: all 0.2s;
}

:deep(.el-button.is-circle:hover) {
  background-color: var(--el-fill-color);
  color: var(--el-color-primary);
  transform: translateY(-1px);
}

/* Dark mode specific overrides handled by CSS variables, 
   but we can add some specific tweaks if needed */
html.dark .topbar {
  background-color: var(--el-bg-color-overlay);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}
</style>
