<template>
  <div class="app-store-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索应用..."
          clearable
          class="search-input"
          size="medium"
          @keyup.enter="refreshApps"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        
        <el-radio-group v-model="activeCategory" class="category-filter" size="medium">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="entertainment">影音</el-radio-button>
          <el-radio-button label="photos">图像</el-radio-button>
          <el-radio-button label="file">文件</el-radio-button>
          <el-radio-button label="hobby">个人</el-radio-button>
          <el-radio-button label="team">协作</el-radio-button>
          <el-radio-button label="knowledge">知识库</el-radio-button>
          <el-radio-button label="game">游戏</el-radio-button>
          <el-radio-button label="productivity">生产</el-radio-button>    
          <el-radio-button label="social">社交</el-radio-button>
          <el-radio-button label="platform">管理</el-radio-button>
          <el-radio-button label="network">网安</el-radio-button>
          <el-radio-button label="other">其他</el-radio-button>
        </el-radio-group>
      </div>

      <div class="filter-right">
         <el-button @click="refreshApps" :loading="loading" plain size="medium">
           <template #icon><el-icon><Refresh /></el-icon></template>
           刷新
         </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <div v-loading="loading" class="scroll-container">
        <div v-if="filteredApps.length > 0" class="app-grid">
          <el-card v-for="app in filteredApps" :key="app.id" class="app-card" shadow="hover">
            <div class="app-card-body">
              <div class="app-icon-wrapper">
                <img :src="resolvePicUrl(app.logo || app.icon)" :alt="app.name" class="app-icon" @error="handleImageError">
              </div>
              <div class="app-info">
                <div class="app-header-row">
                  <h3 class="app-name" :title="app.name">{{ app.name }}</h3>
                  <el-tag size="small" effect="plain">{{ app.version }}</el-tag>
                  <el-tag v-if="isInstalled(app)" type="success" size="small" effect="dark" style="margin-left: 5px">已安装</el-tag>
                </div>
                <p class="app-desc" :title="app.description">{{ app.description }}</p>
              </div>
            </div>
            <div class="app-actions">
              <el-button :type="isInstalled(app) ? 'warning' : 'primary'" plain size="small" @click="handleDeploy(app)">
                <el-icon class="el-icon--left"><Download /></el-icon>{{ isInstalled(app) ? '新安装' : '安装' }}
              </el-button>
              <el-button size="small" @click="showDetail(app)">
                <el-icon class="el-icon--left"><InfoFilled /></el-icon>详情
              </el-button>
            </div>
          </el-card>
        </div>
        <el-empty v-else description="未找到相关应用" />
      </div>
    </div>

    <!-- 应用详情对话框 -->
    <el-dialog
      v-model="detailVisible"
      :title="currentApp?.name"
      width="600px"
      append-to-body
      class="app-detail-dialog"
    >
      <template v-if="currentApp">
        <div class="app-detail-header">
          <img :src="resolvePicUrl(currentApp.logo || currentApp.icon)" :alt="currentApp.name" class="detail-icon" @error="handleImageError">
          <div class="detail-info">
            <p class="detail-desc">{{ currentApp.description }}</p>
            <div class="detail-meta">
              <span class="meta-item">
                <el-icon><PriceTag /></el-icon>
                {{ currentApp.version }}
              </span>
              <span class="meta-item">
                <el-icon><Folder /></el-icon>
                {{ getCategoryLabel(currentApp.category) }}
              </span>
            </div>
          </div>
        </div>
        <div class="app-readme">
          <h4>应用简介</h4>
          <p>{{ currentApp.description }}</p>
        </div>
      </template>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button type="primary" @click="confirmDeploy">去部署</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Download, InfoFilled, PriceTag, Folder, Refresh } from '@element-plus/icons-vue'
import api from '../api'
import request from '../utils/request'

const router = useRouter()
const activeCategory = ref('all')
const searchQuery = ref('')
const detailVisible = ref(false)
const currentApp = ref(null)
const apps = ref([])
const installedProjects = ref([])
const loading = ref(false)
const appStoreBase = ref('')

const CACHE_KEY = 'appstore_projects'
const CACHE_TIME_KEY = 'appstore_cache_time'
const CACHE_DURATION = 5 * 60 * 1000 // 5分钟

const isInstalled = (app) => {
  return installedProjects.value.some(p => p.name === app.name)
}

const handleImageError = (e) => {
  e.target.src = 'https://cdn-icons-png.flaticon.com/512/873/873133.png'
}

const getCategoryLabel = (category) => {
  const map = {
    web: 'Web服务',
    database: '数据库',
    tools: '工具',
    storage: '存储'
  }
  return map[category] || category
}

const filteredApps = computed(() => {
  return apps.value.filter(app => {
    const matchCategory = activeCategory.value === 'all' || app.category === activeCategory.value
    const matchSearch = app.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
                       app.description.toLowerCase().includes(searchQuery.value.toLowerCase())
    return matchCategory && matchSearch
  })
})

