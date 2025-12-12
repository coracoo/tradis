<template>
  <div class="app-deploy-view">
    <el-page-header @back="goBack" title="返回商店">
      <template #content>
        <span class="text-large font-600 mr-3"> 部署应用 - {{ project?.name }} </span>
      </template>
    </el-page-header>

    <div v-loading="loading" class="deploy-content">
      <div v-if="project" class="deploy-container">
        <!-- 应用基本信息卡片 -->
        <el-card class="info-card" shadow="never">
          <div class="app-header">
            <img :src="project.icon" class="app-icon" @error="handleImageError" />
            <div class="app-meta">
              <h2>{{ project.name }} <el-tag>{{ project.version }}</el-tag></h2>
              <p>{{ project.description }}</p>
            </div>
          </div>
        </el-card>

        <!-- 配置表单 -->
        <el-form
          ref="formRef"
          :model="formData"
          :rules="rules"
          label-position="left"
          label-width="200px"
          class="deploy-form"
        >
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
                  <div v-for="config in group.basic" :key="config.name" class="form-row-custom">
                    <el-row :gutter="10" align="middle">
                      <!-- 1. 类型 (自定义可编辑，否则只读) -->
                      <el-col :span="6">
                        <div class="param-type-wrapper" v-if="!config.isCustom">
                          <el-tag effect="plain" type="info">{{ getParamTypeLabel(config) }}</el-tag>
                        </div>
                        <div class="param-type-wrapper" v-else>
                           <el-select v-model="config.paramType" size="small" style="width: 100%">
                              <el-option label="端口" value="port" />
                              <el-option label="路径" value="path" />
                              <el-option label="变量" value="env" />
                              <el-option label="硬件" value="hardware" />
                              <el-option label="其它" value="other" />
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

                <!-- 高级配置 (嵌套折叠) -->
                <div v-if="group.advanced.length > 0" class="config-section advanced-section">
                   <el-divider content-position="left">
                    <span class="advanced-divider-text" @click="toggleAdvanced(serviceName)">
                      高级配置 
                      <el-icon class="advanced-icon" :class="{ 'is-active': activeAdvancedCollapse.includes(`advanced-${serviceName}`) }"><ArrowRight /></el-icon>
                    </span>
                   </el-divider>
                   
                  <el-collapse-transition>
                    <div v-show="activeAdvancedCollapse.includes(`advanced-${serviceName}`)">
                      <div v-for="config in group.advanced" :key="config.name" class="form-row-custom">
                          <el-row :gutter="10" align="middle">
                          <!-- 1. 类型 -->
                          <el-col :span="6">
                            <div class="param-type-wrapper" v-if="!config.isCustom">
                              <el-tag effect="plain" type="info">{{ getParamTypeLabel(config) }}</el-tag>
                            </div>
                            <div class="param-type-wrapper" v-else>
                               <el-select v-model="config.paramType" size="small" style="width: 100%">
                                  <el-option label="端口" value="port" />
                                  <el-option label="路径" value="path" />
                                  <el-option label="变量" value="env" />
                                  <el-option label="硬件" value="hardware" />
                                  <el-option label="其它" value="other" />
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
      </div>
      <el-empty v-else description="加载应用信息失败" />
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { QuestionFilled, ArrowRight, Plus, Remove } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const route = useRoute()
const router = useRouter()
const projectId = route.params.projectId

const loading = ref(false)
const deploying = ref(false)
const project = ref(null)
// 使用 deployConfig 数组来存储可编辑的配置，不再使用 formData 对象
const deployConfig = ref([])
const activeServiceNames = ref([])
const activeAdvancedCollapse = ref([])
const formRef = ref(null)

// 部署日志相关
const showLogs = ref(false)
const deployLogs = ref([])
const logsContent = ref(null)
const deploySuccess = ref(false)

// 表单验证规则
const formData = ref({})
const rules = reactive({})

const handleImageError = (e) => {
  e.target.src = 'https://cdn-icons-png.flaticon.com/512/873/873133.png'
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
  const name = `advanced-${serviceName}`
  const index = activeAdvancedCollapse.value.indexOf(name)
  if (index > -1) {
    activeAdvancedCollapse.value.splice(index, 1)
  } else {
    activeAdvancedCollapse.value.push(name)
  }
}

const handleAddCustomParam = (serviceName) => {
  // 直接向 deployConfig 添加
  const newParamName = `CUSTOM_${serviceName.toUpperCase()}_${Date.now()}`
  
  deployConfig.value.push({
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
  })
}

const handleRemoveParam = (config) => {
  const index = deployConfig.value.findIndex(item => item === config || item.name === config.name)
  if (index > -1) {
    deployConfig.value.splice(index, 1)
    ElMessage.success('已移除参数')
  }
}

