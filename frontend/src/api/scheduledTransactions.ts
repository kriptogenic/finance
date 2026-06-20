import { api, call } from './client'
import type {
  ScheduledTransaction,
  CreateScheduledTransactionRequest,
  UpdateScheduledTransactionRequest,
  Transaction,
} from './types'

export const scheduledTransactionsApi = {
  list: (): Promise<ScheduledTransaction[]> =>
    call(api.GET('/scheduled-transactions', {})).then((d) => d.scheduled_transactions),

  create: (body: CreateScheduledTransactionRequest): Promise<ScheduledTransaction> =>
    call(api.POST('/scheduled-transactions', { body })),

  update: (id: string, body: UpdateScheduledTransactionRequest): Promise<ScheduledTransaction> =>
    call(api.PATCH('/scheduled-transactions/{id}', { params: { path: { id } }, body })),

  remove: (id: string): Promise<unknown> =>
    call(api.DELETE('/scheduled-transactions/{id}', { params: { path: { id } } })),

  run: (id: string): Promise<Transaction> =>
    call(api.POST('/scheduled-transactions/{id}/run', { params: { path: { id } } })),
}
