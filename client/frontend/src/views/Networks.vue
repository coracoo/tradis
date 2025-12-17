<template>
  <div class="networks-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索网络名称..."
          clearable
          class="search-input"
          size="medium"
          @keyup.enter="fetchNetworks"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <div class="filter-right">
        <el-button-group class="main-actions">
          <el-button @click="fetchNetworks" :loading="loading" plain size="medium">
            <template #icon><el-icon><Refresh /></el-icon></template>
            刷新
          </el-button>
          <el-button type="primary" @click="dialogVisible = true" size="medium">
            <template #icon><el-icon><Plus /></el-icon></template>
            新建网络
          </el-button>
        </el-button-group>
        
        <el-dropdown trigger="click" @command="handleGlobalAction">
          <el-button plain class="more-btn" size="medium">
            更多操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="ipv6">配置 Bridge IPv6</el-dropdown-item>
              <el-dropdown-item command="prune" :icon="Remove">清理未使用网络</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table 
        :data="paginatedNetworks" 
        style="width: 100%; height: 100%" 
        v-loading="loading"
        class="main-table"
        @sort-change="handleSortChange"
        :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--el-text-color-primary)', fontWeight: 600, fontSize: '14px', height: '50px' }"
        :row-style="{ height: '60px' }"
      >
        
        <el-table-column prop="Name" label="名称" sortable="custom" min-width="200" header-align="left">
          <template #default="scope">
            <div class="network-name-cell">
              <div class="icon-wrapper network">
                <el-icon><Connection /></el-icon>
              </div>
              <div class="name-info">
                <span class="name-text">{{ scope.row.Name }}</span>
                <span class="type-tag system" v-if="isDefaultNetwork(scope.row.Name)">系统内置</span>
                <span class="type-tag custom" v-else>自定义网络</span>
              </div>
            </div>
          </template>
        </el-table-column>
        
        <el-table-column prop="Driver" label="驱动模式" sortable="custom" width="120" header-align="left">
          <template #default="scope">
            <el-tag size="default" effect="light" class="driver-tag">{{ scope.row.Driver }}</el-tag>
          </template>
        </el-table-column>
        
        <el-table-column prop="Subnet" label="子网 / 网关" min-width="240" sortable="custom" header-align="left">
          <template #default="scope">
            <div class="network-config-info font-mono">
              <div v-if="scope.row.IPAM?.Config?.[0]?.Subnet" class="config-row">
                <span class="label">Subnet:</span> <span class="value">{{ scope.row.IPAM.Config[0].Subnet }}</span>
              </div>
              <div v-if="scope.row.IPAM?.Config?.[0]?.Gateway" class="config-row">
                <span class="label">Gateway:</span> <span class="value">{{ scope.row.IPAM.Config[0].Gateway }}</span>
              </div>
              <div v-if="!scope.row.IPAM?.Config?.length" class="text-gray">-</div>
            </div>
          </template>
        </el-table-column>
        
        <el-table-column prop="Containers" label="连接容器" min-width="200" sortable="custom" header-align="left">
          <template #default="scope">
            <template v-if="scope.row.Containers && Object.keys(scope.row.Containers).length">
              <div class="container-list">
                <template v-for="(container, id, index) in scope.row.Containers" :key="id">
                  <el-tooltip :content="container.Name.substring(0)" placement="top" v-if="index < 3">
                     <el-tag size="medium" class="container-tag" effect="plain">
                      {{ container.Name.substring(0) }}
                    </el-tag>
                  </el-tooltip>
                </template>
                <el-popover
                  v-if="Object.keys(scope.row.Containers).length > 3"
                  placement="top"
                  :width="200"
                  trigger="hover"
                >
                  <template #reference>
                    <el-tag size="medium" type="info" class="container-tag cursor-pointer">
                      +{{ Object.keys(scope.row.Containers).length - 3 }}
                    </el-tag>
                  </template>
                  <div class="popover-container-list">
                    <div v-for="(container, id) in scope.row.Containers" :key="id" class="popover-item">
                      {{ container.Name.substring(0) }}
                    </div>
                  </div>
                </el-popover>
              </div>
            </template>
            <span v-else class="text-gray">-</span>
          </template>
        </el-table-column>
        
        <el-table-column prop="Created" label="创建时间" sortable="custom" width="180" header-align="left">
          <template #default="scope">
            <div class="text-gray font-mono">
              {{ formatTimeTwoLines(scope.row.Created) }}
            </div>
          </template>
        </el-table-column>
        
        <el-table-column label="操作" width="150" fixed="right" header-align="center">
          <template #default="scope">
            <div class="row-ops">
              <el-tooltip content="编辑网络" placement="top">
                <el-button 
                  circle
                  plain
                  type="primary" 
                  :disabled="isDefaultNetwork(scope.row.Name)"
                  @click="openEditDialog(scope.row)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除网络" placement="top">
                <el-button 
                  circle
                  plain
                  type="danger" 
                  :disabled="isDefaultNetwork(scope.row.Name)"
                  @click="deleteNetwork(scope.row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="filteredNetworks.length"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>

    <!-- 创建网络对话框 -->
    <el-dialog 
      v-model="dialogVisible" 
      title="创建网络" 
      width="450px"
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
        <template v-if="networkForm.driver === 'bridge'">
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="IPv4 子网">
                <el-input v-model="networkForm.ipv4Subnet" placeholder="例如: 172.21.0.0/16" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="IPv4 网关">
                <el-input v-model="networkForm.ipv4Gateway" placeholder="例如: 172.21.0.1" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="启用 IPv6">
            <el-switch v-model="networkForm.enableIPv6" />
          </el-form-item>
          <template v-if="networkForm.enableIPv6">
             <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="IPv6 子网">
                  <el-input v-model="networkForm.ipv6Subnet" placeholder="例如: fd00::/64" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="IPv6 网关">
                  <el-input v-model="networkForm.ipv6Gateway" placeholder="例如: fd00::1" />
                </el-form-item>
              </el-col>
            </el-row>
          </template>
        </template>
        <template v-if="networkForm.driver === 'macvlan'">
          <el-form-item label="父接口 parent">
            <el-input v-model="networkForm.parent" placeholder="例如: eth0 或 ens33" />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="IPv4 子网">
                <el-input v-model="networkForm.ipv4Subnet" placeholder="例如: 192.168.1.0/24" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
               <el-form-item label="IPv6 子网">
                <el-input v-model="networkForm.ipv6Subnet" placeholder="例如: 2001:db8::/64" />
              </el-form-item>
            </el-col>
          </el-row>
          <div class="text-gray" style="font-size: 12px; line-height: 1.5; margin-top: 8px;">
            示例: 创建 macvlan 需在宿主机上配置路由，parent 为物理接口。
          </div>
        </template>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitNetwork">确定</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 编辑网络对话框 -->
    <el-dialog 
      v-model="editDialogVisible" 
      title="编辑网络" 
      width="450px"
      class="compact-dialog">
      <el-form :model="editForm" label-position="top">
        <template v-if="editForm.driver === 'bridge'">
          <el-row :gutter="12">
            <el-col :span="12">
               <el-form-item label="IPv4 子网">
                <el-input v-model="editForm.ipv4Subnet" placeholder="172.21.0.0/16" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="IPv4 网关">
                <el-input v-model="editForm.ipv4Gateway" placeholder="172.21.0.1" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="启用 IPv6">
            <el-switch v-model="editForm.enableIPv6" />
          </el-form-item>
          <template v-if="editForm.enableIPv6">
             <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="IPv6 子网">
                  <el-input v-model="editForm.ipv6Subnet" placeholder="fd00::/64" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="IPv6 网关">
                  <el-input v-model="editForm.ipv6Gateway" placeholder="fd00::1" />
                </el-form-item>
              </el-col>
            </el-row>
          </template>
        </template>
        <template v-else-if="editForm.driver === 'macvlan'">
          <el-form-item label="父接口 parent">
            <el-input v-model="editForm.parent" placeholder="eth0 / ens33" />
          </el-form-item>
           <el-row :gutter="12">
            <el-col :span="12">
               <el-form-item label="IPv4 子网">
                <el-input v-model="editForm.ipv4Subnet" placeholder="192.168.1.0/24" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="IPv6 子网">
                <el-input v-model="editForm.ipv6Subnet" placeholder="2001:db8::/64" />
              </el-form-item>
            </el-col>
          </el-row>
        </template>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitEditNetwork">确定</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Bridge IPv6 配置对话框 -->
    <el-dialog
      v-model="ipv6DialogVisible"
      title="配置 Bridge IPv6"
      width="400px"
      class="compact-dialog"
    >
      <div class="ipv6-settings">
        <p class="mb-4 text-gray-500 text-sm">此设置将影响默认 bridge 网络和新创建的 bridge 网络。</p>
        <el-form label-position="top">
          <el-form-item label="启用 IPv6">
             <el-switch v-model="bridgeIPv6.enable" />
          </el-form-item>
          <el-form-item label="Fixed CIDR v6 (可选)">
             <el-input v-model="bridgeIPv6.fixedCIDRv6" placeholder="fd00::/64" />
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="ipv6DialogVisible = false">取消</el-button>
          <el-button type="primary" @click="applyBridgeIPv6">应用并重启 Docker</el-button>
        </span>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Delete, Edit, Search, ArrowDown, Remove, Connection } from '@element-plus/icons-vue'
