import { api, call } from './client'
import { apiBaseUrl, authHeader } from './auth'
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

  // Downloads matching expenses/income as an .xlsx file (raw fetch: the response
  // is a binary blob, not JSON). Transfers are excluded server-side.
  exportXlsx: async (filter: TransactionFilter = {}): Promise<void> => {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(filter)) {
      if (value !== undefined && value !== '' && key !== 'limit' && key !== 'offset' && key !== 'uncategorized') {
        params.set(key, String(value))
      }
    }
    const header = authHeader()
    const res = await fetch(`${apiBaseUrl}/transactions/export?${params}`, {
      headers: header ? { Authorization: header } : {},
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)

    const blob = await res.blob()
    const name = res.headers.get('Content-Disposition')?.match(/filename="?([^"]+)"?/)?.[1] || 'transactions.xlsx'
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  },
}
