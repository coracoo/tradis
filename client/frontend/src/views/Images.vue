<template>
  <div class="images-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索镜像名称、标签 or ID..."
          clearable
          class="search-input"
          size="medium"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <div class="filter-right">
        <el-button-group class="main-actions">
          <el-button @click="fetchImages" :loading="loading" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
          <el-button @click="manualCheckUpdates" :loading="checkingUpdates" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            检测更新
          </el-button>
          <el-button type="primary" @click="pullImage" size="medium">
            <template #icon><el-icon><Download /></el-icon></template>
            拉取镜像
          </el-button>
          <el-button @click="importImage" plain size="medium">
            <template #icon><el-icon><Upload /></el-icon></template>
            导入
          </el-button>
        </el-button-group>

        <el-dropdown trigger="click" @command="handleGlobalAction">
          <el-button plain class="more-btn" size="medium">
            更多操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="bulk-update" :icon="Download">
                一键更新未使用镜像
              </el-dropdown-item>
              <el-dropdown-item command="settings" :icon="Setting">配置镜像加速</el-dropdown-item>
              <el-dropdown-item command="prune" :icon="Delete" divided class="text-danger">清除未使用镜像</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table 
        :data="paginatedImages" 
        style="width: 100%; height: 100%" 
        v-loading="loading"
        @sort-change="handleSortChange"
        :default-sort="{ prop: 'RepoTags', order: 'ascending' }"
        class="main-table"
        :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: '60px' }"
      >
        <el-table-column type="selection" width="40" align="center" />
        
        <el-table-column label="镜像名称" prop="RepoTags" min-width="200" sortable="custom" show-overflow-tooltip>
          <template #default="scope">
            <div class="image-name-cell">
              <div class="icon-wrapper image">
                <el-icon><Files /></el-icon>
              </div>
              <div class="name-info">
                <span class="name-text">{{ getImageName(scope.row.RepoTags?.[0]) }}</span>
                <span class="id-text">ID: {{ scope.row.Id.substring(7, 19) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="标签" prop="Tag" min-width="160" sortable="custom" >
          <template #default="scope">
            <div class="tag-cell">
              <el-tag size="small" effect="light" class="image-tag">
                {{ getImageTag(scope.row.RepoTags?.[0]) }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        
        <el-table-column label="可更新" prop="Updatable" min-width="140" sortable="custom">
          <template #default="scope">
            <div class="tag-cell">
              <el-button
                v-if="isImageUpdatable(scope.row)"
                type="primary"
                text
                size="small"
                class="update-link"
                :loading="isImageUpdating(scope.row)"
                :disabled="isImageUpdating(scope.row)"
                @click="updateImage(scope.row)"
              >
                点击更新
              </el-button>
              <span v-else class="text-gray">-</span>
            </div>
          </template>
        </el-table-column>
        
        <el-table-column label="大小" prop="Size" min-width="120" sortable="custom">
          <template #default="scope">
            <span class="text-gray">{{ formatSize(scope.row.Size) }}</span>
          </template>
        </el-table-column>
        
        <el-table-column label="创建时间" prop="Created" min-width="140" sortable="custom">
          <template #default="scope">
            <div class="text-gray font-mono">
              {{ formatTimeTwoLines(scope.row.Created) }}
            </div>
          </template>
        </el-table-column>
        
        <el-table-column label="使用状态" prop="isInUse" min-width="120" sortable="custom">
          <template #default="scope">
            <div class="status-indicator">
              <span class="status-point" :class="scope.row.isInUse ? 'running' : 'stopped'"></span>
              <span>{{ scope.row.isInUse ? '使用中' : '未使用' }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="220" fixed="left" align="center">
          <template #default="scope">
            <div class="row-ops">
              <el-tooltip content="修改标签" placement="top">
                <el-button 
                  circle 
                  plain
                  type="primary" 
                  :disabled="scope.row.isInUse"
                  @click="tagImage(scope.row)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="导出" placement="top">
                <el-button 
                  circle 
                  plain
                  type="info" 
                  @click="exportImage(scope.row)">
                  <el-icon><Download /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button 
                  circle 
                  plain
                  type="danger" 
                  :disabled="scope.row.isInUse"
                  @click="deleteImage(scope.row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>
	
    <!-- 镜像加速 -->
    <DockerSettings v-model="settingsVisible" />

    <!-- 添加拉取镜像对话框 -->
    <el-dialog
      v-model="pullDialogVisible"
      title="拉取镜像"
      width="500px"
      destroy-on-close
      class="compact-dialog"
    >
      <el-form :model="pullForm" label-position="top">
        <el-form-item label="镜像源 / 名称">
          <div style="display: flex; gap: 8px; width: 100%;">
            <el-select
              v-model="pullForm.registry"
              placeholder="源"
              style="width: 140px"
            >
              <el-option
                v-for="option in registryOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
            <el-input
              v-model="pullForm.name"
              placeholder="例如: nginx:latest"
              style="flex: 1"
              @keyup.enter="handlePullImage"
            />
          </div>
        </el-form-item>
      </el-form>
      
      <!-- 进度条显示 -->
      <div v-if="pullProgress.show" class="pull-progress">
        <div class="progress-header">
          <div class="progress-status">{{ pullProgress.status }}</div>
          <el-progress :percentage="pullProgress.progress" :status="pullProgress.status === 'success' ? 'success' : ''" />
        </div>
        <div class="progress-details" v-if="pullProgress.details.length">
          <div v-for="(detail, index) in pullProgress.details" :key="index" class="detail-item">
            <span class="detail-id">{{ detail.id }}</span>
            <span class="detail-status">{{ detail.status }}</span>
          </div>
        </div>
      </div>
      
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="pullDialogVisible = false" :disabled="pullProgress.show">取消</el-button>
          <el-button type="primary" @click="handlePullImage" :loading="pullProgress.show">开始拉取</el-button>
        </span>
      </template>
    </el-dialog>
	
	<!-- 修改标签对话框 -->
    <el-dialog
      v-model="tagDialogVisible"
      title="修改标签"
      width="400px"
      destroy-on-close
      class="compact-dialog"
    >
      <el-form :model="tagForm" label-position="top">
        <el-form-item label="当前标签">
          <el-tag type="info">{{ tagForm.currentTag }}</el-tag>
        </el-form-item>
        <el-form-item label="新仓库名">
          <el-input v-model="tagForm.repository" placeholder="例如: myrepo/myimage"></el-input>
        </el-form-item>
        <el-form-item label="新标签">
          <el-input v-model="tagForm.tag" placeholder="例如: latest"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="tagDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleTagImage">确定</el-button>
        </span>
      </template>
    </el-dialog>
	
	<!-- 导入镜像对话框 -->
  <el-dialog
      v-model="importDialogVisible"
      title="导入镜像"
      width="400px"
      destroy-on-close
      class="compact-dialog"
    >
      <el-upload
        class="upload-demo"
        drag
        action="/api/images/import"
        :headers="uploadHeaders"
        :on-progress="handleImportProgress"
        :on-success="handleImportSuccess"
        :on-error="handleImportError"
        :before-upload="beforeImportUpload"
        :show-file-list="false"
		accept=".tar"
	  >
		<el-icon class="el-icon--upload"><upload-filled /></el-icon>
		<div class="el-upload__text">
		  拖拽文件到此处 或 <em>点击上传</em>
		</div>
        <template #tip>
          <div class="el-upload__tip text-center">支持 .tar 格式，最大 10GB</div>
        </template>
	  </el-upload>
	  
	  <div v-if="importProgress.show" class="mt-4">
          <div class="mb-2 text-sm">{{ importProgress.status === 'success' ? '上传完成' : '上传中...' }}</div>
          <el-progress 
            :percentage="importProgress.percent" 
            :status="importProgress.status"
          ></el-progress>
      </div>
	</el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, h, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UploadFilled, Download, Upload, Setting, Edit, Delete, Search, ArrowDown, Files } from '@element-plus/icons-vue'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
import DockerSettings from '../components/DockerSettings.vue'
import request from '../utils/request'
import { useSseLogStream } from '../utils/sseLogStream'

import { getRegistries } from '../api/image_registry'

const loading = ref(false)
const images = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const searchQuery = ref('') // 添加搜索关键词
const sortState = ref({ prop: '', order: '' })

// 添加计算属性处理搜索和分页
const filteredImages = computed(() => {
  let result = images.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(image => {
      const name = getImageName(image.RepoTags?.[0]).toLowerCase()
      const tag = getImageTag(image.RepoTags?.[0]).toLowerCase()
      const id = image.Id.toLowerCase()
      return name.includes(query) || tag.includes(query) || id.includes(query)
    })
  }
  
  return result
})

const paginatedImages = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredImages.value.slice(start, end)
})

