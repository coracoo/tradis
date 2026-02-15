import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({
  baseURL: '/api',
  timeout: 300000,
  headers: {
    'Content-Type': 'application/json'
  }
})

api.interceptors.request.use(
  config => {
    if (config.data instanceof FormData) {
      if (config.headers && config.headers['Content-Type']) {
        delete config.headers['Content-Type']
      }
    }
    const token = localStorage.getItem('token')
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`
    }
    return config
  },
  error => Promise.reject(error)
)

api.interceptors.response.use(
  response => response.data,
  error => {
    const status = error?.response?.status
    const data = error?.response?.data
    const serverError = typeof data === 'string' ? data : ''
    const serverMessage = data?.message || data?.details || data?.error || ''
    let errorMessage = serverError || serverMessage || error?.message || '请求失败'
    if (status === 401) {
      errorMessage = '登录已过期，请重新登录'
      localStorage.removeItem('token')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    ElMessage.error(errorMessage)
    return Promise.reject(error)
  }
)

export default api
