<template>
  <div class="project-detail compact-table">
    <div class="header">
      <div class="title">
        <el-button link @click="goBack">
          <el-icon><Back /></el-icon>
        </el-button>
        {{ projectName }}
      </div>
      <div class="actions">
        <el-button-group>
          <el-button type="primary" :disabled="!isRunning" @click="handleStop">
            停止
          </el-button>
          <el-button type="primary" :disabled="isRunning" @click="handleStart">
            启动
          </el-button>
          <el-button type="primary" @click="handleRestart">
            重启
          </el-button>
          <el-button type="warning" @click="handleDown">
            清除
          </el-button>
        </el-button-group>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="detail-tabs" v-loading="isLoading">
      <el-tab-pane label="YAML配置" name="yaml">
        <div class="yaml-editor">
          <div class="editor-header">
            <span>docker-compose.yml</span>
            <div class="editor-actions">
              <el-button type="primary" size="small" @click="handleSaveYaml">
                保存
              </el-button>
            </div>
          </div>
          <el-input
            v-model="yamlContent"
            type="textarea"
            :rows="20"
            class="yaml-textarea"
            :spellcheck="false"
          />
        </div>
      </el-tab-pane>

      <el-tab-pane label="容器" name="containers">
        <el-table :data="containerList" style="width: 100%">
          <el-table-column type="index" label="序号" width="80" header-align="center" />
          <el-table-column prop="name" label="名称" width="150" header-align="center" />
          <el-table-column prop="image" label="镜像" width="150" header-align="center" />
          <el-table-column prop="status" label="状态" width="100" header-align="center">
            <template #default="scope">
              <el-tag :type="scope.row.status === 'running' ? 'success' : 'info'">
                {{ scope.row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="cpu" label="CPU" min-width="100" header-align="center" />
          <el-table-column prop="memory" label="内存" min-width="100" header-align="center" />
          <el-table-column prop="network" label="网络" min-width="120" header-align="center" />
          <el-table-column label="操作" width="150" fixed="right" header-align="center">
            <template #default="scope">
              <el-button-group>
                <el-button 
                  size="small" 
                  type="primary"
                  @click="handleContainerRestart(scope.row)"
                >
                  重启
                </el-button>
                <el-button 
                  size="small" 
                  type="danger"
                  @click="handleContainerStop(scope.row)"
                >
                  停止
                </el-button>
              </el-button-group>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="日志" name="logs">
        <div class="logs-container">
          <div class="logs-header">
            <el-switch
              v-model="autoScroll"
              active-text="自动滚动"
            />
            <el-button @click="handleClearLogs">
              清空日志
            </el-button>
          </div>
          <div class="logs-content" ref="logsRef">
            <pre v-for="(log, index) in logs" :key="index" :class="log.type">
              {{ log.content }}
            </pre>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Back, CircleClose } from '@element-plus/icons-vue'
import api from '../api'

const route = useRoute()
const router = useRouter()
const projectName = ref(route.params.name || '')
const activeTab = ref('yaml')
const isRunning = ref(true)
const isLoading = ref(true)
const autoScroll = ref(true)
const logsRef = ref(null)
const containerList = ref([])  // 修改为空数组，等待从后端获取数据
const logs = ref([])  // 添加日志数组
const logWebSocket = ref(null)  // 移动到这里统一声明
const yamlContent = ref('')

// 添加返回方法
const goBack = () => {
  router.push('/projects')
}

// 添加获取容器列表的方法
const fetchContainers = async () => {
  try {
    const response = await api.compose.getStatus(projectName.value)
    if (response && Array.isArray(response.containers)) {
      containerList.value = response.containers.map(container => ({
        name: container.name,
        image: container.image,
        status: container.state || container.status,
        cpu: container.cpu || '0%',
        memory: container.memory || '0 MB',
        network: `${container.networkRx || '0 B'} / ${container.networkTx || '0 B'}`
      }))
      // 更新项目运行状态
      isRunning.value = containerList.value.some(c => c.status === 'running')
    } else {
      containerList.value = []
      console.warn('返回的容器数据格式不正确:', response)
    }
  } catch (error) {
    console.error('获取容器列表失败:', error.response?.data || error.message)
    ElMessage.error(`获取容器列表失败: ${error.response?.data?.error || '服务器错误'}`)
    containerList.value = []
  }
}

// 修改容器操作方法
const handleContainerRestart = async (container) => {
  try {
    await api.compose.restart(projectName.value)
    ElMessage.success(`重启容器 ${container.name} 成功`)
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error(`重启容器 ${container.name} 失败`)
  }
}

const handleContainerStop = async (container) => {
  try {
    await api.compose.stop(projectName.value)
    ElMessage.success(`停止容器 ${container.name} 成功`)
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error(`停止容器 ${container.name} 失败`)
  }
}

// 修改项目操作方法
const handleStart = async () => {
  try {
    await api.compose.start(projectName.value)
    ElMessage.success('启动项目成功')
    isRunning.value = true
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error('启动项目失败')
  }
}

const handleStop = async () => {
  try {
    await api.compose.stop(projectName.value)
    ElMessage.success('停止项目成功')
    isRunning.value = false
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error('停止项目失败')
  }
}

const handleRestart = async () => {
  try {
    await api.compose.restart(projectName.value)
    ElMessage.success('重启项目成功')
    await fetchContainers() // 刷新容器列表
  } catch (error) {
    ElMessage.error('重启项目失败')
  }
}

const handleDown = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要清除该项目的容器和网络吗？\n这将停止并删除容器，但保留项目文件。',
      '提示',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await api.compose.down(projectName.value)
    ElMessage.success('清除成功')
    isRunning.value = false
    await fetchContainers()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(`清除失败: ${error.response?.data?.error || error.message || '未知错误'}`)
    }
  }
}

