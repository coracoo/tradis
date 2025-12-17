import request from '../utils/request'
import imagesApi, { getProxy, updateProxy } from './images'
import compose from './compose'
import appstore from './appstore'
import ports from './ports'

// 定义 API 对象
const api = {
  containers: {
    list: () => request.get('/containers'),
    listContainers: () => request.get('/containers'), // 保留别名以兼容
    getContainer: (id) => request.get(`/containers/${id}`),
    create: (data) => request.post('/containers/create', data),
    rename: (id, newName) => request.post(`/containers/${id}/rename`, { newName }),
    start: (id) => request.post(`/containers/${id}/start`),
    startContainer: (id) => request.post(`/containers/${id}/start`), // 保留别名
    stop: (id) => request.post(`/containers/${id}/stop`),
    stopContainer: (id) => request.post(`/containers/${id}/stop`), // 保留别名
    restart: (id) => request.post(`/containers/${id}/restart`),
    restartContainer: (id) => request.post(`/containers/${id}/restart`), // 保留别名
    kill: (id) => request.post(`/containers/${id}/kill`),
    pause: (id) => request.post(`/containers/${id}/pause`),
    unpause: (id) => request.post(`/containers/${id}/unpause`),
    remove: (id) => request.delete(`/containers/${id}`),
    removeContainer: (id) => request.delete(`/containers/${id}`), // 保留别名
    stats: (id) => request.get(`/containers/${id}/stats`),
    logs: (id) => request.get(`/containers/${id}/logs`, {
      responseType: 'text',
      timeout: 0
    }),
    prune: () => request.post('/containers/prune')
  },
  
  images: {
    ...imagesApi,
    getProxy,
    updateProxy
  },
  
  compose,
  
  ports,
  appstore,

  volumes: {
    list: () => request.get('/volumes'),
    create: (data) => request.post('/volumes', data),
    remove: (name) => request.delete(`/volumes/${name}`),
    prune: () => request.post('/volumes/prune')
  },
  
  networks: {
    list: () => request.get('/networks'),
    create: (data) => request.post('/networks', data),
    update: (id, data) => request.put(`/networks/${id}`, data),
    remove: (id) => request.delete(`/networks/${id}`),
    prune: () => request.post('/networks/prune')
  },
  
  system: {
    info: () => request.get('/system/info'),
    stats: () => request.get('/system/stats'),
    events: () => request.get('/system/events')
  },

  navigation: {
    list: (params) => request.get('/navigation', { params }),
    add: (data) => request.post('/navigation', data),
    update: (id, data) => request.put(`/navigation/${id}`, data),
    delete: (id) => request.delete(`/navigation/${id}`),
    restore: (id) => request.post(`/navigation/${id}/restore`),
    uploadIcon: (id, file) => {
      const form = new FormData()
      form.append('file', file)
      return request.post(`/navigation/${id}/icon`, form)
    }
  }
}

export default api