const fetchApps = async (force = false) => {
  loading.value = true
  try {
    // 检查缓存
    if (!force) {
      const cachedData = localStorage.getItem(CACHE_KEY)
      const cachedTime = localStorage.getItem(CACHE_TIME_KEY)
      
      if (cachedData && cachedTime) {
        const now = Date.now()
        if (now - parseInt(cachedTime) < CACHE_DURATION) {
          console.log('Using cached apps data')
          apps.value = JSON.parse(cachedData)
          loading.value = false
          // 不 return，继续后台拉取更新 (SWR)
        }
      }
    }

    // 调用API
    console.log('Fetching apps from API...')
    const res = await api.appstore.getProjects()
    // 兼容直接返回数组或带有 data 字段的响应结构
    const data = Array.isArray(res) ? res : (res.data || [])
    if (data) {
      apps.value = data
      // 更新缓存
      localStorage.setItem(CACHE_KEY, JSON.stringify(apps.value))
      localStorage.setItem(CACHE_TIME_KEY, Date.now().toString())
    }
  } catch (error) {
    console.error('Failed to fetch apps:', error)
  } finally {
    loading.value = false
  }
}

const refreshApps = () => {
  fetchApps(true)
}

const showDetail = (app) => {
  currentApp.value = app
  detailVisible.value = true
}

const handleDeploy = (app) => {
  router.push(`/appstore/deploy/${app.id}`)
}

const confirmDeploy = () => {
  if (currentApp.value) {
    handleDeploy(currentApp.value)
    detailVisible.value = false
  }
}

const fetchInstalledProjects = async () => {
  try {
    const res = await api.compose.list()
    installedProjects.value = res.data || res
  } catch (error) {
    console.error('Failed to fetch installed projects:', error)
  }
}

onMounted(() => {
  initAppStoreBase().then(() => fetchApps())
  fetchInstalledProjects()
})

const initAppStoreBase = async () => {
  try {
    const s = await request.get('/settings/global')
    appStoreBase.value = (s && s.appStoreServerUrl) ? s.appStoreServerUrl.replace(/\/$/, '') : 'https://template.cgakki.top:33333'
  } catch (e) {
    appStoreBase.value = 'https://template.cgakki.top:33333'
  }
}

const resolvePicUrl = (u) => {
  if (!u) return ''
  if (u.startsWith('http://') || u.startsWith('https://')) return u
  if (u.startsWith('/')) return appStoreBase.value + u
  return appStoreBase.value + '/' + u
}
</script>

<style scoped>
.app-store-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 24px;
}

/* Filter Bar - Same as Compose.vue */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  background: var(--el-bg-color);
  padding: 12px 20px;
  border-radius: 12px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);
}

.filter-left, .filter-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.search-input {
  width: 300px;
}

/* Content Wrapper - Same as Compose.vue table-wrapper */
.content-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

/* App Grid */
.app-grid {
  display: grid;
  grid-template-columns: repeat(1, 1fr);
  gap: 20px;
}

@media (min-width: 768px) {
  .app-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 1200px) {
  .app-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (min-width: 1600px) {
  .app-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}

/* App Card */
.app-card {
  transition: all 0.3s;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-lighter); /* Lighter border */
  border-radius: 8px;
  background: var(--el-bg-color-overlay);
}

.app-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 10px 20px rgba(0,0,0,0.08);
  border-color: var(--el-color-primary-light-5);
}

.app-card-body {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
}

.app-icon-wrapper {
  width: 60px;
  height: 60px;
  flex-shrink: 0;
  border-radius: 12px;
  overflow: hidden;
  background: var(--el-fill-color-light);
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--el-border-color-lighter);
}

.app-icon {
  width: 80%;
  height: 80%;
  object-fit: contain;
}

.app-info {
  flex: 1;
  overflow: hidden;
}

.app-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.app-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  margin-right: 8px;
}

.app-desc {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  line-height: 1.5;
}

.app-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 15px;
  margin-top: auto;
}

/* Dialog Styles */
.app-detail-header {
  display: flex;
  gap: 20px;
  margin-bottom: 24px;
}

.detail-icon {
  width: 80px;
  height: 80px;
  border-radius: 16px;
  background: var(--el-fill-color-light);
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
}

.detail-info {
  flex: 1;
}

.detail-desc {
  font-size: 14px;
  color: var(--el-text-color-regular);
  line-height: 1.6;
  margin-bottom: 12px;
}

.detail-meta {
  display: flex;
  gap: 20px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
  padding: 4px 10px;
  border-radius: 6px;
}

.app-readme h4 {
  font-size: 16px;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.app-readme p {
  font-size: 14px;
  color: var(--el-text-color-regular);
  line-height: 1.6;
}

:deep(.el-button--medium) {
  padding: 10px 20px;
  height: 36px;
}

.more-btn {
  padding: 10px 16px;
  display: flex;
  align-items: center;
}

</style>
