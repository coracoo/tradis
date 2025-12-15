<template>
  <div class="app-store-view">
    <div class="operation-bar">
      <div class="left-ops">
        <el-radio-group v-model="activeCategory" size="default">
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
      <div class="right-ops">
         <el-button @click="refreshApps"><el-icon><Refresh /></el-icon></el-button>
         <el-input
          v-model="searchQuery"
          placeholder="搜索应用..."
          :prefix-icon="Search"
          clearable
          style="width: 250px"
        />
      </div>
    </div>

    <div v-loading="loading" class="app-content">
      <div v-if="filteredApps.length > 0" class="app-grid">
        <el-card v-for="app in filteredApps" :key="app.id" class="app-card" shadow="hover">
          <div class="app-card-body">
            <div class="app-icon-wrapper">
              <img :src="resolvePicUrl(app.logo || app.icon)" :alt="app.name" class="app-icon" @error="handleImageError">
            </div>
            <div class="app-info">
              <div class="app-header-row">
                <h3 class="app-name">{{ app.name }}</h3>
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
  padding: 0 16px 16px 16px;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.operation-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 10px;
}
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
    grid-template-columns: repeat(4, 1fr);
  }
}

.app-card {
  transition: all 0.3s;
  display: flex;
  flex-direction: column;
}

.app-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 8px 16px rgba(0,0,0,0.1);
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
  border-radius: 10px;
  overflow: hidden;
  background: #f5f7fa;
  display: flex;
  align-items: center;
  justify-content: center;
}

.app-icon {
  width: 100%;
  height: 100%;
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
  margin-bottom: 5px;
}

.app-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-desc {
  margin: 0;
  font-size: 12px;
  color: #666;
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
  border-top: 1px solid #ebeef5;
  padding-top: 15px;
}

.app-detail-header {
  display: flex;
  gap: 20px;
  margin-bottom: 20px;
}

.detail-icon {
  width: 80px;
  height: 80px;
  border-radius: 12px;
  background: #f5f7fa;
  padding: 10px;
}

.detail-info {
  flex: 1;
}

.detail-desc {
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
  margin-bottom: 10px;
}

.detail-meta {
  display: flex;
  gap: 20px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  color: #909399;
}
</style>
