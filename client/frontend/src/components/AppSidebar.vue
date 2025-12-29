<template>
  <aside class="sidebar">
    <div class="logo">
      <el-icon :size="24" class="mr-2"><Box /></el-icon>
      <span class="font-bold text-lg">TRADIS</span>
    </div>
    <el-menu 
      :router="true" 
      class="menu" 
      :default-active="defaultActive">
      
      <div class="menu-group-title">总览</div>
      <el-menu-item index="/overview">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/overview')" :src="getSidebarIcon('/overview')" class="menu-icon-img" alt="仪表盘" />
          <el-icon v-else><Monitor /></el-icon>
        </span>
        <span>仪表盘</span>
      </el-menu-item>
      <el-menu-item index="/navigation">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/navigation')" :src="getSidebarIcon('/navigation')" class="menu-icon-img" alt="导航页" />
          <el-icon v-else><Operation /></el-icon>
        </span>
        <span>导航页</span>
      </el-menu-item>

      <div class="menu-group-title">容器管理</div>
      <el-menu-item index="/app-store">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/app-store')" :src="getSidebarIcon('/app-store')" class="menu-icon-img" alt="应用商店" />
          <el-icon v-else><Shop /></el-icon>
        </span>
        <span>应用商店</span>
      </el-menu-item>
      <el-menu-item index="/projects" v-if="isDS">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/projects')" :src="getSidebarIcon('/projects')" class="menu-icon-img" alt="项目" />
          <el-icon v-else><Folder /></el-icon>
        </span>
        <span>项目</span>
      </el-menu-item>
      <el-menu-item index="/containers" v-if="isDS">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/containers')" :src="getSidebarIcon('/containers')" class="menu-icon-img" alt="容器" />
          <el-icon v-else><Box /></el-icon>
        </span>
        <span>容器</span>
      </el-menu-item>
      <el-menu-item index="/compose" v-if="isCS">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/compose')" :src="getSidebarIcon('/compose')" class="menu-icon-img" alt="Compose" />
          <el-icon v-else><Folder /></el-icon>
        </span>
        <span>Compose</span>
      </el-menu-item>

      <div class="menu-group-title">资源管理</div>
      <el-menu-item index="/images">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/images')" :src="getSidebarIcon('/images')" class="menu-icon-img" alt="镜像" />
          <el-icon v-else><Picture /></el-icon>
        </span>
        <span>镜像</span>
      </el-menu-item>
      <el-menu-item index="/volumes">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/volumes')" :src="getSidebarIcon('/volumes')" class="menu-icon-img" alt="数据卷" />
          <el-icon v-else><Files /></el-icon>
        </span>
        <span>数据卷</span>
      </el-menu-item>
      <el-menu-item index="/networks">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/networks')" :src="getSidebarIcon('/networks')" class="menu-icon-img" alt="网络" />
          <el-icon v-else><Connection /></el-icon>
        </span>
        <span>网络</span>
      </el-menu-item>
      <el-menu-item index="/ports">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/ports')" :src="getSidebarIcon('/ports')" class="menu-icon-img" alt="端口" />
          <el-icon v-else><Connection /></el-icon>
        </span>
        <span>端口</span>
      </el-menu-item>
      
      <div class="menu-group-title">系统</div>
      <el-menu-item index="/settings">
        <span class="menu-icon-slot">
          <img v-if="getSidebarIcon('/settings')" :src="getSidebarIcon('/settings')" class="menu-icon-img" alt="设置" />
          <el-icon v-else><Setting /></el-icon>
        </span>
        <span>设置</span>
      </el-menu-item>
    </el-menu>

    <div class="sidebar-divider"></div>

    <div class="footer-status">
      <div class="status-item">
        <span :class="appStoreConnected ? 'dot connected' : 'dot'" /> 
        <span> {{ appStoreConnected ? '商城服务连接正常' : '商城服务连接异常' }} </span>
      </div>
      <div class="version-info">
        <div class="version-line">
          <span class="version-label">本地</span>
          <span class="version-value">{{ localVersionText }}</span>
        </div>
        <div class="version-line">
          <span class="version-label">服务端</span>
          <span class="version-value">{{ serverVersionText }}</span>
          <el-tag v-if="hasNewVersion" size="small" type="warning" effect="dark" class="update-tag">有新版</el-tag>
        </div>
      </div>
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
const appStoreBaseUrl = ref('')
let pingTimer = null
let versionTimer = null

