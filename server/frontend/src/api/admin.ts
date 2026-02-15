import api from '../shared/http'

export const adminApi = {
  getAdminAllowlist() {
    return api.get('/admin/allowlist')
  },
  updateAdminAllowlist(raw: string) {
    return api.put('/admin/allowlist', { raw })
  },
  getMcpAllowlist() {
    return api.get('/admin/mcp-allowlist')
  },
  updateMcpAllowlist(raw: string) {
    return api.put('/admin/mcp-allowlist', { raw })
  },
  getMcpToken() {
    return api.get('/admin/mcp-token')
  },
  updateMcpToken(token: string) {
    return api.put('/admin/mcp-token', { token })
  }
}

