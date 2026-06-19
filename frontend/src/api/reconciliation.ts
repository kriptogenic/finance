import { api, call } from './client'
import type { ReconciliationReport } from './types'

export const reconciliationApi = {
  list: (): Promise<ReconciliationReport> => call(api.GET('/reconciliation', {})),
}
