import yaml from 'js-yaml'

const isPlainObject = (v) => v && typeof v === 'object' && !Array.isArray(v)

const clampText = (s, maxLen = 200) => {
  const t = String(s || '')
  if (t.length <= maxLen) return t
  return t.slice(0, maxLen) + '...'
}

const normalizeServiceName = (serviceName) => {
  const n = String(serviceName || '').trim()
  return n ? n : 'Global'
}

const stringifyComposeValue = (v) => {
  if (v === null || v === undefined) return ''
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}

const isSensitiveKey = (k) => {
  const s = String(k || '').toLowerCase()
  return s.includes('password') || s.includes('passwd') || s.includes('secret') || s.includes('token') || s.includes('key')
}

const makeSchemaItem = ({ name, serviceName, paramType, defaultValue }) => {
  const n = String(name || '').trim()
  const svc = normalizeServiceName(serviceName)
  const p = String(paramType || '').trim()
  const def = stringifyComposeValue(defaultValue)

  return {
    name: n,
    label: n,
    description: '',
    type: p === 'env' ? (isSensitiveKey(n) ? 'password' : 'string') : (p === 'port' ? 'port' : (p === 'path' ? 'path' : 'string')),
    default: def,
    category: 'basic',
    serviceName: svc,
    paramType: p
  }
}

const addUniqueSchemaItem = (schema, index, item, warnings) => {
  const key = `${item.serviceName}::${item.paramType}::${item.name}`
  if (index.has(key)) {
    warnings.push(`发现重复配置项：${item.serviceName}/${item.paramType}/${item.name}`)
    return false
  }
  index.add(key)
  schema.push(item)
  return true
}

const parsePortString = (raw) => {
  const s = String(raw || '').trim()
  if (!s) return null

  const noProto = s.includes('/') ? s.split('/')[0] : s

  let rest = noProto
  if (rest.startsWith('[')) {
    const idx = rest.indexOf(']')
    if (idx !== -1 && rest[idx + 1] === ':') {
      rest = rest.slice(idx + 2)
    }
  }

  const parts = rest.split(':').map(p => p.trim()).filter(Boolean)
  if (parts.length < 2) return null

  const containerPort = parts[parts.length - 1]
  const hostPort = parts[parts.length - 2]

  if (!hostPort || !containerPort) return null
  return { hostPort, containerPort }
}

const parseVolumeString = (raw) => {
  const s = String(raw || '').trim()
  if (!s) return null
  const parts = s.split(':')
  if (parts.length < 2) return null
  const host = String(parts[0] || '').trim()
  const container = String(parts[1] || '').trim()
  if (!host || !container) return null
  return { host, container }
}

const extractEnvMap = (envNode, warnings, serviceName) => {
  const envMap = new Map()
  const duplicates = new Set()

  const put = (k, v, rawLine) => {
    const key = String(k || '').trim()
    if (!key) return
    if (envMap.has(key)) duplicates.add(key)
    envMap.set(key, v)
    if (v === '' && rawLine) {
      warnings.push(`发现未赋值的环境变量：${normalizeServiceName(serviceName)}/${key}`)
    }
  }

  if (Array.isArray(envNode)) {
    for (const item of envNode) {
      if (typeof item === 'string') {
        const idx = item.indexOf('=')
        if (idx === -1) {
          put(item, '', item)
          continue
        }
        const k = item.slice(0, idx)
        const v = item.slice(idx + 1)
        put(k, v, item)
        continue
      }
      if (isPlainObject(item)) {
        for (const [k, v] of Object.entries(item)) {
          put(k, stringifyComposeValue(v), `${k}=...`)
        }
      }
    }
  } else if (isPlainObject(envNode)) {
    for (const [k, v] of Object.entries(envNode)) {
      const val = (v === null || v === undefined) ? '' : stringifyComposeValue(v)
      put(k, val, `${k}=...`)
    }
  }

  for (const k of duplicates) {
    warnings.push(`发现重复环境变量 key：${normalizeServiceName(serviceName)}/${k}`)
  }

  return envMap
}

