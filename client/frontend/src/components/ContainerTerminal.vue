<template>
  <el-dialog
    v-model="dialogVisible"
    title="容器终端"
    width="80%"
    :close-on-click-modal="false"
    :before-close="handleClose"
    class="terminal-dialog app-dialog"	
  >
    <div class="terminal-container">
      <!-- 左侧会话列表 + 复制/粘贴工具栏（移至列表上方） -->
      <div class="left-panel">
        <div class="session-list">
          <div class="session-header">
            <h4>会话列表</h4>
            <el-popover v-model:visible="showAddProfile" placement="bottom-end" width="260">
              <template #reference>
                <el-button size="small" type="primary">新增</el-button>
              </template>
              <div class="add-profile-pop">
                <el-radio-group v-model="addProfileType">
                  <el-radio label="bash">/bin/bash</el-radio>
                  <el-radio label="sh">/bin/sh</el-radio>
                  <el-radio label="custom">自定义</el-radio>
                </el-radio-group>
                <el-input v-if="addProfileType === 'custom'" v-model="addProfileCustom" placeholder="输入命令路径" size="small" style="margin-top:8px" />
                <div class="actions">
                  <el-button size="small" type="primary" @click="confirmAddProfile">确定</el-button>
                  <el-button size="small" @click="showAddProfile=false">取消</el-button>
                </div>
              </div>
            </el-popover>
          </div>
          <div class="left-toolbar">
            <el-button size="small" @click="copySelection">复制文本</el-button>
            <el-button size="small" @click="pasteClipboard">粘贴文本</el-button>
          </div>
          <el-scrollbar height="240px">
            <ul class="profile-list">
              <li v-for="(p, i) in profiles" :key="p.id" class="profile-item">
                <span :class="['status-dot', getProfileStatus(p.id) ? 'connected' : 'disconnected']" title="连接状态"></span>
                <span class="profile-label" @click="connectProfile(p)">{{ p.label }}</span>
                <span class="profile-remove" @click="removeProfile(i)">移除</span>
              </li>
            </ul>
          </el-scrollbar>
        </div>
      </div>

      <!-- 右侧终端显示区（多会话，可切视图不重置） -->
      <div class="terminal-panel">
        <div ref="terminalHost" class="terminal-host">
          <!-- 为每个会话预留挂载容器，切换只切换可见性，不销毁实例 -->
          <div
            v-for="s in sessions"
            :key="s.id"
            :id="s.id"
            class="terminal-instance"
            v-show="activeSessionId === s.id"
          ></div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, defineProps, defineEmits, nextTick } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
// import { AttachAddon } from 'xterm-addon-attach';
import 'xterm/css/xterm.css'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  container: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['update:modelValue'])
const dialogVisible = ref(props.modelValue)
const terminalHost = ref(null)
// 多会话运行时状态
const sessions = ref([]) // {id,label,cmd,profileId,state,terminal,fitAddon,socket,connectTimer,ready}
const activeSessionId = ref('')
// 当前入口命令
const terminalCommand = ref('/bin/bash')
// 会话配置：用于“会话列表”管理与快捷进入（仅持久化 label/cmd）
const profiles = ref([{ id: `p-${Date.now()}`, label: '/bin/bash', cmd: '/bin/bash' }])
// 会话状态灯映射：key 为 profileId，value 为是否连接（默认新建为 true，事件驱动刷新）
const profileStatusMap = ref({})

// 新增会话弹窗状态
const showAddProfile = ref(false)
const addProfileType = ref('bash')
const addProfileCustom = ref('')

// 监听对话框可见性变化
watch(() => props.modelValue, (val) => {
  dialogVisible.value = val
  if (val && props.container) {
    // 当对话框显示且有容器信息时，直接连接第一个会话，避免重复初始化
    setTimeout(() => {
      // 优先使用 /bin/bash，其次 /bin/sh，最后回退到第一个配置
      const bash = profiles.value.find(p => p.cmd === '/bin/bash')
      const sh = profiles.value.find(p => p.cmd === '/bin/sh')
      const target = bash || sh || profiles.value[0]
      if (target) connectProfile(target)
    }, 100)
  }
})

