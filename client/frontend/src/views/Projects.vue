<template>
  <div class="projects-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索项目名称..."
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
        <el-button-group>
          <el-button @click="handleRefresh" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
          <el-button type="primary" @click="handleCreate" size="medium">
            <template #icon><el-icon><Plus /></el-icon></template>
            新建项目
          </el-button>
        </el-button-group>
      </div>
    </div>

    <el-alert
      v-if="hasSelfProject"
      type="info"
      effect="light"
      title="只读模式"
      description="容器化部署模式下，自身项目/容器不支持操作"
      :closable="false"
      class="self-resource-alert"
    />

    <div class="table-wrapper">
      <el-table 
        :data="paginatedProjects" 
        style="width: 100%" 
        class="main-table"
        v-loading="loading"
        @sort-change="handleSortChange"
        :default-sort="{ prop: 'name', order: 'ascending' }"
        :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: '60px' }"
      >
        <el-table-column type="selection" width="40" align="center" header-align="left" />
        <el-table-column label="名称" prop="name" sortable="custom" min-width="240" show-overflow-tooltip header-align="left">
          <template #default="scope">
            <div class="project-name-cell" 
                 :class="{ 'clickable': scope.row.path.startsWith('project/') }"
                 @click="scope.row.path.startsWith('project/') && handleRowClick(scope.row)">
              <div class="icon-wrapper">
                <el-icon><Folder /></el-icon>
              </div>
              <div class="name-info">
                <span class="name-text">{{ scope.row.name }}</span>
                <el-tag v-if="isSelfProject(scope.row)" size="small" type="warning" effect="plain" style="margin-left: 8px">自身</el-tag>
                <span class="path-text text-gray">{{ scope.row.path }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="容器数量" prop="containers" sortable="custom" width="120" align="center">
          <template #default="scope">
            <div class="count-badge">
              {{ scope.row.containers || 0 }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="运行状态" prop="status" sortable="custom" width="140" header-align="left">
          <template #default="scope">
            <div class="status-indicator">
              <span class="status-point" :class="scope.row.status === '运行中' ? 'running' : 'stopped'"></span>
              <span>{{ scope.row.status }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="createTime" sortable="custom" width="160" header-align="left">
          <template #default="scope">
            <div class="text-gray font-mono text-center whitespace-pre-line">
              {{ formatTimeTwoLines(scope.row.createTime) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right" align="center">
          <template #default="scope">
            <div class="row-ops">
              <el-tooltip content="启动" placement="top">
                <el-button 
                  circle plain size="default"
                  type="primary" 
                  @click="handleStart(scope.row)" 
                  :disabled="scope.row.status === '运行中' || !isManagedProject(scope.row.path) || isSelfProject(scope.row)">
                  <el-icon><VideoPlay /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="停止" placement="top">
                <el-button 
                  circle plain size="default"
                  type="warning" 
                  @click="handleStop(scope.row)" 
                  :disabled="scope.row.status !== '运行中' || !isManagedProject(scope.row.path) || isSelfProject(scope.row)">
                  <el-icon><VideoPause /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="编辑" placement="top">
                <el-button circle plain size="default" type="primary" @click="handleEdit(scope.row)" :disabled="!isManagedProject(scope.row.path) || isSelfProject(scope.row)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              
              <el-dropdown trigger="click" @command="(cmd) => handleProjectCommand(cmd, scope.row)" :disabled="isSelfProject(scope.row)">
                <el-button circle size="default" plain class="ml-2">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="down" :icon="CircleClose" :disabled="!isManagedProject(scope.row.path) || isSelfProject(scope.row)">清除(保留文件)</el-dropdown-item>
                    <el-dropdown-item command="delete" :icon="Delete" divided class="text-danger" :disabled="!isManagedProject(scope.row.path) || isSelfProject(scope.row)">删除项目</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 分页 -->
    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        :total="projectList.length"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        size="small"
      />
    </div>

    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="1200px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      class="project-dialog"
      append-to-body
      @close="handleDialogClose"
    >
      <div class="project-dialog-body">
        <div class="project-form-column">
          <el-form :model="projectForm" label-width="100px" class="compact-form">
            <el-form-item label="项目名称" required>
              <el-input v-model="projectForm.name" placeholder="请输入项目名称" />
            </el-form-item>
            <el-form-item label="存放路径" required>
              <el-input v-model="projectForm.path" placeholder="自动生成" readonly>
                <template #append>
                  <el-tooltip content="项目将存放在 project 目录下">
                    <el-icon><InfoFilled /></el-icon>
                  </el-tooltip>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="Compose" required>
              <div class="compose-editor-container">
                <div class="editor-toolbar">
                  <span class="file-name">docker-compose.yml</span>
                  <el-dropdown @command="handleTemplateSelect" trigger="click">
                    <el-button size="small" link type="primary">
                      插入模板<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                    </el-button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="nginx">Nginx</el-dropdown-item>
                        <el-dropdown-item command="mysql">MySQL</el-dropdown-item>
                        <el-dropdown-item command="redis">Redis</el-dropdown-item>
                        <el-dropdown-item command="wordpress">WordPress</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
                <div ref="editorContainer" class="monaco-editor-wrapper"></div>
              </div>
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="projectForm.autoStart">创建完成后立即运行</el-checkbox>
            </el-form-item>
          </el-form>
        </div>
        <div class="project-logs-column">
          <div class="deploy-logs">
            <div class="logs-header">
              <span>部署日志</span>
            </div>
            <div ref="logsContent" class="logs-content">
              <div
                v-if="deployLogs.length === 0"
                class="log-empty"
              >
                暂无部署日志，点击“立即部署”后将在此展示实时输出。
              </div>
              <div
                v-else
                v-for="(log, index) in deployLogs"
                :key="index"
                :class="['log-line', log.type]"
              >
                {{ log.message }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSave">立即部署</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
// 修改导入语句，添加 nextTick
import { ref, onMounted, shallowRef, nextTick, onBeforeUnmount, computed, watch } from 'vue'
import { formatTimeTwoLines, normalizeComposeProjectName } from '../utils/format'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, InfoFilled, ArrowDown, VideoPlay, VideoPause, Edit, Delete, CircleClose, Search, Folder, MoreFilled } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import api from '../api'
import request from '../utils/request'

// 判断是否为本项目管理的项目
const isManagedProject = (path) => {
  if (!path) return false
  // 检查相对路径
  if (path.startsWith('project/')) return true
  // 检查绝对路径（包含 data/project 目录）
  // 兼容 Windows 反斜杠
  const normalizedPath = path.replace(/\\/g, '/')
  return normalizedPath.includes('project/')
}

// 编辑器配置——新增
const editorInstance = shallowRef(null)
const editorContainer = ref(null)

// 编辑器配置——修改
const editorOptions = {
  value: '',
  language: 'yaml',
  theme: 'vs',
  automaticLayout: true,
  minimap: { enabled: false },
  lineNumbers: 'on',
  roundedSelection: false,
  scrollBeyondLastLine: false,
  fontSize: 14,
  tabSize: 2,
  renderWhitespace: 'all',
  readOnly: false,
  contextmenu: true,
  selectOnLineNumbers: true,
  multiCursorModifier: 'alt',
  wordWrap: 'on',
  dragAndDrop: true,
  formatOnPaste: true,
  mouseWheelZoom: true,
  folding: true,
  links: true,
  copyWithSyntaxHighlighting: true
}

const initEditor = () => {
  if (editorContainer.value) {
    // 如果已存在编辑器实例，先销毁
    if (editorInstance.value) {
      editorInstance.value.dispose()
    }
    
    editorInstance.value = monaco.editor.create(editorContainer.value, editorOptions)
    
    // 监听内容变化
    editorInstance.value.onDidChangeModelContent(() => {
      projectForm.value.compose = editorInstance.value.getValue()
    })

    // 添加快捷键支持
    editorInstance.value.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      handleSave()
    })
  }
}

// 编辑器配置——在组件卸载时清理
onBeforeUnmount(() => {
  if (editorInstance.value) {
    editorInstance.value.dispose()
  }
})

// 修改模板插入方法
const insertTemplate = () => {
  const template = `version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    volumes:
      - ./data:/usr/share/nginx/html
    environment:
      - TZ=Asia/Shanghai
    restart: always`
    
  if (editorInstance.value) {
    editorInstance.value.setValue(template)
  }
}

// 修改对话框关闭处理
const handleDialogClose = () => {
  dialogVisible.value = false
  // 清空部署日志
  deployLogs.value = []
}

const handleCreate = () => {
  dialogTitle.value = '新建项目'
  projectForm.value = {
    name: '',
    path: '',
    compose: '',
    autoStart: true
  }
  dialogVisible.value = true
  deployLogs.value = []
  // 等待 DOM 更新后初始化编辑器
  nextTick(() => {
    initEditor()
  })
}
const handleRefresh = async () => {
  loading.value = true
  try {
    // 调用后端 API 获取项目列表
    const response = await api.compose.list()
    projectList.value = response
  } catch (error) {
    ElMessage.error('获取项目列表失败')
  } finally {
    loading.value = false
  }
}
// 修改编辑处理函数
const handleEdit = async (row) => {
  // 检查项目状态
  if (row.status === '运行中') {
    ElMessage.warning('请先停止项目，然后再进行编辑')
    return
  }

  dialogTitle.value = '编辑项目'
  projectForm.value = { ...row }
  dialogVisible.value = true
  nextTick(() => {
    initEditor()
    if (editorInstance.value) {
      editorInstance.value.setValue(row.compose || '')
    }
  })
}

// 添加日志内容引用
const loading = ref(false)
const dialogVisible = ref(false)
const logsContent = ref(null)
const deployLogs = ref([])
const dialogTitle = ref('新建项目')
const projectForm = ref({
  name: '',
  path: '',
  compose: '',
  autoStart: true
})
const projectRoot = ref('')

watch(() => projectForm.value.name, (newName) => {
  if (dialogTitle.value === '新建项目') {
    const basePath = projectRoot.value ? String(projectRoot.value).replace(/\/$/, '') : 'project'
    const normalized = normalizeComposeProjectName(newName)
    projectForm.value.path = normalized ? `${basePath}/${normalized}` : basePath
  }
})

const projectList = ref([])

const isSelfProject = (row) => !!row?.isSelf
const hasSelfProject = computed(() => (projectList.value || []).some((row) => isSelfProject(row)))

const searchQuery = ref('')
const sortState = ref({ prop: '', order: '' })

const filteredProjects = computed(() => {
  let list = projectList.value
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    list = projectList.value.filter(p =>
      p.name.toLowerCase().includes(query) ||
      (p.path && p.path.toLowerCase().includes(query))
    )
  }
  const { prop, order } = sortState.value
  if (prop && order) {
    list = list.slice().sort((a, b) => {
      let valA, valB
      switch (prop) {
        case 'containers':
          valA = a.containers || 0
          valB = b.containers || 0
          break
        case 'createTime':
          valA = a.createTime || ''
          valB = b.createTime || ''
          break
        default:
          valA = a[prop]
          valB = b[prop]
      }
      if (typeof valA === 'string' && typeof valB === 'string') {
        return order === 'ascending' ? valA.localeCompare(valB) : valB.localeCompare(valA)
      }
      if (valA < valB) return order === 'ascending' ? -1 : 1
      if (valA > valB) return order === 'ascending' ? 1 : -1
      return 0
    })
  }
  return list
})

// 分页相关变量
const currentPage = ref(1)
const pageSize = ref(10)

// 计算分页后的项目列表
const paginatedProjects = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredProjects.value.slice(start, end)
})

