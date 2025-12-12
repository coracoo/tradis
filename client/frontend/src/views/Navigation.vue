<template>
  <div class="container">
    <div class="header">
      <div class="title">导航栏</div>
      <div class="actions">
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>添加应用
        </el-button>
        <el-button @click="handleManageCategories">
          <el-icon><Operation /></el-icon>分类管理
        </el-button>
        <el-button @click="handleRefresh">
          <el-icon><Refresh /></el-icon>刷新
        </el-button>
        <el-button :type="showDeleted ? 'warning' : 'default'" @click="toggleShowDeleted">
          <el-icon><Delete /></el-icon> {{ showDeleted ? '显示正常' : '回收站' }}
        </el-button>
      </div>
    </div>

    <!-- Grouped Applications -->
    <div v-for="(group, groupName) in groupedApps" :key="groupName" class="category-section">
      <div class="category-header">
        <el-icon><Folder /></el-icon>
        <span>{{ groupName }}</span>
      </div>
      <div class="app-grid">
        <el-card 
          v-for="app in group" 
          :key="app.id" 
          class="app-card"
          :body-style="{ padding: '0px' }"
          shadow="hover"
        >
          <div class="app-content">
            <div class="app-icon">
              <el-icon v-if="app.icon && app.icon.startsWith('mdi-')" :size="40">
                 <!-- Icon handling can be improved, simplified for now assuming MDI classes or image -->
                 <component :is="app.icon.replace('mdi-', '')" /> 
              </el-icon>
              <img v-else-if="app.icon" :src="app.icon" :alt="app.title" class="app-icon-img">
              <el-icon v-else :size="40"><Monitor /></el-icon>
            </div>
            <div class="app-info">
              <h3>{{ app.title }}</h3>
              <div class="app-tags">
                 <el-tag v-if="app.is_auto" size="small" type="success">Auto</el-tag>
                 <el-tag v-if="app.is_deleted" size="small" type="danger">Deleted</el-tag>
              </div>
            </div>
          </div>
          
          <div class="app-actions">
            <el-button-group v-if="!app.is_deleted">
              <el-button size="small" type="primary" link @click.stop="handleEdit(app)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" link @click.stop="handleDelete(app)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-button-group>
            <el-button-group v-else>
              <el-button size="small" type="success" link @click.stop="handleRestore(app)">
                <el-icon><RefreshLeft /></el-icon> 恢复
              </el-button>
            </el-button-group>
          </div>

          <div class="app-links">
             <el-button 
               v-if="app.lan_url" 
               type="primary" 
               plain 
               size="small" 
               class="link-btn"
               @click.stop="openApp(app.lan_url)"
             >
               内网访问
             </el-button>
             <el-button 
               v-if="app.wan_url" 
               type="success" 
               plain 
               size="small" 
               class="link-btn"
               @click.stop="openApp(app.wan_url)"
             >
               外网访问
             </el-button>
          </div>
        </el-card>
      </div>
    </div>
    
    <el-empty v-if="Object.keys(groupedApps).length === 0" description="暂无导航项" />

    <!-- 添加/编辑对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="500px"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称" required>
          <el-input v-model="form.title" placeholder="请输入应用名称" />
        </el-form-item>
        <el-form-item label="分类">
           <el-select
              v-model="form.category"
              filterable
              allow-create
              default-first-option
              placeholder="选择或输入分类"
              style="width: 100%"
            >
              <el-option
                v-for="item in categoryOptions"
                :key="item"
                :label="item"
                :value="item"
              />
            </el-select>
        </el-form-item>
        <el-form-item label="图标">
           <el-input v-model="form.icon" placeholder="图标URL或mdi-icon" />
        </el-form-item>
        <el-form-item label="内网URL">
          <el-input v-model="form.lan_url" placeholder="http://192.168.1.100:port" />
        </el-form-item>
        <el-form-item label="外网URL">
          <el-input v-model="form.wan_url" placeholder="https://example.com" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSave">确定</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 分类管理对话框 -->
    <el-dialog
      title="分类管理"
      v-model="manageDialogVisible"
      width="500px"
    >
      <div class="category-manager">
        <div class="category-actions" style="margin-bottom: 15px;">
           <el-button type="primary" @click="handleAddCategory">
             <el-icon><Plus /></el-icon> 新增分类
           </el-button>
        </div>
        <el-table :data="categoryOptions.map(c => ({ name: c }))" style="width: 100%" max-height="400">
           <el-table-column prop="name" label="分类名称" />
           <el-table-column label="操作" width="150" align="right">
             <template #default="scope">
               <el-button-group>
                 <el-button size="small" :disabled="scope.row.name === '容器'" @click="handleRenameCategory(scope.row.name)">
                   <el-icon><Edit /></el-icon>
                 </el-button>
                 <el-button size="small" type="danger" :disabled="scope.row.name === '容器'" @click="handleDeleteCategory(scope.row.name)">
                   <el-icon><Delete /></el-icon>
                 </el-button>
               </el-button-group>
             </template>
           </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="manageDialogVisible = false">关闭</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Folder, Monitor, Edit, Delete, RefreshLeft, Operation } from '@element-plus/icons-vue'
