import { api, call } from './client'
import type {
  CategorySuggestion,
  CreateTransactionRequest,
  SetSplitRequest,
  Transaction,
  TransactionSplit,
  TransactionType,
} from './types'

export interface TransactionFilter {
  account_id?: string
  category_id?: string
  type?: TransactionType
  date_from?: string
  date_to?: string
  tag?: string
  uncategorized?: boolean
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

  patchCategory: (id: string, category_id: string): Promise<Transaction> =>
    call(api.PATCH('/transactions/{id}', { params: { path: { id } }, body: { category_id } })),

  suggestCategories: (id: string): Promise<CategorySuggestion[]> =>
    call(api.GET('/transactions/{id}/category-suggestions', { params: { path: { id } } })).then((d) => d.suggestions),

  remove: (id: string): Promise<unknown> => call(api.DELETE('/transactions/{id}', { params: { path: { id } } })),

  getSplit: (id: string): Promise<TransactionSplit> =>
    call(api.GET('/transactions/{id}/split', { params: { path: { id } } })),

  setSplit: (id: string, body: SetSplitRequest): Promise<TransactionSplit> =>
    call(api.PUT('/transactions/{id}/split', { params: { path: { id } }, body })),
}
