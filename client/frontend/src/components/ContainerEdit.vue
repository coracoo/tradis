<template>
  <el-dialog
    v-model="visible"
    title="编辑容器"
    width="800px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    top="5vh"
  >
    <el-tabs v-model="activeTab" type="border-card" class="edit-tabs">
      <!-- 基础设置 -->
      <el-tab-pane label="基础信息" name="basic">
        <el-form ref="basicFormRef" :model="form" :rules="rules" label-width="100px">
          <el-form-item label="容器名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入新容器名称" />
            <div class="form-tip">
              <el-icon><InfoFilled /></el-icon>
              修改容器名称将重新创建容器，旧容器将被删除
            </div>
          </el-form-item>
          <el-form-item label="镜像" prop="image">
             <el-input v-model="form.image" disabled />
          </el-form-item>
          <el-form-item label="重启策略">
            <el-select v-model="form.restart_policy">
              <el-option label="不重启 (no)" value="no" />
              <el-option label="退出时重启 (on-failure)" value="on-failure" />
              <el-option label="总是重启 (always)" value="always" />
              <el-option label="除非手动停止 (unless-stopped)" value="unless-stopped" />
            </el-select>
          </el-form-item>
          <el-form-item label="网络模式">
            <el-select v-model="form.network_mode">
              <el-option label="Bridge (默认)" value="bridge" />
              <el-option label="Host" value="host" />
              <el-option label="None" value="none" />
              <el-option label="Container" value="container" />
            </el-select>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 端口映射 -->
      <el-tab-pane label="端口映射" name="ports">
        <div class="port-mappings">
          <div v-for="(item, index) in form.ports" :key="index" class="mapping-row">
            <el-input v-model="item.host" placeholder="宿主机端口 (如 8080)" class="port-input" />
            <span class="separator">:</span>
            <el-input v-model="item.container" placeholder="容器端口 (如 80)" class="port-input" />
            <el-select v-model="item.protocol" style="width: 40">
              <el-option label="TCP" value="tcp" />
              <el-option label="UDP" value="udp" />
            </el-select>
            <el-button type="danger" circle @click="removePort(index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button type="primary" plain class="add-btn" @click="addPort">
            <el-icon><Plus /></el-icon> 添加端口映射
          </el-button>
        </div>
      </el-tab-pane>

      <!-- 存储卷 -->
      <el-tab-pane label="存储卷" name="volumes">
        <div class="volume-mappings">
          <div v-for="(item, index) in form.volumes" :key="index" class="mapping-row">
            <el-input v-model="item.host" placeholder="宿主机路径 (如 /data)" />
            <span class="separator">→</span>
            <el-input v-model="item.container" placeholder="容器路径 (如 /app/data)" />
            <el-select v-model="item.mode" style="width: 120px">
              <el-option label="读写 (rw)" value="rw" />
              <el-option label="只读 (ro)" value="ro" />
            </el-select>
            <el-button type="danger" circle @click="removeVolume(index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button type="primary" plain class="add-btn" @click="addVolume">
            <el-icon><Plus /></el-icon> 添加存储卷
          </el-button>
        </div>
      </el-tab-pane>

      <!-- 环境变量 -->
      <el-tab-pane label="环境变量" name="env">
        <div class="env-list">
          <div v-for="(item, index) in form.env" :key="index" class="mapping-row">
            <el-input v-model="item.key" placeholder="变量名 (KEY)" />
            <span class="separator">=</span>
            <el-input v-model="item.value" placeholder="变量值 (VALUE)" />
            <el-button type="danger" circle @click="removeEnv(index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button type="primary" plain class="add-btn" @click="addEnv">
            <el-icon><Plus /></el-icon> 添加环境变量
          </el-button>
        </div>
      </el-tab-pane>

      <!-- 命令设置 -->
      <el-tab-pane label="命令设置" name="command">
        <el-form label-width="100px">
          <el-form-item label="Entrypoint">
             <el-input v-model="form.entrypoint" placeholder="覆盖镜像默认 Entrypoint" />
             <div class="form-tip">
               <span v-if="defaultConfig.entrypoint">默认: {{ defaultConfig.entrypoint }} <el-button link type="primary" size="small" @click="form.entrypoint = defaultConfig.entrypoint">填入</el-button></span>
               <span v-else>无默认 Entrypoint</span>
             </div>
          </el-form-item>
          <el-form-item label="Command">
             <el-input v-model="form.command" placeholder="覆盖镜像默认 CMD" />
             <div class="form-tip">
               <span v-if="defaultConfig.cmd">默认: {{ defaultConfig.cmd }} <el-button link type="primary" size="small" @click="form.command = defaultConfig.cmd">填入</el-button></span>
               <span v-else>无默认 CMD</span>
             </div>
          </el-form-item>
          <el-form-item label="常用命令">
             <div class="quick-cmds">
                <el-tag class="cursor-pointer" @click="setCmd('/bin/sh')">/bin/sh</el-tag>
                <el-tag class="cursor-pointer" @click="setCmd('/bin/bash')">/bin/bash</el-tag>
                <el-tag class="cursor-pointer" @click="setCmd('tail -f /dev/null')">tail -f /dev/null</el-tag>
             </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 高级设置 -->
      <el-tab-pane label="高级设置" name="advanced">
        <el-form label-width="100px">
          <el-form-item label="特权模式">
            <el-switch v-model="form.privileged" />
            <span class="ml-2 text-gray-500">开启后容器将拥有宿主机的所有设备访问权限</span>
          </el-form-item>
          
          <el-divider content-position="left">设备映射 (Device)</el-divider>
          <div class="device-mappings">
            <div v-for="(item, index) in form.devices" :key="index" class="mapping-row">
              <el-input v-model="item.host" placeholder="宿主机设备 (如 /dev/sda)" />
              <span class="separator">→</span>
              <el-input v-model="item.container" placeholder="容器内路径" />
              <el-input v-model="item.perm" placeholder="权限 (rwm)" style="width: 100px" />
              <el-button type="danger" circle @click="removeDevice(index)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button type="primary" plain class="add-btn" @click="addDevice">
              <el-icon><Plus /></el-icon> 添加设备映射
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <span class="dialog-footer">
        <el-button @click="close" :disabled="loading">取消</el-button>
        <el-button type="primary" @click="handleConfirm" :loading="loading">
          {{ loading ? loadingText : '确认替换' }}
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Plus, Delete } from '@element-plus/icons-vue'
import api from '../api'

