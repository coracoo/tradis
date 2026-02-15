<template>
  <div class="nav-view">
    <div class="filter-bar clay-surface">
      <div class="filter-left">
        <el-button type="primary" @click="handleAdd" size="medium">
          <template #icon><IconEpPlus /></template>
          添加应用
        </el-button>
        <el-button @click="handleManageCategories" size="medium">
          <template #icon><IconEpOperation /></template>
          分类管理
        </el-button>
      </div>
      <div class="filter-right">
        <el-button :type="showDeleted ? 'warning' : 'default'" @click="toggleShowDeleted" plain size="medium">
          <template #icon><IconEpDelete /></template>
          {{ showDeleted ? '显示正常' : '回收站' }}
        </el-button>
        <el-button type="danger" @click="handleRebuild" plain size="medium">
          <template #icon><IconEpRefresh /></template>
          重新识别
        </el-button>
        <el-button @click="handleRefresh" plain size="medium">
          <template #icon><IconEpRefresh /></template>
          刷新
        </el-button>
      </div>
    </div>

    <div class="content-wrapper clay-surface">
      <div class="scroll-content">
        <div v-for="(group, groupName) in groupedApps" :key="groupName" class="category-section">
          <div class="category-header">
            <IconEpFolder />
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
                <div class="app-icon-wrapper">
                  <i v-if="isMdiIcon(app.icon_url)" :class="['mdi', app.icon_url, 'mdi-icon']"></i>
                  <img v-else-if="app.icon_url" :src="resolveIconUrl(app.icon_url)" :alt="app.title" class="app-icon-img">
                  <IconEpMonitor v-else class="app-default-icon" />
                </div>
                <div class="app-info">
                  <h3 class="app-title">{{ app.title }}</h3>
                  <div class="app-tags">
                     <el-tag v-if="app.is_auto" size="small" type="danger" effect="plain">Auto</el-tag>
                     <el-tag v-if="app.ai_generated" size="small" type="success" effect="plain">AI</el-tag>
                     <el-tag v-if="app.is_deleted" size="small" type="danger" effect="plain">Deleted</el-tag>
                  </div>
                </div>
              </div>
              
              <div class="app-actions-overlay">
                <el-button-group v-if="!app.is_deleted">
                  <el-button size="small" type="primary" circle @click.stop="handleEdit(app)">
                    <IconEpEdit />
                  </el-button>
                  <el-button size="small" type="danger" circle @click.stop="handleDelete(app)">
                    <IconEpDelete />
                  </el-button>
                </el-button-group>
                <el-button-group v-else>
                  <el-button size="small" type="success" @click.stop="handleRestore(app)">
                    <IconEpRefreshLeft class="el-icon--left" /> 恢复
                  </el-button>
                </el-button-group>
              </div>

              <div class="app-footer">
                 <el-button 
                   type="primary" 
                   plain 
                   size="small" 
                   class="link-btn"
                   @click.stop="openLan(app)"
                 >
                   内网
                 </el-button>
                 <el-button 
                   type="success" 
                   plain 
                   size="small" 
                   class="link-btn"
                   @click.stop="openWan(app)"
                 >
                   外网
                 </el-button>
              </div>
            </el-card>
          </div>
        </div>
        
        <el-empty v-if="Object.keys(groupedApps).length === 0" description="暂无导航项" />
      </div>
    </div>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="500px"
      append-to-body
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
           <el-input v-model="form.icon_url" placeholder="图标URL或mdi-icon" />
           <el-upload
             :show-file-list="true"
             :auto-upload="false"
             :on-change="handleIconChange"
             :on-remove="handleIconRemove"
             accept="image/*"
             style="margin-top: 8px; width: 100%"
           >
             <el-button size="small">上传图片</el-button>
             <template #tip>
               <div class="el-upload__tip">支持 mdi-icon / URL / 本地文件（png/jpg/jpeg/webp/gif/svg/ico/avif/bmp/tif）</div>
             </template>
           </el-upload>
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
      append-to-body
    >
      <div class="category-manager">
        <div class="category-actions" style="margin-bottom: 15px;">
           <el-button type="primary" @click="handleAddCategory" size="small">
             <IconEpPlus class="el-icon--left" /> 新增分类
           </el-button>
        </div>
        <el-table :data="categoryOptions.map(c => ({ name: c }))" style="width: 100%" max-height="400" border>
           <el-table-column prop="name" label="分类名称" />
           <el-table-column label="操作" width="120" align="center">
             <template #default="scope">
               <el-button-group>
                <el-button size="small" :disabled="scope.row.name === '默认'" @click="handleRenameCategory(scope.row.name)">
                  <IconEpEdit />
                </el-button>
                <el-button size="small" type="danger" :disabled="scope.row.name === '默认'" @click="handleDeleteCategory(scope.row.name)">
                  <IconEpDelete />
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
import api from '../api'

