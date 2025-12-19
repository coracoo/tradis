import request from '../utils/request'

// 获取 Docker 配置
export const getProxy = () => {
  return request({
    url: '/images/proxy',
    method: 'get'
  })
}

// 获取代理历史
export const getProxyHistory = () => {
  return request({
    url: '/images/proxy/history',
    method: 'get'
  })
}

// 更新 Docker 配置
export const updateProxy = (data) => {
  return request({
    url: '/images/proxy',
    method: 'post',
    data
  })
}

// 导出默认对象，包含所有镜像相关API
const imagesApi = {
  list: () => {
    return request({
      url: '/images',
      method: 'get'
    })
  },
  remove: (payload) => {
    if (typeof payload === 'string') {
      return request({
        url: `/images/${encodeURIComponent(payload)}`,
        method: 'delete'
      })
    }
    const id = payload && payload.id ? encodeURIComponent(payload.id) : ''
    const repoTag = payload && payload.repoTag ? `?repoTag=${encodeURIComponent(payload.repoTag)}` : ''
    return request({
      url: `/images/${id}${repoTag}`,
      method: 'delete'
    })
  },
  
  // 拉取镜像
  pull: (data) => {
    return request({
      url: '/images/pull',
      method: 'post',
      data,
      timeout: 300000 // 5分钟超时
    })
  },
  
  // 添加拉取镜像进度监听方法
  pullProgress: (name, registry) => {
    const params = new URLSearchParams()
    if (name) params.append('name', name)
    if (registry) params.append('registry', registry)
    
    return `/api/images/pull/progress?${params.toString()}`
  },
  
  // 添加修改标签方法
  tag: (data) => {
    return request({
      url: '/images/tag',
      method: 'post',
      data
    })
  },
  // 添加导出镜像方法
  export: (id) => {
    return request({
      url: `/images/export/${id}`,
      method: 'get',
      responseType: 'blob'
    })
  },
  // 添加导入镜像方法
  import: (formData) => {
    return request({
      url: '/images/import',
      method: 'post',
      data: formData,
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      timeout: 600000 // 10分钟超时
    })
  },
  
  // 清理未使用的镜像
  prune: () => {
    return request({
      url: '/images/prune',
      method: 'post'
    })
  },

  checkUpdates: () => {
    return request({
      url: '/images/updates',
      method: 'get'
    })
  },

  getUpdateStatus: () => {
    return request({
      url: '/images/updates/status',
      method: 'get'
    })
  },

  clearUpdate: (data) => {
    return request({
      url: '/images/updates/clear',
      method: 'post',
      data
    })
  },

  applyUpdates: () => {
    return request({
      url: '/images/updates/apply',
      method: 'post',
      timeout: 600000
    })
  }
} 

export default imagesApi