const props = defineProps({
  modelValue: Boolean,
  container: Object
})

const emit = defineEmits(['update:modelValue', 'success'])

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const activeTab = ref('basic')
const basicFormRef = ref(null)
const loading = ref(false)
const loadingText = ref('处理中...')
const containerInfo = ref({})
const defaultConfig = ref({ cmd: '', entrypoint: '' })

// 表单数据结构
const form = reactive({
  name: '',
  image: '',
  restart_policy: 'no',
  network_mode: 'bridge',
  ports: [], // { host: '', container: '', protocol: 'tcp' }
  volumes: [], // { host: '', container: '', mode: 'rw' }
  env: [], // { key: '', value: '' }
  command: '',
  entrypoint: '',
  privileged: false,
  devices: [] // { host: '', container: '', perm: 'rwm' }
})

const rules = {
  name: [
    { required: true, message: '请输入容器名称', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/, message: '格式不正确', trigger: 'blur' }
  ]
}

// 监听容器变化，初始化表单
watch(() => props.container, async (newVal) => {
  if (newVal && newVal.Id && visible.value) {
    loading.value = true
    try {
      const detail = await api.containers.getContainer(newVal.Id)
      containerInfo.value = detail
      initForm(detail)
    } catch (e) {
      ElMessage.error('获取容器详情失败')
      close()
    } finally {
      loading.value = false
    }
  }
}, { immediate: true })

