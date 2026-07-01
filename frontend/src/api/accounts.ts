import { api, call } from './client'
import type {
  Account,
  AmortizationSchedule,
  CreateAccountRequest,
  LoanScheduleResponse,
  UpdateAccountRequest,
  UpdateLoanScheduleRow,
} from './types'

export const accountsApi = {
  list: (includeArchived = false): Promise<Account[]> =>
    call(api.GET('/accounts', { params: { query: { include_archived: includeArchived } } })).then((d) => d.accounts),

  get: (id: string): Promise<Account> => call(api.GET('/accounts/{id}', { params: { path: { id } } })),

  create: (body: CreateAccountRequest): Promise<Account> => call(api.POST('/accounts', { body })),

  update: (id: string, body: UpdateAccountRequest): Promise<Account> =>
    call(api.PATCH('/accounts/{id}', { params: { path: { id } }, body })),

  remove: (id: string): Promise<unknown> => call(api.DELETE('/accounts/{id}', { params: { path: { id } } })),

  amortization: (id: string): Promise<AmortizationSchedule> =>
    call(api.GET('/accounts/{id}/amortization', { params: { path: { id } } })),

  loanSchedule: (id: string): Promise<LoanScheduleResponse> =>
    call(api.GET('/accounts/{id}/schedule', { params: { path: { id } } })),

  regenerateSchedule: (id: string): Promise<LoanScheduleResponse> =>
    call(api.POST('/accounts/{id}/schedule', { params: { path: { id } } })),

  updateScheduleRow: (id: string, period: number, body: UpdateLoanScheduleRow): Promise<LoanScheduleResponse> =>
    call(api.PATCH('/accounts/{id}/schedule/{period}', { params: { path: { id, period } }, body })),
}
