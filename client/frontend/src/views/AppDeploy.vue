<template>
  <div class="app-deploy-view">
    <div class="filter-bar">
      <div class="filter-left">
        <el-button @click="goBack" circle plain size="small">
          <IconEpArrowLeft />
        </el-button>
        <span class="page-title">部署应用 - {{ project?.name }}</span>
      </div>
      <div class="filter-right">
        <el-button @click="fetchProject" plain size="default">
          <template #icon><IconEpRefresh /></template>
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
        <el-tab-pane label="新手部署" name="deploy">
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
                     >
                       <template #icon><IconEpMagicStick /></template>
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

          <div class="deploy-grid">
            <div class="deploy-left">
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
                      <IconEpPlus class="el-icon--left" /> 添加参数
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
                  <div v-for="(config, idx) in group.basic" :key="idx" class="form-row-custom">
                    <el-row :gutter="8" align="middle">
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
                                  <IconEpQuestionFilled class="help-icon" />
                              </el-tooltip>
                            </template>
                          </el-input>
                        </div>
                      </el-col>
                      
                      <!-- 3. 参数值 (可编辑 - 对应 Default 字段) -->
                      <el-col :span="10">
                        <el-form-item :prop="config.formKey" label-width="0" style="margin-bottom: 0">
                          <el-input 
                            v-if="['text', 'string', 'path', 'port'].includes(config.type)" 
                            v-model="formData[config.formKey]" 
                            :placeholder="String(formData[config.formKey] ?? '')"
                          />
                          
                          <el-input-number 
                            v-else-if="config.type === 'number'" 
                            v-model="formData[config.formKey]"
                            style="width: 100%" 
                          />
                          
                          <el-input 
                            v-else-if="config.type === 'password'" 
                            v-model="formData[config.formKey]" 
                            type="password"
                            show-password
                          />
                          
                          <el-select 
                            v-else-if="config.type === 'select'" 
                            v-model="formData[config.formKey]"
                            style="width: 100%"
                          >
                            <el-option 
                              v-for="opt in config.options" 
                              :key="opt" 
                              :label="opt" 
                              :value="opt" 
                            />
                          </el-select>
                          <div v-if="shouldShowDotenvInheritanceHint(config)" class="inherit-hint">将从 .env 注入</div>
                        </el-form-item>
                      </el-col>

                      <!-- 4. 删除按钮 -->
                      <el-col :span="1" style="text-align: center;">
                        <el-button link type="danger" @click="handleRemoveParam(config)">
                          <IconEpMinus />
                        </el-button>
                      </el-col>
                    </el-row>
                  </div>
                </div>

                <!-- 高级配置 -->
                <div v-if="group.advanced.length > 0" class="config-section">
                  <div class="advanced-header" @click="toggleAdvanced(serviceName)" style="cursor: pointer; padding: 10px 0; display: flex; align-items: center; color: var(--el-text-color-secondary);">
                    <IconEpArrowRight :class="{ 'is-active': activeAdvancedCollapse.includes(`advanced-${serviceName}`) }" style="margin-right: 5px; transition: transform 0.3s;" />
                    <span>高级配置 ({{ group.advanced.length }})</span>
                  </div>
                  <el-collapse-transition>
                    <div v-show="activeAdvancedCollapse.includes(`advanced-${serviceName}`)">
                      <div v-for="(config, idx) in group.advanced" :key="idx" class="form-row-custom">
                        <el-row :gutter="8" align="middle">
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
                                    <IconEpQuestionFilled class="help-icon" />
                                  </el-tooltip>
                                </template>
                              </el-input>
                            </div>
                          </el-col>
                          
                          <!-- 3. 参数值 -->
                          <el-col :span="10">
                            <el-form-item :prop="config.formKey" label-width="0" style="margin-bottom: 0">
                              <el-input 
                                v-if="['text', 'string', 'path', 'port'].includes(config.type)" 
                                v-model="formData[config.formKey]" 
                                :placeholder="String(formData[config.formKey] ?? '')"
                              />
                              
                              <el-input-number 
                                v-else-if="config.type === 'number'" 
                                v-model="formData[config.formKey]"
                                style="width: 100%" 
                              />
                              
                              <el-input 
                                v-else-if="config.type === 'password'" 
                                v-model="formData[config.formKey]" 
                                type="password"
                                show-password
                              />
                              
                              <el-select 
                                v-else-if="config.type === 'select'" 
                                v-model="formData[config.formKey]"
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
                              <IconEpMinus />
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
            </div>
            <div class="deploy-right">
              <el-collapse v-model="activeServiceNames" class="service-collapse-container">
                <el-collapse-item
                  name="__dotenv__"
                  class="service-collapse-item global-env-collapse-item"
                >
                  <template #title>
                    <div class="service-title-header">
                      <div class="service-header-left">
                        <span class="service-name-text">环境变量（.env）</span>
                        <el-button size="small" link type="primary" class="add-param-btn" @click.stop="handleAddGlobalDotenvKey">
                          <IconEpPlus class="el-icon--left" /> 添加参数
                        </el-button>
                      </div>
                      <div class="service-header-right">
                        <el-tag size="small" effect="plain" type="success" class="service-count-tag">已定义 {{ dotenvRows.length }}</el-tag>
                      </div>
                    </div>
                  </template>

                  <div class="service-content">
                    <div v-if="dotenvRows.length === 0">
                      <el-empty description="暂无环境变量" />
                    </div>
                    <div v-else class="envfile-groups">
                      <div v-for="(grp, gidx) in dotenvGroupedRows" :key="grp.path || gidx" class="envfile-group">
                        <div class="service-title-header envfile-group-header">
                          <div class="service-header-left">
                            <span class="service-name-text">{{ grp.name }}</span>
                            <el-tag size="small" effect="plain" type="info" class="service-count-tag">{{ grp.path }}</el-tag>
                          </div>
                          <div class="service-header-right">
                            <el-tag size="small" effect="plain" type="success" class="service-count-tag">{{ grp.rows.length }} 项</el-tag>
                          </div>
                        </div>

                        <div class="config-section global-env-section">
                          <div v-for="(row, idx) in grp.rows" :key="row.key || idx" class="form-row-custom">
                            <el-row :gutter="8" align="middle">
                              <el-col :span="6">
                                <div class="param-type-wrapper">
                                  <el-tag effect="plain" type="success">{{ grp.badge }}</el-tag>
                                </div>
                              </el-col>

                              <el-col :span="6">
                                <div class="left-input-wrapper">
                                  <el-input
                                    :model-value="getDotenvKeyDraft(row.key)"
                                    class="label-input mono-input"
                                    @update:model-value="(val) => setDotenvKeyDraft(row.key, val)"
                                    @keyup.enter="commitDotenvKeyRename(row.key)"
                                    @blur="commitDotenvKeyRename(row.key)"
                                  />
                                </div>
                              </el-col>

                              <el-col :span="10">
                                <el-form-item label-width="0" style="margin-bottom: 0">
                                  <el-input
                                    :model-value="row.value"
                                    :placeholder="String(row.value || '')"
                                    @update:model-value="(val) => handleSetDotenvValue(row.key, val)"
                                  />
                                </el-form-item>
                              </el-col>

                              <el-col :span="1" style="text-align: center;">
                                <el-button link type="danger" @click="handleRemoveDotenvKey(row.key)">
                                  <IconEpMinus />
                                </el-button>
                              </el-col>
                            </el-row>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </el-collapse-item>

                <el-collapse-item
                  v-if="secretParams.length > 0"
                  name="__secrets__"
                  class="service-collapse-item global-env-collapse-item"
                >
                  <template #title>
                    <div class="service-title-header">
                      <div class="service-header-left">
                        <span class="service-name-text">敏感参数（secrets）</span>
                      </div>
                      <div class="service-header-right">
                        <el-tag size="small" effect="plain" type="warning" class="service-count-tag">{{ secretParams.length }} 项</el-tag>
                      </div>
                    </div>
                  </template>

                  <div class="service-content">
                    <div class="config-section global-env-section">
                      <div v-for="(row, idx) in secretParams" :key="row.key || idx" class="form-row-custom">
                        <el-row :gutter="8" align="middle">
                          <el-col :span="6">
                            <div class="param-type-wrapper">
                              <el-tag effect="plain" type="warning">secret</el-tag>
                            </div>
                          </el-col>

                          <el-col :span="6">
                            <div class="left-input-wrapper">
                              <el-input :model-value="row.key" class="label-input mono-input" readonly />
                            </div>
                          </el-col>

                          <el-col :span="10">
                            <el-form-item label-width="0" style="margin-bottom: 0">
                              <el-input
                                v-model="secretValues[row.key]"
                                type="password"
                                show-password
                                :placeholder="row.required ? '必填' : ''"
                              />
                            </el-form-item>
                          </el-col>

                          <el-col :span="1" style="text-align: center;">
                            <el-tag v-if="row.required" size="small" type="danger" effect="plain">必填</el-tag>
                          </el-col>
                        </el-row>
                      </div>
                    </div>
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="form-actions">
            <el-button @click="goBack">取消</el-button>
            <el-button type="primary" :loading="deploying" @click="submitDeploy">
              确认部署
            </el-button>
          </div>
        </el-form>
        </el-tab-pane>
        <el-tab-pane v-if="advancedMode" label="高级部署" name="advanced">
          <div class="advanced-deploy-grid">
            <div class="advanced-left service-collapse-item">
              <div class="advanced-header">docker-compose.yml</div>
              <div ref="advancedComposeEditorContainer" class="monaco-editor-wrapper"></div>
            </div>
            <div class="advanced-right service-collapse-item global-env-collapse-item">
              <div class="advanced-header">.env</div>
              <div ref="advancedEnvEditorContainer" class="monaco-editor-wrapper"></div>
            </div>
          </div>
          <div class="form-actions">
            <el-button @click="goBack">取消</el-button>
            <el-button type="primary" :loading="deploying" @click="submitAdvancedDeploy">确认部署</el-button>
          </div>
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
        <div class="logs-progress">
          <el-progress
            :percentage="deployProgressPercent"
            :status="deployProgressStatus"
            :stroke-width="10"
            :text-inside="true"
          />
          <div v-if="deployProgressText" class="progress-text">{{ deployProgressText }}</div>
        </div>
        <div ref="logsContent" class="logs-content">
          <div v-for="(log, index) in deployLogs" :key="index" :class="['log-line', log.type]">
            {{ log.message }}
          </div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button v-if="deploying" type="warning" @click="runDeployInBackground">后台运行</el-button>
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
import { ref, computed, onMounted, reactive, nextTick, shallowRef, triggerRef, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, ElImageViewer } from 'element-plus'
import { parseDocument, isMap } from 'yaml'
import * as monaco from 'monaco-editor'
import api from '../api'
import request from '../utils/request'
import { useSseLogStream } from '../utils/sseLogStream'
import { composeProjectNamePattern, isValidComposeProjectName, normalizeComposeProjectName } from '../utils/format'

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
const logsContent = ref(null)
const deploySuccess = ref(false)
const activeTab = ref('deploy')
const advancedMode = ref(localStorage.getItem('advancedMode') === '1')
const advancedComposeText = ref('')
const advancedEnvText = ref('')
const advancedComposeEditor = shallowRef(null)
const advancedEnvEditor = shallowRef(null)
const advancedComposeEditorContainer = ref(null)
const advancedEnvEditorContainer = ref(null)
const allocating = ref(false)
const appStoreBase = ref('')
const deployAutoScroll = ref(true)
const taskLogSetRef = shallowRef(null)
const taskTimeoutRef = shallowRef(null)
const lastDeployTaskId = ref(localStorage.getItem('appstore:lastDeployTaskId') || '')

