<template>
  <header class="topbar">
    <div>
      <!-- 可以在这里添加面包屑或其他导航辅助 -->
      <div class="title">{{ displayTitle }}</div>
    </div>
    <div class="actions">
      <el-tooltip content="刷新" placement="bottom">
        <el-button circle text @click="$emit('refresh')">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </el-tooltip>
      
      <el-tooltip content="消息通知" placement="bottom">
        <el-button circle text>
          <el-icon><Bell /></el-icon>
        </el-button>
      </el-tooltip>

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
            <el-dropdown-item>个人中心</el-dropdown-item>
            <el-dropdown-item divided>退出登录</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { Refresh, Bell, Moon, Sunny } from '@element-plus/icons-vue'

const props = defineProps({
  title: { type: String, default: 'DockPier' }
})

const titleMap = {
  'overview': '仪表盘',
  'containers': '容器管理',
  'images': '镜像管理',
  'volumes': '卷管理',
  'networks': '网络管理',
  'app-store': '应用商城',
  'projects': '项目管理',
  'settings': '系统设置',
  'navigation': '导航页'
}

const displayTitle = computed(() => {
  // 处理标题，移除可能存在的路径部分（如 containers/xxx）
  const baseTitle = props.title.split('/')[0]
  return titleMap[baseTitle] || props.title
})

const isDark = ref(false)

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

onMounted(() => {
  initTheme()
  // 监听外部（如设置页面）的主题变更
  window.addEventListener('theme-change', initTheme)
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

.actions {
  display: flex;
  align-items: center;
  gap: 16px;
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