import api from '../api'
import request from '../utils/request'
import { formatTimeTwoLines } from '../utils/format'

const loading = ref(false)
const networks = ref([])
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const dialogVisible = ref(false)
const ipv6DialogVisible = ref(false)
const sortState = ref({ prop: '', order: '' })

const networkForm = ref({
  name: '',
  driver: 'bridge',
  ipv4Subnet: '',
  ipv4Gateway: '',
  enableIPv6: false,
  ipv6Subnet: '',
  ipv6Gateway: '',
  parent: ''
})

const bridgeIPv6 = ref({
  enable: false,
  fixedCIDRv6: ''
})

const editDialogVisible = ref(false)
const editForm = ref({
  driver: 'bridge',
  ipv4Subnet: '',
  ipv4Gateway: '',
  enableIPv6: false,
  ipv6Subnet: '',
  ipv6Gateway: '',
  parent: '',
  options: {}
})
let editingNetwork = null

// 获取网络列表
const fetchNetworks = async () => {
  loading.value = true
  try {
    const data = await api.networks.list()
    networks.value = Array.isArray(data) ? data : []
  } catch (error) {
    ElMessage.error('获取网络列表失败')
    networks.value = []
  } finally {
    loading.value = false
  }
}

// 过滤和排序
const handleSortChange = ({ prop, order }) => {
  sortState.value = { prop, order }
}

