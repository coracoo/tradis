import request from '../utils/request'

const api = {
  // 容器相关接口
  containers: {
    list: () => request.get('/containers'),
    create: (data) => request.post('/containers/create', data),
    start: (id) => request.post(`/containers/${id}/start`),
    stop: (id) => request.post(`/containers/${id}/stop`),
    restart: (id) => request.post(`/containers/${id}/restart`),
    remove: (id) => request.delete(`/containers/${id}`),
    logs: (id) => request.get(`/containers/${id}/logs`),
    stats: (id) => request.get(`/containers/${id}/stats`)
  },

  // 镜像相关接口
  images: {
    list: () => request.get('/images'),
    pull: (data) => request.post('/images/pull', data),
    remove: (id) => request.delete(`/images/${id}`),
    build: (data) => request.post('/images/build', data)
  },

  // 网络相关接口
  networks: {
    list: () => request.get('/networks'),
    create: (data) => request.post('/networks', data),
    remove: (id) => request.delete(`/networks/${id}`)
  },

  // 数据卷相关接口
  volumes: {
    list: () => request.get('/volumes'),
    create: (data) => request.post('/volumes', data),
    remove: (name) => request.delete(`/volumes/${name}`)
  }
}

export default api
