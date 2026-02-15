import { ref, computed, nextTick } from 'vue'

export const defaultInferLogLevel = (line) => {
  const raw = String(line || '')
  const lower = raw.toLowerCase()
  if (lower.startsWith('error:')) return 'error'
  if (lower.startsWith('warning:')) return 'warning'
  if (lower.startsWith('success:')) return 'success'
  if (lower.startsWith('info:')) return 'info'
  if (lower.includes('error') || lower.includes('err')) return 'error'
  if (lower.includes('warn')) return 'warning'
  return 'info'
}

export const getLogClass = (log, { includeSuccessInInfo = false } = {}) => {
  const level = String(log?.level || '').toLowerCase()
  const isSuccess = level === 'success'
  return {
    error: level === 'error',
    warning: level === 'warning',
    info: level === 'info' || (includeSuccessInInfo && isSuccess),
    success: isSuccess && !includeSuccessInInfo
  }
}

export const buildSseUrl = (path, params = {}, { includeToken = true } = {}) => {
  const url = new URL(path, window.location.origin)
  if (includeToken) {
    const token = localStorage.getItem('token') || ''
    if (token && !url.searchParams.has('token') && params?.token == null) {
      url.searchParams.set('token', token)
    }
  }
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value == null || value === '') return
    if (Array.isArray(value)) {
      url.searchParams.delete(key)
      value.forEach((item) => {
        if (item == null || item === '') return
        url.searchParams.append(key, String(item))
      })
      return
    }
    url.searchParams.set(key, String(value))
  })
  return `${url.pathname}${url.search}`
}

const clamp = (n, min, max) => Math.max(min, Math.min(max, n))

const tryParseJSON = (s) => {
  const raw = String(s || '').trim()
  if (!raw) return { ok: false, value: null }
  const first = raw[0]
  if (first !== '{' && first !== '[') return { ok: false, value: null }
  try {
    return { ok: true, value: JSON.parse(raw) }
  } catch {
    return { ok: false, value: null }
  }
}

const inferPercentFromText = (text) => {
  const raw = String(text || '')
  const m1 = raw.match(/(\d{1,3})\s*%/)
  if (m1) return clamp(Number(m1[1]), 0, 100)

  const m2 = raw.match(/step\s+(\d+)\s*\/\s*(\d+)/i)
  if (m2) {
    const cur = Number(m2[1])
    const total = Number(m2[2])
    if (total > 0) return clamp(Math.round((cur / total) * 100), 0, 100)
  }

  const m3 = raw.match(/(\d+)\s*\/\s*(\d+)\s*(?:steps?|阶段|步)/i)
  if (m3) {
    const cur = Number(m3[1])
    const total = Number(m3[2])
    if (total > 0) return clamp(Math.round((cur / total) * 100), 0, 100)
  }

  return null
}

const inferProgressFromPayload = (payload) => {
  if (!payload) return null
  if (typeof payload === 'string') {
    const p = inferPercentFromText(payload)
    if (typeof p === 'number') return { percent: p, text: String(payload || '') }
    return null
  }

  if (typeof payload === 'object') {
    const percent =
      typeof payload.progress === 'number' ? payload.progress :
      typeof payload.percent === 'number' ? payload.percent :
      typeof payload.percentage === 'number' ? payload.percentage :
      null

    if (typeof percent === 'number') {
      const text = String(payload.message || payload.status || '')
      return { percent: clamp(percent, 0, 100), text }
    }

    if (typeof payload.current === 'number' && typeof payload.total === 'number' && payload.total > 0) {
      const p = clamp(Math.round((payload.current / payload.total) * 100), 0, 100)
      const text = String(payload.message || payload.status || '')
      return { percent: p, text }
    }

    const text = String(payload.message || payload.status || '')
    const p = inferPercentFromText(text)
    if (typeof p === 'number') return { percent: p, text }
  }

  return null
}

const isResultPayload = (payload) => {
  if (!payload || typeof payload !== 'object') return false
  const t = String(payload.type || '').toLowerCase()
  return t === 'result' || t === 'done' || t === 'complete' || t === 'completed'
}