// 分页处理函数
const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1 // 重置到第一页
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

const handleSortChange = ({ prop, order }) => {
  if (!prop || !order) {
    sortState.value = { prop: '', order: '' }
    return
  }
  sortState.value = { prop, order }
  try {
    const v = JSON.stringify(sortState.value)
    request.post('/settings/kv/sort_projects', { value: v })
  } catch (e) {}
}

const handleSave = async () => {
  if (!projectForm.value.name || !projectForm.value.compose) {
    ElMessage.warning('请填写必要信息')
    return
  }

  if (dialogTitle.value === '新建项目') {
    const normalizedName = normalizeComposeProjectName(projectForm.value.name)

    if (normalizedName !== projectForm.value.name) {
      try {
        await ElMessageBox.confirm(
          `当前项目名称 "${projectForm.value.name}" 包含 Docker Compose 不支持的字符，将使用规范化名称 "${normalizedName}" 作为项目名进行部署（目录及项目列表将以该名称显示）。是否继续？`,
          '项目名称规范化',
          {
            confirmButtonText: '继续部署',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      } catch (e) {
        return
      }
      projectForm.value.name = normalizedName
    }

    const exists = projectList.value.some(p => p.name === projectForm.value.name)
    if (exists) {
      ElMessage.warning('该项目名称已存在，请使用其他名称')
      return
    }
  }

  // 如果是编辑模式，先确认
  if (dialogTitle.value === '编辑项目') {
    try {
      await ElMessageBox.confirm(
        '重新部署会删除原有项目并重新创建，是否继续？',
        '警告',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }
      )
      
      // 先删除原项目
      await api.compose.remove(projectForm.value.name)
      
    } catch (error) {
      if (error === 'cancel') {
        return
      }
      ElMessage.error(`删除原项目失败: ${error.message || '未知错误'}`)
      return
    }
  }

  // 清空部署日志
  deployLogs.value = []
  
  // 添加基本的YAML格式校验
  try {
    // 可以引入js-yaml库进行更严格的校验
    if (!projectForm.value.compose.includes('services:')) {
      throw new Error('YAML格式错误：缺少services定义')
    }
    // 检查常见错误
    if (projectForm.value.compose.includes('/binsh')) {
      deployLogs.value.push({
        type: 'warning',
        message: '警告：检测到可能的路径错误，"/binsh" 应该为 "/bin/sh"'
      })
      // 自动修复
      projectForm.value.compose = projectForm.value.compose.replace(/\/binsh\b/g, '/bin/sh')
    }
  } catch (error) {
    deployLogs.value.push({
      type: 'error',
      message: `配置验证失败: ${error.message}`
    })
    ElMessage.error(`配置验证失败: ${error.message}`)
    return
  }
  
  try {
    const encodedCompose = encodeURIComponent(projectForm.value.compose)
    const token = localStorage.getItem('token') || ''
    const eventSource = new EventSource(
      `/api/compose/deploy/events?name=${projectForm.value.name}&compose=${encodedCompose}&token=${token}`
    )

    const logSet = new Set()
    const timeout = setTimeout(() => {
      deployLogs.value.push({
        type: 'warning',
        message: '日志连接超时，请稍后在项目列表查看状态'
      })
      eventSource.close()
    }, 600000) // 10分钟超时

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        const key = `${data.type}:${data.message}`
        if (!logSet.has(key)) {
          logSet.add(key)
          deployLogs.value.push({
            type: data.type,
            message: data.message
          })
        }
        nextTick(() => {
          if (logsContent.value) {
            logsContent.value.scrollTop = logsContent.value.scrollHeight
          }
        })
        if (data.type === 'success' && data.message.includes('所有服务已成功启动')) {
          clearTimeout(timeout)
          ElMessage.success(data.message)
          setTimeout(() => {
            eventSource.close()
            dialogVisible.value = false
            handleRefresh()
          }, 500)
        } else if (data.type === 'error') {
          // 错误行直接提示，但保持连接，便于继续接收日志
          ElMessage.error(data.message)
        }
      } catch (e) {
        deployLogs.value.push({
          type: 'error',
          message: `解析服务器消息失败: ${e.message}`
        })
      }
    }

    eventSource.onerror = () => {
      clearTimeout(timeout)
      // 若已收到成功提示，则忽略断连提示
      const hasSuccess = deployLogs.value.some(l => l.type === 'success')
      if (!hasSuccess) {
        deployLogs.value.push({
          type: 'error',
          message: '与服务器连接中断，部署可能已失败'
        })
      }
      eventSource.close()
    }
  } catch (error) {
    deployLogs.value.push({
      type: 'error',
      message: `部署失败: ${error.message || '未知错误'}`
    })
    ElMessage.error(`部署失败: ${error.message || '未知错误'}`)
  }
}

