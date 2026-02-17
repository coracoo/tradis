export const safeJsonParse = (input, fallback = null) => {
  const raw = String(input ?? '').trim()
  if (!raw) return fallback
  try {
    return JSON.parse(raw)
  } catch {
    return fallback
  }
}

export const readJsonFromStorage = (key, fallback = null, { storage = localStorage } = {}) => {
  if (!key || !storage) return fallback
  const raw = storage.getItem(key)
  if (!raw || !String(raw).trim()) return fallback
  try {
    return JSON.parse(raw)
  } catch {
    storage.removeItem(key)
    return fallback
  }
}

