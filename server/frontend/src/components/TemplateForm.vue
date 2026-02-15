<template>
  <el-form :model="form" label-width="100px" class="template-form">
    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="项目名称" required>
          <el-input v-model="form.name" placeholder="请输入项目名称" />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="项目分类" required>
          <el-select v-model="form.category" style="width: 100%" placeholder="请选择分类">
            <el-option label="其他" value="other" />
            <el-option label="影音" value="entertainment" />
            <el-option label="图像" value="photos" />
            <el-option label="文件" value="file" />
            <el-option label="个人" value="hobby" />
            <el-option label="协作" value="team" />
            <el-option label="知识库" value="knowledge" />
            <el-option label="游戏" value="game" />
            <el-option label="生产" value="productivity" />
            <el-option label="社交" value="social" />
            <el-option label="管理" value="platform" />
            <el-option label="网安" value="network" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="版本">
          <el-input v-model="form.version" placeholder="例如: latest, v1.0" />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="项目主页">
          <el-input v-model="form.website" placeholder="官方网站地址" />
        </el-form-item>
      </el-col>
    </el-row>

    <el-form-item label="启用状态">
       <el-switch v-model="form.enabled" active-text="启用" inactive-text="关闭" />
    </el-form-item>

    <el-form-item label="项目描述">
      <el-input v-model="form.description" type="textarea" :rows="3" placeholder="简要描述该项目的功能" />
    </el-form-item>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="项目Logo">
          <el-upload
            class="logo-uploader"
            action="/api/upload"
            :data="{ project: form.name, type: 'icon' }"
            :show-file-list="false"
            :on-success="handleLogoSuccess"
          >
            <img v-if="form.logo" :src="form.logo" class="logo">
            <el-icon v-else class="logo-uploader-icon"><Plus /></el-icon>
          </el-upload>
          <el-input v-model="form.logo" placeholder="可填写互联网 Logo URL (https://...)" style="margin-top:8px" />
          <div class="upload-tip">建议尺寸: 200x200px</div>
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="项目截图">
          <el-upload
            action="/api/upload"
            :data="{ project: form.name, type: 'screenshot', index: (form.screenshots.length + 1) }"
            list-type="picture-card"
            :file-list="screenshotFileList"
            :on-success="handleScreenshotSuccess"
            :on-remove="handleScreenshotRemove"
          >
            <el-icon><Plus /></el-icon>
          </el-upload>
          <div class="screenshot-url-adder">
            <el-input v-model="newScreenshotUrl" placeholder="可填写互联网 Screenshot URL (https://...)" />
            <el-button type="primary" size="small" @click="addScreenshotUrl" style="margin-left:8px">添加URL</el-button>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-divider content-position="left">部署配置</el-divider>

    <div class="global-env-bar">
      <el-form-item label="全局变量（.env）">
        <div class="dotenv-editor-container">
          <div class="dotenv-form">
            <div class="dotenv-form-header">
              <div class="dotenv-form-title">全局变量表单</div>
              <el-upload
                class="dotenv-uploader-inline"
                action="/api/upload"
                :show-file-list="false"
                :on-success="handleDotenvSuccess"
                accept=".env,.txt"
              >
                <el-button type="primary" size="small">从文件导入</el-button>
              </el-upload>
            </div>
            <div class="dotenv-form-desc">这里的变量会写入 .env，供整个 Compose 项目引用</div>
            <div v-if="globalEnvRows.length === 0" class="empty-schema">
              暂无全局变量，请点击“添加全局变量”或从文件导入。
            </div>
            <div v-else class="config-section">
              <div class="section-header">
                <span class="section-title">环境变量 (Env)</span>
                <span class="section-desc">Key=Value</span>
              </div>
              <div v-for="(row, idx) in globalEnvRows" :key="row.key || idx" class="config-row">
                <div class="config-col-type">
                  <el-select :model-value="'env'" size="small" disabled>
                    <el-option label="端口" value="port" />
                    <el-option label="路径" value="path" />
                    <el-option label="变量" value="env" />
                    <el-option label="硬件" value="hardware" />
                    <el-option label="其它" value="other" />
                  </el-select>
                </div>
                <div class="config-label">
                  <div class="config-var-name">{{ row.key }}</div>
                </div>
                <div class="config-value">
                  <el-input
                    :model-value="row.value"
                    placeholder="Value"
                    @update:model-value="(val) => handleSetGlobalEnvValue(row.key, val)"
                  />
                </div>
                <div class="config-meta">
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveGlobalEnv(row.key)" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </el-form-item>
    </div>

    <el-row :gutter="20" class="deploy-config-row">
      <el-col :span="24">
        <el-form-item label="Compose" required>
          <div class="compose-editor-container">
            <el-upload
              class="compose-uploader-btn"
              action="/api/upload"
              :show-file-list="false"
              :on-success="handleComposeSuccess"
              accept=".yml,.yaml"
            >
              <el-button type="primary" size="small">从文件导入</el-button>
            </el-upload>
            <el-input
              type="textarea"
              v-model="form.compose"
              :rows="15"
              placeholder="在此粘贴 Docker Compose 内容"
              class="code-editor"
              @blur="handleParseVariables"
            />
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-divider content-position="left">配置标准</el-divider>

    <el-form-item label="参数配置">
      <div class="schema-editor">
        <div class="schema-actions">
          <el-button type="primary" size="small" @click="handleParseVariables">
            从 Compose 解析变量
          </el-button>
          <el-button type="success" size="small" @click="handleAddGlobalEnv">
            添加全局变量
          </el-button>
          <el-button size="small" @click="handleAddVariable">
            添加自定义参数
          </el-button>
          <el-button size="small" type="danger" @click="handleClearSchema">
            清空所有参数
          </el-button>
        </div>

        <div v-if="parseReport.errors.length || parseReport.warnings.length" class="parse-report">
          <el-alert
            v-if="parseReport.errors.length"
            type="error"
            :closable="false"
            show-icon
            title="解析错误"
            class="parse-report-alert"
          >
            <template #default>
              <div class="parse-report-lines">
                <div v-for="(msg, idx) in parseReport.errors" :key="`e-${idx}`">{{ msg }}</div>
              </div>
            </template>
          </el-alert>

          <el-alert
            v-if="parseReport.warnings.length"
            type="warning"
            :closable="false"
            show-icon
            title="解析告警"
            class="parse-report-alert"
          >
            <template #default>
              <div class="parse-report-lines">
                <div v-for="(msg, idx) in parseReport.warnings" :key="`w-${idx}`">{{ msg }}</div>
              </div>
            </template>
          </el-alert>
        </div>
        
        <div v-if="form.schema.length === 0" class="empty-schema">
          暂无配置参数，请点击"从 Compose 解析变量"或手动添加。
        </div>

        <el-collapse v-model="activeServices" class="schema-collapse">
          <el-collapse-item 
            v-for="(group, serviceName) in groupedSchemaWithoutGlobal" 
            :key="serviceName" 
            :title="`服务: ${serviceName}`" 
            :name="serviceName"
          >
            <!-- Ports -->
            <div v-if="group.ports.length" class="config-section">
              <div class="section-header">
                <span class="section-title">端口映射 (Ports)</span>
                <span class="section-desc">Host Port:Container Port</span>
              </div>
              <div v-for="(item, idx) in group.ports" :key="item._id" class="config-row">
                <div class="config-col-type">
                   <el-select v-model="item.paramType" size="small">
                      <el-option label="端口" value="port" />
                      <el-option label="路径" value="path" />
                      <el-option label="变量" value="env" />
                      <el-option label="硬件" value="hardware" />
                      <el-option label="其它" value="other" />
                   </el-select>
                </div>
                <div class="config-label">
                  <el-input v-model="item.label" placeholder="标签(Label)" size="small" />
                  <el-input v-model="item.description" placeholder="备注(说明)" size="small" />
                  <div class="config-var-name">{{ item.name }}</div>
                </div>
                <div class="config-value">
                  <el-input v-model="item.default" placeholder="8080:80" />
                </div>
                <div class="config-meta">
                  <el-select v-model="item.category" size="small" style="width: 100px">
                    <el-option label="基础" value="basic" />
                    <el-option label="高级" value="advanced" />
                  </el-select>
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveVariable(item)" />
                </div>
              </div>
            </div>

            <!-- Volumes -->
            <div v-if="group.volumes.length" class="config-section">
              <div class="section-header">
                <span class="section-title">存储挂载 (Path)</span>
                <span class="section-desc">Host Path</span>
              </div>
              <div v-for="(item, idx) in group.volumes" :key="item._id" class="config-row">
                <div class="config-col-type">
                   <el-select v-model="item.paramType" size="small">
                      <el-option label="端口" value="port" />
                      <el-option label="路径" value="path" />
                      <el-option label="变量" value="env" />
                      <el-option label="硬件" value="hardware" />
                      <el-option label="其它" value="other" />
                   </el-select>
                </div>
                <div class="config-label">
                  <el-input v-model="item.label" placeholder="标签(Label)" size="small" />
                  <el-input v-model="item.description" placeholder="备注(说明)" size="small" />
                  <div class="config-var-name">{{ item.name }}</div>
                </div>
                <div class="config-value">
                  <el-input v-model="item.default" placeholder="./data" />
                </div>
                <div class="config-meta">
                  <el-select v-model="item.category" size="small" style="width: 100px">
                    <el-option label="基础" value="basic" />
                    <el-option label="高级" value="advanced" />
                  </el-select>
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveVariable(item)" />
                </div>
              </div>
            </div>

            <!-- Environment -->
            <div v-if="group.env.length" class="config-section">
              <div class="section-header">
                <span class="section-title">环境变量 (Env)</span>
                <span class="section-desc">Key=Value</span>
              </div>
            <div v-for="(item, idx) in group.env" :key="item._id" class="config-row">
              <div class="config-col-type">
                   <el-select v-model="item.paramType" size="small">
                      <el-option label="端口" value="port" />
                      <el-option label="路径" value="path" />
                      <el-option label="变量" value="env" />
                      <el-option label="硬件" value="hardware" />
                      <el-option label="其它" value="other" />
                   </el-select>
              </div>
              <div class="config-label">
                <el-input v-model="item.label" placeholder="标签(Label)" size="small" />
                <el-input v-model="item.description" placeholder="备注(说明)" size="small" />
                <div class="config-var-name">
                  {{ item.name }}
                  <el-tag
                    v-if="isDotenvDefined(item.name)"
                    size="small"
                    type="success"
                    effect="plain"
                    class="dotenv-tag"
                  >已在 .env 定义</el-tag>
                  <el-tag
                    v-else
                    size="small"
                    type="warning"
                    effect="plain"
                    class="dotenv-tag"
                  >未定义</el-tag>
                </div>
              </div>
              <div class="config-value">
                <el-input v-model="item.default" placeholder="Value" />
              </div>
                <div class="config-meta">
                  <el-select v-model="item.category" size="small" style="width: 100px">
                    <el-option label="基础" value="basic" />
                    <el-option label="高级" value="advanced" />
                  </el-select>
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveVariable(item)" />
                </div>
              </div>
            </div>

            <!-- Hardware -->
            <div v-if="group.hardware.length" class="config-section">
              <div class="section-header">
                <span class="section-title">硬件配置 (Hardware)</span>
              </div>
              <div v-for="(item, idx) in group.hardware" :key="item._id" class="config-row">
                <div class="config-col-type">
                   <el-select v-model="item.paramType" size="small">
                      <el-option label="端口" value="port" />
                      <el-option label="路径" value="path" />
                      <el-option label="变量" value="env" />
                      <el-option label="硬件" value="hardware" />
                      <el-option label="其它" value="other" />
                   </el-select>
                </div>
                <div class="config-label">
                  <el-input v-model="item.label" placeholder="标签(Label)" size="small" />
                  <el-input v-model="item.description" placeholder="备注(说明)" size="small" />
                  <div class="config-var-name">{{ item.name }}</div>
                </div>
                <div class="config-value">
                  <el-input v-model="item.default" />
                </div>
                <div class="config-meta">
                  <el-select v-model="item.category" size="small" style="width: 100px">
                    <el-option label="基础" value="basic" />
                    <el-option label="高级" value="advanced" />
                  </el-select>
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveVariable(item)" />
                </div>
              </div>
            </div>

            <!-- Other -->
            <div v-if="group.other.length" class="config-section">
              <div class="section-header">
                <span class="section-title">其他配置 (Other)</span>
              </div>
              <div v-for="(item, idx) in group.other" :key="item._id" class="config-row">
                <div class="config-col-type">
                   <el-select v-model="item.paramType" size="small">
                      <el-option label="端口" value="port" />
                      <el-option label="路径" value="path" />
                      <el-option label="变量" value="env" />
                      <el-option label="硬件" value="hardware" />
                      <el-option label="其它" value="other" />
                   </el-select>
                </div>
                <div class="config-label">
                  <el-input v-model="item.label" placeholder="标签(Label)" size="small" />
                  <el-input v-model="item.description" placeholder="备注(说明)" size="small" />
                  <div class="config-var-name">{{ item.name }}</div>
                </div>
                <div class="config-value">
                  <el-input v-model="item.default" />
                </div>
                <div class="config-meta">
                  <el-select v-model="item.category" size="small" style="width: 100px">
                    <el-option label="基础" value="basic" />
                    <el-option label="高级" value="advanced" />
                  </el-select>
                  <el-button type="danger" link :icon="Delete" @click="handleRemoveVariable(item)" />
                </div>
              </div>
            </div>

          </el-collapse-item>
        </el-collapse>
      </div>
    </el-form-item>

    <el-divider content-position="left">使用说明</el-divider>

    <el-form-item label="使用教程">
      <el-tabs v-model="activeTab" type="border-card" class="tutorial-tabs">
        <el-tab-pane label="编辑 (Markdown)" name="edit">
          <el-input
            type="textarea"
            v-model="form.tutorial"
            :rows="10"
            placeholder="支持 Markdown 格式"
            class="code-editor"
          />
        </el-tab-pane>
        <el-tab-pane label="预览" name="preview">
          <div v-html="markdownHtml" class="markdown-body"></div>
        </el-tab-pane>
      </el-tabs>
    </el-form-item>

    <div class="form-footer">
      <el-button @click="handleReset">重置</el-button>
      <el-button type="primary" @click="handleSubmit">保存模板</el-button>
    </div>
  </el-form>