const initForm = (detail) => {
  form.name = detail.Name?.replace(/^\//, '') || ''
  form.image = detail.Config?.Image || ''
  form.restart_policy = detail.HostConfig?.RestartPolicy?.Name || 'no'
  form.network_mode = detail.HostConfig?.NetworkMode || 'bridge'
  form.privileged = detail.HostConfig?.Privileged || false
  
  // 初始化命令
  form.command = detail.Config?.Cmd ? detail.Config.Cmd.join(' ') : ''
  form.entrypoint = detail.Config?.Entrypoint ? detail.Config.Entrypoint.join(' ') : ''
  
  // 保存镜像默认配置（如果后端返回）
  if (detail.ImageConfig) {
    defaultConfig.value = {
      cmd: detail.ImageConfig.Cmd ? detail.ImageConfig.Cmd.join(' ') : '',
      entrypoint: detail.ImageConfig.Entrypoint ? detail.ImageConfig.Entrypoint.join(' ') : ''
    }
  } else {
    defaultConfig.value = { cmd: '', entrypoint: '' }
  }

  // 初始化端口
  form.ports = []
  if (detail.HostConfig?.PortBindings) {
    for (const [portProto, bindings] of Object.entries(detail.HostConfig.PortBindings)) {
      const [port, proto] = portProto.split('/')
      if (bindings && bindings.length > 0) {
        form.ports.push({
          host: bindings[0].HostPort,
          container: port,
          protocol: proto
        })
      }
    }
  }

  // 初始化卷
  form.volumes = []
  if (detail.Mounts) {
    detail.Mounts.forEach(m => {
       // 只处理 Bind 类型的挂载，Volume 类型暂不作为宿主机路径处理
       if (m.Type === 'bind') {
          form.volumes.push({
            host: m.Source,
            container: m.Destination,
            mode: m.RW ? 'rw' : 'ro'
          })
       }
    })
  } else if (detail.HostConfig?.Binds) {
    // 兼容 Binds 格式
     detail.HostConfig.Binds.forEach(b => {
        const parts = b.split(':')
        if (parts.length >= 2) {
           form.volumes.push({
             host: parts[0],
             container: parts[1],
             mode: parts[2] || 'rw'
           })
        }
     })
  }

  // 初始化环境变量
  form.env = []
  if (detail.Config?.Env) {
    detail.Config.Env.forEach(e => {
      const [key, ...valParts] = e.split('=')
      form.env.push({
        key: key,
        value: valParts.join('=')
      })
    })
  }
  
  // 初始化设备
  form.devices = []
  if (detail.HostConfig?.Devices) {
     detail.HostConfig.Devices.forEach(d => {
        form.devices.push({
           host: d.PathOnHost,
           container: d.PathInContainer,
           perm: d.CgroupPermissions
        })
     })
  }
}

// 辅助方法
const addPort = () => form.ports.push({ host: '', container: '', protocol: 'tcp' })
const removePort = (i) => form.ports.splice(i, 1)

const addVolume = () => form.volumes.push({ host: '', container: '', mode: 'rw' })
const removeVolume = (i) => form.volumes.splice(i, 1)

const addEnv = () => form.env.push({ key: '', value: '' })
const removeEnv = (i) => form.env.splice(i, 1)

const addDevice = () => form.devices.push({ host: '', container: '', perm: 'rwm' })
const removeDevice = (i) => form.devices.splice(i, 1)

const setCmd = (cmd) => { form.command = cmd }

const close = () => {
  visible.value = false
  activeTab.value = 'basic'
}

// 提交逻辑
const handleConfirm = async () => {
  if (!basicFormRef.value) return
  
  await basicFormRef.value.validate(async (valid) => {
    if (valid) {
      try {
        await ElMessageBox.confirm(
          '此操作将创建一个新容器并删除旧容器。请确保所有重要数据已保存在挂载卷中，否则将会丢失！',
          '高风险操作警告',
          { confirmButtonText: '我已知晓，继续', cancelButtonText: '取消', type: 'warning' }
        )

        loading.value = true
        loadingText.value = '1/3 创建新容器...'

        // 构造请求数据
        const createData = {
          name: form.name,
          image: form.image,
          network_mode: form.network_mode,
          restart_policy: form.restart_policy,
          privileged: form.privileged,
          // 转换数据格式
          ports: form.ports.map(p => `${p.host}:${p.container}/${p.protocol}`), // 注意：需后端支持解析或在此处适配
          volumes: form.volumes.map(v => `${v.host}:${v.container}:${v.mode}`),
          env: form.env.map(e => `${e.key}=${e.value}`),
          devices: form.devices.map(d => `${d.host}:${d.container}:${d.perm}`),
          command: form.command && form.command.trim() ? form.command.split(' ') : [],
          entrypoint: form.entrypoint && form.entrypoint.trim() ? form.entrypoint.split(' ') : []
        }
        
        // 修正端口格式传给后端
        createData.ports = form.ports.map(p => `${p.host}:${p.container}`)
        
        // 1. 重命名旧容器（防止名称冲突）
        loadingText.value = '1/4 准备环境...'
        const oldId = containerInfo.value.Id
        const oldName = containerInfo.value.Name?.replace(/^\//, '')
        const tempName = `${oldName}_old_${Date.now()}`
        
        // 只有当新名称与旧名称相同时，或者直接创建新容器时，才需要重命名旧容器
        // 这里逻辑改为：无论新名称是否改变，都先将旧容器重命名，释放原名称（如果新名称和旧名称一样）
        // 或者释放旧名称（如果新名称不一样，虽然不用重命名也能创建，但为了逻辑统一和方便回滚，也可以重命名）
        // 最稳妥的：先重命名旧容器。
        await api.containers.rename(oldId, tempName)
        
        // 关键：重命名后，如果旧容器还在运行，端口依然是被占用的。
        // 如果新容器使用了相同的端口，启动时会报错 "Bind for 0.0.0.0:xxxx failed: port is already allocated"。
        // 所以必须先停止旧容器，释放端口。
        // 为了安全起见，我们在重命名后就停止旧容器。如果后续创建/启动新容器失败，我们在 catch 块中重新启动旧容器。
        // 注意：containerInfo.value.State 可能是字符串 "running" 也可能是对象
        const isRunning = (containerInfo.value.State?.Running === true) || 
                          (typeof containerInfo.value.State === 'string' && containerInfo.value.State.toLowerCase() === 'running') ||
                          (typeof containerInfo.value.Status === 'string' && containerInfo.value.Status.toLowerCase() === 'running')
        
        console.log('旧容器状态检查:', containerInfo.value.State, 'isRunning:', isRunning)

        if (isRunning) {
             loadingText.value = '1/4 停止旧容器释放端口...'
             await api.containers.stop(oldId)
             // 确保停止操作完成后再继续，虽然 await 应该会等待，但加个小延迟更稳妥
             await new Promise(r => setTimeout(r, 1000))
        }
        
        let newId = null

        try {
           // 2. 创建新容器
           loadingText.value = '2/4 创建新容器...'
           const createRes = await api.containers.create(createData)
           newId = createRes.id

           // 3. 启动并验证
           loadingText.value = '3/4 启动并验证...'
           await api.containers.start(newId)
           
           // 简单验证
           await new Promise(r => setTimeout(r, 1500))
           // 注意：getContainer 返回的是响应对象还是直接是数据，取决于 api 封装
           // 如果是 axios 拦截器处理过的，可能是直接数据；如果没有拦截器处理，可能是 .data
           // 根据前面的 createRes.id，这里的 api 可能是直接返回 data
           const check = await api.containers.getContainer(newId)
           
           // 检查后端 getContainer 接口返回的结构
           // 后端返回: c.JSON(http.StatusOK, containerInfo)
           // containerInfo 中 State: inspect.State.Status ("running", "exited" 等)
           // 而 check.State 可能是对象也可能是字符串，取决于后端返回的结构
           // 查看后端代码： "State": inspect.State.Status (string)
           // 但也有可能返回的是原始 inspect 结构？
           // 再次确认后端： "State": inspect.State.Status, "Status": inspect.State.Status
           // 所以 check.State 应该是一个字符串 "running"
           // 另外，如果 check 包含了原始 inspect 信息（有些后端会返回），则 check.State 可能是对象
           // 为了保险，打印一下 check
           console.log('新容器状态检查:', check)
           
           // 兼容性判断：
           // 1. 如果 check.State 是字符串 (e.g. "running")
           // 2. 如果 check.State 是对象 (e.g. { Running: true, ... })
           // 3. 如果 check.Status 是字符串 (e.g. "running")
           const isRunning = (check.State?.Running === true) || 
                             (typeof check.State === 'string' && check.State.toLowerCase() === 'running') ||
                             (typeof check.Status === 'string' && check.Status.toLowerCase() === 'running')

           if (!isRunning) {
             throw new Error('新容器启动失败，状态非运行中')
           }

           // 4. 清理旧容器
           loadingText.value = '4/4 清理旧容器...'
           if (containerInfo.value.State?.Running) {
                await api.containers.stop(oldId)
           }
           await api.containers.remove(oldId)

           ElMessage.success('容器替换成功')
           emit('success')
           close()
        } catch (createError) {
           // 回滚：如果创建或启动失败，尝试把旧容器名字改回来
           console.error('创建新容器失败，尝试回滚:', createError)
           ElMessage.error(`操作失败: ${createError.message || '未知错误'}。正在尝试恢复旧容器...`)
           
           try {
              // 如果新容器已经创建（有 newId），需要先删除新容器，才能释放名称供旧容器改回
              if (newId) {
                  console.log('正在清理创建失败/启动失败的新容器:', newId)
                  // 尝试停止并删除新容器，不管它是否在运行
                  try {
                    await api.containers.stop(newId)
                  } catch (e) { /* ignore */ }
                  try {
                    await api.containers.remove(newId)
                  } catch (e) { console.error('删除新容器失败:', e) }
              }

              // 尝试回滚旧容器名称
              await api.containers.rename(oldId, oldName)
              
              // 如果之前为了释放端口停止了旧容器，现在需要重新启动它
              // 使用之前判断的 isRunning 变量
              if (isRunning) {
                  ElMessage.info('正在重新启动旧容器...')
                  await api.containers.start(oldId)
              }
              
              ElMessage.info('已恢复旧容器')
           } catch (rollbackError) {
              // 如果回滚改名失败
              console.error('回滚失败:', rollbackError)
              ElMessage.error('回滚失败，请手动恢复旧容器名称: ' + tempName)
           }
        }

      } catch (error) {
        if (error !== 'cancel') {
          console.error(error)
          ElMessage.error(`操作失败: ${error.message || '未知错误'}`)
        }
      } finally {
        loading.value = false
        loadingText.value = '处理中...'
      }
    }
  })
}
</script>

<style scoped>
.edit-tabs {
  min-height: 400px;
}
.mapping-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.separator {
  color: var(--el-text-color-secondary);
  font-weight: bold;
}
.add-btn {
  width: 100%;
  border-style: dashed;
}
.form-tip {
  font-size: 12px;
  color: var(--el-color-warning);
  line-height: 1.4;
  margin-top: 5px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.quick-cmds {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.cursor-pointer {
  cursor: pointer;
}
</style>

<style>
.app-dialog {
  border-radius: 12px !important;
  overflow: hidden;
  box-shadow: var(--el-box-shadow) !important;
}
.app-dialog .el-dialog__header {
  padding: 20px 24px;
  margin-right: 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.app-dialog .el-dialog__title {
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.app-dialog .el-dialog__body {
  padding: 24px;
}
.app-dialog .el-dialog__footer {
  padding: 20px 24px;
  border-top: 1px solid var(--el-border-color-lighter);
  background-color: var(--el-bg-color);
}
.app-dialog .el-dialog__headerbtn {
  top: 24px;
}
.custom-tabs.el-tabs--border-card {
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  box-shadow: none;
}
.custom-tabs.el-tabs--border-card > .el-tabs__header {
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color);
}
.custom-tabs.el-tabs--border-card > .el-tabs__header .el-tabs__item.is-active {
  background-color: var(--el-bg-color-overlay);
  border-right-color: var(--el-border-color);
  border-left-color: var(--el-border-color);
  color: var(--el-color-primary);
  font-weight: 600;
}
</style>