// 修改获取 YAML 配置的方法
const fetchYamlContent = async () => {
  try {
    console.log('api.compose:', api.compose); // 添加调试日志
    const response = await api.compose.getYaml(projectName.value)
    console.log('YAML Response:', response); // 添加调试日志
    
    if (response && response.content) {
      yamlContent.value = response.content
    } else {
      console.warn('YAML内容为空')
      yamlContent.value = ''
    }
  } catch (error) {
    console.error('获取YAML配置失败:', error)
    if (error.response && error.response.status === 400 && error.response.data && error.response.data.error) {
      ElMessage.warning(error.response.data.error)
    } else {
      ElMessage.error('获取YAML配置失败')
    }
  }
}

// 修改保存 YAML 的方法
const handleSaveYaml = async () => {
  try {
    await api.compose.saveYaml(projectName.value, yamlContent.value)
    ElMessage.success('保存成功')
  } catch (error) {
    console.error('保存YAML失败:', error)
    ElMessage.error('保存失败')
  }
}

onMounted(async () => {
  isLoading.value = true
  try {
    // 并行获取项目信息和YAML配置
    await Promise.all([
      fetchContainers(),
      fetchYamlContent()
    ])

    // 设置定时刷新
    refreshTimer = setInterval(fetchContainers, 5000)
    
    // 设置WebSocket连接
    setupWebSocket()
  } catch (error) {
    console.error('初始化失败:', error)
  } finally {
    isLoading.value = false
  }
})

// 添加定时刷新
let refreshTimer = null

onUnmounted(() => {
  // 清理定时器
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  
  // 清理 EventSource
  if (logWebSocket.value) {
    logWebSocket.value.close()
    logWebSocket.value = null
  }
  
  // 清理数据
  logs.value = []
  containerList.value = []
  yamlContent.value = ''
})

// 替换 setupWebSocket 函数
const setupWebSocket = () => {
  if (logWebSocket.value) {
    logWebSocket.value.close()
  }

  const token = localStorage.getItem('token')
  const tokenParam = token ? `?token=${encodeURIComponent(token)}` : ''
  const eventSource = new EventSource(`/api/compose/${projectName.value}/logs${tokenParam}`)
  
  eventSource.onopen = () => {
    console.log('SSE connection established')
    logs.value.push({
      type: 'info',
      content: '已连接到日志服务'
    })
  }
  
  eventSource.onmessage = (event) => {
    const data = event.data
    if (data.startsWith('error:')) {
      logs.value.push({
        type: 'error',
        content: data.substring(6)
      })
    } else {
      logs.value.push({
        type: 'info',
        content: data
      })
    }
    
    if (autoScroll.value) {
      scrollToBottom()
    }
  }
  
  eventSource.onerror = (error) => {
    console.error('SSE error:', error)
    logs.value.push({
      type: 'error',
      content: '日志连接错误'
    })
    eventSource.close()
  }

  // 保存 EventSource 实例以便后续清理
  logWebSocket.value = eventSource
}

// 修改 onUnmounted 钩子中的清理代码
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (logWebSocket.value) {
    logWebSocket.value.close()
  }
})

// 添加清理日志的方法
const handleClearLogs = () => {
  logs.value = []
}

// 添加自动滚动相关代码
const scrollToBottom = () => {
  if (logsRef.value) {
    nextTick(() => {
      logsRef.value.scrollTop = logsRef.value.scrollHeight
    })
  }
}

// 监听日志变化
watch(logs, () => {
  if (autoScroll.value) {
    scrollToBottom()
  }
})
</script>

<style scoped>
.project-detail { padding: 0; width: 100%; }

.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }

.title { display: flex; align-items: center; font-size: 18px; font-weight: bold; gap: 8px; }

.detail-tabs { background: #fff; border-radius: 4px; padding: 12px; }

.yaml-editor {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.editor-header { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; background-color: #f5f7fa; border-bottom: 1px solid #dcdfe6; }

.yaml-textarea {
  font-family: monospace;
}

.logs-container { height: 600px; display: flex; flex-direction: column; }

.logs-header { display: flex; justify-content: space-between; align-items: center; padding: 8px; background: #f5f7fa; border: 1px solid #dcdfe6; border-bottom: none; }

.logs-content { flex: 1; overflow-y: auto; background: #1e1e1e; color: #fff; padding: 8px; font-family: monospace; border: 1px solid #dcdfe6; }

.logs-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.logs-content .error {
  color: #ff4949;
}

.logs-content .success {
  color: #67c23a;
}

.logs-content .info {
  color: #909399;
}
</style>
