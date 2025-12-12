import request from '../utils/request'

const api = {
  // 获取容器列表
  listContainers() {
    return request({
      url: '/containers',
      method: 'get'
    })
  },

  // 获取单个容器详情
  getContainer(id) {
    return request({
      url: `/containers/${id}`,
      method: 'get'
    })
  },

  // 启动容器
  startContainer(id) {
    return request({
      url: `/containers/${id}/start`,
      method: 'post'
    })
  },

  // 重启容器
  restartContainer(id) {
    return request({
      url: `/containers/${id}/restart`,
      method: 'post'
    })
  },

  // 停止容器
  stopContainer(id) {
    return request({
      url: `/containers/${id}/stop`,
      method: 'post'
    })
  },

  // 删除容器
  removeContainer(id) {
    return request({
      url: `/containers/${id}`,
      method: 'delete'
    })
  },

  // 获取容器日志
  getContainerLogs(id) {
    return request({
      url: `/containers/${id}/logs`,
      method: 'get'
    })
  },
  
  // 创建容器
  create(data) {
    return request({
      url: '/containers/create',
      method: 'post',
      data
    })
  },
  
  // 重命名容器
  rename(id, newName) {
    return request({
      url: `/containers/${id}/rename`,
      method: 'post',
      data: { newName }
    })
  }
}

export default api