onMounted(async () => {
  try {
    const res = await request.get('/settings/kv/sort_projects')
    if (res && res.value) {
      const s = JSON.parse(res.value)
      if (s && s.prop && s.order) {
        sortState.value = s
      }
    }
  } catch (e) {}
  try {
    const res = await api.system.info()
    const data = res?.data || res
    if (data && typeof data.ProjectRoot === 'string') {
      projectRoot.value = data.ProjectRoot
    }
  } catch (e) {}
  handleRefresh()
})

const router = useRouter()

const handleRowClick = (row) => {
  router.push(`/projects/${row.name}`)
}

// 添加启动处理函数
const handleStart = async (row) => {
  if (isSelfProject(row)) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  try {
    await api.compose.start(row.name)
    ElMessage.success('项目启动成功')
    handleRefresh()
  } catch (error) {
    ElMessage.error(`启动失败: ${error.message || '未知错误'}`)
  }
}

// 添加停止处理函数
const handleStop = async (row) => {
  if (isSelfProject(row)) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  try {
    await api.compose.stop(row.name)
    ElMessage.success('项目已停止')
    handleRefresh()
  } catch (error) {
    ElMessage.error(`停止失败: ${error.message || '未知错误'}`)
  }
}

// 添加清除(Down)处理函数
const handleDown = (row) => {
  if (isSelfProject(row)) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  ElMessageBox.confirm(
    `确定要清除项目 "${row.name}" 的容器和网络吗？\n这将停止并删除容器，但保留项目文件。`,
    '提示',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    try {
      await api.compose.down(row.name)
      ElMessage.success('清除成功')
      handleRefresh()
    } catch (error) {
      ElMessage.error(`清除失败: ${error.message || '未知错误'}`)
    }
  }).catch(() => {})
}