</template>

<script setup>
import { ref, computed, watch, defineProps, defineEmits } from 'vue'
import { Plus, Delete } from '@element-plus/icons-vue'
import { marked } from 'marked'
import { ElMessage } from 'element-plus'
import { parseDotenvText } from '../utils/composeTemplateParser'
import { templateApi } from '../api/template'

const props = defineProps({
  template: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['submit'])

const form = ref({
  name: '',
  category: '',
  description: '',
  version: '',
  website: '',
  logo: '',
  screenshots: [],
  dotenv: '',
  compose: '',
  tutorial: '',
  schema: [],
  enabled: true
})
const newScreenshotUrl = ref('')

const activeServices = ref([])
const parseReport = ref({ warnings: [], errors: [] })
const dotenvKeySetRef = ref(new Set())

// 定义 handleReset 为普通函数，确保提升
function handleReset() {
  form.value = {
    name: '',
    category: '',
    description: '',
    version: '',
    website: '',
    logo: '',
    screenshots: [],
    dotenv: '',
    compose: '',
    tutorial: '',
    schema: [],
    enabled: true
  }
  activeServices.value = []
  parseReport.value = { warnings: [], errors: [] }
  dotenvKeySetRef.value = new Set()
}

watch(() => props.template, (newVal) => {
  if (newVal) {
    form.value = { ...newVal }
    if (typeof form.value.dotenv !== 'string') form.value.dotenv = ''
    // 确保 screenshots 是数组
    if (!Array.isArray(form.value.screenshots)) {
        try {
            form.value.screenshots = JSON.parse(form.value.screenshots || '[]')
        } catch (e) {
            form.value.screenshots = []
        }
    }
    // 确保 schema 是数组
    if (!form.value.schema) {
      form.value.schema = []
    }
    
    // 初始化展开的服务
    const services = new Set(form.value.schema.map(item => item.serviceName).filter(Boolean))
    activeServices.value = Array.from(services)
    if (activeServices.value.length === 0) {
        // 如果没有 serviceName 信息（旧数据），默认展开 Global
        activeServices.value = ['Global']
    }
    
  } else {
    handleReset()
  }
}, { immediate: true, deep: true })

const activeTab = ref('edit')

const markdownHtml = computed(() => {
  return marked(form.value.tutorial || '')
})

const screenshotFileList = computed(() => {
  return form.value.screenshots.map((url, index) => ({
    name: `截图${index + 1}`,
    url: url
  }))
})

const groupedSchema = computed(() => {
  const groups = {}
  
  // 确保 schema 是数组
  const schema = form.value.schema || []
  
  schema.forEach(item => {
    // 兼容旧数据：如果没有 serviceName，尝试从变量名推断，或者归为 Global
    let service = item.serviceName
    if (!service) {
        // 尝试从 PORT_NGINX_80 这种格式推断
        const parts = item.name.split('_')
        if (parts.length >= 3 && (parts[0] === 'PORT' || parts[0] === 'VOL')) {
        }
        service = 'Global'
    }

    if (!groups[service]) {
      groups[service] = {
        ports: [],
        volumes: [],
        env: [],
        hardware: [],
        other: []
      }
    }
    
    // 给 item 加个临时 ID 方便 key 绑定（如果还没有）
    if (!item._id) item._id = Math.random().toString(36).substr(2, 9)
    // 兼容旧数据：初始化 paramType
    if (!item.paramType) {
        if (item.type === 'port') item.paramType = 'port'
        else if (item.type === 'path') item.paramType = 'path'
        else if (['string', 'password', 'number', 'boolean'].includes(item.type)) item.paramType = 'env'
        else item.paramType = 'other'
    }

    if (item.paramType === 'port') {
        groups[service].ports.push(item)
    } else if (item.paramType === 'path') {
        groups[service].volumes.push(item)
    } else if (item.paramType === 'env') {
        groups[service].env.push(item)
    } else if (item.paramType === 'hardware') {
        groups[service].hardware.push(item)
    } else {
        groups[service].other.push(item)
    }
  })
  
  return groups
})

const groupedSchemaWithoutGlobal = computed(() => {
  const groups = groupedSchema.value || {}
  const out = {}
  Object.entries(groups).forEach(([k, v]) => {
    if (k === 'Global') return
    out[k] = v
  })
  return out
})

const globalEnvRows = computed(() => {
  const { dotenv } = parseDotenvText(form.value.dotenv || '')
  const dotenvObj = dotenv || {}
  const dotenvKeys = Object.keys(dotenvObj)
  const inDotenvKeySet = new Set(dotenvKeys)

  const schemaKeySet = new Set()
  const schemaKeysInOrder = []

  ;(form.value.schema || []).forEach((item) => {
    const serviceName = String(item?.serviceName || 'Global')
    const paramType = String(inferSchemaParamType(item) || '')
    if (serviceName !== 'Global') return
    if (paramType !== 'env') return
    const k = String(item?.name || '').trim()
    if (!k) return
    if (schemaKeySet.has(k)) return
    schemaKeySet.add(k)
    schemaKeysInOrder.push(k)
  })

  const orderedKeys = [...dotenvKeys]
  schemaKeysInOrder.forEach((k) => {
    if (!inDotenvKeySet.has(k)) orderedKeys.push(k)
  })

  return orderedKeys.map((k) => ({
    key: k,
    value: Object.prototype.hasOwnProperty.call(dotenvObj, k) ? dotenvObj[k] : '',
    inDotenv: Object.prototype.hasOwnProperty.call(dotenvObj, k),
    inSchema: schemaKeySet.has(k)
  }))
})

const handleLogoSuccess = (response) => {
  form.value.logo = response.url
}

const handleScreenshotSuccess = (response) => {
  form.value.screenshots.push(response.url)
}

const handleScreenshotRemove = (file) => {
  const index = form.value.screenshots.indexOf(file.url)
  if (index !== -1) {
    form.value.screenshots.splice(index, 1)
  }
}

const addScreenshotUrl = () => {
  const url = (newScreenshotUrl.value || '').trim()
  if (!url) return
  if (!form.value.screenshots.includes(url)) {
    form.value.screenshots.push(url)
  }
  newScreenshotUrl.value = ''
}

const handleComposeSuccess = async (response) => {
  try {
    const res = await fetch(response.url)
    const text = await res.text()
    
    if (text.trim().toLowerCase().startsWith('<!doctype') || text.trim().toLowerCase().startsWith('<html')) {
      throw new Error('读取到的文件内容格式错误(HTML)，请检查文件路径或服务器配置')
    }
    
    form.value.compose = text
    ElMessage.success('Compose文件上传成功')
    
    handleParseVariables()
  } catch (error) {
    console.error('读取Compose文件失败:', error)
    ElMessage.error(error.message || '读取Compose文件失败')
    form.value.compose = ''
  }
}

const handleDotenvSuccess = async (response) => {
  try {
    const res = await fetch(response.url)
    const text = await res.text()

    if (text.trim().toLowerCase().startsWith('<!doctype') || text.trim().toLowerCase().startsWith('<html')) {
      throw new Error('读取到的文件内容格式错误(HTML)，请检查文件路径或服务器配置')
    }

    form.value.dotenv = text
    ElMessage.success('.env 文件上传成功')
    handleParseVariables()
  } catch (error) {
    console.error('读取 .env 文件失败:', error)
    ElMessage.error(error.message || '读取 .env 文件失败')
    form.value.dotenv = ''
  }
}

const buildUniqueDotenvKey = (baseKey) => {
  const { dotenv } = parseDotenvText(form.value.dotenv || '')
  const exists = new Set(Object.keys(dotenv || {}))
  const raw = String(baseKey || '').trim() || 'NEW_VAR'
  const safe = raw.replace(/[^A-Za-z0-9_]/g, '_').toUpperCase()
  if (!exists.has(safe)) return safe
  for (let i = 2; i < 10000; i++) {
    const k = `${safe}_${i}`
    if (!exists.has(k)) return k
  }
  return `${safe}_${Date.now()}`
}

const appendDotenvLines = (currentText, linesToAdd) => {
  const current = String(currentText || '')
  const lines = Array.isArray(linesToAdd) ? linesToAdd : []
  const content = current.replace(/\r\n/g, '\n')
  const out = content.trimEnd()
  const suffix = lines.filter(Boolean).join('\n')
  if (!suffix) return content
  if (!out) return suffix + '\n'
  return out + '\n' + suffix + '\n'
}

const handleAddGlobalEnv = () => {
  const key = buildUniqueDotenvKey('NEW_VAR')
  form.value.dotenv = appendDotenvLines(form.value.dotenv, [`${key}=`])
  ElMessage.success('已添加全局变量，请在上方全局变量表单编辑')
  handleParseVariables()
}

/**
 * inferSchemaParamType 兼容旧数据：从 schema item 推断 paramType
 */
function inferSchemaParamType(item) {
  const explicit = String(item?.paramType || '').trim()
  if (explicit) return explicit
  const t = String(item?.type || '').trim()
  if (t === 'port') return 'port'
  if (t === 'path') return 'path'
  if (['string', 'password', 'number', 'boolean'].includes(t)) return 'env'
  return 'other'
}

/**
 * formatDotenvValue 将值格式化为 .env 行里的 value（必要时加引号）
 */
function formatDotenvValue(value) {
  const raw = String(value ?? '')
  const needsQuote = /[\s#"'\r\n]/.test(raw)
  if (!needsQuote) return raw
  const escaped = raw.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  return `"${escaped}"`
}

/**
 * upsertDotenvKeyValue 将 key=value 写入 .env 文本（尽量只替换最后一次出现）
 */
function upsertDotenvKeyValue(dotenvText, key, value) {
  const k = String(key || '').trim()
  if (!k) return String(dotenvText || '')

  const lines = String(dotenvText || '').replace(/\r\n/g, '\n').split('\n')
  const keyRegex = new RegExp(`^\\s*(?:export\\s+)?${k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*=`)

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
  return appendDotenvLines(dotenvText, [newLine])
}

/**
 * removeDotenvKey 从 .env 文本中移除指定 key 的所有定义行
 */
function removeDotenvKey(dotenvText, key) {
  const k = String(key || '').trim()
  if (!k) return String(dotenvText || '')
  const lines = String(dotenvText || '').replace(/\r\n/g, '\n').split('\n')
  const keyRegex = new RegExp(`^\\s*(?:export\\s+)?${k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*(?:=|$)`)
  const out = lines.filter((raw) => {
    const trimmed = String(raw || '').trim()
    if (!trimmed || trimmed.startsWith('#')) return true
    return !keyRegex.test(raw)
  })
  return out.join('\n').replace(/\n*$/, '\n')
}

/**
 * refreshDotenvKeySet 重新计算 .env key 集合（用于 UI 里的 “已定义/未定义” 标记）
 */
function refreshDotenvKeySet() {
  const { dotenv } = parseDotenvText(form.value.dotenv || '')
  dotenvKeySetRef.value = new Set(Object.keys(dotenv || {}))
}

/**
 * handleSetGlobalEnvValue 在表单里修改全局变量时，同步写回 .env 文本
 */
function handleSetGlobalEnvValue(key, value) {
  form.value.dotenv = upsertDotenvKeyValue(form.value.dotenv, key, value)
  refreshDotenvKeySet()
}

/**
 * handleRemoveGlobalEnv 删除全局变量（从 .env 中移除）
 */
function handleRemoveGlobalEnv(key) {
  form.value.dotenv = removeDotenvKey(form.value.dotenv, key)
  refreshDotenvKeySet()
}

const isDotenvDefined = (key) => {
  return dotenvKeySetRef.value && dotenvKeySetRef.value.has(String(key || ''))
}

const handleParseVariables = async () => {
  const composeContent = form.value.compose || ''

  const { dotenv, warnings: dotenvWarnings, errors: dotenvErrors } = parseDotenvText(form.value.dotenv || '')
  dotenvKeySetRef.value = new Set(Object.keys(dotenv || {}))

  let parsedRes = null
  try {
    parsedRes = await templateApi.parseVars(composeContent)
  } catch (e) {
    parseReport.value = { warnings: [], errors: ['[compose] 解析失败'] }
    return
  }

  const schema = parsedRes?.schema || []
  const warnings = parsedRes?.warnings || []
  const errors = parsedRes?.errors || []
  const refs = parsedRes?.refs || []

  const missingRefs = (refs || [])
    .filter(r => r && !r.hasDefault)
    .map(r => r.name)
    .filter(name => name && !Object.prototype.hasOwnProperty.call(dotenv, name))

  const mergedErrors = [
    ...(dotenvErrors || []).map(m => `[.env] ${m}`),
    ...(errors || []).map(m => `[compose] ${m}`)
  ]
  const mergedWarnings = [
    ...(dotenvWarnings || []).map(m => `[.env] ${m}`),
    ...(warnings || []).map(m => `[compose] ${m}`),
    ...Array.from(new Set(missingRefs)).map(n => `[变量引用] 发现未定义的 ${n}（未在 .env 中定义，且 compose 未提供 default）`)
  ]

  parseReport.value = { warnings: mergedWarnings, errors: mergedErrors }

  if (mergedErrors.length > 0) {
    ElMessage.error(mergedErrors[0])
  } else if (mergedWarnings.length > 0) {
    ElMessage.warning(mergedWarnings[0])
  }

  const servicesFound = new Set()
  let newCount = 0

  schema.forEach((item) => {
    const serviceName = item.serviceName || 'Global'
    const paramType = item.paramType || 'env'
    servicesFound.add(serviceName)

    if (schemaExists(item.name, serviceName, paramType)) return

    form.value.schema.push({
      ...item,
      _id: Math.random().toString(36).substr(2, 9)
    })
    newCount++
  })

  if (newCount > 0) {
    const nextActive = new Set(activeServices.value || [])
    servicesFound.forEach(s => nextActive.add(s))
    activeServices.value = Array.from(nextActive)
    ElMessage.success(`自动解析出 ${newCount} 个配置项`)
  } else {
    ElMessage.info('未解析出新的参数')
  }
}

// Updated schemaExists to check service and type to avoid collisions if same port used in diff services (though unlikely for host port)
const schemaExists = (name, serviceName, paramType) => {
  return form.value.schema.some(item => 
    item.name === name && 
    item.serviceName === serviceName && 
    item.paramType === paramType
  )
}

const parseRegexVariables = (content) => {
  const variables = new Set()
  // 仅匹配 ${VAR}（兼容 ${VAR:-default} / ${VAR-default} 等形式），不处理 $VAR
  const regex = /\$\{([A-Za-z_][A-Za-z0-9_]*)[^}]*\}/g
  let match
  while ((match = regex.exec(content)) !== null) {
    variables.add(match[1])
  }
  
  let newCount = 0
  variables.forEach(varName => {
    if (!schemaExists(varName, 'Global', 'env')) {
      form.value.schema.push({
        name: varName,
        label: varName,
        default: '',
        category: 'basic',
        type: 'string',
        paramType: 'env',
        serviceName: 'Global', // 正则解析无法确定服务，归为全局
        description: ''
      })
      newCount++
    }
  })
  
  if (newCount > 0) {
    activeServices.value.push('Global')
    ElMessage.success(`正则解析出 ${newCount} 个新变量`)
  }
}

const handleAddVariable = () => {
  form.value.schema.push({
    name: '',
    label: '',
    default: '',
    category: 'basic',
    type: 'string',
    paramType: 'env',
    serviceName: 'Global',
    description: ''
  })
  if (!activeServices.value.includes('Global')) {
      activeServices.value.push('Global')
  }
}

const handleRemoveVariable = (item) => {
  const index = form.value.schema.indexOf(item)
  if (index !== -1) {
    form.value.schema.splice(index, 1)
  }
}

const handleClearSchema = () => {
    form.value.schema = []
}

const handleSubmit = async () => {
  if (!form.value.name || !form.value.category || !form.value.compose) {
    ElMessage.warning('请填写必要信息（名称、分类、Compose文件）')
    return
  }
  emit('submit', form.value)
}
</script>

<style scoped>
.template-form {
  padding: 0 10px;
}

.logo-uploader {
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  width: 120px;
  height: 120px;
  display: flex;
  justify-content: center;
  align-items: center;
  transition: var(--el-transition-duration-fast);
}

.logo-uploader:hover {
  border-color: #409eff;
}

.logo-uploader-icon {
  font-size: 28px;
  color: #8c939d;
  width: 120px;
  height: 120px;
  text-align: center;
  line-height: 120px;
}

.logo {
  width: 120px;
  height: 120px;
  object-fit: contain;
  display: block;
}

.upload-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 5px;
  line-height: 1.2;
}
.screenshot-url-adder { margin-top: 8px; display: flex; align-items: center; }