import request from '../api'

const apps = ref([])
const showDeleted = ref(false)
const dialogVisible = ref(false)
const manageDialogVisible = ref(false)
const dialogTitle = ref('添加应用')
const customCategories = ref([])
const form = ref({
  id: null,
  title: '',
  category: '容器',
  icon: '',
  lan_url: '',
  wan_url: ''
})

// 加载自定义分类
onMounted(() => {
  const saved = localStorage.getItem('custom_navigation_categories')
  if (saved) {
    try {
      customCategories.value = JSON.parse(saved)
    } catch (e) {
      console.error('Failed to load custom categories', e)
    }
  }
  fetchApps()
})

const saveCustomCategories = () => {
  localStorage.setItem('custom_navigation_categories', JSON.stringify(customCategories.value))
}

const categoryOptions = computed(() => {
  const categories = new Set(['容器', ...customCategories.value])
  apps.value.forEach(app => {
    if (app.category) {
      categories.add(app.category)
    }
  })
  return Array.from(categories)
})

const groupedApps = computed(() => {
  const groups = {}
  // 初始化所有分类
  categoryOptions.value.forEach(cat => {
    groups[cat] = []
  })
  
  apps.value.forEach(app => {
    const category = app.category || '容器'
    if (!groups[category]) {
      groups[category] = []
    }
    groups[category].push(app)
  })
  
  // 移除没有应用且不在自定义列表中的分类（可选，目前保留所有categoryOptions中的分类）
  // 如果不想显示空的自动分类，可以在这里过滤
  
  return groups
})

const handleManageCategories = () => {
  manageDialogVisible.value = true
}

const handleAddCategory = async () => {
  try {
    const { value } = await ElMessageBox.prompt('请输入新分类名称', '添加分类', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /\S+/,
      inputErrorMessage: '分类名称不能为空'
    })
    
    if (value) {
      if (categoryOptions.value.includes(value)) {
        ElMessage.warning('分类已存在')
        return
      }
      customCategories.value.push(value)
      saveCustomCategories()
      ElMessage.success('添加成功')
    }
  } catch {
    // Cancelled
  }
}

const handleRenameCategory = async (oldName) => {
  try {
    const { value: newName } = await ElMessageBox.prompt(`重命名分类 "${oldName}"`, '重命名', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputValue: oldName,
      inputPattern: /\S+/,
      inputErrorMessage: '分类名称不能为空'
    })

    if (newName && newName !== oldName) {
      if (categoryOptions.value.includes(newName) && !customCategories.value.includes(newName)) {
         // 如果新名字已存在于apps中但不是自定义的，没关系，我们会合并
      } else if (customCategories.value.includes(newName)) {
         ElMessage.warning('分类名已存在')
         return
      }

      // 1. 更新自定义分类列表
      const index = customCategories.value.indexOf(oldName)
      if (index !== -1) {
        customCategories.value[index] = newName
        saveCustomCategories()
      } else {
        // 如果是之前只存在于app中的分类，现在被重命名了，应该加入到自定义列表中吗？
        // 或者只是重命名应用？
        // 为了方便，我们把它加入自定义列表
        customCategories.value.push(newName)
        saveCustomCategories()
      }

      // 2. 更新所有属于该分类的应用
      const appsToUpdate = apps.value.filter(app => app.category === oldName)
      let updatedCount = 0
      
      for (const app of appsToUpdate) {
        try {
          await request.navigation.update(app.id, { ...app, category: newName })
          updatedCount++
        } catch (e) {
          console.error(`Failed to update app ${app.title}`, e)
        }
      }
      
      await fetchApps()
      ElMessage.success(`重命名成功，更新了 ${updatedCount} 个应用`)
    }
  } catch {
    // Cancelled
  }
}

const handleDeleteCategory = async (categoryName) => {
  try {
    const appsInCat = apps.value.filter(app => app.category === categoryName)
    let action = 'cancel'
    
    if (appsInCat.length > 0) {
      action = await ElMessageBox.confirm(
        `分类 "${categoryName}" 下有 ${appsInCat.length} 个应用。`,
        '删除分类',
        {
          distinguishCancelAndClose: true,
          confirmButtonText: '移动应用到"容器"并删除分类',
          cancelButtonText: '仅删除分类(应用保留但分类丢失)',
          type: 'warning'
        }
      ).then(() => 'move').catch((action) => action === 'cancel' ? 'delete_only' : 'cancel')
      
      // Element Plus confirm result handling is tricky with distinguishCancelAndClose
      // Let's simplify interaction
    }
    
    // Simplified confirmation
    await ElMessageBox.confirm(
       `确定要删除分类 "${categoryName}" 吗？\n该分类下的 ${appsInCat.length} 个应用将被移动到默认分类"容器"。`,
       '删除分类',
       {
         confirmButtonText: '确定删除',
         cancelButtonText: '取消',
         type: 'warning'
       }
    )
    
    // Move apps to default
    for (const app of appsInCat) {
      await request.navigation.update(app.id, { ...app, category: '容器' })
    }
    
    // Remove from custom categories
    const index = customCategories.value.indexOf(categoryName)
    if (index !== -1) {
      customCategories.value.splice(index, 1)
      saveCustomCategories()
    }
    
    await fetchApps()
    ElMessage.success('删除成功')
    
  } catch (e) {
    if (e !== 'cancel') console.error(e)
  }
}

