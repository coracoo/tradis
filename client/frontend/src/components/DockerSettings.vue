<template>
  <el-dialog
    v-model="dialogVisible"
    title="Docker 设置"
    width="800px"
    class="app-dialog"
  >
    <el-tabs v-model="activeTab" class="settings-tabs">
      <el-tab-pane label="注册表" name="registry">
        <el-form :model="registryForm">
          <div class="registry-header">
            <el-button type="primary" @click="addRegistry">新建注册表</el-button>
          </div>
          
          <el-table :data="registryList" style="width: 100%">
            <el-table-column prop="name" label="注册表名称" width="180">
              <template #default="scope">
                <el-input 
                  v-model="scope.row.name" 
                  placeholder="请输入注册表名称（必填）"
                  :disabled="scope.row.key === 'docker.io'"
                  @blur="validateRegistry(scope.row)"
                />
              </template>
            </el-table-column>
            <el-table-column prop="url" label="注册表地址">
              <template #default="scope">
                <el-input 
                  v-model="scope.row.url" 
                  placeholder="请输入注册表地址（必填）"
                  :disabled="scope.row.key === 'docker.io'"
                  @blur="validateRegistry(scope.row)"
                />
              </template>
            </el-table-column>
            <el-table-column prop="username" label="用户名">
              <template #default="scope">
                <el-input v-model="scope.row.username" placeholder="可选" />
              </template>
            </el-table-column>
            <el-table-column prop="password" label="密码">
              <template #default="scope">
                <el-input v-model="scope.row.password" type="password" placeholder="可选" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150">
              <template #default="scope">
                <el-button 
                  v-if="scope.row.key !== 'docker.io'"
                  link 
                  type="danger" 
                  @click="removeRegistry(scope.row.key)"
                >
                  移除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="代理设置" name="proxy">
        <el-form :model="proxyForm" label-width="120px">
          <!-- 添加启用/禁用代理的开关 -->
          <el-form-item label="启用代理">
            <el-switch v-model="proxyForm.enabled" />
          </el-form-item>
          
          <!-- 只有当启用代理时才显示代理设置 -->
          <template v-if="proxyForm.enabled">
            <el-form-item label="HTTP 代理">
              <el-input v-model="proxyForm.http" placeholder="例如: http://192.168.0.129:7890"></el-input>
            </el-form-item>
            <el-form-item label="HTTPS 代理">
              <el-input v-model="proxyForm.https" placeholder="例如: http://192.168.0.129:7890"></el-input>
            </el-form-item>
            <el-form-item label="无需代理">
              <el-input v-model="proxyForm.no" placeholder="例如: localhost,127.0.0.1"></el-input>
            </el-form-item>
            <el-form-item label="历史配置" v-if="proxyHistory.length">
              <el-select v-model="selectedHistory" placeholder="选择历史代理配置填充" style="width: 100%">
                <el-option
                  v-for="item in proxyHistory"
                  :key="item.id"
                  :label="formatHistoryLabel(item)"
                  :value="item"
                />
              </el-select>
            </el-form-item>
          </template>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="镜像加速" name="mirrors">
        <el-form :model="mirrorForm" label-width="120px">
          <el-form-item label="镜像加速器">
            <el-select
              v-model="mirrorForm.mirrors"
              multiple
              filterable
              allow-create
              default-first-option
              style="width: 100%"
              placeholder="请选择或输入镜像加速器地址"
            >
              <el-option
                v-for="mirror in defaultMirrors"
                :key="mirror.value"
                :label="mirror.label"
                :value="mirror.value"
              />
            </el-select>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <span class="dialog-footer">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveSettings">保存</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxy, getProxyHistory, updateProxy } from '../api/images'
import { getRegistries, updateRegistries } from '../api/image_registry'  // 更新导入路径  // 添加新的导入

// 删除 defineEmits 和 defineProps 的导入，因为它们是编译器宏
const props = defineProps({
  modelValue: Boolean
})

const emit = defineEmits(['update:modelValue'])

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const activeTab = ref('registry')

// 添加表单数据的定义
const proxyForm = ref({
  enabled: false,
  http: '',
  https: '',
  no: ''
})

// 历史记录
const proxyHistory = ref([])
const selectedHistory = ref(null)

const mirrorForm = ref({
  mirrors: []
})

const defaultMirrors = [
  { label: '1ms', value: 'https://docker.1ms.run' },
  { label: '1panel', value: 'https://docker.1panel.live' },
  { label: 'agsv', value: 'https://docker.agsv.top' },
  { label: 'CNIX Internal', value: 'https://docker.m.ixdev.cn' },
  { label: '耗子面板', value: 'https://hub.rat.dev' },
  { label: '轩辕镜像', value: 'https://docker.xuanyuan.me' },
  { label: 'DockerProxy', value: 'https://dockerproxy.net' },
  { label: 'DaoCloud', value: 'https://docker.m.daocloud.io' }
]