.deploy-config-row {
  width: 100%;
}

.global-env-bar {
  border: 1px solid rgba(103, 194, 58, 0.35);
  background: rgba(103, 194, 58, 0.08);
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 14px;
}

.dotenv-form {
  margin-bottom: 10px;
}

.dotenv-form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.dotenv-uploader-inline {
  display: inline-flex;
}

.dotenv-form-title {
  font-weight: 600;
  color: #2f7a1c;
}

.dotenv-form-desc {
  font-size: 12px;
  color: rgba(103, 194, 58, 0.9);
}

.dotenv-key {
  font-family: Consolas, Monaco, monospace;
  color: #303133;
}

.dotenv-missing-tag {
  margin-left: 8px;
  transform: translateY(-1px);
}

.dotenv-table :deep(.el-table__header-wrapper th) {
  background: rgba(103, 194, 58, 0.06);
}

.global-env-textarea :deep(.el-textarea__inner) {
  font-family: Consolas, Monaco, monospace;
  border-color: rgba(103, 194, 58, 0.55);
  background: rgba(103, 194, 58, 0.04);
}

.dotenv-hint {
  margin-top: 6px;
  font-size: 12px;
  color: #67c23a;
  line-height: 1.4;
}

.dotenv-editor-container {
  width: 100%;
}