const extractVarRefs = (text) => {
  const s = String(text || '')
  const out = []
  const seen = new Set()

  const push = (name, hasDefault, def, raw) => {
    const n = String(name || '').trim()
    if (!n) return
    const key = `${n}::${hasDefault ? '1' : '0'}::${String(def || '')}`
    if (seen.has(key)) return
    seen.add(key)
    out.push({ name: n, hasDefault, defaultValue: def, raw: raw || '' })
  }

  for (let i = 0; i < s.length; i++) {
    if (s[i] !== '$') continue
    const next = s[i + 1]
    if (next === '$') {
      i += 1
      continue
    }
    if (next === '{') {
      const end = s.indexOf('}', i + 2)
      if (end === -1) continue
      const inner = s.slice(i + 2, end)
      const raw = s.slice(i, end + 1)

      let namePart = inner
      let def = ''
      let hasDefault = false
      const idxColon = inner.indexOf(':-')
      if (idxColon !== -1) {
        namePart = inner.slice(0, idxColon)
        def = inner.slice(idxColon + 2)
        hasDefault = true
      } else {
        const idxDash = inner.indexOf('-')
        if (idxDash !== -1) {
          namePart = inner.slice(0, idxDash)
          def = inner.slice(idxDash + 1)
          hasDefault = true
        }
      }

      const m = String(namePart || '').match(/^[A-Za-z_][A-Za-z0-9_]*$/)
      if (m) push(m[0], hasDefault, def, raw)
      i = end
      continue
    }

    const rest = s.slice(i + 1)
    const m = rest.match(/^([A-Za-z_][A-Za-z0-9_]*)/)
    if (m) {
      push(m[1], false, '', `$${m[1]}`)
      i += m[1].length
    }
  }

  return out
}

/**
 * parseDotenvText 解析 .env 原文为键值对，并输出告警/错误（用于 UI 展示）
 */
export const parseDotenvText = (dotenvText) => {
  const warnings = []
  const errors = []
  const dotenv = {}
  const seen = new Set()

  const lines = String(dotenvText || '').split(/\r?\n/)
  for (let i = 0; i < lines.length; i++) {
    const lineNo = i + 1
    const rawLine = lines[i]
    let line = String(rawLine || '').trim()
    if (!line || line.startsWith('#')) continue
    if (line.startsWith('export ')) line = line.slice('export '.length).trim()

    const idx = line.indexOf('=')
    if (idx === -1) {
      const key = line.trim()
      if (!key) continue
      if (seen.has(key)) warnings.push(`.env 第${lineNo}行：重复 key ${key}`)
      seen.add(key)
      dotenv[key] = ''
      warnings.push(`.env 第${lineNo}行：未赋值 ${key}（已按空值处理）`)
      continue
    }
    if (idx === 0) {
      warnings.push(`.env 第${lineNo}行：无法解析（key 为空）: ${clampText(rawLine)}`)
      continue
    }

    const key = line.slice(0, idx).trim()
    let valRaw = line.slice(idx + 1).trim()
    if (!key) {
      warnings.push(`.env 第${lineNo}行：无法解析（key 为空）: ${clampText(rawLine)}`)
      continue
    }
    if (seen.has(key)) warnings.push(`.env 第${lineNo}行：重复 key ${key}（后者覆盖前者）`)
    seen.add(key)

    let val = valRaw
    if (valRaw.length >= 2) {
      const first = valRaw[0]
      const last = valRaw[valRaw.length - 1]
      if ((first === '"' && last === '"') || (first === '\'' && last === '\'')) {
        val = valRaw.slice(1, -1)
      } else if (first === '"' || first === '\'') {
        warnings.push(`.env 第${lineNo}行：引号未闭合（保留原值）: ${clampText(rawLine)}`)
        val = valRaw
      }
    }

    dotenv[key] = val
  }

  return { dotenv, warnings, errors }
}