// 添加删除处理函数
const handleDelete = (row) => {
  if (isSelfProject(row)) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  ElMessageBox.confirm(
    '确定要删除该项目吗？此操作将停止并删除所有相关容器。',
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    try {
      await api.compose.remove(row.name)
      ElMessage.success('项目已删除')
      handleRefresh()
    } catch (error) {
      ElMessage.error(`删除失败: ${error.message || '未知错误'}`)
    }
  }).catch(() => {})
}

const handleProjectCommand = (command, row) => {
  if (isSelfProject(row)) {
    ElMessage.warning('容器化部署模式下，不支持操作自身项目')
    return
  }
  if (command === 'down') {
    handleDown(row)
  } else if (command === 'delete') {
    handleDelete(row)
  }
}

// 模板选择处理函数
const handleTemplateSelect = (command) => {
  let template = ''
  
  switch (command) {
    case 'nginx':
      template = `version: '3'
services:
  nginx:
    image: nginx:latest
    ports:
      - "80:80"
    volumes:
      - ./data:/usr/share/nginx/html
    environment:
      - TZ=Asia/Shanghai
    restart: always`
      break
    case 'mysql':
      template = `version: '3'
services:
  mysql:
    image: mysql:8
    ports:
      - "3306:3306"
    environment:
      - MYSQL_ROOT_PASSWORD=password
      - MYSQL_DATABASE=mydb
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/var/lib/mysql
    restart: always`
      break
    case 'redis':
      template = `version: '3'
services:
  redis:
    image: redis:latest
    ports:
      - "6379:6379"
    volumes:
      - ./data:/data
    command: redis-server --appendonly yes
    restart: always`
      break
    case 'wordpress':
      template = `version: '3'
services:
  wordpress:
    image: wordpress:latest
    ports:
      - "80:80"
    environment:
      - WORDPRESS_DB_HOST=db
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress
      - WORDPRESS_DB_NAME=wordpress
      - TZ=Asia/Shanghai
    volumes:
      - ./wordpress:/var/www/html
    depends_on:
      - db
    restart: always
  
  db:
    image: mysql:5.7
    environment:
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=wordpress
      - MYSQL_PASSWORD=wordpress
      - MYSQL_RANDOM_ROOT_PASSWORD=yes
      - TZ=Asia/Shanghai
    volumes:
      - ./db:/var/lib/mysql
    restart: always`
      break
  }
  
  if (editorInstance.value && template) {
    editorInstance.value.setValue(template)
  }
}