const initForm = () => {
  if (!project.value || !project.value.schema) return

  // 深拷贝 schema 到 deployConfig，作为表单数据源
  try {
      deployConfig.value = JSON.parse(JSON.stringify(project.value.schema))
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

onMounted(() => {
  if (projectId) {
    fetchProject()
  }
})
</script>

<style scoped>
.app-deploy-view {
  padding: 20px;
}

.deploy-content {
  margin-top: 20px;
  max-width: 1000px;
  margin-left: auto;
  margin-right: auto;
}

.info-card {
  margin-bottom: 20px;
}

.app-header {
  display: flex;
  align-items: center;
  gap: 20px;
}

.app-icon {
  width: 80px;
  height: 80px;
  border-radius: 10px;
}

.app-meta h2 {
  margin: 0 0 10px 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.app-meta p {
  margin: 0;
  color: #666;
}

/* Service Collapse Styles */
.service-collapse-container {
  border: none;
  background: transparent;
}

.service-collapse-item {
  margin-bottom: 24px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fff;
  overflow: hidden;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
}

:deep(.el-collapse-item__header) {
  background-color: #fcfcfc;
  padding: 0 20px;
  font-weight: 600;
  font-size: 14px;
  height: 40px;
  line-height: 40px;
  border-bottom: 1px solid #ebeef5;
  color: #303133;
}

:deep(.el-collapse-item__content) {
  padding-bottom: 20px;
}

.service-title-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.service-header-left {
  display: flex;
  align-items: center;
  gap: 15px;
}

.service-header-right {
  display: flex;
  align-items: center;
}

.service-name-text {
  font-size: 14px;
  color: #303133;
}

.service-count-tag {
  margin-right: 15px;
  border: none;
  background: transparent;
  color: #909399;
}

.service-content {
  padding: 20px;
}

.config-section {
  margin-bottom: 10px;
}

.section-label {
  font-size: 14px;
  font-weight: bold;
  color: #606266;
  margin-bottom: 15px;
  border-left: 3px solid #409eff;
  padding-left: 8px;
}

/* Custom Form Row Styles */
.form-row-custom {
  margin-bottom: 0;
  padding: 12px 0;
  border-bottom: 1px solid #f0f2f5;
  transition: background-color 0.2s;
}

.form-row-custom:hover {
  background-color: #fafafa;
}

.form-row-custom:last-child {
  border-bottom: none;
}

.param-type-wrapper {
  display: flex;
  align-items: center;
}

.left-input-wrapper {
  display: flex;
  flex-direction: column;
}

.label-input :deep(.el-input__inner) {
  font-weight: 500;
  color: #606266;
  text-align: left;
}

.help-icon {
  cursor: pointer;
  color: #909399;
  font-size: 14px;
}

.help-icon:hover {
  color: #409eff;
}

.advanced-collapse-inner {
  margin-top: 10px;
  border: none;
  background: transparent;
}

.advanced-collapse-inner :deep(.el-collapse-item__header) {
  background: transparent;
  font-size: 14px;
  color: #606266;
  height: 40px;
  line-height: 40px;
  border-bottom: 1px dashed #dcdfe6;
}

.advanced-collapse-inner :deep(.el-collapse-item__wrap) {
  background: transparent;
  border: none;
}

.advanced-collapse-inner :deep(.el-collapse-item__content) {
  padding: 15px 0;
}

.advanced-section {
  margin-top: 15px;
}

.advanced-divider-text {
  cursor: pointer;
  display: flex;
  align-items: center;
  font-size: 14px;
  color: #909399;
  user-select: none;
}

.advanced-divider-text:hover {
  color: #409eff;
}

.advanced-icon {
  margin-left: 4px;
  transition: transform 0.3s;
}

.advanced-icon.is-active {
  transform: rotate(90deg);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 15px;
  margin-top: 30px;
  padding-bottom: 50px;
}

/* Deploy Logs Styles */
.deploy-logs-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.logs-header {
  padding: 10px 15px;
  background-color: #f5f7fa;
  border-bottom: 1px solid #dcdfe6;
  font-weight: bold;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.logs-content {
  height: 400px;
  overflow-y: auto;
  padding: 10px;
  background-color: #1e1e1e;
  color: #fff;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', 'source-code-pro', monospace;
  font-size: 12px;
}

.log-line {
  margin-bottom: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line.info { color: #a8c5f5; }
.log-line.success { color: #67c23a; }
.log-line.warning { color: #e6a23c; }
.log-line.error { color: #f56c6c; }

.status-tag {
  margin-left: 10px;
}
</style>
