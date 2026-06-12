import client from './client'

export const categoriesApi = {
  list: (params = {}) => client.get('/categories', { params }).then((r) => r.data.categories),

  get: (id) => client.get(`/categories/${id}`).then((r) => r.data),

  create: (payload) => client.post('/categories', payload).then((r) => r.data),

  update: (id, payload) => client.patch(`/categories/${id}`, payload).then((r) => r.data),

  remove: (id) => client.delete(`/categories/${id}`).then((r) => r.data),
}
