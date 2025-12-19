<template>
  <div class="ports-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="filters.search"
          placeholder="搜索端口 (例: 80)"
          class="search-input"
          clearable
          size="medium"
          @clear="fetchPorts"
          @keyup.enter="fetchPorts"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-radio-group v-model="filters.used" size="medium" @change="fetchPorts">
          <el-radio-button label="all">全部状态</el-radio-button>
          <el-radio-button label="used">已用</el-radio-button>
          <el-radio-button label="unused">空闲</el-radio-button>
        </el-radio-group>

        <el-radio-group v-model="filters.type" size="medium" @change="fetchPorts">
          <el-radio-button label="all">全部类型</el-radio-button>
          <el-radio-button label="host">Host</el-radio-button>
          <el-radio-button label="container">Container</el-radio-button>
        </el-radio-group>
      </div>

      <div class="filter-right">
        <div class="stats-group">
           <el-tag type="info" effect="light" size="medium">总: {{ summary.total }}</el-tag>
           <el-tag type="success" effect="light" size="medium">闲: {{ summary.available }}</el-tag>
           <el-tag type="danger" effect="light" size="medium">用: {{ summary.used }}</el-tag>
        </div>
        
        <el-popover placement="bottom" title="扫描范围设置" :width="340" trigger="click">
          <template #reference>
            <el-button plain size="medium">
               <el-icon style="margin-right: 4px"><Setting /></el-icon> 范围设置
            </el-button>
          </template>
          <div class="range-settings">
             <div class="range-inputs">
               <el-input-number v-model="range.start" :min="0" :max="65535" :controls="false" placeholder="Start" class="range-input-s" />
               <span class="range-sep">-</span>
               <el-input-number v-model="range.end" :min="0" :max="65535" :controls="false" placeholder="End" class="range-input-s" />
             </div>
             <div class="range-actions">
               <el-button type="primary" @click="saveRange" :loading="saving" size="small">锁定范围</el-button>
               <el-button @click="resetRange" :disabled="loading" size="small">重置默认</el-button>
             </div>
          </div>
        </el-popover>

        <el-button @click="fetchPorts" :loading="loading" plain size="medium">
          <template #icon><el-icon><Refresh /></el-icon></template>
          刷新
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <div class="tables-container">
        <div class="table-column">
          <div class="column-header-box">
            <span class="protocol-title">TCP 协议</span>
          </div>
          <div class="table-inner">
             <el-table 
               :data="tcpItems" 
               style="width: 100%; height: 100%" 
               v-loading="loading" 
               :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
               :row-style="{ height: '50px' }"
             >
               <el-table-column prop="port" label="端口号" width="100">
                 <template #default="scope">
                   <span class="port-number">{{ scope.row.port }}</span>
                   <span v-if="scope.row.end_port && scope.row.end_port !== scope.row.port" class="port-range">-{{ scope.row.end_port }}</span>
                 </template>
               </el-table-column>
               <el-table-column prop="type" label="类型" width="100">
                 <template #default="scope">
                   <el-tag v-if="scope.row.type" :type="scope.row.type === 'Host' ? 'primary' : 'warning'" effect="light" size="small">{{ scope.row.type }}</el-tag>
                   <span v-else class="text-gray">-</span>
                 </template>
               </el-table-column>
               <el-table-column prop="used" label="状态" width="80">
                 <template #default="scope">
                   <div class="status-indicator">
                     <span class="status-point" :class="scope.row.used ? 'status-active' : 'status-inactive'"></span>
                     {{ scope.row.used ? '用' : '空' }}
                   </div>
                 </template>
               </el-table-column>
               <el-table-column prop="service" label="服务/用途" min-width="150">
                 <template #default="scope">
                   <el-input 
                     v-model="scope.row.note" 
                     size="small" 
                     @change="saveNote(scope.row)" 
                     :placeholder="scope.row.service || '添加备注...'" 
                     class="note-input"
                   />
                 </template>
               </el-table-column>
             </el-table>
          </div>
        </div>
        
        <div class="table-column">
          <div class="column-header-box">
            <span class="protocol-title">UDP 协议</span>
          </div>
          <div class="table-inner">
             <el-table 
               :data="udpItems" 
               style="width: 100%; height: 100%" 
               v-loading="loading" 
               :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
               :row-style="{ height: '50px' }"
             >
               <el-table-column prop="port" label="端口号" width="100">
                 <template #default="scope">
                   <span class="port-number">{{ scope.row.port }}</span>
                   <span v-if="scope.row.end_port && scope.row.end_port !== scope.row.port" class="port-range">-{{ scope.row.end_port }}</span>
                 </template>
               </el-table-column>
               <el-table-column prop="type" label="类型" width="100">
                 <template #default="scope">
                   <el-tag v-if="scope.row.type" :type="scope.row.type === 'Host' ? 'primary' : 'warning'" effect="light" size="small">{{ scope.row.type }}</el-tag>
                   <span v-else class="text-gray">-</span>
                 </template>
               </el-table-column>
               <el-table-column prop="used" label="状态" width="80">
                 <template #default="scope">
                   <div class="status-indicator">
                     <span class="status-point" :class="scope.row.used ? 'status-active' : 'status-inactive'"></span>
                     {{ scope.row.used ? '用' : '空' }}
                   </div>
                 </template>
               </el-table-column>
               <el-table-column prop="service" label="服务/用途" min-width="150">
                 <template #default="scope">
                   <el-input 
                     v-model="scope.row.note" 
                     size="small" 
                     @change="saveNote(scope.row)" 
                     :placeholder="scope.row.service || '添加备注...'" 
                     class="note-input"
                   />
                 </template>
               </el-table-column>
             </el-table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Setting } from '@element-plus/icons-vue'
