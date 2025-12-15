import request from '../utils/request'

const portsApi = {
  list: (params) => request.get('/ports', { params }),
  getRange: () => request.get('/ports/range'),
  updateRange: (data) => request.post('/ports/range', data),
  saveNote: (data) => request.post('/ports/note', data),
  allocate: (data) => request.post('/ports/allocate', data)
}

export default portsApi