// 监听对话框内部状态变化
watch(() => dialogVisible.value, (val) => {
  emit('update:modelValue', val)
  if (!val) {
    // 关闭WebSocket连接
    closeConnection()
  }
})

// 加载本地会话配置
onMounted(() => {
  try {
    const saved = localStorage.getItem('terminalProfiles')
    if (saved) {
      const arr = JSON.parse(saved)
      if (Array.isArray(arr) && arr.length > 0) {
        // 兼容历史数据：补齐缺失的 id 字段
        profiles.value = arr.map(p => ({
          id: p.id || `p-${Date.now()}-${Math.random().toString(36).slice(2,6)}`,
          label: p.label,
          cmd: p.cmd
        }))
        // 回写一次，防止后续匹配失败导致状态灯不变化
        try { localStorage.setItem('terminalProfiles', JSON.stringify(profiles.value)) } catch {}
        // 初始化状态灯为未连接
        const init = {}
        profiles.value.forEach(p => { init[p.id] = false })
        profileStatusMap.value = init
        refreshProfileStates()
      }
    }
  } catch (e) {}
})

// 创建并挂载某个会话的终端实例
const createTerminalForSession = (session) => {
  if (!terminalHost.value) return
  if (session.terminal) return

  const cssVar = (name, fallback) => {
    try {
      const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
      return v || fallback
    } catch {
      return fallback
    }
  }

  const term = new Terminal({
    cursorBlink: true,
    theme: {
      background: cssVar('--el-bg-color-overlay', '#1e1e1e'),
      foreground: cssVar('--el-text-color-primary', '#f0f0f0')
    },
    fontSize: 14,
    fontFamily: 'Consolas, "Courier New", monospace',
    scrollback: 1000
  })
  const fit = new FitAddon()
  term.loadAddon(fit)

  // 通过预渲染的容器挂载（以 session.id 为容器 id）
  const mountEl = document.getElementById(session.id)
  if (!mountEl) return
  term.open(mountEl)
  try { term.focus() } catch {}
  // 保存引用
  session.terminal = term
  session.fitAddon = fit
  // 启动时清屏并将光标置于左上角，避免非左侧置顶
  try { session.terminal.reset() } catch {}
  safeFitSession(session)
}