import api from '../api'

const loading = ref(false)
const saving = ref(false)
const range = ref({ start: 0, end: 65535, protocol: 'all' })
const filters = ref({ type: 'all', search: '', used: 'used' })
const tcpItems = ref([])
const udpItems = ref([])
const summary = ref({ total: 0, used: 0, available: 0 })
let timer = null

const fetchRange = async () => {
  try {
    const res = await api.ports.getRange()
    if (res && typeof res.start === 'number') {
      range.value.start = res.start
      range.value.end = res.end
    }
  } catch (e) {}
}

const fetchPorts = async () => {
  loading.value = true
  try {
    let queryStart = range.value.start
    let queryEnd = range.value.end
    let querySearch = filters.value.search

    // Check for range search format "start-end"
    const rangeMatch = querySearch.trim().match(/^(\d+)-(\d+)$/)
    if (rangeMatch) {
      const s = parseInt(rangeMatch[1])
      const e = parseInt(rangeMatch[2])
      if (!isNaN(s) && !isNaN(e) && s <= e) {
        queryStart = s
        queryEnd = e
        querySearch = ''
      }
    }

    let usedParam = ''
    if (filters.value.used === 'used') usedParam = 'true'
    else if (filters.value.used === 'unused') usedParam = 'false'
    else usedParam = 'all'

    const commonParams = {
      start: queryStart,
      end: queryEnd,
      type: filters.value.type === 'all' ? '' : filters.value.type,
      search: querySearch,
      used: usedParam,
      page: 1,
      pageSize: 10000 
    }

    const [tcpRes, udpRes] = await Promise.all([
      api.ports.list({ ...commonParams, protocol: 'tcp' }),
      api.ports.list({ ...commonParams, protocol: 'udp' })
    ])

    tcpItems.value = (tcpRes.items || [])
    udpItems.value = (udpRes.items || [])
    
    summary.value.total = Math.max(0, (queryEnd - queryStart + 1)) * 2
    summary.value.used = (tcpRes.used || 0) + (udpRes.used || 0)
    summary.value.available = (tcpRes.available || 0) + (udpRes.available || 0)

  } catch (error) {
    console.error(error)
    ElMessage.error('加载端口数据失败')
  } finally {
    loading.value = false
  }
}

const saveRange = async () => {
  saving.value = true
  try {
    if (range.value.end <= range.value.start) {
      ElMessage.error('范围无效：右侧必须大于左侧')
      return
    }
    await api.ports.updateRange({
      start: range.value.start,
      end: range.value.end,
      protocol: 'all'
    })
    ElMessage.success('范围已锁定')
    await fetchPorts()
  } catch (error) {
    ElMessage.error('保存范围失败')
  } finally {
    saving.value = false
  }
}

const resetRange = async () => {
  range.value.start = 0
  range.value.end = 65535
  await saveRange()
}

const saveNote = async (row) => {
  try {
    await api.ports.saveNote({
      port: row.port,
      type: row.type,
      protocol: row.protocol,
      note: row.note || ''
    })
    ElMessage.success('备注已保存')
  } catch (error) {
    ElMessage.error('保存备注失败')
  }
}

watch(filters, () => {
  fetchPorts()
}, { deep: true })

onMounted(async () => {
  await fetchRange()
  await fetchPorts()
  timer = setInterval(fetchPorts, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.ports-view {
  height: 100%;
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
  width: 240px;
}

.stats-group {
  display: flex;
  gap: 8px;
  margin-right: 8px;
}

.content-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
  padding: 20px;
}

.tables-container {
  flex: 1;
  display: flex;
  gap: 24px;
  overflow: hidden;
  height: 100%;
}

.table-column {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.column-header-box {
  background-color: var(--el-fill-color-light);
  padding: 12px 15px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  text-align: center;
}

.protocol-title {
  font-weight: 600;
  color: var(--el-text-color-primary);
  font-size: 14px;
}

.table-inner {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
}

/* Range Settings Popover */
.range-settings {
  padding: 8px 4px;
}

.range-inputs {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.range-input-s {
  width: 120px;
}

.range-sep {
  color: var(--el-text-color-secondary);
}

.range-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* Status Indicator */
.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-regular);
}

.status-point {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-active {
  background-color: var(--el-color-danger);
  box-shadow: 0 0 0 3px var(--el-color-danger-light-8);
}

.status-inactive {
  background-color: var(--el-color-success);
  box-shadow: 0 0 0 3px var(--el-color-success-light-8);
}

.port-number {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.port-range {
  font-family: 'JetBrains Mono', monospace;
  color: var(--el-text-color-secondary);
}

.text-gray {
  color: var(--el-text-color-placeholder);
}

.note-input :deep(.el-input__wrapper) {
  box-shadow: none;
  background-color: transparent;
  padding: 0;
}

.note-input :deep(.el-input__inner) {
  height: 32px;
}

.note-input:hover :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--el-border-color) inset;
  background-color: var(--el-bg-color);
  padding: 0 11px;
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
