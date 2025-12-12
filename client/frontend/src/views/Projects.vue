<template>
  <div class="projects-view compact-table">
    <div class="operation-bar">
      <div class="left-ops">
        <el-button-group>
          <el-button @click="handleRefresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
          <el-button type="primary" @click="handleCreate">
            <el-icon class="el-icon--left"><Plus /></el-icon>
            新建项目
          </el-button>
        </el-button-group>
      </div>
    </div>

    <div class="projects-table">
      <el-table 
        :data="paginatedProjects" 
        style="width: 100%" 
        height="100%"
        v-loading="loading">
        <el-table-column type="selection" width="40" align="center" header-align="left" />
        <el-table-column prop="name" label="名称" min-width="100" show-overflow-tooltip header-align="left">
          <template #default="scope">
            <span 
              :class="['clickable-name', { 'is-disabled': !scope.row.path.startsWith('data/project/') }]"
              @click="scope.row.path.startsWith('data/project/') && handleRowClick(scope.row)"
            >
              {{ scope.row.name }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="containers" label="容器数量" width="100" align="center" header-align="left" />
        <el-table-column prop="status" label="运行状态" width="100" header-align="left">
          <template #default="scope">
            <div class="status-cell">
              <span :class="['status-dot', scope.row.status === '运行中' ? 'status-running' : 'status-stopped']"></span>
              {{ scope.row.status }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160" header-align="left">
          <template #default="scope">
            <div class="text-gray font-mono text-center whitespace-pre-line">
              {{ formatTimeTwoLines(scope.row.createTime) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="项目路径" min-width="130" show-overflow-tooltip header-align="left">
          <template #default="scope">
            <span v-if="scope.row.path.startsWith('data/project/')" class="text-gray">{{ scope.row.path }}</span>
            <span v-else class="text-gray">非本项目创建，请在容器管理中操作</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right" header-align="left">
          <template #default="scope">
            <el-button-group>
              <el-tooltip content="启动" placement="top">
                <el-button 
                  link
                  type="primary" 
                  @click="handleStart(scope.row)" 
                  :disabled="scope.row.status === '运行中' || !scope.row.path.startsWith('data/project/')">
                  <el-icon><VideoPlay /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="停止" placement="top">
                <el-button 
                  link
                  type="warning" 
                  @click="handleStop(scope.row)" 
                  :disabled="scope.row.status !== '运行中' || !scope.row.path.startsWith('data/project/')">
                  <el-icon><VideoPause /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="编辑" placement="top">
                <el-button link type="primary" @click="handleEdit(scope.row)" :disabled="!scope.row.path.startsWith('data/project/')">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="清除(保留文件)" placement="top">
                <el-button link type="warning" @click="handleDown(scope.row)" :disabled="!scope.row.path.startsWith('data/project/')">
                  <el-icon><CircleClose /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button link type="danger" @click="handleDelete(scope.row)" :disabled="!scope.row.path.startsWith('data/project/')">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 分页 -->
    <div class="pagination">
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

    <!-- 新建/编辑项目对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="800px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      class="project-dialog"
      append-to-body
      @close="handleDialogClose"
    >
      <el-form :model="projectForm" label-width="100px" class="compact-form">
        <el-form-item label="项目名称" required>
          <el-input v-model="projectForm.name" placeholder="请输入项目名称" />
        </el-form-item>
        <el-form-item label="存放路径" required>
          <el-input v-model="projectForm.path" placeholder="自动生成" readonly>
            <template #append>
              <el-tooltip content="项目将存放在 data/project 目录下">
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
      <!-- 添加部署日志区域 -->
      <div v-if="deployLogs.length > 0" class="deploy-logs">
        <div class="logs-header">
          <span>部署日志</span>
        </div>
        <div ref="logsContent" class="logs-content">
          <div v-for="(log, index) in deployLogs" :key="index" :class="['log-line', log.type]">
            {{ log.message }}
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
import { formatTimeTwoLines } from '../utils/format'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, InfoFilled, ArrowDown, VideoPlay, VideoPause, Edit, Delete, CircleClose } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import api from '../api'

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

// 编辑器配置——初始化
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

// 修改对话框显示处理
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

// 监听项目名称变化，自动更新路径
watch(() => projectForm.value.name, (newName) => {
  if (dialogTitle.value === '新建项目') {
    const basePath = '/home/ttdocker/docker-manager/client/backend/data/project'
    projectForm.value.path = newName ? `${basePath}/${newName}` : basePath
  }
})

const projectList = ref([])

// 分页相关变量
const currentPage = ref(1)
const pageSize = ref(10)

// 计算分页后的项目列表
const paginatedProjects = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return projectList.value.slice(start, end)
})

// 分页处理函数
const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1 // 重置到第一页
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

// 修改保存方法，确保正确处理日志
const handleSave = async () => {
  if (!projectForm.value.name || !projectForm.value.compose) {
    ElMessage.warning('请填写必要信息')
    return
  }

  // 检查项目名称是否已存在 (仅新建时)
  if (dialogTitle.value === '新建项目') {
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
    // 获取 Token
    const token = localStorage.getItem('token')
    // 将 Token 作为 Query 参数传递
    const eventSource = new EventSource(
      `/api/compose/deploy/events?name=${projectForm.value.name}&compose=${encodedCompose}&token=${token}`
    )
    
    // 添加超时处理
    const timeout = setTimeout(() => {
      deployLogs.value.push({
        type: 'warning',
        message: '部署超时，请检查服务器状态'
      })
      eventSource.close()
    }, 60000) // 60秒超时
	
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        // 添加日志
        deployLogs.value.push(data)
      
      // 自动滚动到底部
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
        }, 1000)
      } else if (data.type === 'error') {
          clearTimeout(timeout)
          ElMessage.error(data.message)
        }
    }catch (error) {
        deployLogs.value.push({
          type: 'error',
          message: `解析服务器消息失败: ${error.message}`
        })
      }
    }
	
	eventSource.onerror = (event) => {
      clearTimeout(timeout)
      deployLogs.value.push({
        type: 'error',
        message: '与服务器连接中断，部署可能已失败'
      })
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

onMounted(() => {
  handleRefresh()
})

const router = useRouter()

const handleRowClick = (row) => {
  router.push(`/projects/${row.name}`)
}

// 添加启动处理函数
const handleStart = async (row) => {
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
/* 继承 layout.css 的 compact-table 样式 */

.projects-view {
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

.projects-table {
  flex: 1;
  min-height: 0;
}

.clickable-name {
  font-weight: 500;
  color: var(--el-color-primary);
  cursor: pointer;
}

.clickable-name:hover {
  text-decoration: underline;
}

.clickable-name.is-disabled {
  color: var(--el-text-color-secondary);
  cursor: not-allowed;
}

.clickable-name.is-disabled:hover {
  text-decoration: none;
}

.text-gray {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status-running {
  background-color: var(--el-color-success);
}

.status-stopped {
  background-color: var(--el-color-info);
}

/* 覆盖 Element Plus 样式 */
:deep(.el-table__row) {
  height: 44px;
}

:deep(.el-button--link) {
  padding: 4px;
  height: auto;
}

/* 编辑器相关样式 */
.compose-editor-container {
  width: 100%; /* 确保容器占满父元素宽度 */
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
  box-sizing: border-box; /* 包含边框和内边距 */
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

/* 部署日志样式 */
.deploy-logs {
  margin-top: 16px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.logs-header {
  padding: 8px 12px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
  font-size: 13px;
  font-weight: 500;
}

.logs-content {
  height: 200px;
  overflow-y: auto;
  padding: 12px;
  background-color: #1e1e1e;
  color: #d4d4d4;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
}

.log-line {
  margin-bottom: 2px;
}

.log-line.error {
  color: #f87171;
}

.log-line.success {
  color: #4ade80;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-start;
}
</style>