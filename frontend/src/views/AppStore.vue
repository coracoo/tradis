<template>
  <div class="app-store-view">
    <div class="operation-bar">
      <div class="left-ops">
        <el-radio-group v-model="activeCategory" size="default">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="web">Web服务</el-radio-button>
          <el-radio-button label="database">数据库</el-radio-button>
          <el-radio-button label="tools">工具</el-radio-button>
          <el-radio-button label="storage">存储</el-radio-button>
        </el-radio-group>
      </div>
      <div class="right-ops">
        <el-input
          v-model="searchQuery"
          placeholder="搜索应用..."
          :prefix-icon="Search"
          clearable
          style="width: 250px"
        />
      </div>
    </div>

    <div class="app-content">
      <div class="app-grid">
        <el-card v-for="app in filteredApps" :key="app.id" class="app-card" shadow="hover">
          <div class="app-card-body">
            <div class="app-icon-wrapper">
              <img :src="app.icon" :alt="app.name" class="app-icon" @error="handleImageError">
            </div>
            <div class="app-info">
              <div class="app-header-row">
                <h3 class="app-name">{{ app.name }}</h3>
                <el-tag size="small" effect="plain">{{ app.version }}</el-tag>
              </div>
              <p class="app-desc" :title="app.description">{{ app.description }}</p>
            </div>
          </div>
          <div class="app-actions">
            <el-button type="primary" plain size="small" @click="handleDeploy(app)">
              <el-icon class="el-icon--left"><Download /></el-icon>安装
            </el-button>
            <el-button size="small" @click="showDetail(app)">
              <el-icon class="el-icon--left"><InfoFilled /></el-icon>详情
            </el-button>
          </div>
        </el-card>
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
          <img :src="currentApp.icon" :alt="currentApp.name" class="detail-icon" @error="handleImageError">
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
        
        <el-divider content-position="left">部署配置</el-divider>
        
        <el-form :model="deployConfig" label-width="80px" class="compact-form">
          <el-form-item label="端口映射">
            <el-input v-model="deployConfig.port" placeholder="宿主机端口:容器端口 (例如 8080:80)" />
          </el-form-item>
          <el-form-item label="环境变量">
            <el-input
              v-model="deployConfig.env"
              type="textarea"
              placeholder="KEY=VALUE (每行一个)"
              :rows="4"
            />
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="detailVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmDeploy">确认部署</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Search, Download, InfoFilled, PriceTag, Folder } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const activeCategory = ref('all')
const searchQuery = ref('')
const detailVisible = ref(false)
const currentApp = ref(null)
const deployConfig = ref({
  port: '',
  env: ''
})

const handleImageError = (e) => {
  // 设置默认图标
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

// 模拟应用数据
const apps = ref([
  {
    id: 1,
    name: 'Nginx',
    description: '高性能的 HTTP 和反向代理服务器，也可用作邮件代理服务器。',
    icon: 'https://www.nginx.com/wp-content/uploads/2020/05/nginx-plus-icon.svg',
    category: 'web',
    version: '1.24.0'
  },
  {
    id: 2,
    name: 'MySQL',
    description: '世界上最流行的开源关系型数据库管理系统。',
    icon: 'https://labs.mysql.com/common/logos/mysql-logo.svg',
    category: 'database',
    version: '8.0'
  },
  {
    id: 3,
    name: 'Redis',
    description: '开源的内存数据结构存储系统，用作数据库、缓存和消息代理。',
    icon: 'https://redis.io/images/redis-white.png',
    category: 'database',
    version: '7.2'
  },
  {
    id: 4,
    name: 'Portainer',
    description: '轻量级管理 UI，可让您轻松管理 Docker 环境。',
    icon: 'https://www.portainer.io/hubfs/Brand%20Assets/Logos/Portainer%20Logo%20Solid%20Blue.svg',
    category: 'tools',
    version: '2.19'
  },
  {
    id: 5,
    name: 'MinIO',
    description: '高性能对象存储，兼容 Amazon S3 API。',
    icon: 'https://min.io/resources/img/logo/minio_icon.png',
    category: 'storage',
    version: 'RELEASE.2024'
  }
])

const filteredApps = computed(() => {
  return apps.value.filter(app => {
    const matchCategory = activeCategory.value === 'all' || app.category === activeCategory.value
    const matchSearch = app.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
                       app.description.toLowerCase().includes(searchQuery.value.toLowerCase())
    return matchCategory && matchSearch
  })
})

const handleCategoryChange = (category) => {
  activeCategory.value = category
}

const showDetail = (app) => {
  currentApp.value = app
  detailVisible.value = true
  deployConfig.value = {
    port: '',
    env: ''
  }
}

const handleDeploy = (app) => {
  showDetail(app)
}

const confirmDeploy = () => {
  // 这里添加部署逻辑
  ElMessage.success(`${currentApp.value.name} 开始部署`)
  detailVisible.value = false
}
</script>

<style scoped>
.app-store-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.operation-bar {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.app-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  padding-bottom: 20px;
}

.app-card {
  transition: all 0.3s;
  border: 1px solid var(--el-border-color-lighter);
}

.app-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.app-card-body {
  display: flex;
  gap: 16px;
  margin-bottom: 16px;
}

.app-icon-wrapper {
  flex-shrink: 0;
  width: 56px;
  height: 56px;
  border-radius: 8px;
  background-color: var(--el-fill-color-light);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px;
  border: 1px solid var(--el-border-color-lighter);
}

.app-icon {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.app-info {
  flex: 1;
  min-width: 0;
}

.app-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.app-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-desc {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  line-height: 1.5;
}

.app-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 12px;
}

/* 详情对话框样式 */
.app-detail-header {
  display: flex;
  gap: 20px;
  margin-bottom: 24px;
}

.detail-icon {
  width: 80px;
  height: 80px;
  object-fit: contain;
  padding: 10px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background-color: var(--el-fill-color-light);
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
  gap: 16px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  background-color: var(--el-fill-color-light);
  padding: 4px 8px;
  border-radius: 4px;
}

:deep(.el-card__body) {
  padding: 16px;
}
</style>