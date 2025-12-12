import request from '../utils/request'

export default {
  // 获取项目列表
  getProjects: () => {
    return request.get('/appstore/apps')
  },

  // 获取项目详情
  getProjectDetail: (id) => {
    return request.get(`/appstore/apps/${id}`)
  },

  // 部署项目
  deployProject: (data) => {
    return request.post(`/appstore/deploy/${data.projectId}`, data)
  },
  
  // 获取应用状态
  getAppStatus: (id) => {
    return request.get(`/appstore/status/${id}`)
  }
}