const localVersion = ref('')
const serverVersion = ref('')
const hasNewVersion = ref(false)

const localVersionText = computed(() => {
  const v = String(localVersion.value || '').trim()
  return v ? v : '—'
})

const serverVersionText = computed(() => {
  const v = (serverVersion.value || '').trim()
  if (!v) return '—'
  return v.startsWith('v') ? v : `v${v}`
})

const loadVersionStatusFromDB = async () => {
  try {
    const localRes = await request.get('/settings/kv/client_version')
    localVersion.value = (localRes && localRes.value) ? String(localRes.value) : ''
  } catch (e) {}

  try {
    const serverRes = await request.get('/settings/kv/appstore_server_version')
    serverVersion.value = (serverRes && serverRes.value) ? String(serverRes.value) : ''
  } catch (e) {}

  try {
    const flagRes = await request.get('/settings/kv/appstore_has_new_version')
    const raw = (flagRes && flagRes.value) ? String(flagRes.value) : ''
    const v = raw.trim().toLowerCase()
    hasNewVersion.value = (v === '1' || v === 'true' || v === 'yes' || v === 'on')
  } catch (e) {}
}

const iconMode = ref((localStorage.getItem('ui_icon_mode') || 'clay').toLowerCase())
const useClayIcons = computed(() => iconMode.value !== 'element')
const clayIconByRoute = {
  '/app-store': '/icons/clay/appstore.jpg',
  '/compose': '/icons/clay/compose.jpg',
  '/networks': '/icons/clay/network.jpg',
  '/volumes': '/icons/clay/volume.jpg',
  '/settings': '/icons/clay/settings.jpg',
  '/images': '/icons/clay/registry.jpg',
  '/containers': '/icons/clay/compose.jpg',
  '/overview': '/icons/clay/overview.jpg',
  '/navigation': '/icons/clay/navigation.jpg',
  '/ports': '/icons/clay/port.jpg'
}

const getSidebarIcon = (path) => {
  if (!useClayIcons.value) return ''
  return clayIconByRoute[path] || ''
}

const pingAppStore = async () => {
  try {
    const s = await request.get('/settings/global')
    const base = (s && s.appStoreServerUrl) ? s.appStoreServerUrl : 'https://template.cgakki.top:33333'
    const baseTrim = base.replace(/\/$/, '')
    appStoreBaseUrl.value = baseTrim
    const resp = await fetch(baseTrim + '/api/templates', { method: 'GET' })
    appStoreConnected.value = resp.ok
  } catch (e) {
    appStoreConnected.value = false
  }
}

onMounted(() => {
  const onStorage = (e) => {
    if (e && e.key === 'ui_icon_mode') {
      iconMode.value = (localStorage.getItem('ui_icon_mode') || 'clay').toLowerCase()
    }
  }
  window.addEventListener('storage', onStorage)
  pingAppStore()
  loadVersionStatusFromDB()
  pingTimer = setInterval(pingAppStore, 15000)
  versionTimer = setInterval(loadVersionStatusFromDB, 60000)
  onUnmounted(() => {
    window.removeEventListener('storage', onStorage)
  })
})

onUnmounted(() => {
  if (pingTimer) clearInterval(pingTimer)
  if (versionTimer) clearInterval(versionTimer)
})
</script>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--sidebar-bg);
  border: 1px solid var(--clay-border);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  border-radius: var(--radius-5xl);
  overflow: hidden;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  padding: 0 18px;
  color: var(--clay-ink);
  border-bottom: 1px solid var(--clay-border);
  background:
    radial-gradient(120% 100% at 20% 0%, rgba(255, 255, 255, 0.9), rgba(255, 255, 255, 0.45) 55%, rgba(255, 255, 255, 0.2) 100%),
    linear-gradient(135deg, rgba(147, 197, 253, 0.18), rgba(110, 231, 183, 0.12));
}