const apps = ref([])
const showDeleted = ref(false)
const dialogVisible = ref(false)
const manageDialogVisible = ref(false)
const dialogTitle = ref('添加应用')
const customCategories = ref([])
const form = ref({
  id: null,
  title: '',
  category: '默认',
  icon_url: '',
  lan_url: '',
  wan_url: ''
})
const pendingIconFile = ref(null)
const handleIconChange = (file) => {
  pendingIconFile.value = file?.raw || file
}
const handleIconRemove = () => {
  pendingIconFile.value = null
}

const getServerBase = () => {
  const base = import.meta.env.VITE_API_BASE_URL || ''
  if (!base) return ''
  if (base.endsWith('/api')) return base.slice(0, -4)
  return base
}

const isMdiIcon = (iconUrl) => {
  return typeof iconUrl === 'string' && iconUrl.startsWith('mdi-')
}

const resolveIconUrl = (u) => {
  if (!u) return ''
  if (u.startsWith('clay:')) {
    const name = u.slice(5).trim()
    if (!name) return ''
    if (name.includes('.')) return `/icons/clay/${name}`
    return `/icons/clay/${name}.png`
  }
  if (u.startsWith('/icons/clay/')) return u
  if (u.startsWith('http://') || u.startsWith('https://')) return u
  const serverBase = getServerBase()
  if (u.startsWith('/')) return serverBase + u
  return serverBase ? (serverBase + '/' + u) : u
}

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
  const categories = new Set(['默认', ...customCategories.value])
  apps.value.forEach(app => {
    if (app.category) {
      categories.add(app.category)
    }
  })
  return Array.from(categories)
})

const groupedApps = computed(() => {
  const groups = {}
  categoryOptions.value.forEach(cat => {
    groups[cat] = []
  })
  
  apps.value.forEach(app => {
    const category = app.category || '默认'
    if (!groups[category]) {
      groups[category] = []
    }
    groups[category].push(app)
  })
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
  } catch {}
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
      } else if (customCategories.value.includes(newName)) {
         ElMessage.warning('分类名已存在')
         return
      }

      const index = customCategories.value.indexOf(oldName)
      if (index !== -1) {
        customCategories.value[index] = newName
        saveCustomCategories()
      } else {
        customCategories.value.push(newName)
        saveCustomCategories()
      }

      const appsToUpdate = apps.value.filter(app => app.category === oldName)
      let updatedCount = 0
      
      for (const app of appsToUpdate) {
        try {
          await api.navigation.update(app.id, { ...app, category: newName })
          updatedCount++
        } catch (e) {
          console.error(`Failed to update app ${app.title}`, e)
        }
      }
      
      await fetchApps()
      ElMessage.success(`重命名成功，更新了 ${updatedCount} 个应用`)
    }
  } catch {}
}