const {
  logs: deployLogs,
  start: startDeployTaskStream,
  stop: stopDeployTaskStream,
  clear: clearDeployLogs,
  pushLine: pushDeployLine,
  progressPercent: deployProgressPercent,
  progressText: deployProgressText,
  progressStatus: deployProgressStatus,
  setProgress: setDeployProgress
} = useSseLogStream({
  autoScroll: deployAutoScroll,
  scrollElRef: logsContent,
  enableProgress: true,
  makeEntry: (payload) => {
    if (payload && typeof payload === 'object' && payload.type && payload.message) return payload
    const text = String(payload || '')
    const lower = text.toLowerCase()
    const t = lower.includes('error') ? 'error' : (lower.includes('warn') ? 'warning' : 'info')
    return { type: t, message: text, time: new Date().toISOString() }
  },
  getSearchText: (l) => `${String(l?.type || '')} ${String(l?.message || '')}`,
  onOpenLine: '',
  onErrorLine: '',
  onMessage: (event, { pushLine, stop, payload, setProgress }) => {
    const data = (payload && typeof payload === 'object') ? payload : null
    if (!data) {
      pushLine({ type: 'warning', message: String(event?.data || ''), time: new Date().toISOString() })
      return
    }

    if (data && data.type === 'result') {
      if (taskTimeoutRef.value) {
        clearTimeout(taskTimeoutRef.value)
        taskTimeoutRef.value = null
      }
      if (data.status === 'success') setProgress({ percent: 100, status: 'success', text: String(data.message || '任务结束') })
      if (data.status === 'error') setProgress({ percent: 100, status: 'exception', text: String(data.message || '任务结束') })
      stop()
      deploying.value = false
      if (data.status === 'success') {
        deploySuccess.value = true
        pushLine({ type: 'success', message: '部署任务完成！', time: new Date().toISOString() })
        ElMessage.success('部署成功')
        api.appstore.submitDeployCount(String(projectId)).catch((e) => {
          console.warn('Submit deploy count failed', e)
        })
      } else {
        pushLine({ type: 'error', message: `部署失败: ${data.message || '未知错误'}`, time: new Date().toISOString() })
        ElMessage.error('部署失败')
      }
      return
    }

    const logSet = taskLogSetRef.value
    const logKey = `${data?.type || 'info'}:${data?.message || ''}`
    if (logSet && logSet.has(logKey)) return
    if (logSet) logSet.add(logKey)

    pushLine({
      type: data?.type || 'info',
      message: `[${new Date(data?.time || Date.now()).toLocaleTimeString()}] ${data?.message || ''}`,
      time: data?.time || new Date().toISOString()
    })
  },
  onError: () => {}
})

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
const dotenvText = ref('')
const appVarCatalog = ref({ variables: [], params: [], warnings: [] })
const secretValues = reactive({})
const dotenvKeyDraftMap = reactive({})
const getDotenvKeyDraft = (key) => {
  const k = String(key || '').trim()
  if (!k) return ''
  const v = dotenvKeyDraftMap[k]
  if (typeof v === 'string') return v
  dotenvKeyDraftMap[k] = k
  return k
}
const setDotenvKeyDraft = (key, value) => {
  const k = String(key || '').trim()
  if (!k) return
  dotenvKeyDraftMap[k] = String(value ?? '')
}
const commitDotenvKeyRename = (oldKey) => {
  const from = String(oldKey || '').trim()
  if (!from) return
  const nextKey = String(dotenvKeyDraftMap[from] ?? '').trim()
  if (!nextKey || nextKey === from) {
    dotenvKeyDraftMap[from] = from
    return
  }
  handleRenameDotenvKey(from, nextKey)
  delete dotenvKeyDraftMap[from]
  dotenvKeyDraftMap[nextKey] = nextKey
}

/**
 * parseDotenvTextToOrderedMap 解析 .env 文本为 map，并保留 key 的首次出现顺序（后者覆盖前者）
 */