const registryForm = ref({
  registries: {
    'docker.io': {
      name: 'Docker Hub 官方注册表',
      url: 'docker.io',
      username: '',
      password: ''
    }
  }
})

// 添加 registryList 计算属性
const registryList = computed(() => {
  return Object.entries(registryForm.value.registries).map(([key, value]) => ({
    key,
    ...value
  }))
})

// 添加验证方法
const validateRegistry = (registry) => {
  if (!registry.name.trim()) {
    ElMessage.warning('注册表名称不能为空')
    return false
  }
  if (!registry.url.trim()) {
    ElMessage.warning('注册表地址不能为空')
    return false
  }
  return true
}

// 修改保存配置的方法
const saveSettings = async () => {
  try {
    // 验证所有注册表配置
    let hasError = false
    for (const registry of registryList.value) {
      if (!validateRegistry(registry)) {
        hasError = true
        break
      }
    }

    if (hasError) {
      return
    }

    // 保存注册表配置
    await updateRegistries(
      Object.fromEntries(
        registryList.value.map(registry => [
          registry.key,
          {
            name: registry.name.trim(),
            url: registry.url.trim(),
            username: registry.username || '',
            password: registry.password || ''
          }
        ])
      )
    )

    // 保存代理和镜像加速配置
    const proxyConfig = {
      enabled: proxyForm.value.enabled, // 添加 enabled 属性
      'HTTP Proxy': proxyForm.value.enabled ? proxyForm.value.http : '',
      'HTTPS Proxy': proxyForm.value.enabled ? proxyForm.value.https : '',
      'No Proxy': proxyForm.value.enabled ? proxyForm.value.no : '',
      'registry-mirrors': mirrorForm.value.mirrors || []
    }
    
    await updateProxy(proxyConfig)
    ElMessage.success('配置已更新，请重启 Docker 服务以生效')
    dialogVisible.value = false
  } catch (error) {
    console.error('保存配置失败:', error)
    if (error.response?.data?.error) {
      ElMessage.error(error.response.data.error)
    } else {
      ElMessage.error('保存配置失败')
    }
  }
}

// 修改添加仓库的方法
const addRegistry = () => {
  const newRegistry = {
    name: '',
    url: '',
    username: '',
    password: ''
  }
  // 生成唯一键名
  const key = 'registry-' + Date.now()
  registryForm.value.registries[key] = newRegistry
}

// 修改删除仓库的方法
const removeRegistry = (key) => {
  // 不允许删除 docker.io
  if (key === 'docker.io') {
    ElMessage.warning('不能删除默认仓库')
    return
  }
  delete registryForm.value.registries[key]
}

// 修改加载配置的方法
const loadSettings = async () => {
  try {
    // 加载代理配置
    const proxyConfig = await getProxy()
    proxyForm.value = {
      enabled: proxyConfig.enabled || false, // 使用后端返回的 enabled 属性
      http: proxyConfig['HTTP Proxy'] || '',
      https: proxyConfig['HTTPS Proxy'] || '',
      no: proxyConfig['No Proxy'] || ''
    }
    
    mirrorForm.value.mirrors = proxyConfig['registry-mirrors'] || []
    
    // 加载历史记录
    try {
      const history = await getProxyHistory()
      proxyHistory.value = Array.isArray(history) ? history : []
    } catch (e) {
      proxyHistory.value = []
    }
    
    // 加载注册表配置
    const registriesData = await getRegistries()
    const registries = registriesData || {}
    
    // 确保默认仓库存在
    if (!registries['docker.io']) {
      registries['docker.io'] = {
        name: 'Docker Hub',
        url: 'docker.io',
        username: '',
        password: ''
      }
    }
    registryForm.value.registries = registries
  } catch (error) {
    console.error('加载配置失败:', error)
    ElMessage.error('加载配置失败')
  }
}
const editRegistry = (registry) => {
  // 实现编辑逻辑
}

// 历史选择应用
watch(selectedHistory, (val) => {
  if (val) {
    proxyForm.value.enabled = true
    proxyForm.value.http = val.http_proxy || ''
    proxyForm.value.https = val.https_proxy || ''
    proxyForm.value.no = val.no_proxy || ''
  }
})

const formatHistoryLabel = (item) => {
  const hp = item.http_proxy || ''
  const sp = item.https_proxy || ''
  const np = item.no_proxy || ''
  const ct = item.change_type || ''
  return `${ct} | HTTP: ${hp} | HTTPS: ${sp} | NO_PROXY: ${np}`
}

// 监听对话框打开
watch(() => dialogVisible.value, (val) => {
  if (val) {
    loadSettings()
  }
})</script>
<style scoped>
.registry-header {
  margin-bottom: 16px;
  padding-left: 0;
}
</style>
