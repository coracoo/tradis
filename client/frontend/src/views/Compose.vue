<template>
  <div class="compose-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索项目或容器名称..."
          clearable
          class="search-input"
          size="medium"
          @keyup.enter="refreshAll"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        
        <el-radio-group v-model="statusFilter" class="status-filter" size="medium">
          <el-radio-button label="">全部</el-radio-button>
          <el-radio-button label="运行中">运行中</el-radio-button>
          <el-radio-button label="已停止">已停止</el-radio-button>
        </el-radio-group>
      </div>

      <div class="filter-right">
        <el-button-group class="main-actions">
          <el-button @click="refreshAll" :loading="loading" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
          <el-button type="primary" @click="goCreateProject" size="medium">
            <template #icon><el-icon><Plus /></el-icon></template>
            新建项目
          </el-button>
        </el-button-group>
        
        <el-dropdown trigger="click" @command="handleGlobalAction">
          <el-button plain class="more-btn" size="medium">
            更多操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="prune" :icon="Remove">清除已停止容器</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        ref="tableRef"
        :data="paginatedItems"
        style="width: 100%; height: 100%"
        v-loading="loading"
        row-key="key"
        :default-expand-all="false"
        class="main-table"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: '60px' }"
        @sort-change="handleSortChange"
      >
        <el-table-column type="expand" width="50">
          <template #default="props">
            <div class="expanded-container" v-if="props.row.containers?.length">
              <div class="expanded-header">
                <el-icon><Connection /></el-icon> 包含容器 ({{ props.row.containers.length }})
              </div>
              <el-table 
                :data="props.row.containers" 
                size="default" 
                :show-header="true"
                class="inner-table"
                :row-style="{ height: 'auto' }"
              >
                <el-table-column label="容器名称" min-width="180" sortable prop="name">
                  <template #default="scope">
                    <div class="container-name-cell" @click="goContainerDetail(scope.row)">
                      <el-icon class="container-icon" size="18"><Platform /></el-icon>
                      <span class="container-name-text">{{ scope.row.name }}</span>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="镜像" min-width="180" header-align="left" sortable prop="image">
                  <template #default="scope">
                    <div class="image-inline font-mono truncate" :title="scope.row.image">
                      {{ getImageName(scope.row.image) }}:{{ getImageTag(scope.row.image) }}
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="端口" min-width="180" header-align="left" sortable :sort-method="(a, b) => (a.Ports?.length || 0) - (b.Ports?.length || 0)">
                  <template #default="scope">
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
                  </template>
                </el-table-column>
                <el-table-column label="网络" min-width="160" header-align="left" sortable :sort-method="(a, b) => getNetworkNames(a).length - getNetworkNames(b).length">
                  <template #default="scope">
                    <div class="networks-list">
                      <template v-if="getNetworkNames(scope.row).length">
                        <el-tag v-for="(n, idx) in getNetworkNames(scope.row)" :key="idx" size="small" class="network-tag">{{ n }}</el-tag>
                      </template>
                      <span v-else class="text-gray">-</span>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="创建时间" min-width="140" header-align="left" sortable prop="Created">
                  <template #default="scope">
                    <div class="text-gray font-mono whitespace-pre-line">
                      {{ formatTimeTwoLines(scope.row.Created) }}
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="状态" width="120" sortable prop="state">
                  <template #default="scope">
                    <el-tag size="default" :type="isRunning(scope.row.state) ? 'success' : 'info'" effect="light">
                      {{ toCnState(scope.row.state) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="260" align="center">
                  <template #default="scope">
                    <div class="op-buttons">
                      <el-tooltip content="终端" placement="top" :show-after="500">
                        <el-button circle plain size="small" @click="openTerminal(scope.row)">
                          <el-icon><Monitor /></el-icon>
                        </el-button>
                      </el-tooltip>
                      <el-tooltip content="日志" placement="top" :show-after="500">
                        <el-button circle plain size="small" @click="openLogs(scope.row)">
                          <el-icon><Document /></el-icon>
                        </el-button>
                      </el-tooltip>
                      <el-tooltip content="启动" placement="top" :show-after="500">
                        <el-button circle plain size="small" type="primary" @click="startContainer(scope.row)" :disabled="isRunning(scope.row.state)">
                          <el-icon><VideoPlay /></el-icon>
                        </el-button>
                      </el-tooltip>
                      <el-tooltip content="停止" placement="top" :show-after="500">
                        <el-button circle plain size="small" type="warning" @click="stopContainer(scope.row)" :disabled="!isRunning(scope.row.state)">
                          <el-icon><VideoPause /></el-icon>
                        </el-button>
                      </el-tooltip>
                      <el-tooltip content="重启" placement="top" :show-after="500">
                        <el-button circle plain size="small" type="info" @click="restartContainer(scope.row)">
                          <el-icon><Refresh /></el-icon>
                        </el-button>
                      </el-tooltip>
                      <el-tooltip content="删除" placement="top" :show-after="500">
                        <el-button circle plain size="small" type="danger" @click="deleteContainer(scope.row)">
                          <el-icon><Delete /></el-icon>
                        </el-button>
                      </el-tooltip>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div v-else class="empty-expand">
              暂无容器
            </div>
          </template>
        </el-table-column>

        <el-table-column label="项目 / 容器名称" min-width="300" sortable="custom" prop="name">
          <template #default="scope">
            <div class="project-name-cell" @click="handleNameClick(scope.row)">
              <div class="icon-wrapper" :class="scope.row.type">
                <el-icon v-if="scope.row.type === 'compose'"><Folder /></el-icon>
                <el-icon v-else><Box /></el-icon>
              </div>
              <div class="name-info">
                <span class="name-text">{{ scope.row.name }}</span>
                <span class="type-tag" v-if="scope.row.type === 'compose'">Compose</span>
                <span class="type-tag" v-else-if="scope.row.type === 'container'">独立容器</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="项目路径" min-width="280" header-align="left" sortable="custom" prop="path">
          <template #default="scope">
            <div class="path-text text-gray" v-if="scope.row.type === 'compose'">
              {{ isManagedProject(scope.row.path) ? scope.row.path : '非本项目管理Compose' }}
            </div>
            <div class="text-gray" v-else>-</div>
          </template>
        </el-table-column>

        <el-table-column label="服务数量" width="120" align="center" sortable="custom" prop="count">
          <template #default="scope">
            <div class="count-badge">
              {{ scope.row.containers?.length || 0 }}
            </div>
          </template>
        </el-table-column>

        <el-table-column label="创建时间" min-width="160" header-align="left" sortable="custom" prop="createTime">
          <template #default="scope">
            <div class="text-gray font-mono whitespace-pre-line" v-if="scope.row.type === 'compose'">
              {{ formatTimeTwoLines(scope.row.createTime) }}
            </div>
            <div class="text-gray font-mono whitespace-pre-line" v-else>
              {{ formatTimeTwoLines(scope.row.Created) }}
            </div>
          </template>
        </el-table-column>

        <el-table-column label="运行状态" width="160" sortable="custom" prop="status">
          <template #default="scope">
            <div class="status-indicator">
              <span class="status-point" :class="{
                'running': isRunning(scope.row.status || scope.row.state),
                'partial': scope.row.status === '部分运行',
                'stopped': !isRunning(scope.row.status || scope.row.state) && scope.row.status !== '部分运行'
              }"></span>
              <span>{{ toCnState(scope.row.status || scope.row.state) || '未知' }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="240" fixed="right" align="center">
          <template #default="scope">
            <div class="row-ops" v-if="scope.row.type === 'compose'">
              <el-tooltip content="启动项目" placement="top">
                <el-button circle size="default" type="primary" plain @click="startProject(scope.row)">
                  <el-icon><VideoPlay /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="停止项目" placement="top">
                <el-button circle size="default" type="warning" plain @click="stopProject(scope.row)">
                  <el-icon><VideoPause /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="编辑配置" placement="top">
                <el-button circle size="default" type="info" plain @click="editProject(scope.row)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-dropdown trigger="click" @command="(cmd) => handleProjectCommand(cmd, scope.row)">
                <el-button circle size="default" plain class="ml-2">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="down" :icon="CircleClose">清理 (Down)</el-dropdown-item>
                    <el-dropdown-item command="remove" :icon="Delete" divided class="text-danger">删除项目</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <div class="row-ops" v-else>
               <!-- Single Container Ops - Same as inner table for consistency, or simplified -->
               <el-button link type="primary" @click="goContainerDetail(scope.row.containers[0])" size="medium">详情</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="filteredItems.length"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>

    <!-- Components -->
    <ContainerTerminal
      v-model="terminalDialogVisible"
      :container="currentContainer"
    />
    
    <ContainerLogs
      v-model="logDialogVisible"
      :container="currentContainer"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Plus, Refresh, Remove, Search, VideoPlay, VideoPause, Edit, Delete, CircleClose,
  Folder, Box, Platform, Connection, MoreFilled, ArrowDown, Document, Monitor
} from '@element-plus/icons-vue'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
import ContainerTerminal from '../components/ContainerTerminal.vue'
import ContainerLogs from '../components/ContainerLogs.vue'

const router = useRouter()
const loading = ref(false)
const statusFilter = ref('')
const searchQuery = ref('')
const tableRef = ref(null)

// Sort state
const sortState = ref({
  prop: '',
  order: ''
})

const handleSortChange = ({ prop, order }) => {
  sortState.value = { prop, order }
}

const items = ref([])
const currentPage = ref(1)
const pageSize = ref(10)

// Dialog states
const terminalDialogVisible = ref(false)
const logDialogVisible = ref(false)
const currentContainer = ref(null)

const refreshAll = async () => {
  loading.value = true
  try {
    const [projects, containers] = await Promise.all([
      api.compose.list(),
      api.containers.list()
    ])

    const projectMap = new Map()
    for (const p of (projects || [])) {
      projectMap.set(p.name, {
        key: `project:${p.name}`,
        type: 'compose',
        name: p.name,
        status: toCnState(p.status),
        path: p.path,
        createTime: p.createTime, // Added createTime
        containers: []
      })
    }

    const singleContainers = []
    for (const c of (containers || [])) {
      const labels = c.Labels || c.Config?.Labels || {}
      const projectName = labels['com.docker.compose.project']
      const item = {
        key: `container:${c.Id}`,
        type: 'container',
        Id: c.Id,
        id: c.Id,
        Names: c.Names,
        name: (c.Names?.[0] || '').replace(/^\//, ''),
        Image: c.Image,
        image: c.Image,
        State: c.State,
        state: c.State,
        Created: c.Created, // Added Created
        Ports: c.Ports,     // Added Ports
        NetworkSettings: c.NetworkSettings, // Added NetworkSettings
        // Helper for status runtime if needed, though usually in Status string
        Status: c.Status
      }
      if (projectName && projectMap.has(projectName)) {
        const pj = projectMap.get(projectName)
        pj.containers.push(item)
      } else {
        singleContainers.push(item)
      }
    }

    // Update status for projects
    for (const pj of projectMap.values()) {
      const total = pj.containers.length
      if (total === 0) continue
      
      const runningCount = pj.containers.filter(c => isRunning(c.state)).length
      if (runningCount === total) {
        pj.status = '运行中'
      } else if (runningCount > 0) {
        pj.status = '部分运行'
      } else {
        pj.status = '已停止'
      }
    }
    
    // Process single containers to have 'containers' array for uniform structure
    const processedSingleContainers = singleContainers.map(c => ({
      ...c,
      // Create a wrapper for the container itself to show in expanded view
      containers: [c],
      // Ensure status is consistent
      status: toCnState(c.state)
    }))

    items.value = [...projectMap.values(), ...processedSingleContainers]
  } catch (e) {
    console.error(e)
    ElMessage.error('获取数据失败')
  } finally {
    loading.value = false
  }
}

const filteredItems = computed(() => {
  let list = items.value.slice()
  const q = (searchQuery.value || '').trim().toLowerCase()
  if (q) {
    list = list.filter(i => {
      const nameHit = (i.name || '').toLowerCase().includes(q)
      const childHit = (i.containers || []).some(c => (c.name || '').toLowerCase().includes(q))
      return nameHit || childHit
    })
  }
  if (statusFilter.value) {
    list = list.filter(i => {
      const s = toCnState(i.status || i.state || '')
      return s === statusFilter.value
    })
  }

  const { prop, order } = sortState.value
  if (prop && order) {
    list.sort((a, b) => {
      let valA, valB
      
      switch (prop) {
        case 'count':
          valA = a.containers?.length || 0
          valB = b.containers?.length || 0
          break
        case 'createTime':
           valA = a.type === 'compose' ? (a.createTime || '') : (a.Created || '')
           valB = b.type === 'compose' ? (b.createTime || '') : (b.Created || '')
           break
        case 'status':
           valA = toCnState(a.status || a.state)
           valB = toCnState(b.status || b.state)
           break
        default:
          valA = a[prop]
          valB = b[prop]
      }
      
      if (valA === valB) return 0
      
      // String comparison for strings, numeric for numbers
      if (typeof valA === 'string' && typeof valB === 'string') {
        return order === 'ascending' ? valA.localeCompare(valB) : valB.localeCompare(valA)
      }
      
      const compare = (valA > valB) ? 1 : -1
      return order === 'ascending' ? compare : -compare
    })
  }

  return list
})

const paginatedItems = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredItems.value.slice(start, end)
})

