<template>
  <div class="app-deploy-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-button @click="goBack" circle plain size="small">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <span class="page-title">部署应用 - {{ project?.name }}</span>
      </div>
      <div class="filter-right">
        <el-button @click="fetchProject" plain size="medium">
          <template #icon><el-icon><Refresh /></el-icon></template>
          刷新
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <div v-loading="loading" class="scroll-container">
        <div v-if="project" class="deploy-container">
          <!-- 应用基本信息 -->
          <div class="app-info-header">
            <div class="app-icon-wrapper">
              <img :src="resolvePicUrl(project.logo || project.icon)" class="app-icon" @error="handleImageError" />
            </div>
            <div class="app-meta">
              <div class="app-title-row">
                <h2 class="app-name">{{ project.name }}</h2>
                <el-tag effect="dark" size="small" class="version-tag">{{ project.version }}</el-tag>
              </div>
              <p class="app-desc">{{ project.description }}</p>
            </div>
          </div>

          <!-- 配置表单 -->
          <el-tabs v-model="activeTab" class="deploy-tabs">
        <el-tab-pane label="部署" name="deploy">
        <el-form
          ref="formRef"
          :model="formData"
          :rules="rules"
          label-position="left"
          label-width="200px"
          class="deploy-form"
        >
          <div class="auto-allocate-bar">
            <el-alert
              type="info"
              :closable="false"
              class="allocate-alert"
            >
              <template #title>
                <div class="allocate-header">
                  <span class="allocate-title">端口自动分配</span>
                  <div class="allocate-actions">
                     <el-button 
                       type="primary" 
                       size="small" 
                       @click="handleAutoAllocate" 
                       :loading="allocating"
                       icon="MagicStick"
                     >
                       {{ allocating ? '正在分配...' : '一键分配端口' }}
                     </el-button>
                  </div>
                </div>
              </template>
              <template #default>
                <span class="allocate-desc">系统将从锁定的端口范围中，自动查找并填充所需的连续端口段。</span>
              </template>
            </el-alert>
          </div>
          <div v-if="Object.keys(groupedSchema).length === 0">
            <el-empty description="暂无配置参数" />
          </div>

          <!-- 按服务分组展示 -->
          <el-collapse v-model="activeServiceNames" class="service-collapse-container">
            <el-collapse-item 
              v-for="(group, serviceName) in groupedSchema" 
              :key="serviceName" 
              :name="serviceName"
              class="service-collapse-item"
            >
              <template #title>
                <div class="service-title-header">
                  <div class="service-header-left">
                    <span class="service-name-text">{{ serviceName === 'Global' ? '全局配置' : serviceName }}</span>
                    <el-button size="small" link type="primary" class="add-param-btn" @click.stop="handleAddCustomParam(serviceName)">
                      <el-icon><Plus /></el-icon> 添加参数
                    </el-button>
                  </div>
                  <div class="service-header-right">
                    <el-tag size="small" effect="plain" type="info" class="service-count-tag">{{ group.basic.length + group.advanced.length }} 项配置</el-tag>
                  </div>
                </div>
              </template>

              <div class="service-content">
                <!-- 基础配置 -->
                <div v-if="group.basic.length > 0" class="config-section">
                  <div class="section-label" v-if="group.advanced.length > 0">基础配置</div>
                  <!-- Use index as key if config.name is not stable or unique enough during edits, 
                       but ideally config.name should be unique. 
                       However, if config.name is editable, using it as :key will cause re-render on input.
                       Use config itself or a stable ID if available. 
                       Since we don't have stable IDs, let's use index within the group for now or try to not update key.
                  -->
                  <div v-for="(config, idx) in group.basic" :key="idx" class="form-row-custom">
                    <el-row :gutter="10" align="middle">
                      <!-- 1. 类型 (自定义可编辑，否则只读) -->
                      <el-col :span="6">
                        <div class="param-type-wrapper" v-if="!config.isCustom">
                          <el-tag effect="plain" type="info">{{ getParamTypeLabel(config) }}</el-tag>
                        </div>
                        <div class="param-type-wrapper" v-else>
                           <el-select v-model="config.paramType" size="small" style="width: 100%">
                                <el-option label="端口（port）" value="port" />
                                <el-option label="路径（volume）" value="path" />
                                <el-option label="环境变量（environment）" value="env" />
                                <el-option label="硬件（device）" value="hardware" />
                                <el-option label="其它（other）" value="other" />
                           </el-select>
                        </div>
                      </el-col>

                      <!-- 2. 参数定义 (可编辑) -->
                      <el-col :span="7">
                        <div class="left-input-wrapper">
                          <!-- 自定义参数：编辑 Key -->
                          <el-input 
                            v-if="config.isCustom" 
                            v-model="config.customKey" 
                            placeholder="参数名" 
                            class="label-input"
                          />
                          <!-- 预定义参数：编辑 Name (左侧值) -->
                          <el-input 
                            v-else 
                            v-model="config.name" 
                            :placeholder="config.name" 
                            class="label-input"
                          >
                            <template #suffix>
                               <el-tooltip v-if="config.description" :content="config.description" placement="top">
                                <el-icon class="help-icon"><QuestionFilled /></el-icon>
                              </el-tooltip>
                            </template>
                          </el-input>
                        </div>
                      </el-col>
                      
                      <!-- 3. 参数值 (可编辑 - 对应 Default 字段) -->
                      <el-col :span="10">
                        <el-form-item :prop="config.name" label-width="0" style="margin-bottom: 0">
                          <el-input 
                            v-if="['text', 'string', 'path', 'port'].includes(config.type)" 
                            v-model="config.default" 
                            :placeholder="String(config.default || '')"
                          />
                          
                          <el-input-number 
                            v-else-if="config.type === 'number'" 
                            v-model="config.default"
                            style="width: 100%" 
                          />
                          
                          <el-input 
                            v-else-if="config.type === 'password'" 
                            v-model="config.default" 
                            type="password"
                            show-password
                          />
                          
                          <el-select 
                            v-else-if="config.type === 'select'" 
                            v-model="config.default"
                            style="width: 100%"
                          >
                            <el-option 
                              v-for="opt in config.options" 
                              :key="opt" 
                              :label="opt" 
                              :value="opt" 
                            />
                          </el-select>
                        </el-form-item>
                      </el-col>

                      <!-- 4. 删除按钮 -->
                      <el-col :span="1" style="text-align: center;">
                        <el-button link type="danger" @click="handleRemoveParam(config)">
                          <el-icon><Remove /></el-icon>
                        </el-button>
                      </el-col>
                    </el-row>
                  </div>
                </div>

                <!-- 高级配置 -->
                <div v-if="group.advanced.length > 0" class="config-section">
                  <div class="advanced-header" @click="toggleAdvanced(serviceName)" style="cursor: pointer; padding: 10px 0; display: flex; align-items: center; color: var(--el-text-color-secondary);">
                    <el-icon :class="{ 'is-active': activeAdvancedCollapse.includes(`advanced-${serviceName}`) }" style="margin-right: 5px; transition: transform 0.3s;">
                      <ArrowRight />
                    </el-icon>
                    <span>高级配置 ({{ group.advanced.length }})</span>
                  </div>
                  <el-collapse-transition>
                    <div v-show="activeAdvancedCollapse.includes(`advanced-${serviceName}`)">
                      <div v-for="(config, idx) in group.advanced" :key="idx" class="form-row-custom">
                        <el-row :gutter="10" align="middle">
                          <!-- 1. 类型 -->
                          <el-col :span="6">
                            <div class="param-type-wrapper" v-if="!config.isCustom">
                              <el-tag effect="plain" type="info">{{ getParamTypeLabel(config) }}</el-tag>
                            </div>
                            <div class="param-type-wrapper" v-else>
                               <el-select v-model="config.paramType" size="small" style="width: 100%">
                                  <el-option label="端口（port）" value="port" />
                                  <el-option label="路径（volume）" value="path" />
                                  <el-option label="环境变量（environment）" value="env" />
                                  <el-option label="硬件（device）" value="hardware" />
                                  <el-option label="其它（other）" value="other" />
                               </el-select>
                            </div>
                          </el-col>

                          <!-- 2. 参数定义 -->
                          <el-col :span="7">
                            <div class="left-input-wrapper">
                              <el-input 
                                v-if="config.isCustom" 
                                v-model="config.customKey" 
                                placeholder="参数名" 
                                class="label-input"
                              />
                              <el-input 
                                v-else 
                                v-model="config.name" 
                                :placeholder="config.name" 
                                class="label-input"
                              >
                                <template #suffix>
                                  <el-tooltip v-if="config.description" :content="config.description" placement="top">
                                    <el-icon class="help-icon"><QuestionFilled /></el-icon>
                                  </el-tooltip>
                                </template>
                              </el-input>
                            </div>
                          </el-col>
                          
                          <!-- 3. 参数值 -->
                          <el-col :span="10">
                            <el-form-item :prop="config.name" label-width="0" style="margin-bottom: 0">
                              <el-input 
                                v-if="['text', 'string', 'path', 'port'].includes(config.type)" 
                                v-model="config.default" 
                                :placeholder="config.default?.toString()"
                              />
                              
                              <el-input-number 
                                v-else-if="config.type === 'number'" 
                                v-model="config.default"
                                style="width: 100%" 
                              />
                              
                              <el-input 
                                v-else-if="config.type === 'password'" 
                                v-model="config.default" 
                                type="password"
                                show-password
                              />
                              
                              <el-select 
                                v-else-if="config.type === 'select'" 
                                v-model="config.default"
                                style="width: 100%"
                              >
                                <el-option 
                                  v-for="opt in config.options" 
                                  :key="opt" 
                                  :label="opt" 
                                  :value="opt" 
                                  
                                />
                                </el-select>
                            </el-form-item>
                          </el-col>

                          <!-- 4. 删除按钮 -->
                          <el-col :span="1" style="text-align: center;">
                            <el-button link type="danger" @click="handleRemoveParam(config)">
                              <el-icon><Remove /></el-icon>
                            </el-button>
                          </el-col>
                        </el-row>
                      </div>
                    </div>
                  </el-collapse-transition>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>

          <!-- 操作按钮 -->
          <div class="form-actions">
            <el-button @click="goBack">取消</el-button>
            <el-button type="primary" :loading="deploying" @click="submitDeploy">
              确认部署
            </el-button>
          </div>
        </el-form>
        </el-tab-pane>
        <el-tab-pane label="使用教程" name="tutorial">
          <div class="tutorial-wrapper">
            <div v-if="project?.tutorial" class="tutorial-content" v-html="tutorialHtml" @click="handleTutorialClick"></div>
            <div v-else class="tutorial-content empty-tutorial">暂无使用教程</div>
          </div>
        </el-tab-pane>
        </el-tabs>
      </div>
      <el-empty v-else description="加载应用信息失败" />
    </div>
  </div>

    <!-- 部署日志对话框 -->
    <el-dialog
      v-model="showLogs"
      title="正在部署应用"
      width="800px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="!deploying"
      append-to-body
    >
      <div class="deploy-logs-container">
        <div class="logs-header">
          <span>部署进度</span>
          <el-tag v-if="deploying" type="primary" effect="dark" class="status-tag">部署中...</el-tag>
          <el-tag v-else :type="deploySuccess ? 'success' : 'danger'" effect="dark" class="status-tag">
            {{ deploySuccess ? '部署成功' : '部署失败' }}
          </el-tag>
        </div>
        <div ref="logsContent" class="logs-content">
          <div v-for="(log, index) in deployLogs" :key="index" :class="['log-line', log.type]">
            {{ log.message }}
          </div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-if="!deploying && deploySuccess" type="primary" @click="goToContainers">
            查看容器
          </el-button>
          <el-button v-if="!deploying && !deploySuccess" type="warning" @click="submitDeploy">
            重试部署
          </el-button>
          <el-button v-if="!deploying" @click="showLogs = false">关闭</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 图片预览组件 -->
    <el-image-viewer
      v-if="showImageViewer"
      :url-list="previewImageList"
      hide-on-click-modal
      @close="closeImageViewer"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive, nextTick, shallowRef, triggerRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { QuestionFilled, ArrowRight, ArrowLeft, Plus, Remove, Refresh, MagicStick } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, ElImageViewer } from 'element-plus'