// 建立某会话的 WebSocket 连接
const connectSessionSocket = (session) => {
  if (!props.container || !props.container.Id) {
    ElMessage.error('容器信息不完整')
    return
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const token = localStorage.getItem('token')
  const containerId = props.container.Id
  const wsUrl = `${protocol}//${host}/api/containers/${containerId}/terminal?token=${token}`

  console.log('连接WebSocket:', { protocol, host, containerId, wsUrl, cmd: session.cmd })

  try {
    session.socket = new WebSocket(wsUrl)
    session.state = 'connecting'
    session.ready = false
    // 连接超时控制（5秒内无有效数据则判定失败并提示错误）
    try { clearTimeout(session.connectTimer) } catch {}
    session.connectTimer = setTimeout(() => {
      if (!session.ready) {
        try { session.socket?.close() } catch {}
        session.state = 'closed'
        session.terminal?.writeln('\r\n\x1b[31m连接超时(>5s)，请检查容器状态或入口命令。\x1b[0m')
        ElMessage.error('终端连接超时')
      }
    }, 5000)

    session.socket.onopen = () => {
      console.log('WebSocket连接成功')
      session.state = 'open'

      // 发送入口命令（握手阶段）
      session.socket.send(JSON.stringify({ type: 'command', data: session.cmd }))

      // 发送初始尺寸
      const dims = session.terminal && session.terminal.rows && session.terminal.cols ?
        { rows: session.terminal.rows, cols: session.terminal.cols } : { rows: 24, cols: 80 }
      session.socket.send(JSON.stringify({ type: 'resize', data: JSON.stringify(dims) }))

      // 输入桥接（基于该会话的终端）
      session.terminal.onData(data => {
        if (session.socket && session.socket.readyState === WebSocket.OPEN) {
          session.socket.send(JSON.stringify({ type: 'input', data }))
        }
      })
      // 成功打开后刷新所有状态灯
      refreshProfileStates()
    }

    session.socket.onmessage = (event) => {
      // 收到任何数据即视为连接就绪
      session.ready = true
      try { clearTimeout(session.connectTimer) } catch {}
      if (typeof event.data === 'string') {
        session.terminal.write(event.data)
      } else {
        const reader = new FileReader()
        reader.onload = () => {
          // 按 UTF-8 解码 Blob，避免中文在 bash 中显示为 \201 等编码
          try {
            const decoder = new TextDecoder('utf-8')
            const arrBuf = event.data.arrayBuffer ? undefined : undefined
            // FileReader 已按 UTF-8 读取 result；安全起见再按 utf-8 写入
            session.terminal.write(reader.result)
          } catch {
            session.terminal.write(reader.result)
          }
        }
        reader.readAsText(event.data, 'utf-8')
      }
    }

    session.socket.onerror = (error) => {
      console.error('WebSocket错误:', error)
      session.state = 'closed'
      ElMessage.error('终端连接发生错误')
      refreshProfileStates()
    }

    session.socket.onclose = (event) => {
      console.log('WebSocket连接关闭:', event)
      session.state = 'closed'
      // 解除输入桥接
      if (session.terminal) session.terminal.onData(() => {})
      try { clearTimeout(session.connectTimer) } catch {}
      refreshProfileStates()
    }

    window.addEventListener('resize', handleResize)
  } catch (error) {
    console.error('创建WebSocket连接失败:', error)
    session.state = 'closed'
    session.terminal?.writeln(`\r\n\x1b[31m创建WebSocket连接失败: ${error.message}\x1b[0m`)
    ElMessage.error(`无法连接到终端服务: ${error.message}`)
  }
}

// 历史命令区域已移除

// 安全自适应某会话
const safeFitSession = async (session) => {
  try {
    await nextTick()
    const el = document.getElementById(session.id)
    const hasSize = el && el.offsetWidth > 0 && el.offsetHeight > 0
    if (hasSize && session.fitAddon && session.terminal) {
      session.fitAddon.fit()
    }
  } catch (e) {
    console.warn('会话终端自适应失败:', e)
  }
}

// 处理窗口/终端尺寸变更
// 保持终端自适应，并通知后端调整 TTY 尺寸
const handleResize = () => {
  try {
    const active = sessions.value.find(s => s.id === activeSessionId.value)
    if (!active) return
    safeFitSession(active)
    if (active.terminal) {
      const dimensions = active.terminal.rows && active.terminal.cols ?
        { rows: active.terminal.rows, cols: active.terminal.cols } : { rows: 24, cols: 80 }
      if (active.socket && active.socket.readyState === WebSocket.OPEN) {
        active.socket.send(JSON.stringify({ type: 'resize', data: JSON.stringify(dimensions) }))
      }
    }
  } catch (e) {
    console.warn('终端尺寸调整失败:', e)
  }
}

// 连接会话配置
const connectProfile = (p) => {
  // 设计要点：存在则仅切视图；关闭则重连；不存在则创建
  terminalCommand.value = p.cmd
  let session = sessions.value.find(s => s.profileId === p.id)
  if (!session) {
    session = {
      id: `term-${Date.now()}-${Math.random().toString(36).slice(2,8)}`,
      label: p.label,
      cmd: p.cmd,
      profileId: p.id || `p-${Date.now()}-${Math.random().toString(36).slice(2,6)}`,
      state: 'idle',
      terminal: null,
      fitAddon: null,
      socket: null,
      connectTimer: null,
      ready: false
    }
    sessions.value.push(session)
    // 新建会话时默认将状态灯置为正常（绿色），后续由事件刷新真实状态
    profileStatusMap.value[session.profileId] = true
    nextTick(() => {
      createTerminalForSession(session)
      connectSessionSocket(session)
    })
  } else {
    // 已存在：如果未创建终端则创建；如果连接断开则重连；否则仅切视图
    if (!session.terminal) createTerminalForSession(session)
    if (session.state !== 'open' && session.state !== 'connecting') connectSessionSocket(session)
  }
  activeSessionId.value = session.id
  // 切换后确保自适应一次，避免未铺满
  safeFitSession(session)
  // 点击会话时刷新一次所有状态灯
  refreshProfileStates()
}

// 新增会话：支持 /bin/bash、/bin/sh 或自定义
const confirmAddProfile = () => {
  let cmd = '/bin/bash'
  let label = '/bin/bash'
  if (addProfileType.value === 'sh') {
    cmd = '/bin/sh'
    label = '/bin/sh'
  } else if (addProfileType.value === 'custom') {
    const c = (addProfileCustom.value || '').trim()
    if (!c) {
      ElMessage.error('请输入自定义命令')
      return
    }
    cmd = c
    label = c
  }
  const newProfile = { id: `p-${Date.now()}-${Math.random().toString(36).slice(2,6)}`, label, cmd }
  profiles.value.push(newProfile)
  saveProfiles()
  showAddProfile.value = false
  addProfileCustom.value = ''
  // 新增配置后预置状态灯为正常，随后由连接事件修正
  profileStatusMap.value[newProfile.id] = true
  // 新增后立即创建并连接对应会话
  connectProfile(newProfile)
}

// 移除会话配置
const removeProfile = (index) => {
  const [removed] = profiles.value.splice(index, 1)
  // 同步移除该配置关联的会话（按 profileId）
  const toRemove = sessions.value.filter(s => s.profileId === removed?.id)
  toRemove.forEach(s => {
    try { s.socket?.close() } catch {}
    try { s.terminal?.dispose() } catch {}
  })
  sessions.value = sessions.value.filter(s => s.profileId !== removed?.id)
  // 若激活会话属于被移除项，则取消激活
  if (toRemove.some(s => s.id === activeSessionId.value)) {
    activeSessionId.value = ''
  }
  saveProfiles()
  // 移除后刷新所有状态灯
  refreshProfileStates()
}

// 判断某会话是否处于连接状态（以当前 WebSocket 状态为准）
// 获取某配置的状态灯值
const getProfileStatus = (profileId) => {
  return !!profileStatusMap.value[profileId]
}

// 全量刷新所有配置的状态灯，根据各自关联会话是否处于 open/connecting
const refreshProfileStates = () => {
  const states = {}
  profiles.value.forEach(p => {
    const hasOpen = sessions.value.some(s => s.profileId === p.id && s.state === 'open')
    const hasConnecting = sessions.value.some(s => s.profileId === p.id && s.state === 'connecting')
    states[p.id] = hasOpen || hasConnecting
  })
  profileStatusMap.value = states
}

// 保存会话配置
const saveProfiles = () => {
  try {
    localStorage.setItem('terminalProfiles', JSON.stringify(profiles.value))
  } catch (e) {}
}

// 关闭并清理终端与连接
// 确保移除事件、断开 WebSocket、释放终端资源
const closeConnection = () => {
  try {
    window.removeEventListener('resize', handleResize)
  } catch (e) {}
  // 关闭并清理所有会话
  sessions.value.forEach(s => {
    try { s.socket?.close() } catch {}
    try { s.terminal?.dispose() } catch {}
  })
  sessions.value = []
  activeSessionId.value = ''
  // 清理后刷新状态灯（全部置灰）
  refreshProfileStates()
}

// 对话框关闭前的钩子：统一清理资源
const handleClose = (done) => {
  closeConnection()
  done()
}

// 复制/粘贴支持
const copySelection = async () => {
  try {
    const active = sessions.value.find(s => s.id === activeSessionId.value)
    const text = active?.terminal?.getSelection() || ''
    if (!text) {
      ElMessage.warning('没有选中的文本')
      return
    }
    const writeText = navigator?.clipboard?.writeText
    if (typeof writeText === 'function') {
      await writeText.call(navigator.clipboard, text)
      ElMessage.success('已复制到剪贴板')
      return
    }
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.setAttribute('readonly', 'readonly')
    textarea.style.position = 'fixed'
    textarea.style.top = '-9999px'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    const ok = document.execCommand && document.execCommand('copy')
    document.body.removeChild(textarea)
    if (ok) {
      ElMessage.success('已复制到剪贴板')
      return
    }
    throw new Error('clipboard-unavailable')
  } catch (e) {
    ElMessage.error('复制失败')
  }
}

const pasteClipboard = async () => {
  try {
    let text = ''
    const readText = navigator?.clipboard?.readText
    if (typeof readText === 'function') {
      try {
        text = await readText.call(navigator.clipboard)
      } catch (e) {
        text = ''
      }
    }
    if (!text) {
      try {
        const { value } = await ElMessageBox.prompt('浏览器未授权读取剪贴板，请在此输入/粘贴要发送到终端的文本', '粘贴到终端', {
          confirmButtonText: '发送',
          cancelButtonText: '取消',
          inputType: 'textarea',
          inputValue: ''
        })
        text = String(value || '')
      } catch (e) {
        return
      }
    }
    if (!text) return
    const active = sessions.value.find(s => s.id === activeSessionId.value) || sessions.value[sessions.value.length - 1]
    if (active?.socket && active.socket.readyState === WebSocket.OPEN) {
      active.socket.send(JSON.stringify({ type: 'input', data: text }))
      try { active.terminal?.focus() } catch {}
      ElMessage.success('已粘贴到终端')
    } else {
      ElMessage.error('当前会话未连接')
    }
  } catch (e) {
    ElMessage.error('粘贴失败')
  }
}

// 组件卸载时：确保清理终端与连接，避免悬挂事件与资源泄露
onBeforeUnmount(() => {
  closeConnection()
})

// 安全自适应函数已拆至按会话的 safeFitSession
</script>

<style>
/* 修改终端样式 */
.terminal-container {
  display: flex;
  min-height: 500px;
  background-color: var(--el-bg-color-overlay);
  padding: 5px;
  border-radius: 8px;
  gap: 10px;
}

.terminal {
  padding: 10px;
}

.left-panel {
  width: 250px;
  background-color: var(--el-bg-color);
  padding: 10px;
  border-radius: 8px;
}

.session-list {
  margin-bottom: 12px;
}
.session-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.add-profile-pop .actions {
  margin-top: 8px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
.profile-list {
  padding: 0;
  margin: 0;
}
.profile-item {
  display: flex;
  align-items: center;
  height: 34px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 8px;
}
.status-dot.connected { background-color: var(--el-color-success); }
.status-dot.disconnected { background-color: var(--el-text-color-secondary); }
.profile-label {
  flex: 1;
  cursor: pointer;
}
.profile-remove {
  color: var(--el-color-danger);
  cursor: pointer;
  margin-left: 8px;
}

.terminal-panel {
  flex: 1;
  background-color: var(--el-bg-color-overlay);
  border-radius: 4px;
  overflow: hidden;
  position: relative;
}

.terminal-host {
  height: 100%;
  width: 100%;
  padding: 0;
  overflow: hidden; /* 终端区不产生内滚动，滚动由对话框外层处理 */
}

/* 每个会话终端实例容器占满面板 */
.terminal-instance {
  height: 100%;
  width: 100%;
}

.history-item {
  cursor: pointer;
  padding: 5px 10px;
  list-style: none;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.history-item:hover {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.history-list {
  padding: 0;
  margin: 0;
}

.terminal-dialog :deep(.el-dialog__body) {
  padding: 10px;
  overflow: hidden; /* 滚动条位于对话框右侧，紧贴页面但不越界 */
}

.quick-commands h4,
.command-history h4 {
  margin-top: 0;
  margin-bottom: 10px;
  color: var(--el-text-color-primary);
}

.command-history {
  margin-top: 20px;
}

.left-toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

/* 确保 xterm 内容无额外内边距并左上对齐 */
.terminal-panel .terminal-container .xterm {
  padding: 0;
  text-align: left;
}
.terminal-panel .terminal-host .xterm-rows {
  padding: 0 !important;
}

.terminal-host .xterm .xterm-selection div {
  background-color: var(--el-color-primary-light-5) !important;
}

.terminal-host .xterm .xterm-rows ::selection {
  background-color: var(--el-color-primary-light-5);
  color: var(--el-text-color-primary);
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

/* Terminal specific overrides */
.terminal-dialog.app-dialog .el-dialog__body {
  padding: 10px !important;
}
</style>
