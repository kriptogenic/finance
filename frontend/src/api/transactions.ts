import { api, call } from './client'
import type { CreateTransactionRequest, Transaction, TransactionType } from './types'

export interface TransactionFilter {
  account_id?: string
  category_id?: string
  type?: TransactionType
  date_from?: string
  date_to?: string
  tag?: string
  q?: string
  limit?: number
  offset?: number
}

export const transactionsApi = {
  list: (query: TransactionFilter = {}): Promise<Transaction[]> =>
    call(api.GET('/transactions', { params: { query } })).then((d) => d.transactions),

  get: (id: string): Promise<Transaction> => call(api.GET('/transactions/{id}', { params: { path: { id } } })),

  create: (body: CreateTransactionRequest): Promise<Transaction> => call(api.POST('/transactions', { body })),

  update: (id: string, body: CreateTransactionRequest): Promise<Transaction> =>
    call(api.PUT('/transactions/{id}', { params: { path: { id } }, body })),

  remove: (id: string): Promise<unknown> => call(api.DELETE('/transactions/{id}', { params: { path: { id } } })),
}