</script>

<style scoped>
.projects-view {
  height: 100%;
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

.self-resource-alert {
  margin: 0 0 12px;
  border-radius: 12px;
}

.filter-left, .filter-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.search-input {
  width: 300px;
}

/* 表格容器 */
.table-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-light);
}

.main-table {
  flex: 1;
}

/* Custom Table Styles */
.project-name-cell {
  display: flex;
  align-items: center;
  gap: 16px;
  cursor: default;
  padding: 8px 0;
}

.project-name-cell.clickable {
  cursor: pointer;
}

.project-name-cell.clickable:hover .icon-wrapper {
  transform: scale(1.05);
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
  transition: transform 0.2s;
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
  color: var(--el-text-color-secondary);
}

/* Count Badge */
.count-badge {
  background: var(--el-fill-color);
  color: var(--el-text-color-regular);
  padding: 4px 12px;
  border-radius: 16px;
  font-size: 14px;
  font-weight: 600;
  display: inline-block;
}

/* Status Indicator */
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

/* Action Buttons */
.row-ops {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  align-items: center;
}

.text-danger {
  color: var(--el-color-danger);
}

.ml-2 {
  margin-left: 8px;
}

/* Pagination */
.pagination-bar {
  padding: 16px 24px;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
}

/* Utils */
.text-gray {
  color: var(--el-text-color-secondary);
}

