let lastLoadLevel = 'normal'
let lastSuggestedRefreshSeconds = 0
let lastUpdatedAt = 0

const nowMs = () => Date.now()

const normalizeLevel = (v) => {
  const s = String(v || '').trim().toLowerCase()
  if (s === 'high' || s === 'critical' || s === 'normal') return s
  return 'normal'
}

export const updateLoadSheddingFromHeaders = (headers) => {
  if (!headers || typeof headers !== 'object') return
  const level = headers['x-tradis-load-level']
  const suggested = headers['x-tradis-suggested-refresh-seconds']

  const nextLevel = normalizeLevel(level)
  let nextSuggested = Number(suggested || 0)
  if (!Number.isFinite(nextSuggested) || nextSuggested < 0) nextSuggested = 0

  lastLoadLevel = nextLevel
  if (nextSuggested > 0) lastSuggestedRefreshSeconds = nextSuggested
  lastUpdatedAt = nowMs()
}

export const getLoadLevel = () => lastLoadLevel

export const getSuggestedRefreshMs = (defaultMs = 5000) => {
  const age = nowMs() - lastUpdatedAt
  if (age > 60_000) return defaultMs

  if (lastSuggestedRefreshSeconds > 0) {
    const ms = Math.round(lastSuggestedRefreshSeconds * 1000)
    if (ms > 0) return ms
  }

  switch (lastLoadLevel) {
    case 'critical':
      return 20000
    case 'high':
      return 10000
    default:
      return defaultMs
  }
}

export const getSuggestedIntervalMs = (defaultMs) => {
  const suggested = getSuggestedRefreshMs(defaultMs)
  return Math.max(defaultMs, suggested)
}

export const getSuggestedOperationDelayMs = (defaultMs) => {
  const level = lastLoadLevel
  if (level === 'critical') return Math.max(defaultMs, 10000)
  if (level === 'high') return Math.max(defaultMs, 5000)
  return defaultMs
}

