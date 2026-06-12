import client from './client'

export const accountsApi = {
  list: (includeArchived = false) =>
    client.get('/accounts', { params: { include_archived: includeArchived } }).then((r) => r.data.accounts),

  get: (id) => client.get(`/accounts/${id}`).then((r) => r.data),

  create: (payload) => client.post('/accounts', payload).then((r) => r.data),

  update: (id, payload) => client.patch(`/accounts/${id}`, payload).then((r) => r.data),

  remove: (id) => client.delete(`/accounts/${id}`).then((r) => r.data),
}