// 监听 filteredImages 变化更新总数
watch(filteredImages, (newVal) => {
  total.value = newVal.length
  // 如果当前页超过总页数，重置为第一页
  if (currentPage.value > Math.ceil(total.value / pageSize.value)) {
    currentPage.value = 1
  }
})

// Global Action Handler
const handleGlobalAction = (command) => {
  if (command === 'settings') {
    settingsVisible.value = true
  } else if (command === 'prune') {
    clearImages()
  } else if (command === 'bulk-update') {
    handleBulkUpdate()
  }
}

// 添加处理镜像名称和标签的函数
const getImageName = (repoTag) => {
  if (!repoTag) return '<none>'
  const parts = repoTag.split(':')
  return parts[0]
}

const getImageTag = (repoTag) => {
  if (!repoTag) return '<none>'
  const parts = repoTag.split(':')
  return parts[1] || 'latest'
}

const normalizeImageTag = (repoTag) => {
  return repoTag || ''
}

// 删除 proxyDialogVisible 和 proxyForm 相关代码
const settingsVisible = ref(false)

const updateStatusMap = ref({})
const updatingMap = ref({})
const bulkUpdating = ref(false)
const checkingUpdates = ref(false)
let updateTimer = null

const normalizeUpdateStatusMap = (raw) => {
  const normalized = {}
  if (!raw) {
    return normalized
  }
  if (Array.isArray(raw)) {
    raw.forEach((tag) => {
      if (typeof tag === 'string') {
        const t = normalizeImageTag(tag)
        if (t && t !== '<none>:<none>') {
          normalized[t] = true
        }
      }
    })
    return normalized
  }
  if (typeof raw === 'object') {
    Object.keys(raw).forEach((key) => {
      const value = raw[key]
      const t = normalizeImageTag(key)
      if (!t || t === '<none>:<none>') {
        return
      }
      if (typeof value === 'boolean') {
        if (value) {
          normalized[t] = true
        }
      } else if (value) {
        normalized[t] = true
      }
    })
  }
  return normalized
}

