import request from '../utils/request'

export default {
  // 获取项目列表
  getProjects: (params) => {
    return request.get('/appstore/apps', { params })
  },

  // 获取项目详情
  getProjectDetail: (id) => {
    return request.get(`/appstore/apps/${id}`)
  },

  getProjectVars: (id) => {
    return request.get(`/appstore/apps/${id}/vars`)
  },

  // 部署项目
  deployProject: (data) => {
    return request.post(`/appstore/deploy/${data.projectId}`, data)
  },

  // 提交部署次数统计
  submitDeployCount: (id) => {
    return request.post(`/appstore/deploy_count/${id}`)
  },
  
  // 获取应用状态
  getAppStatus: (id) => {
    return request.get(`/appstore/status/${id}`)
  }
}