.font-mono {
  font-family: monospace;
}

.whitespace-pre-line {
  white-space: pre-line;
}

/* 编辑器相关样式 */
.compose-editor-container {
  width: 100%;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
  box-sizing: border-box;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
}

.file-name {
  font-size: 13px;
  color: var(--el-text-color-regular);
  font-family: monospace;
}

.monaco-editor-wrapper {
  height: 400px;
  width: 100%;
}

.project-dialog-body {
  display: flex;
  gap: 16px;
  align-items: stretch;
}

.project-form-column {
  flex: 3;
  min-width: 0;
}

.project-logs-column {
  flex: 2;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.deploy-logs {
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.logs-header {
  padding: 8px 12px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
  font-size: 13px;
  font-weight: 500;
}

.logs-content {
  background-color: var(--el-fill-color-darker);
  color: var(--el-text-color-primary);
  padding: 16px;
  font-family: monospace;
  height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.log-line {
  margin-bottom: 2px;
}

.log-empty {
  color: var(--el-text-color-secondary);
}

.log-line.error {
  color: #f87171;
}

.log-line.success {
  color: #4ade80;
}

/* Override Element Styles */
:deep(.el-button--medium) {
  padding: 10px 20px;
  height: 36px;
}

:deep(.el-table th.el-table__cell) {
  background-color: var(--el-fill-color-light) !important;
}
</style>
