import api from '../shared/http'

export const templateApi = {
  list() {
    return api.get('/templates')
  },

  get(id: string) {
    return api.get(`/templates/${id}`)
  },

  create(data: any) {
    return api.post('/templates', data)
  },

  update(id: string, data: any) {
    return api.put(`/templates/${id}`, data)
  },

  setEnabled(id: string, enabled: boolean) {
    return enabled ? api.post(`/templates/${id}/enable`) : api.post(`/templates/${id}/disable`)
  },

  delete(id: string) {
    return api.delete(`/templates/${id}`)
  },

  syncToGithub() {
    return api.post('/templates/sync')
  },

  parseVars(compose: string) {
    return api.post('/templates/parse-vars', { compose })
  }
}

export const versionApi = {
  get() {
    return api.get('/version')
  },
  update(serverVersion: string) {
    return api.put('/version', { server_version: serverVersion })
  }
}
