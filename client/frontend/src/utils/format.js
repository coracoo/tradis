import dayjs from 'dayjs'

export const formatTime = (timestamp) => {
  if (!timestamp) return '-'
  
  // 处理字符串格式的时间
  if (typeof timestamp === 'string') {
    return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss')
  }
  
  // 处理 Unix 时间戳
  return dayjs(timestamp * 1000).format('YYYY-MM-DD HH:mm:ss')
}

export const formatTimeTwoLines = (timestamp) => {
  if (!timestamp) return '-'
  
  const d = typeof timestamp === 'string' ? dayjs(timestamp) : dayjs(timestamp * 1000)
  return d.format('YYYY-MM-DD\nHH:mm:ss')
}

export const composeProjectNamePattern = /^[a-z0-9][a-z0-9_-]*$/

export const normalizeComposeProjectName = (name) => {
  const lower = String(name || '').toLowerCase().trim()
  const sanitized = lower.replace(/[^a-z0-9_-]/g, '')
  const trimmed = sanitized.replace(/^[^a-z0-9]+/, '')
  return trimmed || 'project'
}

export const isValidComposeProjectName = (name) => composeProjectNamePattern.test(String(name || '').toLowerCase().trim())

export const formatBytes = (bytes, { decimals = 2 } = {}) => {
  const b = Number(bytes || 0)
  if (!Number.isFinite(b) || b <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(sizes.length - 1, Math.floor(Math.log(b) / Math.log(k)))
  const value = b / Math.pow(k, i)
  const fixed = i === 0 ? 0 : Math.max(0, Math.min(6, Number(decimals)))
  return `${parseFloat(value.toFixed(fixed))} ${sizes[i]}`
}
