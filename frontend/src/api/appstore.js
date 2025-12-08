import request from '../utils/request'

export default {
  // 获取所有模板
  getTemplates() {
    return request({
      url: 'http://localhost:3001/api/templates',
      method: 'get'
    })
  },

  // 获取单个模板
  getTemplate(id) {
    return request({
      url: `http://localhost:3001/api/templates/${id}`,
      method: 'get'
    })
  },

  // 部署模板
  deployTemplate(template) {
    return request({
      url: '/compose/project',
      method: 'post',
      data: {
        name: template.name.toLowerCase(),
        compose: template.compose,
        autoStart: true
      }
    })
  }
}