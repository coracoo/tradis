<template>
  <div class="images-view compact-table">
    <!-- 顶部操作栏 -->
    <div class="operation-bar">
      <el-button-group>
        <el-button @click="fetchImages" :loading="loading">
          <el-icon><Refresh /></el-icon>
        </el-button>
        <el-button type="primary" @click="pullImage">
          <el-icon><Download /></el-icon> 拉取镜像
        </el-button>
        <el-button @click="importImage">
          <el-icon><Upload /></el-icon> 导入
        </el-button>
        <el-button @click="settingsVisible = true">
           <el-icon><Setting /></el-icon> 配置
        </el-button>
      </el-button-group>

      <!-- 搜索框 -->
      <div class="search-box">
        <el-input
          v-model="searchQuery"
          placeholder="搜索镜像名称、标签或ID"
          clearable
          prefix-icon="Search"
          style="width: 300px"
        />
      </div>
    </div>

    <!-- 镜像列表 -->
    <el-table 
      :data="paginatedImages" 
      style="width: 100%" 
      height="100%"
      v-loading="loading"
      @sort-change="handleSortChange"
      :default-sort="{ prop: 'RepoTags', order: 'ascending' }"
      class="images-table"
      :header-cell-style="{ background: 'transparent' }">
      
      <el-table-column type="selection" width="40" align="center" header-align="left" />
      
      <el-table-column 
        label="ID" 
        width="140" 
        prop="Id" 
        sortable="custom"
        header-align="left">
        <template #default="scope">
          <el-tooltip :content="scope.row.Id" placement="top">
            <span class="font-mono text-xs">{{ scope.row.Id.substring(7, 19) }}</span>
          </el-tooltip>
        </template>
      </el-table-column>

      <el-table-column 
        label="镜像名称" 
        prop="RepoTags" 
        sortable="custom"
        min-width="200"
        header-align="left"
        show-overflow-tooltip>
        <template #default="scope">
          <span class="image-name">{{ getImageName(scope.row.RepoTags?.[0]) }}</span>
        </template>
      </el-table-column>

      <el-table-column 
        label="状态" 
        min-width="160" 
        prop="isInUse" 
        sortable="custom"
        header-align="left"
        align="left">
        <template #default="scope">
          <div class="status-dot" :class="scope.row.isInUse ? 'status-used' : 'status-unused'">
            {{ scope.row.isInUse ? '使用中' : '未使用' }}
          </div>
        </template>
      </el-table-column>
      
      <el-table-column 
        label="标签" 
        prop="RepoTags"
        min-width="160"
        header-align="left">
        <template #default="scope">
          <el-tag size="small" effect="plain" class="image-tag">
            {{ getImageTag(scope.row.RepoTags?.[0]) }}
          </el-tag>
        </template>
      </el-table-column>
      
      <el-table-column 
        label="大小" 
        prop="Size" 
        min-width="160"
        sortable="custom"
        header-align="left">
        <template #default="scope">
          <span class="text-gray">{{ formatSize(scope.row.Size) }}</span>
        </template>
      </el-table-column>
      
      <el-table-column 
        label="创建时间" 
        prop="Created" 
        min-width="160"
        sortable="custom"
        header-align="left">
        <template #default="scope">
          <div class="text-gray font-mono text-center whitespace-pre-line">
            {{ formatTimeTwoLines(scope.row.Created) }}
          </div>
        </template>
      </el-table-column>
      
      <!-- 操作列 -->
      <el-table-column label="操作" width="140" fixed="right" header-align="left">
        <template #default="scope">
          <div class="operation-buttons">
            <el-tooltip content="修改标签" placement="top">
              <el-button 
                link 
                type="primary" 
                :disabled="scope.row.isInUse"
                @click="tagImage(scope.row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="导出" placement="top">
              <el-button 
                link 
                type="primary" 
                @click="exportImage(scope.row)">
                <el-icon><Download /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="删除" placement="top">
              <el-button 
                link 
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

    <!-- 分页 -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        :total="total"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        size="small"
      />
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
import { ref, onMounted, h, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UploadFilled, Download, Upload, Setting, Edit, Delete, Search } from '@element-plus/icons-vue'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
import DockerSettings from '../components/DockerSettings.vue'

import { getRegistries } from '../api/image_registry'

const loading = ref(false)
const images = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const searchQuery = ref('') // 添加搜索关键词

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

// 删除 proxyDialogVisible 和 proxyForm 相关代码
const settingsVisible = ref(false)
const showProxyDialog = () => {
  settingsVisible.value = true
}