const clearImages = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要清除所有未使用的镜像吗？这将删除所有未被容器引用的镜像，此操作不可恢复。',
      '警告',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    loading.value = true
    const res = await api.images.prune()
    
    let msg = '清理完成'
    if (res && res.report) {
      const deletedCount = res.report.ImagesDeleted ? res.report.ImagesDeleted.length : 0
      const spaceReclaimed = res.report.SpaceReclaimed || 0
      msg = `清理完成，删除了 ${deletedCount} 个镜像，释放了 ${formatSize(spaceReclaimed)} 空间`
    }
    
    ElMessage.success(msg)
    await fetchImages()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('清理镜像失败:', error)
      ElMessage.error('清理镜像失败: ' + (error.message || '未知错误'))
    }
  } finally {
    loading.value = false
  }
}

// 格式化文件大小
const formatSize = (size) => {
  if (!size) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(size) / Math.log(k))
  return (size / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

const fetchImages = async () => {
  loading.value = true
  try {
    const imagesData = await api.images.list()
    const containersData = await api.containers.list()
    
    const usedImageTags = new Set()
    const usedImageIds = new Set()

    if (containersData && Array.isArray(containersData)) {
      containersData.forEach(container => {
        if (container) {
          if (container.Image) {
            const imageName = container.Image
            if (imageName.includes(':')) {
              usedImageTags.add(imageName)
            } else {
              usedImageTags.add(`${imageName}:latest`)
            }
          }
          if (container.ImageID) {
            usedImageIds.add(container.ImageID)
          }
        }
      })
    }
    
    const processedImages = []
    if (imagesData && Array.isArray(imagesData)) {
      imagesData.forEach(image => {
        const fullId = image.Id || ''
        const usedById = fullId && usedImageIds.has(fullId)

        if (!image.RepoTags || image.RepoTags.length === 0 || (image.RepoTags.length === 1 && image.RepoTags[0] === '<none>:<none>')) {
          processedImages.push({
            ...image,
            RepoTags: ['<none>:<none>'],
            isInUse: !!usedById
          })
        } else {
          image.RepoTags.forEach(tag => {
            const usedByTag = usedImageTags.has(tag)
            processedImages.push({
              ...image,
              RepoTags: [tag],
              isInUse: !!usedById || usedByTag
            })
          })
        }
      })
    }
    
    images.value = processedImages
    total.value = processedImages.length
    const { prop, order } = sortState.value
    if (prop && order) {
      handleSortChange({ prop, order })
    } else {
      handleSortChange({ prop: 'RepoTags', order: 'ascending' })
    }
  } catch (error) {
    console.error('获取镜像列表错误:', error)
    ElMessage.error('获取镜像列表失败')
    images.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 排序处理
const handleSortChange = ({ prop, order }) => {
  if (!prop || !order) {
    images.value = [...images.value]
    return
  }
  sortState.value = { prop, order }
  try {
    const v = JSON.stringify(sortState.value)
    request.post('/settings/kv/sort_images', { value: v })
  } catch (e) {}

  images.value.sort((a, b) => {
    let aValue, bValue

    switch (prop) {
      case 'Id':
        aValue = a.Id
        bValue = b.Id
        break
      case 'isInUse':
        aValue = a.isInUse ? 1 : 0
        bValue = b.isInUse ? 1 : 0
        break
      case 'Updatable':
        aValue = isImageUpdatable(a) ? 1 : 0
        bValue = isImageUpdatable(b) ? 1 : 0
        break
      case 'RepoTags':
        // 只比较镜像名称部分
        aValue = getImageName(a.RepoTags?.[0] || '')
        bValue = getImageName(b.RepoTags?.[0] || '')
        break
      case 'Tag':
        aValue = getImageTag(a.RepoTags?.[0] || '')
        bValue = getImageTag(b.RepoTags?.[0] || '')
        break
      case 'Size':
        aValue = a.Size
        bValue = b.Size
        break
      case 'Created':
        aValue = a.Created
        bValue = b.Created
        break
      default:
        aValue = a[prop]
        bValue = b[prop]
    }

    return order === 'ascending' ? 
      (aValue > bValue ? 1 : -1) : 
      (aValue < bValue ? 1 : -1)
  })
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

// 拉取镜像
// 修改拉取镜像的对话框
// 添加拉取镜像相关的响应式变量
const pullDialogVisible = ref(false)
const pullForm = ref({
  registry: 'docker.io',
  name: ''
})
const registryOptions = ref([
  {
    label: 'Docker Hub',
    value: 'docker.io'
  }
])

// 修改拉取镜像函数
const pullImage = async () => {
  try {
    // 获取注册表列表
    const registriesData = await getRegistries()
    const registries = registriesData || {}
    
    // 更新注册表选项，避免重复的 Docker Hub
    registryOptions.value = Object.entries(registries)
      .filter(([key]) => key !== 'docker.io')  // 过滤掉可能存在的 docker.io
      .map(([key, registry]) => ({
        label: registry.name,
        value: key
      }))
    
    // 确保 Docker Hub 始终在第一位
    registryOptions.value.unshift({
      label: 'Docker Hub',
      value: 'docker.io'
    })

    // 重置表单并显示对话框
    pullForm.value = {
      registry: 'docker.io',
      name: ''
    }
    pullDialogVisible.value = true
  } catch (error) {
    ElMessage.error('加载注册表失败：' + (error.message || '未知错误'))
  }
}

// 添加进度相关的响应式变量
const pullProgress = ref({
  show: false,
  status: '',
  progress: 0,
  details: []
})

const {
  start: startPullStream,
  stop: stopPullStream
} = useSseLogStream({
  onOpenLine: '',
  onErrorLine: '',
  onMessage: (event, { stop }) => {
    let data = null
    try {
      data = JSON.parse(event.data)
    } catch (e) {
      return
    }

    if (data && data.type === 'done') {
      stop()
      pullProgress.value.show = false
      pullProgress.value.status = '拉取完成'
      pullProgress.value.progress = 100
      ElMessage.success('镜像拉取成功')
      pullDialogVisible.value = false
      fetchImages()
      return
    }

    if (data && data.error) {
      stop()
      pullProgress.value.show = false
      pullProgress.value.status = '拉取失败'
      const raw = String(data.errorDetail?.message || data.error || '拉取失败')
      let msg = raw
      const lower = raw.toLowerCase()
      if (lower.includes('connection reset by peer') || lower.includes('timeout')) {
        msg = '连接到镜像仓库失败，可能是网络问题或代理设置有误。请检查 Docker 的网络设置。'
      } else if (lower.includes('not found')) {
        msg = '镜像未找到，请检查镜像名称是否正确。'
      } else if (lower.includes('unauthorized')) {
        msg = '认证失败，请检查仓库的用户名和密码设置。'
      } else {
        msg = `拉取失败: ${raw}`
      }
      ElMessage.error(msg)
      return
    }

    if (data.status) {
      pullProgress.value.status = data.status
    }

    if (data.progressDetail && data.progressDetail.current && data.progressDetail.total) {
      const current = Number(data.progressDetail.current || 0)
      const total = Number(data.progressDetail.total || 0)
      if (total > 0) {
        pullProgress.value.progress = Math.max(0, Math.min(100, Math.round((current / total) * 100)))
      }
    }

    if (data.id) {
      const existingDetail = pullProgress.value.details.find(d => d.id === data.id)
      if (existingDetail) {
        existingDetail.status = data.status || existingDetail.status
        existingDetail.progress = data.progress || existingDetail.progress
      } else {
        pullProgress.value.details.unshift({
          id: data.id,
          status: data.status || '',
          progress: data.progress || ''
        })
      }
    }
  },
  onError: ({ stop }) => {
    if (!pullProgress.value.show) return
    stop()
    pullProgress.value.show = false
    pullProgress.value.status = '连接中断'
    ElMessage.error('镜像拉取进度连接中断，请重试')
  }
})

watch(pullDialogVisible, (visible) => {
  if (!visible) {
    stopPullStream()
    pullProgress.value.show = false
  }
})

// 添加导入镜像相关变量
const importDialogVisible = ref(false)
const importProgress = ref({
  show: false,
  percent: 0,
  status: '',
  uploading: false
})
const uploadHeaders = computed(() => {
  const token = localStorage.getItem('token') || ''
  return token ? { Authorization: `Bearer ${token}` } : {}
})

// 导入镜像
const importImage = () => {
  importDialogVisible.value = true
  importProgress.value = {
    show: false,
    uploading: false,
    percent: 0,
    status: ''
  }
}

// 上传前检查
const beforeImportUpload = (file) => {
  if (!file) {
    return false
  }
  
  // 检查文件类型
  if (!file.name.endsWith('.tar')) {
    ElMessage.error('只能上传 .tar 格式的镜像文件')
    return false
  }
  
  // 检查文件大小，限制为 10GB
  const maxSize = 10 * 1024 * 1024 * 1024
  if (file.size > maxSize) {
    ElMessage.error('文件大小不能超过 10GB')
    return false
  }
  
  return true
}

// 处理上传进度
const handleImportProgress = (event, file) => {
  if (!file) {
    return
  }
  
  importProgress.value.show = true
  importProgress.value.uploading = true
  importProgress.value.percent = Math.round(event.percent)
}

// 处理上传成功
const handleImportSuccess = (response) => {
  importProgress.value.uploading = false
  importProgress.value.status = 'success'
  importProgress.value.percent = 100
  
  // 打印响应信息，用于调试
  console.log('导入镜像响应:', response)
  
  // 检查响应中是否包含镜像信息
  if (response && response.imageInfo) {
    const imageInfo = response.imageInfo
    ElMessage.success(`镜像导入成功: ${imageInfo.repoTags ? imageInfo.repoTags.join(', ') : imageInfo.id}`)
  } else {
    ElMessage.success('镜像导入成功')
  }
  
  importDialogVisible.value = false
  // 延迟一下再刷新镜像列表，确保后端处理完成
  setTimeout(() => {
    fetchImages() // 刷新镜像列表
  }, 500)
}

// 处理上传错误
const handleImportError = (error) => {
  // 检查是否是用户取消上传
  if (error && error.status === 0) {
    // 用户可能取消了文件选择对话框，不显示错误
    importProgress.value.show = false
    importProgress.value.uploading = false
    return
  }
  
  importProgress.value.uploading = false
  importProgress.value.status = 'exception'
  console.error('导入镜像失败:', error)
  ElMessage.error('导入镜像失败: ' + (error.message || '未知错误'))
}

const handlePullImage = async () => {
  if (!pullForm.value.name) {
    ElMessage.warning('请输入镜像名称')
    return
  }

  pullProgress.value = {
    show: true,
    status: '准备拉取镜像...',
    progress: 0,
    details: []
  }

  const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
  const token = localStorage.getItem('token')
  const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
  const url = `${baseUrl}/api/images/pull/progress?name=${encodeURIComponent(pullForm.value.name)}&registry=${encodeURIComponent(pullForm.value.registry)}${tokenParam}`

  startPullStream(url, { reset: true })
}

// 导出镜像
const exportImage = async (image) => {
  try {
    const token = localStorage.getItem('token') || ''
    const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
    const id = image.Id
    const url = `${baseUrl}/api/images/export/${encodeURIComponent(id)}${token ? `?token=${encodeURIComponent(token)}` : ''}`
    const a = document.createElement('a')
    a.href = url
    a.rel = 'noopener'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    ElMessage.success('开始导出镜像')
  } catch (error) {
    console.error('导出失败:', error)
    ElMessage.error('导出失败: ' + (error.message || '未知错误'))
  }
}

const tagDialogVisible = ref(false)
const tagForm = ref({
  imageId: '',
  currentTag: '',
  repository: '',
  tag: ''
})

const tagImage = (image) => {
  if (image.isInUse) {
    ElMessage.warning('该镜像正在被容器使用，无法修改标签')
    return
  }
  
  const repoTag = image.RepoTags?.[0] || ''
  const [repo, tag] = repoTag.split(':')
  
  tagForm.value = {
    imageId: image.Id,
    currentTag: repoTag,
    repository: repo,
    tag: tag || 'latest'
  }
  
  tagDialogVisible.value = true
}

const handleTagImage = async () => {
  if (!tagForm.value.repository) {
    ElMessage.warning('请输入仓库名')
    return
  }
  
  if (!tagForm.value.tag) {
    ElMessage.warning('请输入标签名')
    return
  }
  
  try {
    await api.images.tag({
      id: tagForm.value.imageId,
      repo: tagForm.value.repository,
      tag: tagForm.value.tag
    })
    
    // 如果新标签与原标签不同，且原标签不是 <none>，则询问是否删除原标签
    const newTag = `${tagForm.value.repository}:${tagForm.value.tag}`
    if (tagForm.value.currentTag !== newTag && tagForm.value.currentTag !== '<none>:<none>') {
      try {
        const result = await ElMessageBox.confirm(
          `是否删除原标签 "${tagForm.value.currentTag}"？\n(镜像本身不会被删除，只会移除标签引用)`,
          '提示',
          {
            confirmButtonText: '删除标签',
            cancelButtonText: '保留标签',
            type: 'info'
          }
        )
        
        await api.images.remove({
          id: tagForm.value.imageId,
          repoTag: tagForm.value.currentTag
        })
        ElMessage.success('已删除原标签')
      } catch (e) {
        // 用户选择保留原标签
        ElMessage.info('已保留原标签')
      }
    }
    
    ElMessage.success('镜像标签修改成功')
    tagDialogVisible.value = false
    fetchImages()
  } catch (error) {
    console.error('修改标签错误详情:', error)
    ElMessage.error('修改标签失败: ' + (error.message || '未知错误'))
  }
}

const deleteImage = async (image) => {
  if (image.isInUse) {
    ElMessage.warning('该镜像正在被容器使用，无法删除')
    return
  }
  
  try {
    await ElMessageBox.confirm('确定要删除该镜像吗？', '警告', {
      type: 'warning'
    })
    const repoTag = image.RepoTags?.[0] || ''
    const payload = repoTag && repoTag !== '<none>:<none>'
      ? { id: image.Id, repoTag }
      : image.Id
    await api.images.remove(payload)
    ElMessage.success('镜像已删除')
    fetchImages()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.message || '未知错误'))
    }
  }
}

const isImageUpdatable = (image) => {
  const originalTag = image.RepoTags?.[0] || ''
  if (!originalTag || originalTag === '<none>:<none>') {
    return false
  }
  if (updateStatusMap.value[originalTag]) {
    return true
  }
  return false
}

const isImageUpdating = (image) => {
  const tag = image.RepoTags?.[0] || ''
  if (!tag) {
    return false
  }
  return !!updatingMap.value[tag]
}

const pushNotification = (type, message) => {
  const tempId = `${Date.now()}-${Math.random().toString(16).slice(2)}`
  const time = new Date().toLocaleTimeString()
  try {
    window.dispatchEvent(new CustomEvent('dockpier-notification', { detail: { type, message, tempId, time, read: false } }))
  } catch (e) {
    console.error('发送通知失败:', e)
  }
  api.system.addNotification({ type, message }).then((saved) => {
    if (!saved || !saved.id) {
      return
    }
    try {
      window.dispatchEvent(new CustomEvent('dockpier-notification', {
        detail: {
          type,
          message,
          tempId,
          dbId: saved.id,
          createdAt: saved.created_at,
          read: !!saved.read
        }
      }))
    } catch (e) {
      console.error('发送通知失败:', e)
    }
  }).catch((err) => {
    console.error('保存通知失败:', err)
  })
}

const checkImageUpdates = async () => {
  try {
    const res = await api.images.getUpdateStatus()
    const data = res.data || res
    const updates = Array.isArray(data.updates) ? data.updates : []
    const raw = {}
    updates.forEach((item) => {
      if (item && item.repoTag) {
        raw[item.repoTag] = true
      }
    })
    updateStatusMap.value = normalizeUpdateStatusMap(raw)
  } catch (error) {
    console.error('加载镜像更新状态失败:', error)
  }
}

const manualCheckUpdates = async () => {
  if (checkingUpdates.value) {
    return
  }
  try {
    checkingUpdates.value = true
    const res = await api.images.checkUpdates({ force: true })
    const data = res?.data || res || {}
    const remoteErrors = data.remoteErrors || 0
    const skippedBackoff = data.skippedBackoff || 0
    const skippedUnavailable = data.skippedUnavailable || 0
    const msg = `检测完成：远端错误 ${remoteErrors}，跳过退避 ${skippedBackoff}，跳过不可用 ${skippedUnavailable}`
    ElMessage.success(msg)
    await checkImageUpdates()
  } catch (e) {
    console.error('手动检测镜像更新失败:', e)
    ElMessage.error('手动检测镜像更新失败: ' + (e.message || '未知错误'))
  } finally {
    checkingUpdates.value = false
  }
}

const handleBulkUpdate = async () => {
  if (bulkUpdating.value) {
    return
  }
  try {
    bulkUpdating.value = true
    const res = await api.images.applyUpdates()
    const data = res.data || res
    const total = data.total || 0
    const attempted = data.attempted || 0
    const success = data.success || 0
    const failed = data.failed || 0
    const skippedUsed = data.skippedUsed || 0
    const message = `完成了 ${success}/${attempted}/${total} 镜像更新，跳过使用中镜像 ${skippedUsed} 个`
    pushNotification('success', message)
    if (failed > 0 && Array.isArray(data.failedTags) && data.failedTags.length) {
      const list = data.failedTags.slice(0, 5).join('、')
      const more = data.failedTags.length > 5 ? ` 等 ${data.failedTags.length} 个` : ''
      const failMsg = `以下镜像更新失败：${list}${more}`
      pushNotification('error', failMsg)
    }
    await checkImageUpdates()
    await fetchImages()
  } catch (e) {
    console.error('一键更新镜像失败:', e)
    ElMessage.error('一键更新镜像失败: ' + (e.message || '未知错误'))
  } finally {
    bulkUpdating.value = false
  }
}

const updateImage = async (image) => {
  const tag = image.RepoTags?.[0] || ''
  if (!tag || tag === '<none>:<none>') {
    ElMessage.warning('该镜像没有有效标签，无法更新')
    return
  }
  if (image.isInUse) {
    ElMessage.warning('该镜像正在被容器使用，请在项目编排或容器页面中执行升级')
    pushNotification('warning', `${tag} 镜像正在被容器使用，请从项目编排或容器页面中升级`)
    return
  }
  if (isImageUpdating(image)) {
    ElMessage.info(`镜像 ${tag} 正在更新中，请稍候`)
    return
  }
  try {
    updatingMap.value = {
      ...updatingMap.value,
      [tag]: true
    }
    ElMessage.info(`开始更新镜像 ${tag}`)
    pushNotification('info', `${tag} 镜像开始更新`)
    await api.images.pull({ name: tag, registry: '' })
    ElMessage.success(`镜像 ${tag} 已更新到最新版本`)
    pushNotification('success', `${tag} 镜像更新成功`)
    try {
      await api.images.clearUpdate({ repoTag: tag })
    } catch (e) {
      console.error('清除镜像更新记录失败:', e)
    }
    try {
      const map = { ...updateStatusMap.value }
      delete map[tag]
      updateStatusMap.value = map
    } catch (e) {
      console.error('本地更新状态清理失败:', e)
    }
    try {
      await checkImageUpdates()
    } catch (e) {
      console.error('刷新镜像更新状态失败:', e)
    }
    await fetchImages()
  } catch (error) {
    console.error('更新镜像失败:', error)
    ElMessage.error('更新镜像失败: ' + (error.message || '未知错误'))
    pushNotification('error', `${tag} 镜像更新失败`)
  } finally {
    const map = { ...updatingMap.value }
    delete map[tag]
    updatingMap.value = map
  }
}

onMounted(async () => {
  try {
    const res = await request.get('/settings/kv/sort_images')
    if (res && res.value) {
      const s = JSON.parse(res.value)
      if (s && s.prop && s.order) {
        sortState.value = s
      }
    }
  } catch (e) {}
  fetchImages()
  checkImageUpdates()
})

onUnmounted(() => {
  stopPullStream()
})
</script>

<style scoped>
.images-view {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 24px;
}

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

.table-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.main-table {
  flex: 1;
}

/* Custom Table Styles */
.image-name-cell {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 0;
}

.icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
  transition: transform 0.2s;
}

