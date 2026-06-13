import { api, call } from './client'
import type { CashFlowReport, NetWorthReport, SpendingReport } from './types'

interface DateRange {
  date_from?: string
  date_to?: string
}

export const reportsApi = {
  netWorth: (): Promise<NetWorthReport> => call(api.GET('/reports/net-worth', {})),

  spending: (query: DateRange = {}): Promise<SpendingReport> =>
    call(api.GET('/reports/spending', { params: { query } })),

  cashFlow: (query: DateRange = {}): Promise<CashFlowReport> =>
    call(api.GET('/reports/cash-flow', { params: { query } })),
}