const parseDotenvTextToOrderedMap = (text) => {
  const out = {}
  const order = []
  const seen = new Set()

  const lines = String(text || '').split(/\r?\n/)
  lines.forEach((raw) => {
    let line = String(raw || '').trim()
    if (!line || line.startsWith('#')) return
    if (line.startsWith('export ')) line = line.slice(7).trim()

    const idx = line.indexOf('=')
    if (idx < 0) {
      const key = line.trim()
      if (!key) return
      if (!seen.has(key)) {
        seen.add(key)
        order.push(key)
      }
      if (!(key in out)) out[key] = ''
      return
    }
    const key = line.slice(0, idx).trim()
    if (!key) return
    let valRaw = line.slice(idx + 1).trim()
    let val = valRaw
    if (valRaw.length >= 2) {
      const first = valRaw[0]
      const last = valRaw[valRaw.length - 1]
      if ((first === '"' && last === '"') || (first === "'" && last === "'")) {
        val = valRaw.slice(1, -1)
      } else if (first === '"' || first === "'") {
        val = valRaw
      }
    }
    if (!seen.has(key)) {
      seen.add(key)
      order.push(key)
    }
    out[key] = val
  })

  return { map: out, order }
}

/**
 * formatDotenvValue 将值格式化为 .env 行里的 value（必要时加引号）
 */
const formatDotenvValue = (value) => {
  const raw = String(value ?? '')
  const needsQuote = /[\s#"'\r\n]/.test(raw)
  if (!needsQuote) return raw
  const escaped = raw.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  return `"${escaped}"`
}

/**
 * upsertDotenvKeyValue 将 key=value 写入 .env 文本（尽量只替换最后一次出现）
 */
const upsertDotenvKeyValue = (dotenvTextValue, key, value) => {
  const k = String(key || '').trim()
  if (!k) return String(dotenvTextValue || '')

  const lines = String(dotenvTextValue || '').replace(/\r\n/g, '\n').split('\n')
  const escapedKey = k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const keyRegex = new RegExp(`^\\s*(?:export\\s+)?${escapedKey}\\s*=`)

  let lastIdx = -1
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]
    const trimmed = String(raw || '').trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    if (keyRegex.test(raw)) lastIdx = i
  }

  const newLine = `${k}=${formatDotenvValue(value)}`
  if (lastIdx >= 0) {
    lines[lastIdx] = newLine
    return lines.join('\n').replace(/\n*$/, '\n')
  }
  return appendDotenvLines(dotenvTextValue, [newLine])
}

/**
 * removeDotenvKey 从 .env 文本中移除指定 key 的所有定义行
 */
const removeDotenvKey = (dotenvTextValue, key) => {
  const k = String(key || '').trim()
  if (!k) return String(dotenvTextValue || '')
  const escapedKey = k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const keyRegex = new RegExp(`^\\s*(?:export\\s+)?${escapedKey}\\s*(?:=|$)`)
  const lines = String(dotenvTextValue || '').replace(/\r\n/g, '\n').split('\n')
  const out = lines.filter((raw) => {
    const trimmed = String(raw || '').trim()
    if (!trimmed || trimmed.startsWith('#')) return true
    return !keyRegex.test(raw)
  })
  return out.join('\n').replace(/\n*$/, '\n')
}

const appendDotenvLines = (text, lines) => {
  const base = String(text || '').replace(/\s+$/, '')
  const appendPart = (lines || []).filter(Boolean).join('\n')
  if (!appendPart) return base ? base + '\n' : ''
  if (!base) return appendPart + '\n'
  return base + '\n' + appendPart + '\n'
}

const buildUniqueDotenvKey = (baseKey) => {
  const base = String(baseKey || '').trim() || 'NEW_VAR'
  const { map: m } = parseDotenvTextToOrderedMap(dotenvText.value)
  if (!(base in m)) return base
  let i = 2
  while (i < 9999) {
    const next = `${base}_${i}`
    if (!(next in m)) return next
    i++
  }
  return `${base}_${Date.now()}`
}

const handleImageError = (e) => {
  e.target.src = 'https://cdn-icons-png.flaticon.com/512/873/873133.png'
}

const allowAutoAllocPort = ref(false)
const autoAllocTriggered = ref(false)

