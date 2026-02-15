<template>
  <div class="volumes-view">
    <div class="filter-bar clay-surface">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索存储卷名称..."
          clearable
          class="search-input"
          size="medium"
          @keyup.enter="fetchVolumes"
        >
          <template #prefix>
            <IconEpSearch />
          </template>
        </el-input>
      </div>

      <div class="filter-right">
        <el-button-group class="main-actions">
          <el-button @click="fetchVolumes" :loading="loading" plain size="medium">
            <template #icon><IconEpRefresh /></template>
            刷新
          </el-button>
          <el-button type="primary" @click="dialogVisible = true" size="medium">
            <template #icon><IconEpPlus /></template>
            新建卷
          </el-button>
        </el-button-group>

        <el-dropdown trigger="click" @command="handleGlobalAction">
          <el-button plain class="more-btn" size="medium">
            更多操作<IconEpArrowDown class="el-icon--right" />
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="prune" :icon="IconEpDelete">清除未使用的存储卷</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="table-wrapper clay-surface">
      <el-table 
        :data="paginatedVolumes" 
        style="width: 100%; height: 100%" 
        v-loading="loading"
        class="main-table"
        @sort-change="handleSortChange"
        :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: '60px' }"
      >
        <el-table-column type="selection" width="40" />
        <el-table-column prop="Name" label="卷名称" sortable="custom" min-width="240" show-overflow-tooltip>
          <template #default="scope">
            <div class="volume-name-cell">
              <div class="icon-wrapper volume">
                <IconEpCoin />
              </div>
              <span class="volume-name-text">{{ scope.row.Name }}</span>
            </div>
          </template>
        </el-table-column>

        <!--<el-table-column prop="Driver" label="驱动" sortable="custom" min-width="100" />-->
        
        <el-table-column prop="Containers" label="关联容器" sortable="custom" min-width="80">
          <template #default="scope">
            <div class="container-list" v-if="scope.row.Containers && Object.keys(scope.row.Containers).length">
              <el-tooltip 
                v-for="(container, id) in scope.row.Containers" 
                :key="id"
                :content="container.Name.substring(1)"
                placement="top">
                <el-tag size="small" effect="light" class="container-tag">
                  {{ container.Name.substring(1) }}
                </el-tag>
              </el-tooltip>
            </div>
            <span v-else class="text-gray">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="Mountpoint" label="挂载点" sortable="custom" min-width="200" show-overflow-tooltip>
          <template #default="scope">
            <span class="text-gray font-mono">{{ scope.row.Mountpoint }}</span>
          </template>
        </el-table-column>
        
        <el-table-column prop="Created" label="创建时间" sortable="custom" width="160">
          <template #default="scope">
            <div class="text-gray font-mono">
              {{ formatTimeTwoLines(scope.row.CreatedAt) }}
            </div>
          </template>
        </el-table-column>
        
        <el-table-column prop="Status" label="状态" sortable="custom" width="120">
          <template #default="scope">
            <div class="status-indicator">
              <span class="status-point" :class="scope.row.InUse ? 'running' : 'stopped'"></span>
              <span>{{ scope.row.InUse ? '使用中' : '未使用' }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="160" fixed="left" align="center" class-name="col-ops">
          <template #default="scope">
            <div class="row-ops">
              <el-tooltip content="浏览文件" placement="top">
                <el-button
                  circle
                  plain
                  :loading="browsingName === scope.row.Name"
                  @click="browseVolume(scope.row)">
                  <IconEpFolderOpened />
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除存储卷" placement="top">
                <el-button 
                  circle 
                  plain
                  type="danger" 
                  :disabled="scope.row.InUse"
                  @click="deleteVolume(scope.row)">
                  <IconEpDelete />
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 30, 50]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>

    <!-- 创建存储卷对话框 -->
    <el-dialog v-model="dialogVisible" title="创建存储卷" width="400px" append-to-body>
      <el-form :model="volumeForm" label-width="80px" class="compact-form">
        <el-form-item label="名称">
          <el-input v-model="volumeForm.name" placeholder="存储卷名称" />
        </el-form-item>
        <!--<el-form-item label="驱动">
          <el-select v-model="volumeForm.driver" placeholder="选择驱动" style="width: 100%">
            <el-option label="local" value="local" />
          </el-select>
        </el-form-item>-->
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitVolume">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
import request from '../utils/request'

const loading = ref(false)
const volumes = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const dialogVisible = ref(false)
const searchQuery = ref('')
const sortState = ref({ prop: '', order: '' })
const volumeForm = ref({
  name: '',
  driver: 'local'
})
const browsingName = ref('')

// 获取存储卷列表
const fetchVolumes = async () => {
  loading.value = true
  try {
    const response = await api.volumes.list()
    volumes.value = Array.isArray(response.Volumes) ? response.Volumes.map(volume => ({
      ...volume,
      InUse: volume.Containers && Object.keys(volume.Containers).length > 0
    })) : []
  } catch (error) {
    ElMessage.error('获取存储卷列表失败')
    volumes.value = []
  } finally {
    loading.value = false
  }
}

const handleSortChange = ({ prop, order }) => {
  sortState.value = { prop, order }
  try {
    const v = JSON.stringify(sortState.value)
    request.post('/settings/kv/sort_volumes', { value: v })
  } catch (e) {}
}