.image-name-cell:hover .icon-wrapper {
  transform: scale(1.05);
}

.icon-wrapper.image {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.name-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.name-text {
  font-weight: 600;
  color: var(--el-text-color-primary);
  font-size: 14px;
}

.id-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: monospace;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 500;
}

.status-point {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-point.running {
  background-color: #22c55e;
  box-shadow: 0 0 0 3px rgba(34,197,94,0.2);
}

.status-point.stopped {
  background-color: #94a3b8;
}

.tag-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.update-link {
  padding: 0;
  font-size: 12px;
}

.text-gray {
  color: #64748b;
  font-size: 13px;
}

.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.text-danger {
  color: #ef4444;
}

.row-ops {
  display: flex;
  justify-content: center;
  gap: 8px;
  align-items: center;
}

/* Pagination */
.pagination-bar {
  padding: 16px 24px;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
}

/* Pull Progress */
.pull-progress {
  margin-top: 16px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  border: 1px solid var(--el-border-color-lighter);
}

.progress-header {
  margin-bottom: 8px;
}
.progress-status {
  font-size: 13px;
  margin-bottom: 4px;
  color: var(--el-text-color-regular);
}

.progress-details {
  margin-top: 8px;
  max-height: 150px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 12px;
}

.detail-item {
  display: flex;
  gap: 12px;
  padding: 2px 0;
  color: var(--el-text-color-secondary);
}
.detail-id {
  color: var(--el-color-primary);
  width: 80px;
}

/* Override Element Styles */
:deep(.el-table th.el-table__cell) {
  background-color: var(--el-fill-color-light) !important;
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