.dotenv-raw-editor {
  position: relative;
  width: 100%;
}

.dotenv-uploader-btn {
  position: absolute;
  right: 10px;
  top: 5px;
  z-index: 10;
}

.compose-editor-container {
  position: relative;
  width: 100%;
}

.compose-uploader-btn {
  position: absolute;
  right: 10px;
  top: 5px;
  z-index: 10;
}

.tutorial-tabs {
  width: 100%;
}

.code-editor :deep(.el-textarea__inner) {
  font-family: Consolas, Monaco, monospace;
}

.schema-editor {
    border: 1px solid #dcdfe6;
    border-radius: 4px;
    padding: 10px;
    background-color: #f5f7fa;
}

.schema-actions {
    margin-bottom: 10px;
    display: flex;
    gap: 10px;
}

.parse-report {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 10px;
}

.parse-report-lines {
    font-size: 12px;
    line-height: 1.5;
    word-break: break-word;
}

.dotenv-tag {
    margin-left: 8px;
    transform: translateY(-1px);
}

.empty-schema {
    text-align: center;
    color: #909399;
    padding: 20px;
}

.schema-collapse {
    border: none;
    background: transparent;
}

.schema-collapse :deep(.el-collapse-item__header) {
    background-color: transparent;
    font-weight: bold;
    font-size: 14px;
}

.schema-collapse :deep(.el-collapse-item__content) {
    padding-bottom: 10px;
}

.config-section {
    background: white;
    padding: 10px;
    border-radius: 4px;
    margin-bottom: 10px;
    border: 1px solid #ebeef5;
}

.section-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
    border-bottom: 1px dashed #ebeef5;
    padding-bottom: 5px;
}

.section-title {
    font-weight: bold;
    color: #606266;
    font-size: 13px;
}

.section-desc {
    color: #909399;
    font-size: 12px;
}

.config-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    margin-bottom: 12px;
    padding-bottom: 12px;
    border-bottom: 1px solid #f0f2f5;
}

.config-row:last-child {
    margin-bottom: 0;
    border-bottom: none;
    padding-bottom: 0;
}

.config-col-type {
    flex: 0 0 90px;
    padding-top: 2px;
}

.config-label {
    flex: 0 0 220px;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.config-var-name {
    font-size: 12px;
    color: #909399;
    font-family: monospace;
}

.config-value {
    flex: 1;
}

.config-meta {
    flex: 0 0 140px;
    display: flex;
    align-items: flex-start;
    gap: 5px;
    padding-top: 2px;
}
</style>
