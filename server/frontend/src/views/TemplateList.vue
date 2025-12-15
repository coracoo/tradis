<template>
  <div class="template-list">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>模板列表</span>
          <el-button type="primary" :icon="Plus" @click="handleCreate">
            新建模板
          </el-button>
        </div>
      </template>

      <el-table :data="templates" style="width: 100%" stripe>
        <el-table-column label="Logo" width="80" align="center">
          <template #default="{ row }">
            <el-image 
              style="width: 40px; height: 40px; border-radius: 4px;"
              :src="row.logo" 
              fit="cover"
              :preview-src-list="[row.logo]"
              preview-teleported
            >
              <template #error>
                <div class="image-slot">
                  <el-icon><Picture /></el-icon>
                </div>
              </template>
            </el-image>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" width="150" sortable />
        <el-table-column prop="category" label="分类" width="100">
          <template #default="{ row }">
            <el-tag :type="getCategoryType(row.category)">{{ getCategoryLabel(row.category) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" type="primary" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
              <el-button size="small" type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="70%"
      destroy-on-close
      top="5vh"
    >
      <template-form
        ref="formRef"
        :template="currentTemplate"
        @submit="handleSubmit"
      />
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import TemplateForm from '../components/TemplateForm.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Picture } from '@element-plus/icons-vue'
import { templateApi } from '../api/template'

const templates = ref([])
const dialogVisible = ref(false)
const dialogTitle = ref('新建模板')
const currentTemplate = ref(null)
const formRef = ref(null)

const getCategoryLabel = (val) => {
  const map = {
    entertainment: '影音',
    photos: '图像',
    file: '文件',
    hobby: '个人',
    team: '协作',
    knowledge: '知识库',
    game: '游戏',
    productivity: '生产',
    social: '社交',
    platform: '管理',
    network: '网安',
    other: '其他'
  }
  return map[val] || val
}

const getCategoryType = (val) => {
  const map = {
    web: '',
    database: 'success',
    development: 'warning',
    other: 'info'
  }
  return map[val] || 'info'
}

// 获取模板列表
const fetchTemplates = async () => {
  try {
    const response = await templateApi.list()
    templates.value = response.data
  } catch (error) {
    ElMessage.error('获取模板列表失败')
  }
}

const handleCreate = () => {
  currentTemplate.value = null
  dialogTitle.value = '新建模板'
  dialogVisible.value = true
}

const handleEdit = (row) => {
  currentTemplate.value = { ...row }
  dialogTitle.value = '编辑模板'
  dialogVisible.value = true
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除模板 "${row.name}" 吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await templateApi.delete(row.id)
    ElMessage.success('删除成功')
    await fetchTemplates()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const handleSubmit = async (formData) => {
  try {
    if (currentTemplate.value) {
      await templateApi.update(currentTemplate.value.id, formData)
    } else {
      await templateApi.create(formData)
    }
    dialogVisible.value = false
    ElMessage.success('保存成功')
    await fetchTemplates()
  } catch (error) {
    ElMessage.error('保存失败')
  }
}

onMounted(() => {
  fetchTemplates()
})
</script>

<style scoped>
.template-list {
  height: 100%;
}

.box-card {
  height: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.image-slot {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  height: 100%;
  background: #f5f7fa;
  color: #909399;
  font-size: 20px;
}
</style>