// 修改获取镜像列表的函数，添加更详细的错误处理
const fetchImages = async () => {
  loading.value = true
  try {
    const imagesData = await api.images.list()
    const containersData = await api.containers.list({ all: true })
    
    // 获取使用中的镜像信息
    const usedImages = new Set()
    
    // 添加空值检查
    if (containersData && Array.isArray(containersData)) {
      containersData.forEach(container => {
        if (container && container.Image) {
          const imageName = container.Image
          // 如果镜像名称中没有标签，添加 :latest
          usedImages.add(imageName.includes(':') ? imageName : `${imageName}:latest`)
        }
      })
    }
    
    // 处理镜像数据，将每个标签作为单独的行
    const processedImages = []
    if (imagesData && Array.isArray(imagesData)) {
      imagesData.forEach(image => {
        if (!image.RepoTags || image.RepoTags.length === 0 || (image.RepoTags.length === 1 && image.RepoTags[0] === '<none>:<none>')) {
          processedImages.push({
            ...image,
            RepoTags: ['<none>:<none>'],
            isInUse: false
          })
        } else {
          image.RepoTags.forEach(tag => {
            processedImages.push({
              ...image,
              RepoTags: [tag],
              isInUse: usedImages.has(tag)
            })
          })
        }
      })
    }
    
    // 打印处理后的镜像数据，用于调试
    console.log('处理后的镜像数据:', processedImages)
    
    images.value = processedImages
    total.value = processedImages.length
    
    // 添加默认排序
    handleSortChange({ prop: 'RepoTags', order: 'ascending' })
  } catch (error) {
    console.error('获取镜像列表错误:', error)
    ElMessage.error('获取镜像列表失败')
    images.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
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

// 添加导入镜像相关变量
const importDialogVisible = ref(false)
const importProgress = ref({
  show: false,
  percent: 0,
  status: '',
  uploading: false
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

  try {
    pullProgress.value = {
      show: true,
      status: '准备拉取镜像...',
      progress: 0,
      details: []
    }

    // 使用 POST 请求拉取镜像
    const data = {
      name: pullForm.value.name,
      registry: pullForm.value.registry
    }
    
    // 创建 EventSource 监听进度，使用正确的 URL 格式
    const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
    let eventSource = null
    
    try {
      const token = localStorage.getItem('token')
      const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
      eventSource = new EventSource(`${baseUrl}/api/images/pull/progress?name=${encodeURIComponent(pullForm.value.name)}&registry=${encodeURIComponent(pullForm.value.registry)}${tokenParam}`)
      
      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          
          // 更新总体状态
          if (data.status) {
            pullProgress.value.status = data.status
          }
          
          // 更新进度百分比
          if (data.progressDetail && data.progressDetail.current && data.progressDetail.total) {
            pullProgress.value.progress = Math.round(
              (data.progressDetail.current / data.progressDetail.total) * 100
            )
          }
          
          // 更新详细信息
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
        } catch (e) {
          console.error('解析进度数据失败:', e)
        }
      }
      
      eventSource.onerror = (event) => {
        if (eventSource) {
          eventSource.close()
          eventSource = null
        }
        // 不在这里显示错误，让 POST 请求处理错误
        console.warn('进度监听中断:', event)
      }
    } catch (e) {
      console.error('创建 EventSource 失败:', e)
      // 继续执行，不中断流程
    }
    
    // 使用正确的 API 函数
    try {
      await api.images.pull(data)
      // POST 请求成功完成
      if (eventSource) {
        eventSource.close()
      }
      ElMessage.success('镜像拉取成功')
      pullDialogVisible.value = false
      fetchImages()
    } catch (error) {
      // POST 请求失败
      if (eventSource) {
        eventSource.close()
      }
      
      console.error('拉取失败:', error)
      
      // 提取更有用的错误信息
      let errorMsg = error.message || '拉取失败'
      if (error.response?.data?.error) {
        const dockerError = error.response.data.error
        
        // 检查是否是代理连接问题
        if (dockerError.includes('connection reset by peer') || dockerError.includes('timeout')) {
          errorMsg = '连接到镜像仓库失败，可能是网络问题或代理设置有误。请检查 Docker 的网络设置。'
        } else if (dockerError.includes('not found')) {
          errorMsg = '镜像未找到，请检查镜像名称是否正确。'
        } else if (dockerError.includes('unauthorized')) {
          errorMsg = '认证失败，请检查仓库的用户名和密码设置。'
        } else {
          // 提取 Docker 守护进程返回的错误信息
          errorMsg = '拉取失败: ' + dockerError
        }
      }
      
      ElMessage.error(errorMsg)
    } finally {
      pullProgress.value.show = false
    }
  } catch (error) {
    console.error('拉取操作异常:', error)
    ElMessage.error('拉取操作异常: ' + (error.message || '未知错误'))
    pullProgress.value.show = false
  }
}

// 添加修改标签相关变量
const tagDialogVisible = ref(false)
const tagForm = ref({
  imageId: '',
  currentTag: '',
  repository: '',
  tag: ''
})

// 修改标签
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
        
        await api.images.remove(tagForm.value.currentTag)
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


const exportImage = async (image) => {
  try {
    const imageId = image.Id
    
    const response = await api.images.export(imageId)
    
    // 处理文件下载
    const blob = new Blob([response], { type: 'application/x-tar' })
    const url = window.URL.createObjectURL(blob)
    
    // 创建下载链接
    const link = document.createElement('a')
    link.href = url
    
    // 设置文件名
    let fileName = image.Id.substring(7, 19)
    if (image.RepoTags && image.RepoTags.length > 0 && image.RepoTags[0] !== '<none>:<none>') {
      fileName = image.RepoTags[0].replace(/[\/\:]/g, '_')
    }
    link.download = `${fileName}.tar`
    
    // 触发下载
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    
    ElMessage.success('镜像导出成功')
  } catch (error) {
    console.error('导出失败:', error)
    ElMessage.error('导出失败: ' + (error.message || '未知错误'))
  }
}

// 删除镜像
const deleteImage = async (image) => {
  if (image.isInUse) {
    ElMessage.warning('该镜像正在被容器使用，无法删除')
    return
  }
  
  try {
    await ElMessageBox.confirm('确定要删除该镜像吗？', '警告', {
      type: 'warning'
    })
    await api.images.remove(image.Id)
    ElMessage.success('镜像已删除')
    fetchImages()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.message || '未知错误'))
    }
  }
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  fetchImages()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchImages()
}