/**
 * parseComposeTemplateVariables 解析 docker-compose.yaml 的可编辑变量
 * 返回：{ schema, warnings, errors, refs }
 */
export const parseComposeTemplateVariables = (composeContent) => {
  const schema = []
  const warnings = []
  const errors = []
  const index = new Set()

  const content = String(composeContent || '')

  let parsed = null
  try {
    parsed = yaml.load(content)
  } catch (e) {
    errors.push(`YAML 解析失败：${clampText(e?.message || e)}`)
  }

  if (parsed && isPlainObject(parsed) && isPlainObject(parsed.services)) {
    for (const [serviceName, service] of Object.entries(parsed.services)) {
      if (!isPlainObject(service)) continue

      const svc = normalizeServiceName(serviceName)

      const ports = service.ports
      if (Array.isArray(ports)) {
        for (const p of ports) {
          if (typeof p === 'string') {
            const parsedPort = parsePortString(p)
            if (!parsedPort) {
              warnings.push(`发现无法解析的端口映射：${svc}/${clampText(p)}`)
              continue
            }
            addUniqueSchemaItem(
              schema,
              index,
              makeSchemaItem({ name: parsedPort.hostPort, serviceName: svc, paramType: 'port', defaultValue: parsedPort.containerPort }),
              warnings
            )
            continue
          }
          if (isPlainObject(p)) {
            const host = (p.published !== undefined && p.published !== null) ? String(p.published) : ''
            const target = (p.target !== undefined && p.target !== null) ? String(p.target) : ''
            if (!host || !target) {
              warnings.push(`发现无法解析的端口对象：${svc}/${clampText(JSON.stringify(p))}`)
              continue
            }
            addUniqueSchemaItem(
              schema,
              index,
              makeSchemaItem({ name: host, serviceName: svc, paramType: 'port', defaultValue: target }),
              warnings
            )
          }
        }
      }

      const volumes = service.volumes
      if (Array.isArray(volumes)) {
        for (const v of volumes) {
          if (typeof v === 'string') {
            const parsedVol = parseVolumeString(v)
            if (!parsedVol) {
              warnings.push(`发现无法解析的挂载配置：${svc}/${clampText(v)}`)
              continue
            }
            if (!/^(\.\/|\.\.\/|\/)/.test(parsedVol.host)) continue
            addUniqueSchemaItem(
              schema,
              index,
              makeSchemaItem({ name: parsedVol.host, serviceName: svc, paramType: 'path', defaultValue: parsedVol.container }),
              warnings
            )
            continue
          }
          if (isPlainObject(v)) {
            const source = (v.source !== undefined && v.source !== null) ? String(v.source) : ''
            const target = (v.target !== undefined && v.target !== null) ? String(v.target) : ''
            if (!source || !target) continue
            if (!/^(\.\/|\.\.\/|\/)/.test(source)) continue
            addUniqueSchemaItem(
              schema,
              index,
              makeSchemaItem({ name: source, serviceName: svc, paramType: 'path', defaultValue: target }),
              warnings
            )
          }
        }
      }

      const envNode = service.environment
      if (envNode !== undefined) {
        const envMap = extractEnvMap(envNode, warnings, svc)
        for (const [k, v] of envMap.entries()) {
          if (String(k).toUpperCase() === 'PATH') continue
          addUniqueSchemaItem(
            schema,
            index,
            makeSchemaItem({ name: k, serviceName: svc, paramType: 'env', defaultValue: v }),
            warnings
          )
        }
      }
    }
  }

  const refs = extractVarRefs(content)
  for (const r of refs) {
    const item = makeSchemaItem({
      name: r.name,
      serviceName: 'Global',
      paramType: 'env',
      defaultValue: r.hasDefault ? r.defaultValue : ''
    })
    const added = addUniqueSchemaItem(schema, index, item, warnings)
    if (added && !r.hasDefault) {
      warnings.push(`发现未赋值的变量引用：${r.raw || '${' + r.name + '}'}`)
    }
  }

  return { schema, warnings, errors, refs }
}
