import client from './client'

export const reportsApi = {
  netWorth: () => client.get('/reports/net-worth').then((r) => r.data),

  spending: (params = {}) => client.get('/reports/spending', { params }).then((r) => r.data),

  cashFlow: (params = {}) => client.get('/reports/cash-flow', { params }).then((r) => r.data),
}