const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

const goCreateProject = () => router.push('/projects')

const handleGlobalAction = (cmd) => {
  if (cmd === 'prune') pruneContainers()
}

const handleProjectCommand = (cmd, row) => {
  if (cmd === 'down') downProject(row)
  if (cmd === 'remove') removeProject(row)
}

const pruneContainers = async () => {
  try {
    await ElMessageBox.confirm('确定要清理所有已停止的容器吗？此操作只会删除容器，不会影响镜像、网络或卷。', '清理容器', {
      confirmButtonText: '确认清理',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.containers.prune()
    ElMessage.success('清理完成')
    refreshAll()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('清理失败')
  }
}

const handleNameClick = (row) => {
  // Toggle row expansion instead of navigating
  if (tableRef.value) {
    tableRef.value.toggleRowExpansion(row)
  }
}

const goContainerDetail = (c) => {
  if (c && c.name) router.push(`/containers/${c.name}`)
}

const startProject = async (row) => {
  try {
    await api.compose.start(row.name)
    ElMessage.success('项目启动成功')
    setTimeout(refreshAll, 2000)
  } catch (e) {
    ElMessage.error('启动失败')
  }
}
const stopProject = async (row) => {
  try {
    await api.compose.stop(row.name)
    ElMessage.success('项目已停止')
    setTimeout(refreshAll, 2000)
  } catch (e) {
    ElMessage.error('停止失败')
  }
}
const editProject = (row) => router.push(`/projects/${row.name}`)
const downProject = async (row) => {
  try {
    await ElMessageBox.confirm(`清除 "${row.name}" 的容器与网络？保留文件。`, '提示', { type: 'warning' })
    await api.compose.down(row.name)
    ElMessage.success('清除完成')
    refreshAll()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('清除失败')
  }
}
const removeProject = async (row) => {
  try {
    await ElMessageBox.confirm(`删除项目 "${row.name}"？此操作不可恢复。`, '警告', { type: 'warning' })
    await api.compose.remove(row.name)
    ElMessage.success('删除完成')
    refreshAll()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

const startContainer = async (c) => {
  try {
    await api.containers.start(c.Id)
    ElMessage.success('容器启动成功')
    refreshAll()
  } catch (e) {
    ElMessage.error('启动失败')
  }
}
const stopContainer = async (c) => {
  try {
    await api.containers.stop(c.Id)
    ElMessage.success('容器已停止')
    refreshAll()
  } catch (e) {
    ElMessage.error('停止失败')
  }
}
const restartContainer = async (c) => {
  try {
    await api.containers.restart(c.Id)
    ElMessage.success('容器已重启')
    refreshAll()
  } catch (e) {
    ElMessage.error('重启失败')
  }
}
const deleteContainer = async (c) => {
  try {
    await ElMessageBox.confirm(`删除容器 "${c.name}"？`, '警告', { type: 'warning' })
    await api.containers.remove(c.Id)
    ElMessage.success('容器已删除')
    refreshAll()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

const toCnState = (s) => {
  if (!s) return ''
  const map = { running: '运行中', exited: '已停止', created: '已创建', paused: '已暂停', restarting: '重启中', dead: '异常' }
  return map[String(s).toLowerCase()] || s
}

const isRunning = (s) => {
  const t = String(s || '').toLowerCase()
  return t === 'running' || t === '运行中'
}

// Helpers
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

const getContainerIP = (container) => {
  if (container.NetworkSettings?.Networks?.host) return 'host'
  return container.NetworkSettings?.Networks?.bridge?.IPAddress || '-'
}

const formatPortWithIP = (port) => {
  if (port.PublicPort) {
    const ip = port.IP || '0.0.0.0'
    return `${ip}:${port.PublicPort}:${port.PrivatePort}/${port.Type}`
  }
  return `${port.PrivatePort}/${port.Type}`
}

const getNetworkNames = (container) => {
  const nets = container.NetworkSettings?.Networks || {}
  return Object.keys(nets)
}

const openTerminal = (container) => {
  currentContainer.value = container
  nextTick(() => {
    terminalDialogVisible.value = true
  })
}

const openLogs = (container) => {
  currentContainer.value = container
  logDialogVisible.value = true
}

const isManagedProject = (path) => {
  if (!path) return false
  if (String(path).startsWith('data/project/')) return true
  const normalizedPath = String(path).replace(/\\/g, '/')
  return normalizedPath.includes('/data/project/')
}

onMounted(refreshAll)
</script>

<style scoped>
.compose-view {
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
  min-height: 0;
}

/* Custom Table Styles */
.project-name-cell {
  display: flex;
  align-items: center;
  gap: 16px;
  cursor: pointer;
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

.project-name-cell:hover .icon-wrapper {
  transform: scale(1.05);
}

.icon-wrapper.compose {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.icon-wrapper.container {
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
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

.path-text {
  font-size: 12px;
}

.type-tag {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-start;
  font-weight: 500;
}

.count-badge {
  background: var(--el-fill-color-lighter);
  color: var(--el-text-color-regular);
  padding: 4px 12px;
  border-radius: 16px;
  font-size: 14px;
  font-weight: 600;
  display: inline-block;
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
  background-color: var(--el-text-color-disabled);
}

.status-point.partial {
  background-color: #f59e0b;
}

.row-ops {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  align-items: center;
}

.text-danger {
  color: #ef4444;
}

.ml-2 {
  margin-left: 8px;
}

/* Expanded Row Styles */
.expanded-container {
  padding: 16px 24px;
  background: var(--el-fill-color-light);
  border-left: 4px solid var(--el-color-primary);
  margin: 0;
}

.expanded-header {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.inner-table {
  background: transparent !important;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-header-bg-color: transparent;
}

.container-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  transition: color 0.2s;
}

.container-name-cell:hover {
  color: var(--el-color-primary);
}

.container-name-text {
  font-size: 14px;
  font-weight: 500;
}

.container-icon {
  color: var(--el-text-color-secondary);
}

.op-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.empty-expand {
  padding: 20px;
  text-align: center;
  color: var(--el-text-color-placeholder);
  font-size: 14px;
}

/* Pagination */
.pagination-bar {
  padding: 16px 24px;
  border-top: 1px solid var(--el-border-color-lighter);
  display: flex;
  justify-content: flex-end;
}

/* Override Element Styles for cleaner look */
:deep(.el-table__expanded-cell) {
  padding: 0 !important;
}

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

 /* New Styles from Docker.vue */
 .image-cell { display: flex; flex-direction: column; gap: 4px; }
 .image-name { font-size: 13px; color: var(--el-text-color-regular); }
 .image-tag { width: fit-content; }
 .image-inline { font-size: 13px; color: var(--el-text-color-regular); }
 .ports-list { display: flex; flex-wrap: wrap; gap: 4px; }
 .port-tag { border: none; background: var(--el-fill-color); color: var(--el-text-color-secondary); }
 .more-ports { background: var(--el-fill-color); color: var(--el-text-color-secondary); }
 .ports-tooltip-content { display: flex; flex-direction: column; gap: 4px; padding: 4px; }
 .port-item { font-size: 12px; }
 .text-gray { color: var(--el-text-color-secondary); }
 .font-mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
 .whitespace-pre-line { white-space: pre-line; }
 .networks-list { display: flex; flex-wrap: wrap; gap: 4px; }
 .network-tag { border: none; background: var(--el-fill-color); color: var(--el-text-color-secondary); }
 </style>