const filteredVolumes = computed(() => {
  let list = [...volumes.value] // Create a copy
  const q = (searchQuery.value || '').trim().toLowerCase()
  if (q) {
    list = list.filter(v => v.Name.toLowerCase().includes(q))
  }
  
  if (sortState.value.prop && sortState.value.order) {
    const { prop, order } = sortState.value
    list.sort((a, b) => {
      let valA, valB
      switch (prop) {
        case 'Name':
          valA = a.Name || ''
          valB = b.Name || ''
          break
        case 'Mountpoint':
          valA = a.Mountpoint || ''
          valB = b.Mountpoint || ''
          break
        case 'Driver':
          valA = a.Driver || ''
          valB = b.Driver || ''
          break
        case 'Status':
          valA = a.InUse ? 1 : 0
          valB = b.InUse ? 1 : 0
          break
        case 'Containers':
          valA = Object.keys(a.Containers || {}).length
          valB = Object.keys(b.Containers || {}).length
          break
        case 'Created':
          valA = a.CreatedAt || ''
          valB = b.CreatedAt || ''
          break
        default:
          valA = a[prop]
          valB = b[prop]
      }
      if (valA < valB) return order === 'ascending' ? -1 : 1
      if (valA > valB) return order === 'ascending' ? 1 : -1
      return 0
    })
  } else {
    list.sort((a, b) => a.Name.localeCompare(b.Name))
  }
  return list
})

const paginatedVolumes = computed(() => {
  total.value = filteredVolumes.value.length
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredVolumes.value.slice(start, end)
})

const handleGlobalAction = (command) => {
  if (command === 'prune') {
    pruneVolumes()
  }
}

// 创建存储卷
const submitVolume = async () => {
  try {
    await api.volumes.create(volumeForm.value)
    ElMessage.success('存储卷创建成功')
    dialogVisible.value = false
    fetchVolumes()
  } catch (error) {
    ElMessage.error('创建存储卷失败')
  }
}

// 删除存储卷
const deleteVolume = async (volume) => {
  try {
    await ElMessageBox.confirm('确定要删除该存储卷吗？', '警告', {
      type: 'warning'
    })
    await api.volumes.remove(volume.Name)
    ElMessage.success('存储卷已删除')
    fetchVolumes()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 添加清除无用卷的方法
const pruneVolumes = async () => {
  try {
    await ElMessageBox.confirm(
      '此操作将清除所有未被使用的存储卷，是否继续？',
      '警告',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await api.volumes.prune()
    ElMessage.success('无用存储卷已清除')
    fetchVolumes()  // 刷新列表
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('清除失败：' + (error.response?.data?.error || '未知错误'))
    }
  }
}

const browseVolume = async (volume) => {
  if (!volume?.Name) return
  browsingName.value = volume.Name
  try {
    const res = await api.volumes.browseStart(volume.Name)
    const token = localStorage.getItem('token') || ''
    const url = res?.url
    if (!url) {
      ElMessage.error('打开失败：未返回浏览地址')
      return
    }
    const full = new URL(url, window.location.origin)
    if (token) full.searchParams.set('token', token)
    window.open(full.toString(), '_blank', 'noopener')
    if (res?.readOnly) {
      ElMessage.info('卷正在使用中，已以只读方式打开')
    }
  } catch (error) {
    ElMessage.error('打开失败：' + (error.response?.data?.error || error.message))
  } finally {
    browsingName.value = ''
  }
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

onMounted(async () => {
  try {
    const res = await request.get('/settings/kv/sort_volumes')
    if (res && res.value) {
      const s = JSON.parse(res.value)
      if (s && s.prop && s.order) {
        sortState.value = s
      }
    }
  } catch (e) {}
  fetchVolumes()
})
</script>

<style scoped>
.volumes-view {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 16px;
  background-color: var(--clay-bg);
  gap: 12px;
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
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
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.main-table {
  flex: 1;
}

/* Custom Table Styles */
.volume-name-cell {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 0;
  min-width: 0;
}

.icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
  transition: transform 0.2s;
  box-sizing: border-box;
  padding: 4px;
  margin: 2px;
  box-shadow: var(--shadow-clay-btn), var(--shadow-clay-inner);
  border: 1px solid rgba(55, 65, 81, 0.08);
}

.volume-name-cell:hover .icon-wrapper {
  transform: scale(1.03);
}

.icon-wrapper.volume {
  background: var(--icon-bg-image);
  color: var(--clay-ink);
}

.volume-name-text {
  font-weight: 900;
  color: var(--el-text-color-primary);
  font-size: 14px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 800;
}

.status-point {
  width: 12px;
  height: 12px;
  border-radius: 999px;
}

.status-point.running {
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0.2) 42%, rgba(255, 255, 255, 0) 65%),
    radial-gradient(circle at 55% 60%, rgba(0, 0, 0, 0.08), rgba(0, 0, 0, 0) 55%),
    linear-gradient(135deg, var(--clay-mint), var(--clay-mint-2));
  box-shadow: 0 0 0 6px rgba(110, 231, 183, 0.18), 2px 2px 6px rgba(0, 0, 0, 0.08), inset 1px 1px 2px rgba(255, 255, 255, 0.65);
}

.status-point.stopped {
  background: var(--status-idle-bg);
  box-shadow: var(--status-idle-shadow);
}

.container-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.container-tag {
  margin: 0;
  cursor: default;
}

.text-gray {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.row-ops {
  display: flex;
  justify-content: center;
  gap: 8px;
  align-items: center;
}

/* Pagination */
.pagination-bar {
  padding: 14px 16px;
  border-top: 1px solid rgba(55, 65, 81, 0.12);
  display: flex;
  justify-content: flex-end;
  background: transparent;
}

/* Override Element Styles */
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
</style>