const filteredNetworks = computed(() => {
  let list = [...networks.value]
  
  // 1. 搜索过滤
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(n => {
      return n.Name.toLowerCase().includes(q) || n.Driver.toLowerCase().includes(q)
    })
  }

  // 2. 排序
  if (sortState.value.prop && sortState.value.order) {
    const { prop, order } = sortState.value
    list.sort((a, b) => {
      let valA, valB
      switch (prop) {
        case 'Name':
          valA = a.Name || ''
          valB = b.Name || ''
          break
        case 'Driver':
          valA = a.Driver || ''
          valB = b.Driver || ''
          break
        case 'Subnet':
          valA = a.IPAM?.Config?.[0]?.Subnet || ''
          valB = b.IPAM?.Config?.[0]?.Subnet || ''
          break
        case 'Containers':
          valA = Object.keys(a.Containers || {}).length
          valB = Object.keys(b.Containers || {}).length
          break
        case 'Created':
          valA = a.Created || ''
          valB = b.Created || ''
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
    // 默认排序：特殊网络在前，然后按名称
    const specialOrder = ['none', 'bridge', 'host']
    list.sort((a, b) => {
      const aIndex = specialOrder.indexOf(a.Name)
      const bIndex = specialOrder.indexOf(b.Name)
      
      if (aIndex !== -1 && bIndex !== -1) return aIndex - bIndex
      if (aIndex !== -1) return -1
      if (bIndex !== -1) return 1
      return a.Name.localeCompare(b.Name)
    })
  }
  
  return list
})

const paginatedNetworks = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredNetworks.value.slice(start, end)
})

const handleGlobalAction = (command) => {
  if (command === 'ipv6') {
    ipv6DialogVisible.value = true
  } else if (command === 'prune') {
    pruneNetworks()
  }
}