// 格式化文件大小
const formatSize = (size) => {
  if (!size) return '0 MB'
  const mb = size / (1024 * 1024)
  return `${mb.toFixed(2)} MB`
}

// 添加排序相关变量
const sortBy = ref('')
const sortOrder = ref('ascending')

// 添加排序处理函数
const handleSortChange = ({ prop, order }) => {
  if (!prop || !order) {
    images.value = [...images.value]
    return
  }

  // 移除空值检查，保证排序状态
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
      case 'RepoTags':
        // 只比较镜像名称部分
        aValue = getImageName(a.RepoTags?.[0] || '')
        bValue = getImageName(b.RepoTags?.[0] || '')
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

onMounted(() => {
  fetchImages()
})
</script>

<style scoped>
/* 继承 layout.css 的 compact-table 样式 */

.images-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden; /* 防止撑开父容器 */
  padding-right: 4px;
}

.operation-bar {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.images-table {
  flex: 1;
  /* 允许表格在容器内滚动 */
  min-height: 0; 
  overflow: hidden; /* 确保表格不会溢出 */
}

/* 状态圆点 */
.status-dot {
  display: inline-flex;
  align-items: center;
  font-size: 13px; /* 稍微调大一点以匹配其他文本 */
  white-space: nowrap; /* 防止换行 */
}
.status-dot::before {
  content: '';
  width: 6px; /* 调整大小 */
  height: 6px;
  border-radius: 50%;
  margin-right: 8px;
  flex-shrink: 0;
}
.status-used {
  color: var(--el-color-success);
}
.status-used::before {
  background-color: var(--el-color-success);
  box-shadow: 0 0 0 2px var(--el-color-success-light-9);
}
.status-unused {
  color: var(--el-text-color-secondary);
}
.status-unused::before {
  background-color: var(--el-color-info-light-3);
}

.image-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.image-tag {
  font-family: monospace;
}

.text-gray {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.operation-buttons {
  display: flex;
  gap: 4px;
}

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

/* 覆盖 Element Plus 样式以适应紧凑视图 */
:deep(.el-table__row) {
  height: 44px;
}
:deep(.el-button--link) {
  padding: 4px;
  height: auto;
}

.text-center {
  text-align: left;
}

.whitespace-pre-line {
  white-space: pre-line;
}

.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.text-xs {
  font-size: 0.75rem;
  line-height: 1rem;
}
</style>