const initAppStoreBase = async () => {
  try {
    const s = await request.get('/settings/global')
    appStoreBase.value = (s && s.appStoreServerUrl) ? s.appStoreServerUrl.replace(/\/$/, '') : 'https://template.cgakki.top:33333'
    allowAutoAllocPort.value = !!(s && s.allowAutoAllocPort)
  } catch (e) {
    appStoreBase.value = 'https://template.cgakki.top:33333'
    allowAutoAllocPort.value = false
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
  if (serviceName === 'Global') {
    handleAddGlobalDotenvKey()
    return
  }
  // 直接向 deployConfig 添加
  const newParamName = `CUSTOM_${serviceName.toUpperCase()}_${Date.now()}`
  
  const cfg = reactive({
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
  cfg.formKey = buildConfigFormKey(cfg, deployConfig.value.length)
  deployConfig.value.push(cfg)
  formData.value[cfg.formKey] = cfg.default
  triggerRef(deployConfig)
}

const dotenvRows = computed(() => {
  const { map, order } = parseDotenvTextToOrderedMap(dotenvText.value)
  return order
    .filter(k => k && Object.prototype.hasOwnProperty.call(map, k))
    .map((k) => ({ key: k, value: map[k] }))
})

const dotenvGroupedRows = computed(() => {
  const params = appVarCatalog.value && Array.isArray(appVarCatalog.value.params) ? appVarCatalog.value.params : []
  const envParams = params.filter(p => p && p.kind === 'env' && String(p.key || '').trim())
  const keyToFiles = new Map()
  for (const p of envParams) {
    const key = String(p.key || '').trim()
    if (!key) continue
    const bindings = Array.isArray(p.bindings) ? p.bindings : []
    const files = bindings
      .map(b => String(b?.file || '').trim())
      .filter(Boolean)
    if (files.length > 0) {
      keyToFiles.set(key, files)
    }
  }

  const groups = new Map()
  const getBaseName = (p) => {
    const s = String(p || '').trim()
    if (!s) return ''
    const parts = s.split('/')
    return parts[parts.length - 1] || s
  }
  for (const row of dotenvRows.value) {
    const key = String(row?.key || '').trim()
    const files = keyToFiles.get(key) || []
    const file = files.length > 0 ? files[0] : '.env'
    if (!groups.has(file)) {
      groups.set(file, {
        name: getBaseName(file),
        path: file,
        badge: getBaseName(file),
        rows: []
      })
    }
    groups.get(file).rows.push(row)
  }

  const items = Array.from(groups.values())
  items.sort((a, b) => {
    if (a.path === '.env' && b.path !== '.env') return 1
    if (b.path === '.env' && a.path !== '.env') return -1
    return String(a.path).localeCompare(String(b.path))
  })
  return items
})

const dotenvMapForVars = computed(() => {
  return parseDotenvTextToOrderedMap(dotenvText.value).map || {}
})

const shouldShowDotenvInheritanceHint = (config) => {
  if (!config) return false
  const svc = String(config?.serviceName || '').trim()
  if (!svc || svc === 'Global') return false
  if (!isEnvConfigItem(config)) return false
  const key = String(config?.name || '').trim()
  if (!key) return false
  const cur = String(formData.value?.[config.formKey] ?? '').trim()
  if (cur) return false
  return Object.prototype.hasOwnProperty.call(dotenvMapForVars.value, key)
}

const catalogVariables = computed(() => {
  const params = appVarCatalog.value && Array.isArray(appVarCatalog.value.params) ? appVarCatalog.value.params : []
  const envParams = params.filter(p => p && p.kind === 'env' && String(p.key || '').trim())
  if (envParams.length > 0) {
    return envParams.map(p => {
      const rawSources = Array.isArray(p.sources) ? p.sources.map(s => String(s || '').trim()).filter(Boolean) : []
      const normalizedSources = rawSources.map(s => {
        if (s === 'compose_ref' || s === 'compose_default') return 'compose'
        return s
      })
      return {
        name: String(p.key || '').trim(),
        required: !!p.required,
        defaultValue: String(p.defaultValue ?? ''),
        sources: normalizedSources,
        usages: Array.isArray(p.usages) ? p.usages.map(s => String(s || '').trim()).filter(Boolean) : [],
        examples: Array.isArray(p.examples) ? p.examples.map(s => String(s || '').trim()).filter(Boolean) : []
      }
    })
  }
  const list = appVarCatalog.value && Array.isArray(appVarCatalog.value.variables) ? appVarCatalog.value.variables : []
  return list
    .filter(v => v && typeof v === 'object' && String(v.name || '').trim())
    .map(v => ({
      name: String(v.name || '').trim(),
      required: !!v.required,
      defaultValue: String(v.defaultValue ?? v.value ?? ''),
      sources: Array.isArray(v.sources) ? v.sources.map(s => String(s || '').trim()).filter(Boolean) : [],
      usages: [],
      examples: Array.isArray(v.examples) ? v.examples.map(s => String(s || '').trim()).filter(Boolean) : []
    }))
})

const requiredVarsMissingCount = computed(() => {
  const map = dotenvMapForVars.value || {}
  return catalogVariables.value.filter(v => v.required && !Object.prototype.hasOwnProperty.call(map, v.name)).length
})

const secretParams = computed(() => {
  const params = appVarCatalog.value && Array.isArray(appVarCatalog.value.params) ? appVarCatalog.value.params : []
  return params
    .filter(p => p && p.kind === 'secret' && String(p.key || '').trim())
    .map(p => ({
      key: String(p.key || '').trim(),
      required: !!p.required,
      bindings: Array.isArray(p.bindings) ? p.bindings : []
    }))
})

/**
 * handleSetDotenvValue 在表单里修改全局变量时，同步写回 .env 文本
 */
const handleSetDotenvValue = (key, value) => {
  dotenvText.value = upsertDotenvKeyValue(dotenvText.value, key, value)
}

const handleRenameDotenvKey = (oldKey, newKey) => {
  const from = String(oldKey || '').trim()
  const to = String(newKey || '').trim()
  if (!from || from === to) {
    return
  }

  const pattern = /^[A-Za-z_][A-Za-z0-9_]*$/
  if (!pattern.test(to)) {
    ElMessage.warning('变量名仅支持字母、数字、下划线，且不能以数字开头')
    return
  }

  const { map } = parseDotenvTextToOrderedMap(dotenvText.value)
  if (Object.prototype.hasOwnProperty.call(map, to)) {
    ElMessage.warning(`变量名 ${to} 已存在`)
    return
  }

  const val = Object.prototype.hasOwnProperty.call(map, from) ? map[from] : ''
  let next = removeDotenvKey(dotenvText.value, from)
  next = upsertDotenvKeyValue(next, to, val)
  dotenvText.value = next
}

/**
 * handleRemoveDotenvKey 删除全局变量（从 .env 中移除）
 */
const handleRemoveDotenvKey = (key) => {
  dotenvText.value = removeDotenvKey(dotenvText.value, key)
}

/**
 * handleAddGlobalDotenvKey 添加一条新的全局变量到 .env
 */
const handleAddGlobalDotenvKey = () => {
  const key = buildUniqueDotenvKey('NEW_VAR')
  dotenvText.value = appendDotenvLines(dotenvText.value, [`${key}=`])
  ElMessage.success('已添加全局变量')
  if (!activeServiceNames.value.includes('__dotenv__')) {
    activeServiceNames.value = ['__dotenv__', ...activeServiceNames.value]
  }
}

const handleRemoveParam = (config) => {
  const index = deployConfig.value.findIndex(item => item === config || item.name === config.name)
  if (index > -1) {
    try {
      if (config && config.formKey && Object.prototype.hasOwnProperty.call(formData.value, config.formKey)) {
        delete formData.value[config.formKey]
      }
    } catch (e) {}
    deployConfig.value.splice(index, 1)
    triggerRef(deployConfig)
    ElMessage.success('已移除参数')
  }
}

const buildConfigFormKey = (cfg, idx) => {
  const svc = String(cfg?.serviceName || 'Global').trim() || 'Global'
  const raw = String(cfg?.name || '').trim()
  const base = raw || 'param'
  return `${svc}:${base}:${idx}`
}

const initForm = () => {
  const originalDotenv = String(project.value?.dotenv || '')
  const hasOriginalDotenv = originalDotenv.trim().length > 0
  const { map: dotenvMap } = parseDotenvTextToOrderedMap(originalDotenv)
  const addedDotenvLines = []

  dotenvText.value = originalDotenv
  if (!project.value || !project.value.schema) return

  try {
    const raw = JSON.parse(JSON.stringify(project.value.schema))
    const filtered = []

    raw.forEach((item) => {
      if (!item || typeof item !== 'object') return
      if (!item.serviceName) item.serviceName = 'Global'

      const key = String(item.name || '').trim()
      const paramType = String(item.paramType || '').trim()
      const typ = String(item.type || '').trim()
      const isEnv = paramType === 'env' || paramType === 'environment' || (!paramType && ['string', 'password', 'number', 'boolean', 'text'].includes(typ))

      if (item.serviceName === 'Global' && isEnv && key) {
        if (hasOriginalDotenv) {
          if (!(key in dotenvMap)) {
            dotenvMap[key] = String(item.default || '')
            addedDotenvLines.push(`${key}=${String(item.default || '')}`)
          }
          return
        }
        filtered.push(item)
        return
      }

      filtered.push(item)
    })

    if (hasOriginalDotenv && addedDotenvLines.length > 0) {
      dotenvText.value = appendDotenvLines(originalDotenv, addedDotenvLines)
    }

    deployConfig.value = filtered.map((item) => reactive(item))
  } catch (e) {
    deployConfig.value = []
  }

  // 初始化验证规则
  formData.value = {}
  deployConfig.value.forEach(config => {
    // 确保 label 存在，方便界面编辑
    if (!config.label) {
      config.label = config.name
    }

    if (!config.formKey) {
      const idx = deployConfig.value.indexOf(config)
      config.formKey = buildConfigFormKey(config, idx)
    }
    if (!Object.prototype.hasOwnProperty.call(formData.value, config.formKey)) {
      formData.value[config.formKey] = config.default
    }

    // 生成验证规则
    if (config.description && config.description.includes('required')) {
      rules[config.formKey] = [
        { required: true, message: `请输入${config.label || config.name}`, trigger: 'blur' }
      ]
    }
  })
  
  // 默认展开所有服务
  nextTick(() => {
    const keys = Object.keys(groupedSchema.value)
    const hasDotenv = dotenvRows.value.length > 0
    activeServiceNames.value = hasDotenv ? ['__dotenv__', ...keys] : keys
    autoAllocatePortsIfNeeded()
  })
}

const ensureDotenvContainsRequiredVars = () => {
  const vars = catalogVariables.value || []
  if (vars.length === 0) return
  const { map } = parseDotenvTextToOrderedMap(dotenvText.value)
  const missing = []
  vars.forEach((v) => {
    if (!v || !v.required) return
    const k = String(v.name || '').trim()
    if (!k) return
    if (Object.prototype.hasOwnProperty.call(map, k)) return
    missing.push(`${k}=`)
  })
  if (missing.length > 0) {
    dotenvText.value = appendDotenvLines(dotenvText.value, missing)
  }
}

const isValidPortNumber = (v) => {
  const t = String(v ?? '').trim()
  if (!t) return false
  const n = Number(t)
  return Number.isInteger(n) && n > 0 && n <= 65535
}

const getHostPortValueFromConfig = (cfg) => {
  if (!cfg) return ''
  if (cfg.isCustom) return cfg.customKey
  return cfg.name
}

const setHostPortValueToConfig = (cfg, port) => {
  const v = String(port ?? '').trim()
  if (!cfg) return
  if (cfg.isCustom) {
    cfg.customKey = v
    cfg.name = v
    return
  }
  cfg.name = v
}

const fetchProject = async () => {
  loading.value = true
  try {
    const res = await api.appstore.getProjectVars(projectId)
    const data = res.data || res
    const app = data && data.app ? data.app : null
    if (app) {
      project.value = app
      if (typeof data.dotenv === 'string') {
        project.value.dotenv = data.dotenv
      }
      advancedComposeText.value = String(project.value?.compose || '')
      advancedEnvText.value = String(project.value?.dotenv || '')
      appVarCatalog.value = {
        variables: Array.isArray(data.variables) ? data.variables : [],
        params: Array.isArray(data.params) ? data.params : [],
        warnings: Array.isArray(data.warnings) ? data.warnings : []
      }
      const params = Array.isArray(appVarCatalog.value.params) ? appVarCatalog.value.params : []
      params
        .filter(p => p && p.kind === 'secret' && String(p.key || '').trim())
        .forEach((p) => {
          const k = String(p.key || '').trim()
          if (!k) return
          if (typeof secretValues[k] !== 'string') secretValues[k] = ''
        })
      autoAllocTriggered.value = false
      initForm()
      ensureDotenvContainsRequiredVars()
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
  if ((import.meta.env.VITE_MANAGEMENT_MODE || 'CS').toUpperCase() === 'CS') {
    router.push('/compose')
  } else {
    router.push('/projects')
  }
}

// 简单对象转 YAML 字符串（按固定顺序输出 Compose 关键字段）
const toYaml = (obj, indent = 0, scope = '') => {
  const spaces = ' '.repeat(indent)
  let yaml = ''

  const orderKeys = (keys, first, last) => {
    const firstSet = new Set(first)
    const lastSet = new Set(last)
    const out = []
    first.forEach(k => {
      if (keys.includes(k)) out.push(k)
    })
    keys.forEach(k => {
      if (firstSet.has(k) || lastSet.has(k)) return
      out.push(k)
    })
    last.forEach(k => {
      if (keys.includes(k)) out.push(k)
    })
    return out
  }

  const rawKeys = Object.keys(obj || {})
  const keys = (() => {
    if (scope === '__root__') {
      return orderKeys(rawKeys, ['version', 'name', 'services', 'networks', 'volumes', 'configs', 'secrets'], [])
    }
    if (scope === '__service__') {
      return orderKeys(rawKeys, ['image', 'ports', 'volumes', 'env_file', 'environment'], ['healthcheck', 'command'])
    }
    return rawKeys
  })()
  
  for (const key of keys) {
    const value = obj[key]
    if (Array.isArray(value)) {
      yaml += `${spaces}${key}:\n`
      value.forEach(item => {
        yaml += `${spaces}  - "${item}"\n` // 强制加引号避免解析错误
      })
    } else if (typeof value === 'object' && value !== null) {
      yaml += `${spaces}${key}:\n`
      if (scope === '__root__' && key === 'services') {
        yaml += toYaml(value, indent + 2, '__services__')
      } else if (scope === '__services__') {
        yaml += toYaml(value, indent + 2, '__service__')
      } else {
        yaml += toYaml(value, indent + 2, key)
      }
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

/**
 * injectEnvFileForServices 在 compose.yaml 中为指定服务注入 env_file: [.env]
 * 只在该服务块内不存在 env_file 时注入
 */
const injectEnvFileForServices = (yamlText, serviceNameSet) => {
  const need = serviceNameSet instanceof Set ? serviceNameSet : new Set()
  if (!need || need.size === 0) return String(yamlText || '')

  const lines = String(yamlText || '').replace(/\r\n/g, '\n').split('\n')
  const findIndent = (s) => {
    const m = String(s || '').match(/^(\s*)/)
    return m ? m[1].length : 0
  }
  const isKeyLineAtIndent = (idx, indent, key) => {
    const line = String(lines[idx] || '')
    if (findIndent(line) !== indent) return false
    return line.trimStart().startsWith(`${key}:`)
  }
  const isServiceHeaderAtIndent = (idx, indent) => {
    const line = String(lines[idx] || '')
    if (findIndent(line) !== indent) return null
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) return null
    if (!trimmed.endsWith(':')) return null
    const name = trimmed.slice(0, -1).trim()
    if (!name) return null
    if (name.includes(' ')) return null
    return name
  }
  const findBlockEnd = (startIdx, baseIndent) => {
    for (let i = startIdx + 1; i < lines.length; i++) {
      const line = String(lines[i] || '')
      if (!line.trim()) continue
      const ind = findIndent(line)
      if (ind <= baseIndent) return i
    }
    return lines.length
  }

  const servicesIdx = lines.findIndex(l => /^\s*services\s*:\s*(#.*)?$/.test(String(l || '')))
  if (servicesIdx < 0) return String(yamlText || '')
  const servicesIndent = findIndent(lines[servicesIdx])
  const detectSvcHeaderIndent = () => {
    for (let k = servicesIdx + 1; k < lines.length; k++) {
      const line = String(lines[k] || '')
      if (!line.trim()) continue
      const ind = findIndent(line)
      if (ind <= servicesIndent) return servicesIndent + 2
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#')) continue
      if (trimmed.endsWith(':')) return ind
    }
    return servicesIndent + 2
  }
  const svcHeaderIndent = detectSvcHeaderIndent()
  const svcItemIndent = svcHeaderIndent + 2

  let i = servicesIdx + 1
  while (i < lines.length) {
    const name = isServiceHeaderAtIndent(i, svcHeaderIndent)
    if (!name) {
      const ind = findIndent(lines[i])
      if (lines[i] && lines[i].trim() && ind <= servicesIndent) break
      i++
      continue
    }

    const blockEnd = findBlockEnd(i, svcHeaderIndent)
    if (!need.has(name)) {
      i = blockEnd
      continue
    }

    let hasEnvFile = false
    for (let j = i + 1; j < blockEnd; j++) {
      if (isKeyLineAtIndent(j, svcItemIndent, 'env_file')) {
        hasEnvFile = true
        break
      }
    }
    if (hasEnvFile) {
      i = blockEnd
      continue
    }

    const insertLines = [
      `${' '.repeat(svcItemIndent)}env_file:`,
      `${' '.repeat(svcItemIndent)}  - .env`
    ]

    const findInsertPos = () => {
      for (let j = i + 1; j < blockEnd; j++) {
        if (isKeyLineAtIndent(j, svcItemIndent, 'environment')) return j
      }

      const keys = ['volumes', 'ports', 'image']
      for (const k of keys) {
        for (let j = i + 1; j < blockEnd; j++) {
          if (!isKeyLineAtIndent(j, svcItemIndent, k)) continue
          const end = findBlockEnd(j, svcItemIndent)
          return Math.min(end, blockEnd)
        }
      }
      return i + 1
    }

    const pos = findInsertPos()
    lines.splice(pos, 0, ...insertLines)
    i = blockEnd + insertLines.length
  }

  return lines.join('\n').replace(/\n*$/, '\n')
}

/**
 * injectEnvFileForServicesAst 用 YAML AST 方式为指定服务注入 env_file: ['.env']
 * 优先保留原始 YAML 的注释/缩进/锚点等结构；解析失败时回退到文本注入逻辑
 */
const injectEnvFileForServicesAst = (yamlText, serviceNameSet) => {
  const need = serviceNameSet instanceof Set ? serviceNameSet : new Set()
  if (!need || need.size === 0) return String(yamlText || '')

  const input = String(yamlText || '')
  try {
    const doc = parseDocument(input, { keepSourceTokens: true })
    const servicesNode = doc.get('services', true)
    if (!servicesNode || !isMap(servicesNode)) return injectEnvFileForServices(input, need)

    for (const svcName of need) {
      const svcNode = doc.getIn(['services', svcName], true)
      if (!svcNode || !isMap(svcNode)) continue

      const envFileNode = doc.getIn(['services', svcName, 'env_file'], true)
      if (envFileNode != null) continue

      doc.setIn(['services', svcName, 'env_file'], ['.env'])
    }

    return String(doc).replace(/\n*$/, '\n')
  } catch (e) {
    console.warn('compose.yaml YAML AST 解析失败，回退到文本注入逻辑', e)
    return injectEnvFileForServices(input, need)
  }
}

const removeDotenvEnvFileRefsAst = (yamlText) => {
  const input = String(yamlText || '')
  try {
    const doc = parseDocument(input, { keepSourceTokens: true })
    const servicesNode = doc.get('services', true)
    if (!servicesNode || !isMap(servicesNode)) return input

    for (const pair of servicesNode.items || []) {
      const svcName = String(pair?.key?.value ?? '').trim()
      if (!svcName) continue
      const envFileNode = doc.getIn(['services', svcName, 'env_file'], true)
      if (envFileNode == null) continue

      if (typeof envFileNode?.value === 'string') {
        if (String(envFileNode.value).trim() === '.env') doc.deleteIn(['services', svcName, 'env_file'])
        continue
      }

      if (Array.isArray(envFileNode?.items)) {
        const keep = []
        for (const it of envFileNode.items) {
          const v = it?.value
          if (typeof v === 'string') {
            if (String(v).trim() === '.env') continue
            keep.push(v)
            continue
          }
          const obj = typeof it?.toJSON === 'function' ? it.toJSON() : null
          if (obj && typeof obj === 'object' && String(obj.path || '').trim() === '.env') continue
          keep.push(obj || v)
        }
        if (keep.length === 0) {
          doc.deleteIn(['services', svcName, 'env_file'])
        } else {
          doc.setIn(['services', svcName, 'env_file'], keep)
        }
      }
    }

    return String(doc).replace(/\n*$/, '\n')
  } catch (e) {
    return input
  }
}

const hasInterpolationInAnyValue = (v) => {
  if (typeof v === 'string') {
    return /\$\{[A-Za-z_][A-Za-z0-9_]*[^}]*\}/.test(v)
  }
  if (Array.isArray(v)) {
    return v.some(hasInterpolationInAnyValue)
  }
  if (v && typeof v === 'object') {
    return Object.values(v).some(hasInterpolationInAnyValue)
  }
  return false
}

const isEnvConfigItem = (cfg) => {
  const paramType = String(cfg?.paramType || '').trim()
  const typ = String(cfg?.type || '').trim()
  if (paramType) {
    return paramType === 'env' || paramType === 'environment'
  }
  return ['string', 'password', 'number', 'boolean', 'text'].includes(typ)
}

const collectServicesNeedEnvFileFromCompose = (yamlText, opts = {}) => {
  const { config, dotenvMap } = opts || {}
  const need = new Set()
  const input = String(yamlText || '')
  try {
    const doc = parseDocument(input)
    const servicesNode = doc.get('services', true)
    if (!servicesNode || !isMap(servicesNode)) return need

    for (const pair of servicesNode.items || []) {
      const svcName = String(pair?.key?.value ?? '').trim()
      if (!svcName) continue
      const svcNode = doc.getIn(['services', svcName], true)
      if (!svcNode) continue
      const json = typeof svcNode.toJSON === 'function' ? svcNode.toJSON() : null
      if (hasInterpolationInAnyValue(json)) need.add(svcName)
    }

    const envMap = dotenvMap && typeof dotenvMap === 'object' ? dotenvMap : null
    const cfgList = Array.isArray(config) ? config : null
    if (envMap && cfgList) {
      for (const cfg of cfgList) {
        const svc = String(cfg?.serviceName || '').trim()
        if (!svc || svc === 'Global') continue
        if (!isEnvConfigItem(cfg)) continue
        const key = String(cfg?.name || '').trim()
        if (!key) continue
        if (Object.prototype.hasOwnProperty.call(envMap, key)) {
          need.add(svc)
        }
      }
    }
    return need
  } catch (e) {
    return need
  }
}

const submitDeploy = async () => {
  try {
    if (formRef.value && typeof formRef.value.validate === 'function') {
      await formRef.value.validate()
    }
  } catch (e) {
    return
  }

  try {
    ;(deployConfig.value || []).forEach((cfg) => {
      if (!cfg || !cfg.formKey) return
      if (Object.prototype.hasOwnProperty.call(formData.value, cfg.formKey)) {
        cfg.default = formData.value[cfg.formKey]
      }
    })
  } catch (e) {}

  try {
    // 0. 构建最终的环境变量/参数映射 (为了兼容性保留 Env Map，虽然主要靠 Config 数组)
    const finalEnv = {}
    const dotenvKeySet = new Set()
    let dotenvMap = {}
    try {
      const parsed = parseDotenvTextToOrderedMap(String(dotenvText.value || ''))
      dotenvMap = parsed?.map || {}
      Object.keys(dotenvMap || {}).forEach(k => {
        const kk = String(k || '').trim()
        if (kk) dotenvKeySet.add(kk)
      })
    } catch (e) {}

    const shouldDropEnvConfigByDotenv = (cfg) => {
      const key = String(cfg?.name || '').trim()
      if (!key) return false
      const val = String(cfg?.default ?? '').trim()
      if (!val) return false
      const isEnv = (cfg?.paramType === 'env' || cfg?.paramType === 'environment') || (!cfg?.paramType && cfg?.type !== 'port' && cfg?.type !== 'path' && cfg?.type !== 'volume')
      if (!isEnv) return false
      const isPlaceholder = val === `[${key}]` || val === `\${${key}}` || val.startsWith(`\${${key}:`) || val.startsWith(`\${${key}-`) || val.startsWith(`\${${key}?`) || val.startsWith(`\${${key}+`)
      if (!isPlaceholder) return false
      return !dotenvKeySet.has(key)
    }

    // 处理 Config 数组中的自定义 Key
    const effectiveConfig = []
    deployConfig.value.forEach(config => {
        if (config.isCustom && config.customKey) {
            config.name = config.customKey // 提交前更新 name
        }

        if (shouldDropEnvConfigByDotenv(config)) {
          return
        }
        effectiveConfig.push(config)

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
      yamlContent = toYaml(composeObj, 0, '__root__')
    } else {
       console.warn('No compose or services found in project definition')
    }
    const hasDotenv = String(dotenvText.value || '').trim().length > 0

    const rawName = project.value.name || ''
    let projectName = normalizeComposeProjectName(rawName)

    if (projectName !== rawName) {
      try {
        await ElMessageBox.confirm(
          `当前应用名称 "${rawName}" 包含 Docker Compose 不支持的字符，将使用规范化名称 "${projectName}" 作为项目名进行部署（目录及项目列表将以该名称显示）。是否继续？`,
          '项目名称规范化',
          {
            confirmButtonText: '继续部署',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      } catch (e) {
        return
      }
    }

    // 检查项目是否已存在
    try {
      const installedRes = await api.compose.list()
      const installedList = installedRes.data || installedRes
      const exists = (name) => installedList.some(p => p.name === name)

      while (exists(projectName)) {
        try {
          const { value } = await ElMessageBox.prompt(
            `项目${projectName}已存在，请输入新的项目名（文件名）以继续安装`,
            '项目已存在',
            {
              confirmButtonText: '继续安装',
              cancelButtonText: '取消',
              inputValue: `${projectName}-2`,
              inputPattern: composeProjectNamePattern,
              inputErrorMessage: '仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头'
            }
          )

          const nextName = String(value || '').toLowerCase().trim()
          if (!isValidComposeProjectName(nextName)) continue
          projectName = nextName
        } catch (e) {
          return
        }
      }
    } catch (error) {
       console.warn('Check installed projects failed', error)
    }

    // 2. 初始化部署状态
    showLogs.value = true
    deploying.value = true
    deploySuccess.value = false
    deployLogs.value = []
    taskLogSetRef.value = new Set()
    if (taskTimeoutRef.value) {
      clearTimeout(taskTimeoutRef.value)
      taskTimeoutRef.value = null
    }
    stopDeployTaskStream()

    // 3. 调用部署接口
    const deployData = {
      projectId: String(projectId),
      projectName,
      compose: yamlContent,
      env: finalEnv, // 兼容旧逻辑
      dotenv: hasDotenv ? String(dotenvText.value || '') : '',
      config: effectiveConfig // 新逻辑：传递完整配置数组
    }
    const secretsPayload = {}
    secretParams.value.forEach((p) => {
      const k = String(p?.key || '').trim()
      if (!k) return
      const v = String(secretValues[k] ?? '')
      if (v.trim() === '') return
      secretsPayload[k] = v
    })
    deployData.secrets = secretsPayload

    try {
      const res = await api.appstore.deployProject(deployData)
      // 兼容返回结构
      const responseData = res.data || res
      const taskId = responseData.taskId
      
      if (!taskId) {
        throw new Error('未获取到任务ID')
      }
      lastDeployTaskId.value = String(taskId)
      localStorage.setItem('appstore:lastDeployTaskId', String(taskId))

      ElMessage.success('部署任务已提交，正在执行...')
      
      // 4. 使用 SSE 监听任务进度
      const token = localStorage.getItem('token') || ''
      const url = `/api/appstore/tasks/${taskId}/events?token=${encodeURIComponent(token)}`

      taskTimeoutRef.value = setTimeout(() => {
        pushDeployLine({ type: 'warning', message: '日志连接超时，请稍后在容器列表查看状态', time: new Date().toISOString() })
        stopDeployTaskStream()
        deploying.value = false
      }, 600000)

      startDeployTaskStream(url, { reset: true })

    } catch (error) {
      console.error(error)
      deploying.value = false
      stopDeployTaskStream()
      if (taskTimeoutRef.value) {
        clearTimeout(taskTimeoutRef.value)
        taskTimeoutRef.value = null
      }
      ElMessage.error('部署失败: ' + (error.response?.data?.error || error.message))
    }

  } catch (error) {
    console.error(error)
    ElMessage.error('准备部署失败: ' + error.message)
    deploying.value = false
    stopDeployTaskStream()
    if (taskTimeoutRef.value) {
      clearTimeout(taskTimeoutRef.value)
      taskTimeoutRef.value = null
    }
  }
}

const runDeployInBackground = () => {
  showLogs.value = false
  const msg = project.value?.name ? `部署任务已后台运行：${project.value.name}` : '部署任务已后台运行'
  api.system.addNotification({ type: 'info', message: msg })
    .then((saved) => {
      window.dispatchEvent(new CustomEvent('dockpier-notification', { detail: { type: 'info', message: msg, dbId: saved?.id, createdAt: saved?.created_at, read: saved?.read } }))
    })
    .catch(() => {
      window.dispatchEvent(new CustomEvent('dockpier-notification', { detail: { type: 'info', message: msg } }))
    })
}

const getPortConfigs = () => {
  const list = []
  ;(deployConfig.value || []).forEach((cfg) => {
    const isPort = (cfg?.paramType === 'port') || (cfg?.type === 'port')
    if (isPort) list.push(cfg)
  })

  const parsePortNum = (v) => {
    const n = Number(String(v ?? '').trim())
    return Number.isFinite(n) ? n : Number.NaN
  }

  list.sort((a, b) => {
    const sa = String(a?.serviceName || '').trim()
    const sb = String(b?.serviceName || '').trim()
    if (sa !== sb) return sa.localeCompare(sb)

    const ca = parsePortNum(a?.default)
    const cb = parsePortNum(b?.default)
    const caOk = Number.isFinite(ca)
    const cbOk = Number.isFinite(cb)
    if (caOk && cbOk && ca !== cb) return ca - cb
    if (caOk !== cbOk) return caOk ? -1 : 1

    const la = String(a?.label || a?.name || '').trim()
    const lb = String(b?.label || b?.name || '').trim()
    return la.localeCompare(lb)
  })

  return list
}

const allocatePortsToConfigs = async (ports, opts = {}) => {
  const { silent = false } = opts || {}
  const list = Array.isArray(ports) ? ports : []
  if (!list.length) {
    if (!silent) ElMessage.info('当前无端口参数需要分配')
    return
  }
  allocating.value = true
  try {
    const res = await api.ports.allocate({ count: list.length, protocol: 'tcp', type: 'host', useAllocRange: true, dryRun: true })
    if (res && res.segments && res.segments.length > 0) {
      const seg = res.segments[0]
      if (seg.length !== list.length) {
        ElMessage.error('分配端口数量不足')
        return
      }
      for (let i = 0; i < list.length; i++) {
        setHostPortValueToConfig(list[i], seg[i])
      }
      triggerRef(deployConfig)
      if (!silent) ElMessage.success('已自动分配端口')
    } else {
      ElMessage.error('分配失败: 未获取到端口段')
    }
  } catch (error) {
    ElMessage.error('自动分配失败: ' + (error.response?.data?.error || error.message))
  } finally {
    allocating.value = false
  }
}

const handleAutoAllocate = async () => {
  await allocatePortsToConfigs(getPortConfigs(), { silent: false })
}

const autoAllocatePortsIfNeeded = async () => {
  if (!allowAutoAllocPort.value) return
  if (autoAllocTriggered.value) return
  if (allocating.value) return

  const ports = getPortConfigs()
  if (!ports.length) {
    autoAllocTriggered.value = true
    return
  }
  const need = ports.filter(p => !isValidPortNumber(getHostPortValueFromConfig(p)))
  if (!need.length) {
    autoAllocTriggered.value = true
    return
  }

  autoAllocTriggered.value = true
  await allocatePortsToConfigs(need, { silent: true })
  ElMessage.success('已按设置自动分配端口')
}

const syncAdvancedMode = () => {
  advancedMode.value = localStorage.getItem('advancedMode') === '1'
  if (advancedComposeEditor.value) {
    advancedComposeEditor.value.updateOptions({ readOnly: !advancedMode.value })
  }
  if (advancedEnvEditor.value) {
    advancedEnvEditor.value.updateOptions({ readOnly: !advancedMode.value })
  }
}

const initAdvancedEditors = () => {
  if (!advancedComposeEditorContainer.value || !advancedEnvEditorContainer.value) return

  if (!advancedComposeEditor.value) {
    advancedComposeEditor.value = monaco.editor.create(advancedComposeEditorContainer.value, {
      value: String(advancedComposeText.value || ''),
      language: 'yaml',
      theme: 'vs',
      automaticLayout: true,
      minimap: { enabled: false },
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
      fontSize: 14,
      tabSize: 2,
      wordWrap: 'on',
      readOnly: !advancedMode.value
    })
    advancedComposeEditor.value.onDidChangeModelContent(() => {
      advancedComposeText.value = advancedComposeEditor.value.getValue()
    })
  } else {
    advancedComposeEditor.value.setValue(String(advancedComposeText.value || ''))
    advancedComposeEditor.value.updateOptions({ readOnly: !advancedMode.value })
  }

  if (!advancedEnvEditor.value) {
    advancedEnvEditor.value = monaco.editor.create(advancedEnvEditorContainer.value, {
      value: String(advancedEnvText.value || ''),
      language: 'ini',
      theme: 'vs',
      automaticLayout: true,
      minimap: { enabled: false },
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
      fontSize: 14,
      tabSize: 2,
      wordWrap: 'on',
      readOnly: !advancedMode.value
    })
    advancedEnvEditor.value.onDidChangeModelContent(() => {
      advancedEnvText.value = advancedEnvEditor.value.getValue()
    })
  } else {
    advancedEnvEditor.value.setValue(String(advancedEnvText.value || ''))
    advancedEnvEditor.value.updateOptions({ readOnly: !advancedMode.value })
  }
}

watch(activeTab, async (v) => {
  if (v !== 'advanced') return
  await nextTick()
  initAdvancedEditors()
})

const submitAdvancedDeploy = async () => {
  if (!advancedMode.value) {
    ElMessage.warning('未开启高级模式')
    return
  }

  const yamlContent = String(advancedComposeEditor.value ? advancedComposeEditor.value.getValue() : advancedComposeText.value || '').trim()
  const dotenvContent = String(advancedEnvEditor.value ? advancedEnvEditor.value.getValue() : advancedEnvText.value || '')
  if (!yamlContent) {
    ElMessage.error('docker-compose.yml 不能为空')
    return
  }

  try {
    const rawName = project.value?.name || ''
    let projectName = normalizeComposeProjectName(rawName)

    if (projectName !== rawName) {
      try {
        await ElMessageBox.confirm(
          `当前应用名称 "${rawName}" 包含 Docker Compose 不支持的字符，将使用规范化名称 "${projectName}" 作为项目名进行部署（目录及项目列表将以该名称显示）。是否继续？`,
          '项目名称规范化',
          { confirmButtonText: '继续部署', cancelButtonText: '取消', type: 'warning' }
        )
      } catch (e) {
        return
      }
    }

    try {
      const installedRes = await api.compose.list()
      const installedList = installedRes.data || installedRes
      const exists = (name) => installedList.some(p => p.name === name)
      while (exists(projectName)) {
        try {
          const { value } = await ElMessageBox.prompt(
            `项目${projectName}已存在，请输入新的项目名（文件名）以继续安装`,
            '项目已存在',
            {
              confirmButtonText: '继续安装',
              cancelButtonText: '取消',
              inputValue: `${projectName}-2`,
              inputPattern: composeProjectNamePattern,
              inputErrorMessage: '仅支持小写字母/数字，且可包含 _ -，并以字母或数字开头'
            }
          )
          const nextName = String(value || '').toLowerCase().trim()
          if (!isValidComposeProjectName(nextName)) continue
          projectName = nextName
        } catch (e) {
          return
        }
      }
    } catch (error) {
      console.warn('Check installed projects failed', error)
    }

    showLogs.value = true
    deploying.value = true
    deploySuccess.value = false
    deployLogs.value = []
    taskLogSetRef.value = new Set()
    if (taskTimeoutRef.value) {
      clearTimeout(taskTimeoutRef.value)
      taskTimeoutRef.value = null
    }
    stopDeployTaskStream()

    const deployData = {
      projectId: String(projectId),
      projectName,
      compose: yamlContent,
      env: {},
      dotenv: String(dotenvContent || ''),
      config: []
    }
    const secretsPayload = {}
    secretParams.value.forEach((p) => {
      const k = String(p?.key || '').trim()
      if (!k) return
      const v = String(secretValues[k] ?? '')
      if (v.trim() === '') return
      secretsPayload[k] = v
    })
    deployData.secrets = secretsPayload

    const res = await api.appstore.deployProject(deployData)
    const responseData = res.data || res
    const taskId = responseData.taskId
    if (!taskId) throw new Error('未获取到任务ID')
    lastDeployTaskId.value = String(taskId)
    localStorage.setItem('appstore:lastDeployTaskId', String(taskId))

    ElMessage.success('部署任务已提交，正在执行...')
    const token = localStorage.getItem('token') || ''
    const url = `/api/appstore/tasks/${taskId}/events?token=${encodeURIComponent(token)}`
    taskTimeoutRef.value = setTimeout(() => {
      pushDeployLine({ type: 'warning', message: '日志连接超时，请稍后在容器列表查看状态', time: new Date().toISOString() })
      stopDeployTaskStream()
      deploying.value = false
    }, 600000)
    startDeployTaskStream(url, { reset: true })
  } catch (error) {
    console.error(error)
    deploying.value = false
    stopDeployTaskStream()
    if (taskTimeoutRef.value) {
      clearTimeout(taskTimeoutRef.value)
      taskTimeoutRef.value = null
    }
    ElMessage.error('部署失败: ' + (error.response?.data?.error || error.message))
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('advanced-mode-change', syncAdvancedMode)
  if (advancedComposeEditor.value) advancedComposeEditor.value.dispose()
  if (advancedEnvEditor.value) advancedEnvEditor.value.dispose()
})

onMounted(() => {
  window.addEventListener('advanced-mode-change', syncAdvancedMode)
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
  padding: 12px 16px;
  background-color: var(--clay-bg);
  gap: 12px;
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0;
  background: var(--clay-card);
  padding: 14px 16px;
  border-radius: var(--radius-5xl);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
}

.filter-left, .filter-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 16px;
  font-weight: 900;
  color: var(--clay-ink);
}

.inherit-hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.2;
}

.content-wrapper {
  flex: 1;
  overflow: hidden;
  background: var(--clay-card);
  border-radius: var(--radius-5xl);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
  border: 1px solid var(--clay-border);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.scroll-container {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
  padding-bottom: 72px;
}

.app-info-header {
  display: flex;
  gap: 24px;
  padding: 18px;
  background: transparent;
  border-bottom: 1px solid rgba(55, 65, 81, 0.12);
}

/* App Icon */
.app-icon-wrapper {
  width: 48px;
  height: 48px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  background: var(--clay-highlight-top), var(--clay-gradient-primary);
  color: white;
  box-sizing: border-box;
  padding: 4px;
  margin: 2px;
  box-shadow: var(--shadow-clay-btn), var(--shadow-clay-inner);
  border: 1px solid rgba(255, 255, 255, 0.4);
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
  font-weight: 900;
  color: var(--clay-ink);
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

.global-env-bar {
  margin-bottom: 18px;
  padding: 14px 16px;
  border: 1px solid var(--el-color-success-light-5);
  border-radius: 8px;
  background: var(--tag-bg-success);
}

.dotenv-editor-container {
  width: 100%;
}

.dotenv-form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.dotenv-form-title {
  font-weight: 600;
  color: var(--el-color-success);
}

.dotenv-key {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  color: var(--el-text-color-primary);
}

.dotenv-table :deep(.el-table__header-wrapper th) {
  background: var(--tag-bg-success);
}

.global-env-textarea :deep(.el-textarea__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  line-height: 1.5;
}

.dotenv-hint {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
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
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  overflow: hidden;
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.global-env-collapse-item {
  border-color: var(--el-color-success-light-5);
}

.global-env-collapse-item :deep(.el-collapse-item__header) {
  background: var(--tag-bg-success);
}

.global-env-section {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--tag-bg-success);
}

.envfile-group {
  margin-bottom: 16px;
  border: 1px solid var(--clay-border);
  border-radius: var(--radius-5xl);
  overflow: hidden;
  background: var(--clay-card);
  box-shadow: var(--shadow-clay-card), var(--shadow-clay-inner);
}

.envfile-group:last-child {
  margin-bottom: 0;
}

.envfile-group-header {
  padding: 12px 16px;
  border-bottom: 1px solid rgba(55, 65, 81, 0.12);
}

.envfile-group :deep(.service-count-tag) {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mono-input :deep(.el-input__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

:deep(.el-collapse-item__header) {
  background-color: transparent;
  padding: 0 16px;
  height: 48px;
  border-bottom: 1px solid rgba(55, 65, 81, 0.12);
  font-weight: 900;
  color: var(--clay-ink);
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
  background: transparent;
}

.form-row-custom {
  margin-bottom: 0;
  padding: 12px 8px;
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

.logs-progress {
  padding: 12px 16px;
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.progress-text {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  word-break: break-word;
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
  color: var(--el-text-color-regular);
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
  color: var(--clay-ink);
}

:deep(.tutorial-content a) {
  color: var(--el-color-primary);
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
  background-color: var(--el-border-color-lighter);
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

.deploy-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;
}

.deploy-left {
  min-width: 0;
}

.deploy-right {
  min-width: 0;
}

.advanced-deploy-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;
}

.advanced-header {
  padding: 14px 16px;
  font-weight: 900;
  border-bottom: 1px solid rgba(55, 65, 81, 0.12);
  background: transparent;
}

.monaco-editor-wrapper {
  height: 560px;
}

.mono-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

@media (max-width: 1100px) {
  .deploy-grid {
    grid-template-columns: 1fr;
  }
  .advanced-deploy-grid {
    grid-template-columns: 1fr;
  }
}
</style>