.mr-2 { margin-right: 8px; }
.font-bold { font-weight: 700; }
.text-lg { font-size: 18px; }

.menu {
  flex: 1;
  border-right: none;
  padding: 12px 14px 10px;
  overflow-y: auto;
  overflow-x: visible;
  background: transparent;
}

.menu-group-title {
  padding: 14px 14px 8px;
  font-size: 12px;
  font-weight: 800;
  color: var(--clay-text-secondary);
  letter-spacing: 0.4px;
}

/* 覆盖 el-menu 样式 */
:deep(.el-menu-item) {
  height: 50px;
  line-height: 50px;
  margin: 6px 6px;
  border-radius: 18px;
  width: auto;
  color: var(--clay-text);
  position: relative;
  overflow: visible;
}

:deep(.el-menu-item .el-icon) {
  font-size: 18px;
  margin-right: 0;
}

.menu-icon-slot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-right: 10px;
  flex-shrink: 0;
}

.menu-icon-img {
  width: 20px;
  height: 20px;
  object-fit: contain;
  filter: drop-shadow(2px 4px 10px rgba(0, 0, 0, 0.12));
  transition: transform 0.15s ease, filter 0.15s ease;
}

:deep(.el-menu-item:hover) .menu-icon-img {
  transform: translateY(-1px);
  filter: drop-shadow(2px 5px 14px rgba(0, 0, 0, 0.14)) saturate(1.05);
}

:deep(.el-menu-item.is-active) .menu-icon-img {
  filter: drop-shadow(2px 6px 16px rgba(0, 0, 0, 0.18)) saturate(1.06);
}

:deep(.el-menu-item:not(.is-active):hover) {
  background: var(--clay-card) !important;
  box-shadow: var(--shadow-clay-inner);
  color: var(--clay-ink) !important;
  z-index: 2;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, var(--clay-pink), var(--clay-pink-2));
  color: #fff !important;
  font-weight: 800;
  box-shadow: var(--shadow-clay-btn);
  z-index: 3;
}

:deep(.el-menu-item.is-active:hover) {
  background: linear-gradient(135deg, var(--clay-pink), var(--clay-pink-2)) !important;
  box-shadow: var(--shadow-clay-btn);
  color: #fff !important;
  z-index: 3;
}

/* 滚动条样式 */
.menu::-webkit-scrollbar {
  width: 4px;
}
.menu::-webkit-scrollbar-thumb {
  background: var(--clay-border);
  border-radius: 2px;
}
.menu::-webkit-scrollbar-track {
  background: transparent;
}

.sidebar-divider {
  height: 1px;
  background-color: var(--clay-border);
  margin: 0 18px;
}

.footer-status {
  padding: 14px 18px;
  background: transparent;
  border-top: 1px solid var(--clay-border);
  font-size: 12px;
  color: var(--clay-text-secondary);
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
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, #fda4af, var(--clay-coral));
  box-shadow: 2px 2px 6px rgba(0, 0, 0, 0.08), inset 1px 1px 2px rgba(255, 255, 255, 0.6);
}

.dot.connected {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0) 60%),
    linear-gradient(135deg, var(--clay-mint), var(--clay-mint-2));
  box-shadow: 0 0 0 6px rgba(110, 231, 183, 0.18), 2px 2px 6px rgba(0, 0, 0, 0.08), inset 1px 1px 2px rgba(255, 255, 255, 0.65);
}

.version-info {
  margin-top: 6px;
  opacity: 0.75;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.version-line {
  display: flex;
  align-items: center;
  gap: 8px;
  line-height: 1.2;
}

.version-label {
  opacity: 0.9;
}

.version-value {
  color: var(--clay-ink);
  opacity: 0.9;
}

.update-tag {
  margin-left: 6px;
}
</style>
