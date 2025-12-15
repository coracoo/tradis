import request from '../utils/request'

export default {
  list() {
    return request({
      url: '/compose/list',
      method: 'get'
    })
  },
  
  deploy(data) {
    return request({
      url: '/compose/project',
      method: 'post',
      data
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
  
  saveYaml(name, content) {
    return request({
      url: `/compose/${name}/yaml`,
      method: 'post',
      data: { content }
    })
  }
}