import api from '../api'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const projectId = route.params.projectId

const loading = ref(false)
const deploying = ref(false)
const project = ref(null)
// 使用 deployConfig 数组来存储可编辑的配置，不再使用 formData 对象
const deployConfig = shallowRef([])
const activeServiceNames = ref([])
const activeAdvancedCollapse = ref([])
const formRef = ref(null)

// 部署日志相关
const showLogs = ref(false)
const deployLogs = ref([])
const logsContent = ref(null)
const deploySuccess = ref(false)
const activeTab = ref('deploy')
const allocating = ref(false)
const appStoreBase = ref('')

// 图片预览相关
const showImageViewer = ref(false)
const previewImageList = ref([])
const closeImageViewer = () => {
  showImageViewer.value = false
}
const handleTutorialClick = (e) => {
  if (e.target.tagName === 'IMG') {
    previewImageList.value = [e.target.src]
    showImageViewer.value = true
  }
}

const escapeHtml = (str) => {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

const renderMarkdown = (md) => {
  let html = md
  html = html.replace(/```([\s\S]*?)```/g, (_, code) => `<pre><code>${escapeHtml(code)}</code></pre>`)
  html = html.replace(/^###### (.*)$/gm, '<h6>$1</h6>')
  html = html.replace(/^##### (.*)$/gm, '<h5>$1</h5>')
  html = html.replace(/^#### (.*)$/gm, '<h4>$1</h4>')
  html = html.replace(/^### (.*)$/gm, '<h3>$1</h3>')
  html = html.replace(/^## (.*)$/gm, '<h2>$1</h2>')
  html = html.replace(/^# (.*)$/gm, '<h1>$1</h1>')
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  html = html.replace(/^\s*[-*]\s+(.*)$/gm, '<ul><li>$1</li></ul>')
  html = html.replace(/\n<ul>\n/g, '<ul>')
  html = html.replace(/\n<\/li><\/ul>/g, '</li></ul>')
  html = html.replace(/!\[([^\]]*)\]\(([^\)]+)\)/g, '<img alt="$1" src="$2" />')
  html = html.replace(/\[([^\]]+)\]\(([^\)]+)\)/g, '<a href="$2" target="_blank">$1</a>')
  html = html.replace(/\n{2,}/g, '<br/>')
  return html
}
const tutorialHtml = computed(() => renderMarkdown(project.value?.tutorial || ''))

// 表单验证规则
const formData = ref({})
const rules = reactive({})

const handleImageError = (e) => {
  e.target.src = 'https://cdn-icons-png.flaticon.com/512/873/873133.png'
}

const initAppStoreBase = async () => {
  try {
    const s = await request.get('/settings/global')
    appStoreBase.value = (s && s.appStoreServerUrl) ? s.appStoreServerUrl.replace(/\/$/, '') : 'https://template.cgakki.top:33333'
  } catch (e) {
    appStoreBase.value = 'https://template.cgakki.top:33333'
  }
}

const resolvePicUrl = (u) => {
  if (!u) return ''
  if (u.startsWith('http://') || u.startsWith('https://')) return u
  if (u.startsWith('/')) return appStoreBase.value + u
  return appStoreBase.value + '/' + u
}

const groupedSchema = computed(() => {
  const groups = {}
  // 使用可编辑的配置源
  const schema = deployConfig.value || []

  schema.forEach(config => {
    // 兼容旧数据：如果没有 serviceName，尝试从变量名推断，或者归为 Global
    let service = config.serviceName
    if (!service) {
        // 简单推断：PORT_WEB_80 -> WEB
        const parts = config.name.split('_')
        if (parts.length >= 3 && (parts[0] === 'PORT' || parts[0] === 'VOL')) {
            // 这里其实很难准确推断，暂时归为 Global 更安全，或者不做推断
        }
        service = 'Global'
    }

    if (!groups[service]) {
      groups[service] = {
        basic: [],
        advanced: []
      }
    }

    if (config.category === 'advanced') {
      groups[service].advanced.push(config)
    } else {
      groups[service].basic.push(config)
    }
  })

  return groups
})

const getParamTypeLabel = (config) => {
  if (config.paramType) {
    const map = {
      port: '端口（port）',
      path: '路径（volume）',
      env: '环境变量（environment）',
      hardware: '硬件（device）',
      other: '其它（other）'
    }
    return map[config.paramType] || '其它'
  }
  
  // 兼容旧数据 fallback
  if (config.type === 'port') return '端口（port）'
  if (config.type === 'path') return '路径（volume）'
  return '环境变量（environment）'
}

const toggleAdvanced = (serviceName) => {
  const key = `advanced-${serviceName}`
  const index = activeAdvancedCollapse.value.indexOf(key)
  if (index > -1) {
    activeAdvancedCollapse.value.splice(index, 1)
  } else {
    activeAdvancedCollapse.value.push(key)
  }
}

const handleAddCustomParam = (serviceName) => {
  // 直接向 deployConfig 添加
  const newParamName = `CUSTOM_${serviceName.toUpperCase()}_${Date.now()}`
  
  deployConfig.value.push(reactive({
    name: newParamName,
    label: '自定义参数',
    customKey: '', // 用户输入的参数名
    default: '',   // 用户输入的参数值
    description: '用户自定义参数',
    type: 'string',
    paramType: 'env', // 默认环境变量
    category: 'basic', // 默认基础配置
    serviceName: serviceName,
    isCustom: true // 标记为自定义
  }))
  triggerRef(deployConfig)
}

const handleRemoveParam = (config) => {
  const index = deployConfig.value.findIndex(item => item === config || item.name === config.name)
  if (index > -1) {
    deployConfig.value.splice(index, 1)
    triggerRef(deployConfig)
    ElMessage.success('已移除参数')
  }
}

const initForm = () => {
  if (!project.value || !project.value.schema) return

  // 深拷贝 schema 到 deployConfig，作为表单数据源
  try {
      const raw = JSON.parse(JSON.stringify(project.value.schema))
      // 将每个配置项转为 reactive，确保属性变化能被 UI (如 label) 响应
      deployConfig.value = raw.map(item => {
        // 确保 serviceName 存在，避免 groupedSchema 依赖 name 导致编辑时重渲染
        if (!item.serviceName) {
           item.serviceName = 'Global'
        }
        return reactive(item)
      })
  } catch (e) {
      deployConfig.value = []
  }

  // 初始化验证规则
  deployConfig.value.forEach(config => {
    // 确保 label 存在，方便界面编辑
    if (!config.label) {
      config.label = config.name
    }

    // 生成验证规则
    if (config.description && config.description.includes('required')) {
      rules[config.name] = [
        { required: true, message: `请输入${config.name}`, trigger: 'blur' }
      ]
    }
  })
  
  // 默认展开所有服务
  nextTick(() => {
    activeServiceNames.value = Object.keys(groupedSchema.value)
  })
}

const fetchProject = async () => {
  loading.value = true
  try {
    const res = await api.appstore.getProjectDetail(projectId)
    // 兼容处理：如果响应本身就是对象数据（无 data 属性），或者包含 data 属性
    const data = res.data || res
    if (data) {
      project.value = data
      initForm()
    } else {
      ElMessage.error('未找到该应用')
    }
  } catch (error) {
    console.error(error)
    ElMessage.error('获取应用详情失败')
  } finally {
    loading.value = false
  }
}

const goBack = () => {
  router.back()
}

const goToContainers = () => {
  router.push('/containers')
}

// 简单对象转 YAML 字符串
const toYaml = (obj, indent = 0) => {
  const spaces = ' '.repeat(indent)
  let yaml = ''
  
  for (const key in obj) {
    const value = obj[key]
    if (Array.isArray(value)) {
      yaml += `${spaces}${key}:\n`
      value.forEach(item => {
        yaml += `${spaces}  - "${item}"\n` // 强制加引号避免解析错误
      })
    } else if (typeof value === 'object' && value !== null) {
      yaml += `${spaces}${key}:\n`
      yaml += toYaml(value, indent + 2)
    } else {
      const strValue = String(value)
      // 如果是 version 字段，或者包含特殊字符，或者是纯数字字符串，加上引号
      if (key === 'version' || strValue.includes(':') || strValue.includes('#') || strValue.includes('=') || /^\d+(\.\d+)?$/.test(strValue)) {
        yaml += `${spaces}${key}: "${strValue}"\n`
      } else {
        yaml += `${spaces}${key}: ${value}\n`
      }
    }
  }
  return yaml
}

const submitDeploy = async () => {
  // 手动验证必填项
  for (const config of deployConfig.value) {
    if (config.description && config.description.includes('required') && !config.default) {
      ElMessage.warning(`请填写必填项: ${config.label || config.name}`)
      return
    }
  }

  try {
    // 0. 构建最终的环境变量/参数映射 (为了兼容性保留 Env Map，虽然主要靠 Config 数组)
    const finalEnv = {}
    // 处理 Config 数组中的自定义 Key
    deployConfig.value.forEach(config => {
        if (config.isCustom && config.customKey) {
            config.name = config.customKey // 提交前更新 name
        }
        
        // 只将环境变量类型的配置加入 finalEnv，避免将端口映射或卷映射路径写入 .env 文件导致报错
        // 检查 paramType (新标准) 或 type (兼容旧数据)
        const isEnv = (config.paramType === 'env' || config.paramType === 'environment') || 
                      (!config.paramType && config.type !== 'port' && config.type !== 'path' && config.type !== 'volume')
        
        if (isEnv) {
            finalEnv[config.name] = config.default || ''
        }
    })

    // 1. 准备 YAML 模板
    let yamlContent = ''
    if (project.value.compose) {
      yamlContent = project.value.compose
    } else if (project.value.services) {
      const services = project.value.services
      const composeObj = {
        version: '3',
        services: services
      }
      yamlContent = toYaml(composeObj)
    } else {
       console.warn('No compose or services found in project definition')
    }

    const projectName = project.value.name

    // 检查项目是否已存在
    try {
      const installedRes = await api.compose.list()
      const installedList = installedRes.data || installedRes
      if (installedList.some(p => p.name === projectName)) {
         await ElMessageBox.confirm(
          `项目 "${projectName}" 已存在，继续部署将覆盖原有项目。是否继续？`,
          '项目已存在',
          {
            confirmButtonText: '覆盖部署',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      }
    } catch (error) {
       if (error === 'cancel') return
       console.warn('Check installed projects failed', error)
    }

    // 2. 初始化部署状态
    showLogs.value = true
    deploying.value = true
    deploySuccess.value = false
    deployLogs.value = []

    // 3. 调用部署接口
    const deployData = {
      projectId: projectName, 
      compose: yamlContent,
      env: finalEnv, // 兼容旧逻辑
      config: deployConfig.value // 新逻辑：传递完整配置数组
    }

    try {
      const res = await api.appstore.deployProject(deployData)
      // 兼容返回结构
      const responseData = res.data || res
      const taskId = responseData.taskId
      
      if (!taskId) {
        throw new Error('未获取到任务ID')
      }

      ElMessage.success('部署任务已提交，正在执行...')
      
      // 4. 使用 SSE 监听任务进度
      const token = localStorage.getItem('token')
      const eventSource = new EventSource(
        `/api/appstore/tasks/${taskId}/events?token=${token}`
      )
      
      // 用于去重
      const logSet = new Set()

      // 添加超时处理 (防止任务卡死或SSE断连)
      const timeout = setTimeout(async () => {
        deployLogs.value.push({
          type: 'warning',
          message: '日志连接超时，请稍后在容器列表查看状态'
        })
        eventSource.close()
        deploying.value = false
      }, 600000) // 10分钟超时

      eventSource.onmessage = async (event) => {
        try {
          const data = JSON.parse(event.data)
          
          if (data.type === 'result') {
            // 任务结束
            clearTimeout(timeout)
            eventSource.close()
            deploying.value = false
            
            if (data.status === 'success') {
              deploySuccess.value = true
              deployLogs.value.push({ type: 'success', message: '部署任务完成！' })
              ElMessage.success('部署成功')
            } else {
              deployLogs.value.push({ type: 'error', message: `部署失败: ${data.message || '未知错误'}` })
              ElMessage.error('部署失败')
            }
          } else {
            // 普通日志
            const logKey = `${data.type}:${data.message}`
            if (!logSet.has(logKey)) {
               logSet.add(logKey)
               deployLogs.value.push({
                type: data.type,
                message: `[${new Date(data.time).toLocaleTimeString()}] ${data.message}`
              })
            }
          }

          // 自动滚动到底部
          nextTick(() => {
            if (logsContent.value) {
              logsContent.value.scrollTop = logsContent.value.scrollHeight
            }
          })
        } catch (error) {
          console.error('解析消息失败', error)
        }
      }

      eventSource.onerror = (event) => {
        if (deploying.value && eventSource.readyState === EventSource.CLOSED) {
           clearTimeout(timeout)
           deployLogs.value.push({
             type: 'error',
             message: '日志连接中断'
           })
           deploying.value = false
           eventSource.close()
        }
      }

    } catch (error) {
      console.error(error)
      deploying.value = false
      ElMessage.error('部署失败: ' + (error.response?.data?.error || error.message))
    }

  } catch (error) {
    console.error(error)
    ElMessage.error('准备部署失败: ' + error.message)
    deploying.value = false
  }
}

const getPortConfigs = () => {
  const list = [];
  (deployConfig.value || []).forEach(cfg => {
    const isPort = (cfg.paramType === 'port') || (cfg.type === 'port')
    if (isPort) list.push(cfg)
  })
  return list
}

const handleAutoAllocate = async () => {
  const ports = getPortConfigs()
  if (!ports.length) {
    ElMessage.info('当前无端口参数需要分配')
    return
  }
  allocating.value = true
  try {
    const res = await api.ports.allocate({ count: ports.length, protocol: 'tcp', type: 'host', useAllocRange: true, dryRun: true })
    if (res && res.segments && res.segments.length > 0) {
      const seg = res.segments[0]
      if (seg.length !== ports.length) {
        ElMessage.error('分配端口数量不足')
        return
      }
      for (let i = 0; i < ports.length; i++) {
        ports[i].name = String(seg[i])
      }
      triggerRef(deployConfig) // 触发更新
      ElMessage.success('已自动分配端口')
    } else {
       ElMessage.error('分配失败: 未获取到端口段')
    }
  } catch (error) {
    ElMessage.error('自动分配失败: ' + (error.response?.data?.error || error.message))
  } finally {
    allocating.value = false
  }
}


onMounted(() => {
  initAppStoreBase().then(() => {
    if (projectId) {
      fetchProject()
    }
  })
})
</script>

<style scoped>
.app-deploy-view {
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

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.content-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05), 0 4px 6px -2px rgba(0, 0, 0, 0.025);
  display: flex;
  flex-direction: column;
}

.scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.app-info-header {
  display: flex;
  gap: 24px;
  padding: 24px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

/* App Icon */
.app-icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
  border: 1px solid var(--el-border-color-lighter);
}

.app-icon {
  width: 80%;
  height: 80%;
  object-fit: contain;
}

.app-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.app-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.app-name {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.app-desc {
  margin: 0;
  font-size: 14px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

/* Deploy Form Styles */
.deploy-form {
  margin-top: 20px;
}

.auto-allocate-bar {
  margin-bottom: 24px;
}

.allocate-alert {
  border-radius: 8px;
}

.allocate-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.allocate-title {
  font-weight: 600;
  font-size: 14px;
}

.service-collapse-container {
  border: none;
}

.service-collapse-item {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

:deep(.el-collapse-item__header) {
  background-color: var(--el-fill-color-light);
  padding: 0 16px;
  height: 48px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-weight: 600;
  color: var(--el-text-color-primary);
}

:deep(.el-collapse-item__content) {
  padding: 0;
}

.service-title-header {
  flex: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-right: 12px;
}

.service-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.add-param-btn {
  font-weight: normal;
}

.service-content {
  padding: 20px;
  background: var(--el-bg-color);
}

.form-row-custom {
  margin-bottom: 0;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.form-row-custom:last-child {
  border-bottom: none;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 32px;
  padding-top: 20px;
  border-top: 1px solid #e2e8f0;
}

/* Retain other specific styles */
.section-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 12px;
  display: flex;
  align-items: center;
}

.section-label::before {
  content: '';
  width: 3px;
  height: 14px;
  background: #3b82f6;
  margin-right: 8px;
  border-radius: 2px;
}

/* Deploy Logs Styles */
.deploy-logs-container {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.logs-header {
  padding: 12px 16px;
  background-color: var(--el-fill-color-lighter);
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-weight: 600;
  color: var(--el-text-color-regular);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.logs-content {
  height: 400px;
  overflow-y: auto;
  padding: 16px;
  background-color: var(--el-fill-color-darker);
  color: var(--el-text-color-primary);
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  line-height: 1.5;
}

.log-line {
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line.info { color: var(--el-color-info); }
.log-line.success { color: var(--el-color-success); }
.log-line.warning { color: var(--el-color-warning); }
.log-line.error { color: var(--el-color-danger); }

.tutorial-content {
  line-height: 1.7;
  font-size: 14px;
  color: #334155;
  word-break: break-word;
  overflow-wrap: break-word;
  white-space: pre-wrap;
}

:deep(.tutorial-content img) {
  max-width: 70%;
  height: auto;
  border-radius: 8px;
  margin: 12px 0;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  cursor: zoom-in;
  transition: transform 0.2s;
}

:deep(.tutorial-content img:hover) {
  opacity: 0.9;
}

:deep(.tutorial-content pre) {
  background: var(--el-fill-color-darker);
  color: var(--el-text-color-primary);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 16px 0;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
}

:deep(.tutorial-content code) {
  background: var(--el-fill-color-lighter);
  color: var(--el-text-color-regular);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
}

:deep(.tutorial-content pre code) {
  background: transparent;
  color: inherit;
  padding: 0;
  border-radius: 0;
}

:deep(.tutorial-content h1),
:deep(.tutorial-content h2),
:deep(.tutorial-content h3) {
  margin-top: 24px;
  margin-bottom: 12px;
  font-weight: 600;
  color: #1e293b;
}

:deep(.tutorial-content a) {
  color: #3b82f6;
  text-decoration: none;
}

:deep(.tutorial-content a:hover) {
  text-decoration: underline;
}

:deep(.tutorial-content ul) {
  padding-left: 20px;
  margin: 8px 0;
}

:deep(.tutorial-content li) {
  margin-bottom: 4px;
}

/* Overrides */
:deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background-color: #e2e8f0;
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

.tutorial-wrapper {
  padding: 24px;
  background: var(--el-fill-color-light);
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
}

.empty-tutorial {
  text-align: center;
  color: var(--el-text-color-secondary);
  padding: 40px;
}
</style>