// Prune Networks
const pruneNetworks = async () => {
  try {
    await ElMessageBox.confirm('确定要清理所有未使用的网络吗？', '清理网络', {
      type: 'warning'
    })
    await api.networks.prune()
    ElMessage.success('清理完成')
    fetchNetworks()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('清理失败')
  }
}

// 创建网络
const submitNetwork = async () => {
  try {
    if (networkForm.value.driver === 'bridge') {
      const dup = networks.value.some(n => n.Name === networkForm.value.name && n.Driver === 'bridge')
      if (dup) {
        ElMessage.warning('同名的 bridge 网络已存在')
        return
      }
    }
    await api.networks.create(networkForm.value)
    ElMessage.success('网络创建成功')
    dialogVisible.value = false
    fetchNetworks()
  } catch (error) {
    ElMessage.error('创建网络失败')
  }
}

const applyBridgeIPv6 = async () => {
  try {
    const payload = {
      fixedCIDRv6: bridgeIPv6.value.fixedCIDRv6 || ''
    }
    // Assuming backend endpoint exists as per previous code
    await request.post('/networks/bridge/enable-ipv6', payload)
    ElMessage.success('已启用 IPv6，请重启 Docker')
    ipv6DialogVisible.value = false
  } catch (e) {
    ElMessage.error('启用 IPv6 失败')
  }
}

const openEditDialog = (network) => {
  editingNetwork = network
  editForm.value = {
    driver: network.Driver,
    ipv4Subnet: '',
    ipv4Gateway: '',
    enableIPv6: false,
    ipv6Subnet: '',
    ipv6Gateway: '',
    parent: network.Options?.parent || '',
    options: { ...network.Options }
  }
  const configs = (network.IPAM && Array.isArray(network.IPAM.Config)) ? network.IPAM.Config : []
  configs.forEach(c => {
    if (c.Subnet && c.Subnet.includes(':')) {
      editForm.value.enableIPv6 = true
      editForm.value.ipv6Subnet = c.Subnet || ''
      editForm.value.ipv6Gateway = c.Gateway || ''
    } else {
      editForm.value.ipv4Subnet = c.Subnet || ''
      editForm.value.ipv4Gateway = c.Gateway || ''
    }
  })
  editDialogVisible.value = true
}

const submitEditNetwork = async () => {
  if (!editingNetwork) return
  try {
    await api.networks.update(editingNetwork.Id, editForm.value)
    ElMessage.success('网络修改成功')
    editDialogVisible.value = false
    fetchNetworks()
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || '网络修改失败')
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
  currentPage.value = 1
}

const handleCurrentChange = (val) => {
  currentPage.value = val
}

onMounted(() => {
  fetchNetworks()
})

const isDefaultNetwork = (name) => {
  const defaultNetworks = ['none', 'host', 'bridge']
  return defaultNetworks.includes(name)
}
</script>

<style scoped>
.networks-view {
  height: 100%;
  width: 100%;
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
  width: 300px;
}

.table-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.main-table {
  flex: 1;
}

/* Custom Table Styles */
.network-name-cell {
  display: flex;
  align-items: center;
  gap: 16px;
}

.icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
}

.icon-wrapper.network {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.name-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.name-text {
  font-weight: 600;
  color: var(--el-text-color-primary);
  font-size: 14px;
}

.type-tag {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-start;
  font-weight: 500;
}

.type-tag.custom {
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
}

.type-tag.system {
  color: var(--el-color-success);
  background: var(--el-color-success-light-9);
}

.network-config-info {
  font-size: 13px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-row {
  display: flex;
  gap: 8px;
}

.config-row .label {
  color: var(--el-text-color-secondary);
  width: 60px;
}

.container-tag {
  color: var(--el-text-color-regular);
}

.container-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
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

.row-ops {
  display: flex;
  justify-content: center;
  gap: 8px;
}

/* Pagination */
.pagination-bar {
  padding: 16px 24px;
  border-top: 1px solid var(--el-border-color-light);
  display: flex;
  justify-content: flex-end;
}

.text-gray {
  color: var(--el-text-color-secondary);
}

.font-mono {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Utilities */
.cursor-pointer {
  cursor: pointer;
}

.mb-4 {
  margin-bottom: 16px;
}

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