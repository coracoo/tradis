<template>
  <div class="containers-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索容器名称、ID、镜像或端口"
          clearable
          class="search-input"
          size="medium"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="statusFilter" placeholder="状态" clearable class="status-select" size="medium">
          <el-option label="所有" value="" />
          <el-option label="运行中" value="running" />
          <el-option label="已停止" value="stopped" />
          <el-option label="已暂停" value="paused" />
          <el-option label="已创建" value="created" />
        </el-select>
      </div>
      
      <div class="filter-right">
        <el-button-group>
          <el-button @click="fetchContainers" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
          <el-button type="primary" @click="createContainer" size="medium">
            <template #icon><el-icon><Plus /></el-icon></template>
            新建容器
          </el-button>
        </el-button-group>

        <el-dropdown trigger="click" @command="handleGlobalCommand">
          <el-button plain class="more-btn" size="medium">
             更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="prune" :icon="Delete">清除未使用容器</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="table-wrapper">
      <!-- 容器列表 -->
      <el-table 
        :data="filteredContainers" 
        class="containers-table"
        v-loading="loading"
        @selection-change="handleSelectionChange"
        @sort-change="handleSortChange"
        :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: 'auto' }"
      >
      <el-table-column type="selection" width="40" />
      <el-table-column prop="Names" label="名称 / ID" sortable="custom" min-width="220">
        <template #default="scope">
          <div class="name-cell-wrapper">
            <div class="icon-wrapper">
              <el-icon><Box /></el-icon>
            </div>
            <div class="name-col">
              <el-button 
                link 
                type="primary" 
                class="container-name-btn"
                @click="goToContainerDetail(scope.row)"
              >
                {{ scope.row.Names?.[0]?.replace(/^\//, '') || '-' }}
              </el-button>
              <div class="container-short-id font-mono">{{ (scope.row.Id || '').slice(0,12) }}</div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="State" label="状态" sortable="custom" width="140" header-align="left">
        <template #default="scope">
          <div class="status-cell">
            <div class="status-dot" :class="scope.row.State?.toLowerCase() === 'running' ? 'status-used' : 'status-unused'">
              {{ stateMap[scope.row.State.toLowerCase()] || scope.row.State }}
            </div>
            <div class="status-time" v-if="scope.row.RunningTime">{{ scope.row.RunningTime }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="Image" label="镜像" sortable="custom" min-width="150" header-align="left">
        <template #default="scope">
          <div class="image-cell">
            <div class="truncate image-name" :title="scope.row.Image">{{ getImageName(scope.row.Image) }}</div>
            <el-tag size="small" class="image-tag font-mono">{{ getImageTag(scope.row.Image) }}</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="网络 / 端口" min-width="130" sortable="custom" prop="Network" header-align="left">
        <template #default="scope">
          <div class="network-info">
            <div class="text-gray ip-address font-mono">{{ getContainerIP(scope.row) }}</div>
            <div class="ports-list">
              <template v-if="scope.row.Ports && scope.row.Ports.length">
                <el-tag 
                  v-for="(port, index) in scope.row.Ports.slice(0, 3)" 
                  :key="index" 
                  size="small" 
                  class="port-tag font-mono"
                  effect="plain"
                >
                  {{ formatPortWithIP(port) }}
                </el-tag>
                <el-tooltip
                  v-if="scope.row.Ports.length > 3"
                  placement="top"
                  effect="light"
                  popper-class="ports-tooltip"
                >
                  <template #content>
                    <div class="ports-tooltip-content">
                      <div v-for="(port, index) in scope.row.Ports" :key="index" class="port-item font-mono">
                        {{ formatPortWithIP(port) }}
                      </div>
                    </div>
                  </template>
                  <el-tag size="small" type="info" class="port-tag more-ports cursor-pointer">
                    +{{ scope.row.Ports.length - 3 }}
                  </el-tag>
                </el-tooltip>
              </template>
              <span v-else class="text-gray">-</span>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="资源" min-width="130" sortable="custom" prop="Resources" header-align="left" >
        <template #default="scope">
          <div class="resource-line">
            <span class="label">CPU</span>
            <div class="bar"><div class="fill cpu" :style="{width: (scope.row.CPUPerc || '0%')}"/></div>
            <span class="value">{{ scope.row.CPUPerc || '0.00%' }}</span>
          </div>
          <div class="resource-line">
            <span class="label">RAM</span>
            <div class="bar"><div class="fill ram" :style="{width: (scope.row.MemPerc || '0%')}"/></div>
            <span class="value">{{ scope.row.MemPerc || '0.00%' }}</span>
          </div>
        </template>
      </el-table-column>
      <!-- IP 与端口已合并到“网络 / 端口”列；运行时长合并到状态列显示 -->
      <el-table-column prop="Created" label="创建时间" sortable="custom" min-width="100"  header-align="left">
        <template #default="scope">
          <div class="text-gray font-mono text-center whitespace-pre-line">
            {{ formatTimeTwoLines(scope.row.Created) }}
          </div>
        </template>
      </el-table-column>
      <!-- 操作列右对齐并使用图标按钮 -->
      <el-table-column label="操作" width="240" align="left" header-align="left">
        <template #default="scope">
          <el-button-group>
            <el-button size="small" @click="openLogs(scope.row)" title="日志"><el-icon><Document /></el-icon></el-button>
            <el-button size="small" @click="openTerminal(scope.row)" title="终端"><el-icon><Monitor /></el-icon></el-button>
            <el-button size="small" @click="openEdit(scope.row)" title="编辑"><el-icon><Edit /></el-icon></el-button>
            <el-dropdown trigger="click">
              <el-button size="small">
                更多<el-icon class="el-icon--right"><arrow-down /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="handleAction(scope.row, 'start')">启动</el-dropdown-item>
                  <el-dropdown-item @click="handleAction(scope.row, 'stop')">停止</el-dropdown-item>
                  <el-dropdown-item @click="handleAction(scope.row, 'restart')">重启</el-dropdown-item>
                  <el-dropdown-item @click="handleAction(scope.row, 'pause')">暂停</el-dropdown-item>
                  <el-dropdown-item @click="handleAction(scope.row, 'unpause')">恢复</el-dropdown-item>
                  <el-dropdown-item divided @click="handleDelete(scope.row)">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        :total="filteredContainers.length"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        size="small"
      />
    </div>
  </div>
	
	<!-- 添加组件使用 -->
    <ContainerTerminal
      v-model="terminalDialogVisible"
      :container="currentContainer"
    />
    
    <ContainerLogs
      v-model="logDialogVisible"
      :container="currentContainer"
    />

    <ContainerEdit
      v-model="editDialogVisible"
      :container="currentContainer"
      @success="fetchContainers"
    />
	
  </div>
</template>

<!-- 在 script setup 中添加相关变量和方法 -->
<script setup>
import { ref, onMounted, computed, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Plus, Refresh, VideoPlay, VideoPause, CircleClose, Document, Monitor, Edit, Search, Delete, Box } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import { formatTimeTwoLines } from '../utils/format'
import api from '../api'
import ContainerTerminal from '../components/ContainerTerminal.vue'
import ContainerLogs from '../components/ContainerLogs.vue'
import ContainerEdit from '../components/ContainerEdit.vue'
import { useRouter, useRoute } from 'vue-router'

// 变量定义
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const containers = ref([])
const selectedContainers = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const statusFilter = ref('')
const searchQuery = ref('') // 添加搜索关键词
const currentContainer = ref(null)
const terminalDialogVisible = ref(false)
const logDialogVisible = ref(false)
const editDialogVisible = ref(false) // 新增编辑弹窗控制
const logs = ref('')
const batchStart = () => batchAction('start')
const batchStop = () => batchAction('stop')
const batchRestart = () => batchAction('restart')
const batchForceStop = () => batchAction('kill')
const batchPause = () => batchAction('pause')
const batchResume = () => batchAction('unpause')
const batchDelete = () => batchAction('remove')

const handleGlobalCommand = (command) => {
  if (command === 'prune') {
    clearContainers()
  }
}

// 打开编辑弹窗
const openEdit = (container) => {
  currentContainer.value = container
  editDialogVisible.value = true
}

// 批量操作函数
const batchAction = async (action) => {
  if (selectedContainers.value.length === 0) {
    ElMessage.warning('请选择容器')
    return
  }
  
  try {
    const actionMap = {
      'start': '启动',
      'stop': '停止',
      'restart': '重启',
      'kill': '强制停止',
      'pause': '暂停',
      'unpause': '恢复',
      'remove': '删除'
    }
    
    await ElMessageBox.confirm(`确定要${actionMap[action]}选中的 ${selectedContainers.value.length} 个容器吗？`, '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await Promise.all(
      selectedContainers.value.map(container => 
        api.containers[action](container.Id)
      )
    )
    
    ElMessage.success(`已${actionMap[action]}${selectedContainers.value.length}个容器`)
    fetchContainers()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('批量操作失败:', error)
      ElMessage.error(`操作失败: ${error.message || '未知错误'}`)
    }
  }
}

// 清理容器函数
const clearContainers = async () => {
  try {
    await ElMessageBox.confirm('确定要清理所有已停止的容器吗？', '警告', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.containers.prune()
    ElMessage.success('已清理所有已停止的容器')
    fetchContainers()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('清理容器失败:', error)
      ElMessage.error(`清理失败: ${error.message || '未知错误'}`)
    }
  }
}

// 添加处理单个容器操作的函数
const handleAction = async (container, action) => {
  try {
    const actionMap = {
      'start': '启动',
      'stop': '停止',
      'restart': '重启',
      'pause': '暂停',
      'unpause': '恢复'
    }
    
    await api.containers[action](container.Id)
    ElMessage.success(`容器已${actionMap[action]}`)
    fetchContainers()
  } catch (error) {
    console.error(`容器操作失败:`, error)
    ElMessage.error(`操作失败: ${error.message || '未知错误'}`)
  }
}

// 添加处理单个容器删除的函数
const handleDelete = async (container) => {
  try {
    const containerName = container.Names?.[0]?.replace(/^\//, '') || container.Id.substring(0, 12)
    
    await ElMessageBox.confirm(`确定要删除容器 "${containerName}" 吗？`, '警告', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.containers.remove(container.Id)
    ElMessage.success('容器已删除')
    fetchContainers()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除容器失败:', error)
      ElMessage.error(`删除失败: ${error.message || '未知错误'}`)
    }
  }
}

// 创建容器函数
const createContainer = () => {
  ElMessage.info('不想做，建议用项目新建')
  // 这里可以添加创建容器的对话框逻辑
}

// 添加打开终端和日志的方法
const openTerminal = (container) => {
  currentContainer.value = container
  nextTick(() => {
    terminalDialogVisible.value = true;
  });
};

const openLogs = (container) => {
  currentContainer.value = container
  logDialogVisible.value = true
}

// 获取容器列表
const fetchContainers = async () => {
  loading.value = true
  try {
    const data = await api.containers.list()
    containers.value = Array.isArray(data) ? data : []
    total.value = containers.value.length
  } catch (error) {
    console.error('Error fetching containers:', error)
    ElMessage.error('获取容器列表失败')
    containers.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 格式化端口映射
const formatPorts = (ports) => {
  if (!Array.isArray(ports)) return '-'
  return ports.map(port => {
    if (port.PublicPort) {
      return `${port.PublicPort}:${port.PrivatePort}/${port.Type}`
    }
    return `${port.PrivatePort}/${port.Type}`
  }).join(', ')
}
// 添加格式化端口函数
const formatPortWithIP = (port) => {
  if (port.PublicPort) {
    const ip = port.IP || '0.0.0.0'
    return `${ip}:${port.PublicPort}:${port.PrivatePort}/${port.Type}`
  }
  return `${port.PrivatePort}/${port.Type}`
}

// 添加状态映射
const stateMap = {
  'running': '运行中',
  'exited': '已停止',
  'created': '已创建',
  'paused': '已暂停',
  'restarting': '重启中',
  'removing': '删除中',
  'dead': '已死亡'
}

// 排序处理
const handleSortChange = ({ prop, order }) => {
  sortState.value = { prop, order }
}

// 状态标签类型获取函数
const getStatusType = (status) => {
  const types = {
    'running': 'success',
    'exited': 'danger',
    'created': 'info',
    'paused': 'warning',
    'restarting': 'warning',
    'removing': 'danger',
    'dead': 'danger'
  }
  return types[status.toLowerCase()] || 'info'
}

// 添加计算属性用于过滤容器列表
const filteredContainers = computed(() => {
  let result = containers.value

  // 状态筛选
  if (statusFilter.value) {
    result = result.filter(container => {
      const state = container.State.toLowerCase()
      switch (statusFilter.value) {
        case 'running':
          return state === 'running'
        case 'stopped':
          return state === 'exited'
        case 'paused':
          return state === 'paused'
        case 'created':
          return state === 'created'
        default:
          return true
      }
    })
  }

  // 关键词搜索
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(container => {
      const name = (container.Names?.[0] || '').toLowerCase()
      const id = (container.Id || '').toLowerCase()
      const image = (container.Image || '').toLowerCase()
      // 搜索端口
      const ports = (container.Ports || []).map(p => {
        if (p.PublicPort) return `${p.PublicPort}:${p.PrivatePort}`
        return `${p.PrivatePort}`
      }).join(' ')
      
      return name.includes(query) || 
             id.includes(query) || 
             image.includes(query) || 
             ports.includes(query)
    })
  }

  // 排序
  if (sortState.value.prop && sortState.value.order) {
    const { prop, order } = sortState.value
    result.sort((a, b) => {
      let valA, valB
      switch (prop) {
        case 'Names':
          valA = a.Names?.[0] || ''
          valB = b.Names?.[0] || ''
          break
        case 'State':
          valA = a.State || ''
          valB = b.State || ''
          break
        case 'Image':
          valA = a.Image || ''
          valB = b.Image || ''
          break
        case 'Network':
          valA = getContainerIP(a)
          valB = getContainerIP(b)
          break
        case 'Resources':
          // 按 CPU 排序
          valA = parseFloat(a.CPUPerc?.replace('%', '') || 0)
          valB = parseFloat(b.CPUPerc?.replace('%', '') || 0)
          break
        case 'Created':
          valA = a.Created || ''
          valB = b.Created || ''
          break
        default:
          valA = a[prop]
          valB = b[prop]
      }
      
      if (valA < valB) return order === 'ascending' ? -1 : 1
      if (valA > valB) return order === 'ascending' ? 1 : -1
      return 0
    })
  }

  return result
})

// 表格选择变化
const handleSelectionChange = (selection) => {
  selectedContainers.value = selection
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  fetchContainers()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchContainers()
}

onMounted(() => {
  const q = route.query.status
  if (typeof q === 'string') {
    statusFilter.value = q
  }
  fetchContainers()
})
watch(() => route.query.status, (val) => {
  if (typeof val === 'string') {
    statusFilter.value = val
  }
})
// 添加获取容器 IP 的函数
const getContainerIP = (container) => {
  // 如果是 host 网络模式，返回 host
  if (container.NetworkSettings?.Networks?.host) {
    return 'host'
  }
  
  // 获取容器 IP
  const ip = container.NetworkSettings?.Networks?.bridge?.IPAddress || '-'
  return ip
}
// 添加容器详情页面跳转方法
const goToContainerDetail = (container) => {
  const containerName = container.Names?.[0]?.replace(/^\//, '') || ''
  if (containerName) {
    router.push(`/containers/${containerName}`)
  }
}

// 获取镜像名称和标签
const getImageName = (image) => {
  if (!image) return ''
  const index = image.lastIndexOf(':')
  if (index > -1 && !image.substring(index + 1).includes('/')) {
    return image.substring(0, index)
  }
  return image
}

const getImageTag = (image) => {
  if (!image) return ''
  const index = image.lastIndexOf(':')
  if (index > -1 && !image.substring(index + 1).includes('/')) {
    return image.substring(index + 1)
  }
  return 'latest'
}
</script>

<style scoped>
  /* 页面容器 - 统一风格 */
  .containers-view {
    height: 100%;
    width: 100%;
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
    overflow: hidden;
    padding: 12px 24px;
    background-color: var(--el-bg-color-page);
  }

/* 顶部操作栏 */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  background: var(--el-bg-color);
  padding: 12px 20px;
  border-radius: 12px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);
  border: 1px solid var(--el-border-color-light);
}

.filter-left, .filter-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.search-input {
  width: 300px;
}

.status-select {
  width: 160px;
}

/* 表格容器 */
.table-wrapper {
  flex: 1;
  overflow: hidden;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.containers-table {
  flex: 1;
  min-height: 0; 
}

/* 分页 */
.pagination-bar {
  padding: 16px 24px;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
}

/* --------------------------------------------------------- */
/* 以下保留原有的列内容样式 */

.name-cell-wrapper {
  display: flex;
  align-items: center;
  gap: 16px;
}

.icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
}

.name-col {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.container-name-btn {
  padding: 0;
  height: auto;
  font-weight: 600;
  font-size: 14px;
  color: var(--el-color-primary);
  justify-content: flex-start;
}

.container-short-id {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

/* 状态列 */
.status-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.status-dot {
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  white-space: nowrap;
}
.status-dot::before {
  content: '';
  width: 6px;
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
.status-time {
  padding-left: 14px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

/* 镜像列 */
.image-name {
  color: var(--el-text-color-regular);
}

.image-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
}

.image-tag {
  height: 20px;
  line-height: 18px;
}

/* 网络/端口列 */
.network-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ip-address {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.ports-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.port-tag {
  border: 1px solid var(--el-border-color-lighter);
}

.more-ports {
  cursor: pointer;
  transition: all 0.2s;
}

.more-ports:hover {
  background-color: var(--el-fill-color-dark);
}

/* 资源列样式 */
.resource-line {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 20px;
}

.resource-line .label {
  width: 32px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.resource-line .bar {
  flex: 1;
  height: 6px;
  background-color: var(--el-fill-color-darker);
  border-radius: 3px;
  overflow: hidden;
  min-width: 60px;
}

.resource-line .fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s ease;
}

.resource-line .fill.cpu {
  background-color: #409EFF; /* CPU 蓝色 */
}

.resource-line .fill.ram {
  background-color: #9F59F0; /* 内存 紫色 */
}

.resource-line .value {
  width: 48px;
  font-size: 12px;
  text-align: right;
  font-family: monospace;
  color: var(--el-text-color-primary);
}

/* 字体工具类 */
.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.text-gray {
  color: var(--el-text-color-secondary);
}

.truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.text-center {
  text-align: left;
}

.whitespace-pre-line {
  white-space: pre-line;
}

/* 全局样式（用于 tooltip 等） */
.ports-tooltip {
  max-width: 300px;
}

.ports-tooltip-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
}

.port-item {
  font-size: 12px;
  color: var(--el-text-color-regular);
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