export const useSseLogStream = ({
  autoScroll,
  scrollElRef,
  makeEntry,
  getSearchText,
  onOpen,
  onMessage,
  onError,
  eventNames,
  onOpenLine = 'info: 已连接到日志服务',
  onErrorLine = 'error: 日志连接错误',
  enableProgress = false,
  autoTimeProgress = true
} = {}) => {
  const logs = ref([])
  const logFilter = ref('')
  const isOpen = ref(false)
  const progressPercent = ref(0)
  const progressText = ref('')
  const progressStatus = ref('')
  let eventSource = null
  let lastErrorAt = 0
  let progressTimer = null

  const makeEntrySafe = makeEntry || ((payload) => {
    const text = typeof payload === 'string' ? payload : JSON.stringify(payload)
    return { content: String(text || ''), level: defaultInferLogLevel(text) }
  })
  const getSearchTextSafe = getSearchText || ((entry) => String(entry?.content || ''))
  const eventNamesSafe = Array.isArray(eventNames)
    ? eventNames
    : (typeof eventNames === 'string' ? [eventNames] : [])
  const extraEventNames = eventNamesSafe.filter((name) => name && name !== 'message')

  const filteredLogs = computed(() => {
    if (!logFilter.value) return logs.value
    const q = logFilter.value.toLowerCase()
    return logs.value.filter((entry) => String(getSearchTextSafe(entry)).toLowerCase().includes(q))
  })

  const setProgress = ({ percent, text, status } = {}) => {
    if (typeof percent === 'number' && Number.isFinite(percent)) {
      progressPercent.value = clamp(percent, 0, 100)
    }
    if (typeof text === 'string') progressText.value = text
    if (typeof status === 'string') progressStatus.value = status
  }

  const resetProgress = () => {
    progressPercent.value = 0
    progressText.value = ''
    progressStatus.value = ''
  }

  const startProgressTimer = () => {
    if (!enableProgress || !autoTimeProgress) return
    if (progressTimer) return
    progressTimer = setInterval(() => {
      if (!isOpen.value) return
      if (progressPercent.value >= 95) return
      const next = progressPercent.value < 70 ? progressPercent.value + 1 : progressPercent.value + 0.3
      progressPercent.value = clamp(next, 0, 95)
    }, 1000)
  }

  const stopProgressTimer = () => {
    if (!progressTimer) return
    clearInterval(progressTimer)
    progressTimer = null
  }

  const scrollToBottom = () => {
    if (!autoScroll?.value) return
    if (!scrollElRef?.value) return
    nextTick(() => {
      try {
        scrollElRef.value.scrollTop = scrollElRef.value.scrollHeight
      } catch {}
    })
  }

  const pushLine = (line) => {
    if (enableProgress) {
      const inferred = inferProgressFromPayload(line)
      if (inferred && typeof inferred.percent === 'number') {
        const next = Math.max(progressPercent.value, inferred.percent)
        setProgress({ percent: next, text: inferred.text || progressText.value })
      }
      if (isResultPayload(line)) {
        const status = String(line.status || '').toLowerCase()
        if (status === 'success' || status === 'completed') setProgress({ percent: 100, status: 'success' })
        if (status === 'error' || status === 'failed') setProgress({ percent: Math.max(progressPercent.value, 100), status: 'exception' })
      }
    }
    logs.value.push(makeEntrySafe(line))
    scrollToBottom()
  }

  const stop = () => {
    if (eventSource) {
      try { eventSource.close() } catch {}
      eventSource = null
    }
    isOpen.value = false
    stopProgressTimer()
  }

  const start = (url, { reset = true } = {}) => {
    stop()
    if (reset) logs.value = []
    if (!url) return
    if (enableProgress) resetProgress()

    const es = new EventSource(url)
    eventSource = es
    isOpen.value = false

    es.onopen = () => {
      isOpen.value = true
      startProgressTimer()
      if (typeof onOpen === 'function') {
        try { onOpen({ pushLine, stop, isOpen, setProgress }) } catch {}
      }
      if (onOpenLine) pushLine(onOpenLine)
    }

    const handleEvent = (event) => {
      const parsed = tryParseJSON(event?.data)
      const payload = parsed.ok ? parsed.value : event?.data
      if (typeof onMessage === 'function') {
        try { onMessage(event, { pushLine, stop, isOpen, payload, setProgress }) } catch {}
        return
      }
      pushLine(payload)
      if (isResultPayload(payload)) {
        stop()
      }
    }

    es.onmessage = handleEvent
    if (extraEventNames.length > 0) {
      extraEventNames.forEach((name) => {
        try {
          es.addEventListener(name, handleEvent)
        } catch {}
      })
    }

    es.onerror = () => {
      isOpen.value = false
      const now = Date.now()
      if (onErrorLine && (now - lastErrorAt) > 2000) {
        lastErrorAt = now
        pushLine(onErrorLine)
      }
      if (typeof onError === 'function') {
        try { onError({ pushLine, stop, isOpen, setProgress }) } catch {}
      }
    }
  }

  const clear = () => {
    logs.value = []
  }

  return {
    logs,
    logFilter,
    filteredLogs,
    isOpen,
    progressPercent,
    progressText,
    progressStatus,
    setProgress,
    start,
    stop,
    clear,
    pushLine
  }
}