const handleDeleteCategory = async (categoryName) => {
  try {
    const appsInCat = apps.value.filter(app => app.category === categoryName)
    
    await ElMessageBox.confirm(
       `确定要删除分类 "${categoryName}" 吗？\n该分类下的 ${appsInCat.length} 个应用将被移动到默认分类"默认"。`,
       '删除分类',
       {
         confirmButtonText: '确定删除',
         cancelButtonText: '取消',
         type: 'warning'
       }
    )
    
    for (const app of appsInCat) {
      await api.navigation.update(app.id, { ...app, category: '默认' })
    }
    
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
    const response = await api.navigation.list({ include_deleted: showDeleted.value })
    if (Array.isArray(response)) {
      apps.value = response
    } else if (response && Array.isArray(response.data)) {
      apps.value = response.data
    } else if (response && response.items) {
      apps.value = response.items
    } else {
      apps.value = []
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
    category: '默认',
    icon_url: '',
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
    category: app.category || '默认',
    icon_url: app.icon_url,
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
      await api.navigation.update(form.value.id, {
        title: form.value.title,
        category: form.value.category,
        icon_url: form.value.icon_url,
        lan_url: form.value.lan_url,
        wan_url: form.value.wan_url
      })
      if (pendingIconFile.value) {
        try {
          const upRes = await api.navigation.uploadIcon(form.value.id, pendingIconFile.value)
          if (upRes && upRes.icon_url) {
            form.value.icon_url = upRes.icon_url
            await api.navigation.update(form.value.id, {
              title: form.value.title,
              category: form.value.category,
              icon_url: form.value.icon_url,
              lan_url: form.value.lan_url,
              wan_url: form.value.wan_url
            })
          }
        } catch (e) {
          console.error('上传图标失败', e)
          ElMessage.warning('图标上传失败')
        } finally {
          pendingIconFile.value = null
        }
      }
      ElMessage.success('更新成功')
    } else {
      const created = await api.navigation.add({
        title: form.value.title,
        category: form.value.category,
        icon_url: form.value.icon_url,
        lan_url: form.value.lan_url,
        wan_url: form.value.wan_url
      })
      if (created && created.id && pendingIconFile.value) {
        try {
          const upRes = await api.navigation.uploadIcon(created.id, pendingIconFile.value)
          if (upRes && upRes.icon_url) {
            form.value.icon_url = upRes.icon_url
            await api.navigation.update(created.id, {
              title: form.value.title,
              category: form.value.category,
              icon_url: form.value.icon_url,
              lan_url: form.value.lan_url,
              wan_url: form.value.wan_url
            })
          }
        } catch (e) {
          console.error('上传图标失败', e)
          ElMessage.warning('图标上传失败')
        } finally {
          pendingIconFile.value = null
        }
      }
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
    await ElMessageBox.confirm('确定要删除这个导航项吗？', '提示', { type: 'warning' })
    await api.navigation.delete(app.id)
    ElMessage.success('已移至回收站')
    fetchApps()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const handleRestore = async (app) => {
  try {
    await api.navigation.restore(app.id)
    ElMessage.success('恢复成功')
    fetchApps()
  } catch (error) {
    ElMessage.error('恢复失败')
  }
}

const openApp = (url) => {
  if (!url) {
    ElMessageBox.alert('请先到设置中维护URL地址', '提示')
    return
  }
  let targetUrl = url
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    targetUrl = 'http://' + url
  }
  window.open(targetUrl, '_blank')
}
const openLan = (app) => {
  openApp(app.lan_url)
}
const openWan = (app) => {
  openApp(app.wan_url)
}

const handleRebuild = async () => {
  try {
    await ElMessageBox.confirm('将清空导航数据库并按当前容器重新生成，确定继续？', '提示', { type: 'warning' })
    await api.system.navigationRebuild()
    const aiResult = await api.ai.enrichNavigation({ force: true, limit: 50 })
    const attempted = aiResult?.attempted ?? aiResult?.data?.attempted ?? 0
    if (attempted > 0) {
      ElMessage.success(`导航已重新识别，AI 已触发 ${attempted} 项`)
    } else {
      ElMessage.success('导航已重新识别，AI 未匹配到可处理项')
    }
    fetchApps()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('重新识别失败')
  }
}
</script>

<style scoped>
.nav-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
  padding: 12px 16px;
  background-color: var(--clay-bg);
  gap: 12px;
}

/* Filter Bar - Extracted to layout.css */
/* Content Wrapper - Extracted to layout.css */

.scroll-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.category-section {
  margin-bottom: 30px;
}

.category-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 900;
  color: var(--clay-ink);
  margin-bottom: 15px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--clay-border);
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 20px;
}

.app-card {
  position: relative;
  transition: all 0.3s;
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  overflow: visible;
}

.app-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-clay-float), var(--shadow-clay-inner);
  border-color: var(--clay-border);
}

.app-content {
  padding: 20px 20px 10px 20px;
  display: flex;
  align-items: center;
  gap: 15px;
}

.app-icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 18px;
  background: var(--icon-bg-app);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.app-icon-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 18px;
}

.mdi-icon {
  font-size: 28px;
  color: var(--el-color-primary);
}

.app-default-icon {
  font-size: 28px;
  color: var(--el-color-primary);
}

.app-info {
  flex: 1;
  overflow: hidden;
}

.app-title {
  margin: 0 0 5px 0;
  font-size: 15px;
  font-weight: 900;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--el-text-color-primary);
}

.app-tags {
  display: flex;
  gap: 5px;
}

.app-actions-overlay {
  position: absolute;
  top: 10px;
  right: 10px;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 5;
}

.app-card:hover .app-actions-overlay {
  opacity: 1;
}

.app-footer {
  padding: 10px 20px 15px 20px;
  display: flex;
  gap: 10px;
}

.link-btn {
  flex: 1;
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
