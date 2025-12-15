import axios from 'axios'
import { ElMessage } from 'element-plus'

const service = axios.create({
  // baseURL: import.meta.env.VITE_API_BASE_URL || (import.meta.env.PROD ? 'http://localhost:8080/api' : '/api'),s
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 300000,  // 5分钟超时，因为拉取镜像需要较长时间
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
service.interceptors.request.use(
  config => {
    // 自动添加 Token
    const token = localStorage.getItem('token')
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`
    }

    // 对于拉取镜像的请求，使用特殊配置
    if (config.url?.includes('/images/pull')) {
      config.timeout = 300000  // 5分钟
      config.responseType = 'text'  // 使用 text 类型接收流数据
    }
    return config
  },
  error => {
    console.error('请求错误:', error)
    return Promise.reject(error)
  }
)

// 响应拦截器
service.interceptors.response.use(
  response => {
    // 处理流式响应
    if (response.config.url?.includes('/images/pull')) {
      return response.data
    }
    return response.data
  },
  error => {
    console.error('响应错误:', error)
    
    // 处理不同类型的错误
    let errorMessage = '请求失败'
    
    if (error.code === 'ECONNABORTED') {
      errorMessage = '请求超时，请检查网络连接'
    } else if (error.response) {
      const status = error.response.status
      const data = error.response.data
    
      switch (status) {
        case 401:
          errorMessage = '登录已过期，请重新登录'
          // 清除 token 并跳转到登录页
          localStorage.removeItem('token')
          if (window.location.pathname !== '/login') {
            window.location.href = '/login'
          }
          break
        case 404:
          errorMessage = '请求的资源不存在'
          break
        case 500:
          errorMessage = data.error || '服务器内部错误'
          break
        default:
          errorMessage = data.error || error.message || '未知错误'
      }
    
      // 特殊处理镜像拉取错误
      if (error.config.url?.includes('/images/pull')) {
        if (data.error?.includes('no proxy configured')) {
          errorMessage = '未配置代理，请在设置中配置代理后重试'
        } else if (data.error?.includes('no mirror configured')) {
          errorMessage = '未配置镜像加速器，请在设置中配置后重试'
        } else if (data.error?.includes('network timeout')) {
          errorMessage = '网络连接超时，请检查网络或代理设置'
        }
      }
    }
    
    ElMessage.error(errorMessage)
    return Promise.reject(error)
  }
)

export default service
