<template>
  <aside class="sidebar">
    <div class="logo">
      <el-icon :size="24" class="mr-2"><Box /></el-icon>
      <span class="font-bold text-lg">TRADIS</span>
    </div>
    <el-menu 
      :router="true" 
      class="menu" 
      :default-active="defaultActive"
      background-color="#001529"
      text-color="#a6adb4"
      active-text-color="#fff">
      
      <div class="menu-group-title">总览</div>
      <el-menu-item index="/overview">
        <el-icon><Monitor /></el-icon>
        <span>仪表盘</span>
      </el-menu-item>
      <el-menu-item index="/navigation">
        <el-icon><Operation /></el-icon>
        <span>导航页</span>
      </el-menu-item>

      <div class="menu-group-title">容器管理</div>
      <el-menu-item index="/app-store">
        <el-icon><Shop /></el-icon>
        <span>应用商店</span>
      </el-menu-item>
      <el-menu-item index="/projects" v-if="isDS">
        <el-icon><Folder /></el-icon>
        <span>项目</span>
      </el-menu-item>
      <el-menu-item index="/containers" v-if="isDS">
        <el-icon><Box /></el-icon>
        <span>容器</span>
      </el-menu-item>
      <el-menu-item index="/compose" v-if="isCS">
        <el-icon><Folder /></el-icon>
        <span>Compose</span>
      </el-menu-item>

      <div class="menu-group-title">资源管理</div>
      <el-menu-item index="/images">
        <el-icon><Picture /></el-icon>
        <span>镜像</span>
      </el-menu-item>
      <el-menu-item index="/volumes">
        <el-icon><Files /></el-icon>
        <span>数据卷</span>
      </el-menu-item>
      <el-menu-item index="/networks">
        <el-icon><Connection /></el-icon>
        <span>网络</span>
      </el-menu-item>
      <el-menu-item index="/ports">
        <el-icon><Connection /></el-icon>
        <span>端口</span>
      </el-menu-item>
      
      <div class="menu-group-title">系统</div>
      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <span>设置</span>
      </el-menu-item>
    </el-menu>

    <div class="sidebar-divider"></div>

    <div class="footer-status">
      <div class="status-item">
        <span :class="appStoreConnected ? 'dot connected' : 'dot'" /> 
        <span> {{ appStoreConnected ? '商城服务连接正常' : '商城服务连接异常' }} </span>
      </div>
      <div class="version-info">v0.4.0 (开发中)</div>
    </div>
  </aside>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import request from '../utils/request'

const route = useRoute()
const managementMode = ((window.__ENV__ && window.__ENV__.MANAGEMENT_MODE) || import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toLowerCase()
const isCS = managementMode === 'centralized' || managementMode === 'cs'
const isDS = managementMode === 'distributed' || managementMode === 'ds'
const defaultActive = computed(() => {
  const path = route.path
  if (path.startsWith('/containers/')) return '/containers'
  if (path.startsWith('/projects/')) return '/projects'
  if (path.startsWith('/compose')) return '/compose'
  return path
})

const appStoreConnected = ref(false)
let pingTimer = null

const pingAppStore = async () => {
  try {
    const s = await request.get('/settings/global')
    const base = (s && s.appStoreServerUrl) ? s.appStoreServerUrl : 'https://template.cgakki.top:33333'
    const resp = await fetch(base.replace(/\/$/, '') + '/api/templates', { method: 'GET' })
    appStoreConnected.value = resp.ok
  } catch (e) {
    appStoreConnected.value = false
  }
}

onMounted(() => {
  pingAppStore()
  pingTimer = setInterval(pingAppStore, 15000)
})

onUnmounted(() => {
  if (pingTimer) clearInterval(pingTimer)
})
</script>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  height: 100%;
  border-right: 1px solid rgba(255,255,255,0.05);
  background-color: #001529; /* 黑色背景 */
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 24px;
  color: #fff;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  background-color: #002140; /* Logo 区域稍微亮一点 */
}

.mr-2 { margin-right: 8px; }
.font-bold { font-weight: 700; }
.text-lg { font-size: 18px; }

.menu {
  flex: 1;
  border-right: none;
  padding: 16px 0;
  overflow-y: auto;
  overflow-x: hidden;
}

.menu-group-title {
  padding: 16px 24px 8px;
  font-size: 12px;
  font-weight: 600;
  color: #5c6b7f;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* 覆盖 el-menu 样式 */
:deep(.el-menu-item) {
  height: 50px;
  line-height: 50px;
  margin: 4px 12px;
  border-radius: 4px;
  width: auto;
}

:deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.05) !important;
  color: #fff !important;
}

:deep(.el-menu-item.is-active) {
  background-color: var(--color-primary);
  color: #fff !important;
  font-weight: 500;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

:deep(.el-menu-item .el-icon) {
  font-size: 18px;
  margin-right: 10px;
}

/* 滚动条样式 */
.menu::-webkit-scrollbar {
  width: 4px;
}
.menu::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.1);
  border-radius: 2px;
}
.menu::-webkit-scrollbar-track {
  background: transparent;
}

.sidebar-divider {
  height: 1px;
  background-color: rgba(255, 255, 255, 0.05);
  margin: 0 24px;
}

.footer-status {
  padding: 16px 24px;
  background: #001529;
  border-top: 1px solid rgba(255,255,255,0.05);
  font-size: 12px;
  color: #6b7280;
  margin-top: auto;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #ef4444;
}

.dot.connected {
  background-color: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.4);
}

.version-info {
  margin-top: 4px;
  opacity: 0.6;
}
</style>
