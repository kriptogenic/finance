import { api, call } from './client'
import type { Budget, CreateBudgetRequest, UpdateBudgetRequest } from './types'

export const budgetsApi = {
  list: (): Promise<Budget[]> => call(api.GET('/budgets', {})).then((d) => d.budgets),

  create: (body: CreateBudgetRequest): Promise<Budget> => call(api.POST('/budgets', { body })),

  update: (id: string, body: UpdateBudgetRequest): Promise<Budget> =>
    call(api.PATCH('/budgets/{id}', { params: { path: { id } }, body })),

  remove: (id: string): Promise<unknown> => call(api.DELETE('/budgets/{id}', { params: { path: { id } } })),
}
