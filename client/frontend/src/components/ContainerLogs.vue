<template>
  <el-dialog
    v-model="visible"
    title="容器日志"
    width="80%"
    @close="handleClose"
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
import { ref, watch, nextTick } from 'vue'
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

watch(() => props.modelValue, (newVal) => {
  visible.value = newVal
  if (newVal && props.container) {
    fetchLogs()
  }
})

watch(() => visible.value, (newVal) => {
  emit('update:modelValue', newVal)
})

const fetchLogs = async () => {
  if (!props.container) return
  
  logs.value = ''
  
  try {
    const token = localStorage.getItem('token')
    const response = await fetch(`/api/containers/${props.container.Id}/logs`, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    const reader = response.body.getReader()
    
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      
      const text = new TextDecoder().decode(value)
      logs.value += text
      
      if (autoScroll.value && logContainer.value) {
        nextTick(() => {
          logContainer.value.scrollTop = logContainer.value.scrollHeight
        })
      }
    }
  } catch (error) {
    console.error('Error fetching logs:', error)
    ElMessage.error('获取日志失败')
    logs.value = '获取日志失败'
  }
}

const clearLogs = () => {
  logs.value = ''
}

const handleClose = () => {
  visible.value = false
  logs.value = ''
}
</script>

<style scoped>
.log-container {
  height: 500px;
  overflow-y: auto;
  background-color: #1e1e1e;
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