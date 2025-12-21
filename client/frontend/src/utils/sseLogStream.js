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

export const useSseLogStream = ({
  autoScroll,
  scrollElRef,
  makeEntry,
  getSearchText,
  onOpen,
  onMessage,
  onError,
  onOpenLine = 'info: 已连接到日志服务',
  onErrorLine = 'error: 日志连接错误'
} = {}) => {
  const logs = ref([])
  const logFilter = ref('')
  const isOpen = ref(false)
  let eventSource = null
  let lastErrorAt = 0

  const makeEntrySafe = makeEntry || ((line) => ({ content: String(line || ''), level: defaultInferLogLevel(line) }))
  const getSearchTextSafe = getSearchText || ((entry) => String(entry?.content || ''))

  const filteredLogs = computed(() => {
    if (!logFilter.value) return logs.value
    const q = logFilter.value.toLowerCase()
    return logs.value.filter((entry) => String(getSearchTextSafe(entry)).toLowerCase().includes(q))
  })

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
    logs.value.push(makeEntrySafe(line))
    scrollToBottom()
  }

  const stop = () => {
    if (eventSource) {
      try { eventSource.close() } catch {}
      eventSource = null
    }
    isOpen.value = false
  }

  const start = (url, { reset = true } = {}) => {
    stop()
    if (reset) logs.value = []
    if (!url) return

    const es = new EventSource(url)
    eventSource = es
    isOpen.value = false

    es.onopen = () => {
      isOpen.value = true
      if (typeof onOpen === 'function') {
        try { onOpen({ pushLine, stop, isOpen }) } catch {}
      }
      if (onOpenLine) pushLine(onOpenLine)
    }

    es.onmessage = (event) => {
      if (typeof onMessage === 'function') {
        try { onMessage(event, { pushLine, stop, isOpen }) } catch {}
        return
      }
      pushLine(event.data)
    }

    es.onerror = () => {
      isOpen.value = false
      const now = Date.now()
      if (onErrorLine && (now - lastErrorAt) > 2000) {
        lastErrorAt = now
        pushLine(onErrorLine)
      }
      if (typeof onError === 'function') {
        try { onError({ pushLine, stop, isOpen }) } catch {}
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
    start,
    stop,
    clear,
    pushLine
  }
}
