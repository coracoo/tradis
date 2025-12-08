<template>
  <div class="networks-view compact-table">
    <!-- 顶部操作栏 -->
    <div class="operation-bar">
      <el-button-group>
        <el-button @click="fetchNetworks" :loading="loading">
          <el-icon><Refresh /></el-icon>
        </el-button>
        <el-button type="primary" @click="dialogVisible = true">
          <el-icon class="el-icon--left"><Plus /></el-icon> 
           新建网络
        </el-button>
      </el-button-group>
    </div>

    <!-- 网络列表 -->
    <el-table 
      :data="sortedNetworks" 
      style="width: 100%" 
      v-loading="loading"
      class="networks-table"
      :header-cell-style="{ background: 'transparent' }">
      
      <el-table-column type="selection" width="40" header-align="left" />
      
      <el-table-column 
        prop="Name" 
        label="名称" 
        min-width="150"
        header-align="left">
        <template #default="scope">
          <span class="network-name">{{ scope.row.Name }}</span>
          <el-tag v-if="isDefaultNetwork(scope.row.Name)" size="small" type="info" class="ml-2">System</el-tag>
        </template>
      </el-table-column>
      
      <el-table-column 
        prop="Driver" 
        label="驱动模式"
        width="120"
        header-align="left">
        <template #default="scope">
          <el-tag size="small" effect="plain">{{ scope.row.Driver }}</el-tag>
        </template>
      </el-table-column>
      
      <el-table-column label="子网 / 网关" min-width="200" header-align="left">
        <template #default="scope">
          <div class="network-info font-mono">
            <div v-if="scope.row.IPAM?.Config?.[0]?.Subnet">
              <span class="label">Subnet:</span> {{ scope.row.IPAM.Config[0].Subnet }}
            </div>
            <div v-if="scope.row.IPAM?.Config?.[0]?.Gateway">
              <span class="label">Gateway:</span> {{ scope.row.IPAM.Config[0].Gateway }}
            </div>
            <div v-if="!scope.row.IPAM?.Config?.length" class="text-gray">-</div>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column label="连接容器" min-width="200" header-align="left">
        <template #default="scope">
          <template v-if="scope.row.Containers && Object.keys(scope.row.Containers).length">
            <div class="container-list">
              <template v-for="(container, id, index) in scope.row.Containers" :key="id">
                <el-tooltip :content="container.Name.substring(0)" placement="top" v-if="index < 4">
                   <el-tag size="small" class="container-tag">
                    {{ container.Name.substring(0) }}
                  </el-tag>
                </el-tooltip>
              </template>
              <el-popover
                v-if="Object.keys(scope.row.Containers).length > 4"
                placement="top"
                :width="200"
                trigger="click"
              >
                <template #reference>
                  <el-tag size="small" type="info" class="container-tag cursor-pointer">
                    +{{ Object.keys(scope.row.Containers).length - 4 }}
                  </el-tag>
                </template>
                <div class="popover-container-list">
                  <div v-for="(container, id) in scope.row.Containers" :key="id" class="popover-item">
                    {{ container.Name.substring(1) }}
                  </div>
                </div>
              </el-popover>
            </div>
          </template>
          <span v-else class="text-gray">-</span>
        </template>
      </el-table-column>
      
      <el-table-column 
        prop="Created" 
        label="创建时间"
        width="160"
        header-align="left">
        <template #default="scope">
          <div class="text-gray font-mono text-center whitespace-pre-line">
            {{ formatTimeTwoLines(scope.row.Created) }}
          </div>
        </template>
      </el-table-column>
      
      <el-table-column label="操作" width="100" fixed="right" header-align="left">
        <template #default="scope">
          <el-tooltip content="删除网络" placement="top">
            <el-button 
              link
              type="danger" 
              :disabled="isDefaultNetwork(scope.row.Name)"
              @click="deleteNetwork(scope.row)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        :total="total"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        size="small"
      />
    </div>

    <!-- 创建网络对话框 -->
    <el-dialog 
      v-model="dialogVisible" 
      title="创建网络" 
      width="400px"
      class="compact-dialog">
      <el-form :model="networkForm" label-position="top">
        <el-form-item label="网络名称">
          <el-input v-model="networkForm.name" placeholder="请输入网络名称" />
        </el-form-item>
        <el-form-item label="驱动模式">
          <el-select v-model="networkForm.driver" placeholder="请选择网络模式" style="width: 100%">
            <el-option label="Bridge" value="bridge" />
            <el-option label="Host" value="host" />
            <el-option label="None" value="none" />
            <el-option label="Overlay" value="overlay" />
            <el-option label="Macvlan" value="macvlan" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitNetwork">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Delete } from '@element-plus/icons-vue'
import api from '../api'
import { formatTimeTwoLines } from '../utils/format'
const loading = ref(false)
const networks = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const dialogVisible = ref(false)
const networkForm = ref({
  name: '',
  driver: 'bridge'
})

// 获取网络列表
const fetchNetworks = async () => {
  loading.value = true
  try {
    const data = await api.networks.list()
    networks.value = Array.isArray(data) ? data : []
    total.value = networks.value.length
  } catch (error) {
    ElMessage.error('获取网络列表失败')
    networks.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 创建网络
const submitNetwork = async () => {
  try {
    await api.networks.create(networkForm.value)
    ElMessage.success('网络创建成功')
    dialogVisible.value = false
    fetchNetworks()
  } catch (error) {
    ElMessage.error('创建网络失败')
  }
}

// 删除网络
const deleteNetwork = async (network) => {
  try {
    await ElMessageBox.confirm('确定要删除该网络吗？', '警告', {
      type: 'warning'
    })
    await api.networks.remove(network.Id)
    ElMessage.success('网络已删除')
    fetchNetworks()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  fetchNetworks()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchNetworks()
}

// 添加计算属性处理特殊排序
const sortedNetworks = computed(() => {
  const specialOrder = ['none', 'bridge', 'host']
  return [...networks.value].sort((a, b) => {
    const aIndex = specialOrder.indexOf(a.Name)
    const bIndex = specialOrder.indexOf(b.Name)
    
    if (aIndex !== -1 && bIndex !== -1) return aIndex - bIndex
    if (aIndex !== -1) return -1
    if (bIndex !== -1) return 1
    return a.Name.localeCompare(b.Name)
  })
})

// 删除 handleSortChange 函数

onMounted(() => {
  fetchNetworks()
})

// 添加判断是否为默认网络的方法
const isDefaultNetwork = (name) => {
  const defaultNetworks = ['none', 'host', 'bridge']
  return defaultNetworks.includes(name)
}
</script>

<style scoped>
/* 继承 layout.css 的 compact-table 样式 */

.networks-view {
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

.networks-table {
  flex: 1;
  min-height: 0;
}

.network-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.network-info {
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.label {
  color: var(--el-text-color-secondary);
  margin-right: 4px;
}

.text-gray {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.container-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.container-tag {
  margin: 0;
}

.popover-container-list {
  max-height: 200px;
  overflow-y: auto;
}

.popover-item {
  padding: 6px 8px;
  font-size: 13px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  color: var(--el-text-color-regular);
}

.popover-item:last-child {
  border-bottom: none;
}

/* 覆盖 Element Plus 样式 */
:deep(.el-table__row) {
  height: 44px;
}
:deep(.el-button--link) {
  padding: 4px;
  height: auto;
}

.ml-2 {
  margin-left: 8px;
}
.cursor-pointer {
  cursor: pointer;
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
