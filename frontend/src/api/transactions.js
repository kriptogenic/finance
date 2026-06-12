import client from './client'

export const transactionsApi = {
  list: (params = {}) => client.get('/transactions', { params }).then((r) => r.data.transactions),

  get: (id) => client.get(`/transactions/${id}`).then((r) => r.data),

  create: (payload) => client.post('/transactions', payload).then((r) => r.data),

  update: (id, payload) => client.put(`/transactions/${id}`, payload).then((r) => r.data),

  remove: (id) => client.delete(`/transactions/${id}`).then((r) => r.data),
}
