import axios from 'axios'

const api = {
  // 容器相关接口
  containers: {
    list: () => axios.get('/api/containers'),
    create: (data) => axios.post('/api/containers/create', data),
    start: (id) => axios.post(`/api/containers/${id}/start`),
    stop: (id) => axios.post(`/api/containers/${id}/stop`),
    restart: (id) => axios.post(`/api/containers/${id}/restart`),
    remove: (id) => axios.delete(`/api/containers/${id}`),
    logs: (id) => axios.get(`/api/containers/${id}/logs`),
    stats: (id) => axios.get(`/api/containers/${id}/stats`)
  },

  // 镜像相关接口
  images: {
    list: () => axios.get('/api/images'),
    pull: (data) => axios.post('/api/images/pull', data),
    remove: (id) => axios.delete(`/api/images/${id}`),
    build: (data) => axios.post('/api/images/build', data)
  },

  // 网络相关接口
  networks: {
    list: () => axios.get('/api/networks'),
    create: (data) => axios.post('/api/networks', data),
    remove: (id) => axios.delete(`/api/networks/${id}`)
  },

  // 数据卷相关接口
  volumes: {
    list: () => axios.get('/api/volumes'),
    create: (data) => axios.post('/api/volumes', data),
    remove: (name) => axios.delete(`/api/volumes/${name}`)
  }
}

export default api