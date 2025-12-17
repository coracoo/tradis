<template>
  <el-dialog
    v-model="visible"
    title="容器日志"
    width="80%"
    @close="handleClose"
    class="app-dialog"
  >
    <div ref="logContainer" class="log-container">
      <pre class="logs">{{ logs }}</pre>
    </div>
    <template #footer>
      <el-button @click="clearLogs">清空</el-button>
      <el-button @click="handleClose">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch, nextTick, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps({
  modelValue: Boolean,
  container: Object
})

const emit = defineEmits(['update:modelValue'])

const visible = ref(false)
const logs = ref('')
const logContainer = ref(null)
const autoScroll = ref(true)
let logController = null

watch(() => props.modelValue, (newVal) => {
  visible.value = newVal
  if (newVal && props.container) {
    fetchLogs()
  }
})

watch(() => visible.value, (newVal) => {
  emit('update:modelValue', newVal)
  if (!newVal && logController) {
    try { logController.abort() } catch {}
    logController = null
  }
})

const fetchLogs = async () => {
  if (!props.container) return
  
  logs.value = ''
  
  try {
    if (logController) {
      try { logController.abort() } catch {}
      logController = null
    }
    logController = new AbortController()
    const token = localStorage.getItem('token')
    const response = await fetch(`/api/containers/${props.container.Id}/logs`, {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      signal: logController.signal
    })
    if (!response.ok) {
      const msg = await response.text().catch(() => '')
      ElMessage.error(`获取日志失败${msg ? `: ${msg}` : ''}`)
      logs.value = msg || '获取日志失败'
      return
    }
    const reader = response.body?.getReader()
    const decoder = new TextDecoder('utf-8')
    
    if (!reader) return
    while (true) {
      let chunk
      try {
        const { value, done } = await reader.read()
        if (done) break
        chunk = value
      } catch (err) {
        if (err?.name === 'AbortError') {
          return
        }
        throw err
      }
      
      const text = decoder.decode(chunk, { stream: true })
      logs.value += text
      
      if (autoScroll.value && logContainer.value) {
        nextTick(() => {
          logContainer.value.scrollTop = logContainer.value.scrollHeight
        })
      }
    }
    // Flush any remaining bytes
    logs.value += decoder.decode()
  } catch (error) {
    console.error('Error fetching logs:', error)
    if (error?.name !== 'AbortError') {
      ElMessage.error('获取日志失败')
      logs.value = '获取日志失败'
    }
  }
}

const clearLogs = () => {
  logs.value = ''
}

const handleClose = () => {
  visible.value = false
  logs.value = ''
  if (logController) {
    try { logController.abort() } catch {}
    logController = null
  }
}

onUnmounted(() => {
  if (logController) {
    try { logController.abort() } catch {}
    logController = null
  }
})
</script>

<style scoped>
.log-container {
  height: 500px;
  overflow-y: auto;
  background-color: var(--el-fill-color-darker);
  padding: 10px;
  border-radius: 4px;
}

.logs {
  margin: 0;
  color: #fff;
  font-family: monospace;
  white-space: pre-wrap;
  word-wrap: break-word;
}
</style>
