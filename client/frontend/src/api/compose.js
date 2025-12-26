import request from '../utils/request'

export default {
  list() {
    return request({
      url: '/compose/list',
      method: 'get'
    })
  },
  
  deployTask(data) {
    return request({
      url: '/compose/deploy',
      method: 'post',
      data
    })
  },

  listTasks(params) {
    return request({
      url: '/compose/tasks',
      method: 'get',
      params
    })
  },

  getTask(id) {
    return request({
      url: `/compose/tasks/${id}`,
      method: 'get'
    })
  },
  
  start(name) {
    return request({
      url: `/compose/${name}/start`,
      method: 'post'
    })
  },

  stop(name) {
    return request({
      url: `/compose/${name}/stop`,
      method: 'post'
    })
  },

  restart(name) {
    return request({
      url: `/compose/${name}/restart`,
      method: 'post'
    })
  },

  build(name) {
    return request({
      url: `/compose/${name}/build`,
      method: 'post'
    })
  },
  
  remove(name) {
    return request({
      url: `/compose/remove/${name}`,
      method: 'delete'
    })
  },
  
  down(name) {
    return request({
      url: `/compose/${name}/down`,
      method: 'delete'
    })
  },
  
  getStatus(name) {
    return request({
      url: `/compose/${name}/status`,
      method: 'get'
    })
  },
  
  getYaml(name) {
    return request({
      url: `/compose/${name}/yaml`,
      method: 'get'
    })
  },

  getEnv(name) {
    return request({
      url: `/compose/${name}/env`,
      method: 'get'
    })
  },

  saveEnv(name, content) {
    return request({
      url: `/compose/${name}/env`,
      method: 'post',
      data: { content }
    })
  },
  
  saveYaml(name, content) {
    return request({
      url: `/compose/${name}/yaml`,
      method: 'post',
      data: { content }
    })
  }
}
