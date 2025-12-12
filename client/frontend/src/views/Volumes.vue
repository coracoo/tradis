<template>
  <div class="volumes-view compact-table">
    <!-- 修改顶部操作栏 -->
    <div class="operation-bar">
      <div class="left-ops">
        <el-button-group>
          <el-button @click="fetchVolumes">
            <el-icon><Refresh /></el-icon>
          </el-button>
          <el-button type="primary" @click="dialogVisible = true">
            <el-icon class="el-icon--left"><Plus /></el-icon>
             新建卷
          </el-button>
        </el-button-group>
        <el-tooltip content="清除未使用的存储卷" placement="top">
          <el-button type="danger" plain @click="pruneVolumes">
            <el-icon class="el-icon--left"><Delete /></el-icon>
            清除未使用的存储卷
          </el-button>
        </el-tooltip>
      </div>
    </div>

    <!-- 存储卷列表 -->
    <div class="volumes-table">
      <el-table 
        :data="sortedVolumes" 
        style="width: 100%" 
        height="100%"
        v-loading="loading">
        <el-table-column type="selection" width="40" />
        <el-table-column prop="Name" label="名称" min-width="200" show-overflow-tooltip>
          <template #default="scope">
            <span class="volume-name">{{ scope.row.Name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="Mountpoint" label="挂载点" min-width="200" show-overflow-tooltip>
          <template #default="scope">
            <span class="text-gray font-mono">{{ scope.row.Mountpoint }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="Driver" label="驱动" width="100" />
        <!-- 添加使用状态列 -->
        <el-table-column label="状态" width="120" header-align="left">
          <template #default="scope">
            <div class="status-dot" :class="scope.row.InUse ? 'status-used' : 'status-unused'">
              {{ scope.row.InUse ? '使用中' : '未使用' }}
            </div>
          </template>
        </el-table-column>
        <!-- 添加使用容器列 -->
        <el-table-column label="关联容器" min-width="100">
          <template #default="scope">
            <div class="container-list" v-if="scope.row.Containers && Object.keys(scope.row.Containers).length">
              <el-tooltip 
                v-for="(container, id) in scope.row.Containers" 
                :key="id"
                :content="container.Name.substring(1)"
                placement="top">
                <el-tag size="small" effect="plain" class="container-tag">
                  {{ container.Name.substring(1) }}
                </el-tag>
              </el-tooltip>
            </div>
            <span v-else class="text-gray">-</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160" header-align="left">
          <template #default="scope">
            <div class="text-gray font-mono text-center whitespace-pre-line">
              {{ formatTimeTwoLines(scope.row.CreatedAt) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" header-align="left">
          <template #default="scope">
            <el-tooltip content="删除存储卷" placement="top">
              <el-button 
                link 
                type="danger" 
                :disabled="scope.row.InUse"
                @click="deleteVolume(scope.row)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 分页 -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 30, 50]"
        layout="total, sizes, prev, pager, next"
        :total="total"
        size="small"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

    <!-- 创建存储卷对话框 -->
    <el-dialog v-model="dialogVisible" title="创建存储卷" width="400px" append-to-body>
      <el-form :model="volumeForm" label-width="80px" class="compact-form">
        <el-form-item label="名称">
          <el-input v-model="volumeForm.name" placeholder="存储卷名称" />
        </el-form-item>
        <el-form-item label="驱动">
          <el-select v-model="volumeForm.driver" placeholder="选择驱动" style="width: 100%">
            <el-option label="local" value="local" />
          </el-select>
        </el-form-item>
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
import { Refresh, Plus, Delete } from '@element-plus/icons-vue'

const loading = ref(false)
const volumes = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const dialogVisible = ref(false)
const volumeForm = ref({
  name: '',
  driver: 'local'
})

// 获取存储卷列表
const fetchVolumes = async () => {
  loading.value = true
  try {
    const response = await api.volumes.list()
    volumes.value = Array.isArray(response.Volumes) ? response.Volumes.map(volume => ({
      ...volume,
      InUse: volume.Containers && Object.keys(volume.Containers).length > 0
    })) : []
    total.value = volumes.value.length
  } catch (error) {
    ElMessage.error('获取存储卷列表失败')
    volumes.value = []
    total.value = 0
  } finally {
    loading.value = false
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

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  fetchVolumes()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchVolumes()
}

// 添加计算属性处理排序
const sortedVolumes = computed(() => {
  return [...volumes.value].sort((a, b) => {
    return a.Name.localeCompare(b.Name)
  })
})

// 删除 handleSortChange 函数，因为我们使用计算属性来处理排序

onMounted(() => {
  fetchVolumes()
})
</script>

<style scoped>
/* 继承 layout.css 的 compact-table 样式 */

.volumes-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-right: 4px;
}

.operation-bar {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.volumes-table {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* 状态圆点 - 统一使用 Images.vue 的样式 */
.status-dot {
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  white-space: nowrap;
}
.status-dot::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 8px;
  flex-shrink: 0;
}
.status-used {
  color: var(--el-color-success);
}
.status-used::before {
  background-color: var(--el-color-success);
  box-shadow: 0 0 0 2px var(--el-color-success-light-9);
}
.status-unused {
  color: var(--el-text-color-secondary);
}
.status-unused::before {
  background-color: var(--el-color-info-light-3);
}

.volume-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.text-gray {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-used {
  background-color: var(--el-color-success);
  box-shadow: 0 0 0 2px var(--el-color-success-light-9);
}

.status-unused {
  background-color: var(--el-color-info-light-3);
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

/* 覆盖 Element Plus 样式 */
:deep(.el-table__row) {
  height: 44px;
}

:deep(.el-button--link) {
  padding: 4px;
  height: auto;
}

.text-center {
  text-align: left;
}

.whitespace-pre-line {
  white-space: pre-line;
}

.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>