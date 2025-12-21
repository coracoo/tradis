<template>
  <el-dialog
    v-model="visible"
    title="容器日志"
    width="80%"
    @close="handleClose"
    class="app-dialog"
  >
    <div class="logs-container">
      <div class="logs-header">
        <div class="logs-options">
          <el-switch
            v-model="autoScroll"
            active-text="自动滚动"
          />
          <el-input
            v-model="logFilter"
            placeholder="检索日志"
            style="width: 220px"
            size="default"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
        <el-button @click="clearLogs">清空</el-button>
      </div>
      <div ref="logContainer" class="logs-content">
        <pre
          v-for="(log, index) in filteredLogs"
          :key="index"
          :class="getLogClass(log)"
        >{{ log.content }}</pre>
      </div>
    </div>
    <template #footer>
      <el-button @click="handleClose">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { useSseLogStream } from '../utils/sseLogStream'

const props = defineProps({
  modelValue: Boolean,
  container: Object
})

const emit = defineEmits(['update:modelValue'])

const visible = ref(false)
const logContainer = ref(null)
const autoScroll = ref(true)
const {
  logs,
  logFilter,
  filteredLogs,
  start: startStream,
  stop: stopStream,
  clear: clearLogs,
  pushLine
} = useSseLogStream({
  autoScroll,
  scrollElRef: logContainer
})

const getLogClass = (log) => ({
  'error': log.level === 'error',
  'warning': log.level === 'warning',
  'info': log.level === 'info',
  'success': log.level === 'success'
})

watch(() => props.modelValue, (newVal) => {
  visible.value = newVal
  if (newVal && props.container) {
    startLogsStream()
  }
})

watch(() => visible.value, (newVal) => {
  emit('update:modelValue', newVal)
  if (!newVal) {
    stopLogsStream()
  }
})

const startLogsStream = () => {
  if (!props.container?.Id) return

  const token = localStorage.getItem('token') || ''
  const url = `/api/containers/${props.container.Id}/logs/events?tail=200&token=${encodeURIComponent(token)}`
  try {
    startStream(url, { reset: true })
  } catch (e) {
    ElMessage.error('日志连接失败')
  }
}

const stopLogsStream = () => stopStream()

const handleClose = () => {
  visible.value = false
  clearLogs()
  stopStream()
}

onUnmounted(() => {
  stopStream()
})
</script>

<style scoped>
.logs-container {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.logs-options {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logs-content {
  height: 500px;
  overflow-y: auto;
  background: var(--el-bg-color-overlay);
  padding: 12px;
  border-radius: 10px;
  border: 1px solid var(--el-border-color);
}

.logs-content pre {
  margin: 0;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--el-text-color-primary);
}

.logs-content pre.error {
  color: var(--el-color-danger);
}

.logs-content pre.warning {
  color: var(--el-color-warning);
}

.logs-content pre.success {
  color: var(--el-color-success);
}
</style>