const fetchApps = async () => {
  try {
    const response = await request.navigation.list({ include_deleted: showDeleted.value })
    // 处理返回的数据格式
    if (Array.isArray(response)) {
      apps.value = response
    } else if (response && Array.isArray(response.data)) {
      apps.value = response.data
    } else if (response && response.items) {
      apps.value = response.items
    } else {
      apps.value = []
      console.warn('Unknown navigation data format:', response)
    }
  } catch (error) {
    console.error('获取导航失败', error)
    ElMessage.error('获取导航列表失败')
  }
}

const handleRefresh = () => {
  fetchApps()
}

const toggleShowDeleted = () => {
  showDeleted.value = !showDeleted.value
  fetchApps()
}

const handleAdd = () => {
  dialogTitle.value = '添加应用'
  form.value = {
    id: null,
    title: '',
    category: '容器',
    icon: '',
    lan_url: '',
    wan_url: ''
  }
  dialogVisible.value = true
}

const handleEdit = (app) => {
  dialogTitle.value = '编辑应用'
  form.value = {
    id: app.id,
    title: app.title,
    category: app.category || '容器',
    icon: app.icon,
    lan_url: app.lan_url || '',
    wan_url: app.wan_url || ''
  }
  dialogVisible.value = true
}

const handleSave = async () => {
  if (!form.value.title) {
    ElMessage.warning('请输入应用名称')
    return
  }
  if (!form.value.lan_url && !form.value.wan_url) {
    ElMessage.warning('请至少输入一个访问地址')
    return
  }

  try {
    if (form.value.id) {
      await request.put(`/navigation/${form.value.id}`, {
          title: form.value.title,
          category: form.value.category,
          icon: form.value.icon,
          lanUrl: form.value.lan_url,
          wanUrl: form.value.wan_url
      })
      ElMessage.success('更新成功')
    } else {
      await request.post('/navigation', {
          title: form.value.title,
          category: form.value.category,
          icon: form.value.icon,
          lanUrl: form.value.lan_url,
          wanUrl: form.value.wan_url
      })
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    fetchApps()
  } catch (error) {
    console.error('Save failed:', error)
    ElMessage.error('保存失败')
  }
}

const handleDelete = async (app) => {
  try {
    await ElMessageBox.confirm('确定要删除这个导航项吗？', '提示', {
      type: 'warning'
    })
    await request.navigation.delete(app.id)
    ElMessage.success('已移至回收站')
    fetchApps()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除失败', error)
      ElMessage.error('删除失败')
    }
  }
}

const handleRestore = async (app) => {
  try {
    await request.navigation.restore(app.id)
    ElMessage.success('恢复成功')
    fetchApps()
  } catch (error) {
    console.error('恢复失败', error)
    ElMessage.error('恢复失败')
  }
}

const openApp = (url) => {
  if (!url) {
    ElMessage.warning('无效的URL')
    return
  }
  // 确保URL包含协议
  let targetUrl = url
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    targetUrl = 'http://' + url
  }
  window.open(targetUrl, '_blank')
}

onMounted(() => {
  fetchApps()
})
</script>

<style scoped>
.container {
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.title {
  font-size: 24px;
  font-weight: bold;
}

.category-section {
  margin-bottom: 30px;
}

.category-header {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 15px;
  color: var(--el-text-color-regular);
  border-bottom: 1px solid var(--el-border-color-lighter);
  padding-bottom: 5px;
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 20px;
}

.app-card {
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: default;
  position: relative;
  display: flex;
  flex-direction: column;
}

.app-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}

.app-content {
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 15px;
}

.app-icon {
  width: 60px;
  height: 60px;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: var(--el-fill-color-light);
  border-radius: 12px;
  color: var(--el-color-primary);
}

.app-icon-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 12px;
}

.app-info {
  flex: 1;
  overflow: hidden;
}

.app-info h3 {
  margin: 0 0 5px 0;
  font-size: 16px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-tags {
  display: flex;
  gap: 5px;
}

.app-actions {
  position: absolute;
  top: 10px;
  right: 10px;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 10;
}

.app-card:hover .app-actions {
  opacity: 1;
}

.app-links {
  padding: 0 20px 20px 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-start;
}

.link-btn {
  flex: 1;
}
</style>