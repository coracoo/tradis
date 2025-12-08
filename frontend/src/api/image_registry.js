import request from '../utils/request'

export function getRegistries() {
  return request({
    url: '/image-registry',
    method: 'get'
  })
}

export function updateRegistries(data) {
  return request({
    url: '/image-registry',
    method: 'post',
    data
  })
}