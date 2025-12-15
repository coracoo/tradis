<template>
  <div class="ports-view compact-table">
    <div class="operation-bar">
      <div class="left-controls">
        <div class="controls-row">
          <el-input-number v-model="range.start" :min="0" :max="65535" :step="1" :disabled="loading" placeholder="Start" style="width: 160px" />
          <span class="range-sep"> 至 </span>
          <el-input-number v-model="range.end" :min="0" :max="65535" :step="1" :disabled="loading" placeholder="End" style="width: 160px" />
          <el-button type="primary" @click="saveRange" :loading="saving">锁定范围</el-button>
          <el-button @click="resetRange" :disabled="loading" icon="Refresh">恢复默认</el-button>
        </div>
      </div>
      <div class="right-controls">
        <div class="controls-row">
          <div class="stats-row">
            <el-tag type="info" effect="plain" size="small">总: {{ summary.total }}</el-tag>
            <el-tag type="success" effect="plain" size="small">闲: {{ summary.available }}</el-tag>
            <el-tag type="danger" effect="plain" size="small">用: {{ summary.used }}</el-tag>
          </div>
          <div class="divider-vertical"></div>
          <el-select v-model="filters.used" placeholder="状态" style="width: 100px" clearable>
            <el-option label="全部" value="all" />
            <el-option label="已用" value="used" />
            <el-option label="空闲" value="unused" />
          </el-select>
          <el-select v-model="filters.type" placeholder="类型" style="width: 100px" clearable>
            <el-option label="全部" value="all" />
            <el-option label="Host" value="host" />
            <el-option label="Container" value="container" />
          </el-select>
          <el-input v-model="filters.search" placeholder="搜索端口 (例: 80 或 8000-9000)" style="width: 200px" clearable @clear="fetchPorts" @keyup.enter="fetchPorts" />
          <el-button @click="fetchPorts" :loading="loading" icon="Search" circle></el-button>
        </div>
      </div>
    </div>

    <div class="tables-container">
      <div class="table-column">
        <div class="column-header">TCP 协议</div>
        <el-table :data="tcpItems" style="width: 100%" v-loading="loading" height="100%" class="ports-table" :header-cell-style="{ background: 'transparent' }">
          <el-table-column prop="port" label="端口号" width="140" header-align="left">
            <template #default="scope">
              <span>{{ scope.row.port }}</span>
              <span v-if="scope.row.end_port && scope.row.end_port !== scope.row.port">:{{ scope.row.end_port }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="type" label="类型" width="120" header-align="left">
            <template #default="scope">
              <el-tag v-if="scope.row.type" :type="scope.row.type === 'Host' ? 'primary' : 'warning'" effect="plain" size="small">{{ scope.row.type }}</el-tag>
              <span v-else class="text-gray">  / </span>
            </template>
          </el-table-column>
          <el-table-column prop="used" label="状态" width="100" header-align="left">
            <template #default="scope">
              <div class="status-dot" :class="scope.row.used ? 'status-used' : 'status-unused'">
                 {{ scope.row.used ? '用' : '空' }}
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="service" label="服务/用途" min-width="150" header-align="left">
            <template #default="scope">
              <div v-if="scope.row.service" class="service-name">
                <span class="service-text">{{ scope.row.service }}</span>
              </div>
              <el-input v-else v-model="scope.row.note" size="small" @change="saveNote(scope.row)" placeholder="用途" />
            </template>
          </el-table-column>
        </el-table>
      </div>
      
      <div class="table-column">
        <div class="column-header">UDP 协议</div>
        <el-table :data="udpItems" style="width: 100%" v-loading="loading" height="100%" class="ports-table" :header-cell-style="{ background: 'transparent' }">
          <el-table-column prop="port" label="端口号" width="140" header-align="left">
            <template #default="scope">
              <span>{{ scope.row.port }}</span>
              <span v-if="scope.row.end_port && scope.row.end_port !== scope.row.port">:{{ scope.row.end_port }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="type" label="类型" width="120" header-align="left">
            <template #default="scope">
              <el-tag v-if="scope.row.type" :type="scope.row.type === 'Host' ? 'primary' : 'warning'" effect="plain" size="small">{{ scope.row.type }}</el-tag>
              <span v-else class="text-gray"> / </span>
            </template>
          </el-table-column>
          <el-table-column prop="used" label="状态" width="100" header-align="left">
            <template #default="scope">
              <div class="status-dot" :class="scope.row.used ? 'status-used' : 'status-unused'">
                 {{ scope.row.used ? '用' : '空' }}
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="service" label="服务/用途" min-width="150" header-align="left">
            <template #default="scope">
              <div v-if="scope.row.service" class="service-name">
                <span class="service-text">{{ scope.row.service }}</span>
              </div>
              <el-input v-else v-model="scope.row.note" size="small" @change="saveNote(scope.row)" placeholder="用途" />
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
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
      // Protocol is now ignored for display split, but we keep it for range setting if needed
      // const proto = res.protocol || 'TCP+UDP'
      // range.value.protocol = (proto === 'TCP+UDP' || proto === 'ALL') ? 'all' : proto
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

    // Map 'used'/'unused' to 'true'/'false' for backend
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
      pageSize: 10000 // Large page size to fetch all aggregated items
    }

    // Fetch TCP and UDP in parallel
    const [tcpRes, udpRes] = await Promise.all([
      api.ports.list({ ...commonParams, protocol: 'tcp' }),
      api.ports.list({ ...commonParams, protocol: 'udp' })
    ])

    tcpItems.value = (tcpRes.items || [])
    udpItems.value = (udpRes.items || [])
    
    // Summary is sum of both (rough approximation or take one if backend returns global stats?)
    // Actually backend returns stats for the query. Since we query separately, we should sum them up?
    // Or just use the global stats from one of them?
    // The backend `listPorts` returns stats based on the query range.
    // If we split queries, the stats in `tcpRes` are for TCP only, `udpRes` for UDP only.
    // So we should sum them.
    
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
      protocol: 'all' // Always save as all since we display both
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

// Watch filters to auto-refresh
watch(filters, () => {
  fetchPorts()
}, { deep: true })

onMounted(async () => {
  await fetchRange()
  await fetchPorts()
  // 10 seconds refresh rate
  timer = setInterval(fetchPorts, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.ports-view { height: 100%; display: flex; flex-direction: column; overflow: hidden; padding-right: 4px; }
.operation-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.left-controls { display: flex; align-items: center; }
.right-controls { display: flex; align-items: center; }
.controls-row { display: flex; align-items: center; gap: 8px; }
.stats-row { display: flex; align-items: center; gap: 6px; }
.divider-vertical { width: 1px; height: 20px; background-color: var(--el-border-color); margin: 0 8px; }
.range-sep { margin: 0 4px; }
.tables-container { flex: 1; min-height: 0; display: flex; gap: 16px; overflow: hidden; }
.table-column { flex: 1; display: flex; flex-direction: column; min-width: 0; overflow: hidden; }
.column-header { font-weight: bold; margin-bottom: 8px; text-align: center; background-color: var(--el-fill-color-light); padding: 4px; border-radius: 4px; }
.ports-table { flex: 1; min-height: 0; overflow: hidden; }
.service-name { font-weight: 500; color: var(--el-text-color-primary); }
</style>
