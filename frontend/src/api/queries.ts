import { useQuery, type QueryClient } from '@tanstack/vue-query'
import type { MaybeRefOrGetter } from 'vue'
import { toValue } from 'vue'
import { accountsApi } from './accounts'
import { categoriesApi } from './categories'
import { budgetsApi } from './budgets'
import { reportsApi, type DateRange } from './reports'
import { receiptsApi } from './receipts'
import { scheduledTransactionsApi } from './scheduledTransactions'
import { transactionsApi, type TransactionFilter } from './transactions'

// Central query definitions. Global default is staleTime: 0 (see main.ts), so
// every navigation shows cached data instantly and refetches in the background.

// Static defs — shared by the composables and prefetch so cache keys match.
export const accountsDef = (includeArchived = false) => ({
  queryKey: ['accounts', includeArchived],
  queryFn: () => accountsApi.list(includeArchived),
})
export const categoriesDef = (query: { type?: string; include_archived?: boolean } = {}) => ({
  queryKey: ['categories', query],
  queryFn: () => categoriesApi.list(query as never),
})
export const transactionsDef = (filter: TransactionFilter) => ({
  queryKey: ['transactions', filter],
  queryFn: () => transactionsApi.list(filter),
})

export function useAccountsQuery(includeArchived = false) {
  return useQuery(accountsDef(includeArchived))
}

export function useCategoriesQuery(query: { type?: string; include_archived?: boolean } = {}) {
  return useQuery(categoriesDef(query))
}

export function useNetWorthQuery() {
  return useQuery({ queryKey: ['net-worth'], queryFn: () => reportsApi.netWorth() })
}

export function useSpendingQuery(params: MaybeRefOrGetter<DateRange>) {
  return useQuery({
    queryKey: ['spending', params],
    queryFn: () => reportsApi.spending(toValue(params)),
  })
}

export function useCashFlowQuery(params: MaybeRefOrGetter<DateRange>) {
  return useQuery({
    queryKey: ['cash-flow', params],
    queryFn: () => reportsApi.cashFlow(toValue(params)),
  })
}

export function useForecastQuery(month: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: ['forecast', month],
    queryFn: () => reportsApi.forecast(toValue(month)),
  })
}

export function useBudgetsQuery() {
  return useQuery({ queryKey: ['budgets'], queryFn: () => budgetsApi.list() })
}

export function useSchedulesQuery() {
  return useQuery({ queryKey: ['schedules'], queryFn: () => scheduledTransactionsApi.list() })
}

export function useReceiptsQuery(page: MaybeRefOrGetter<number>) {
  return useQuery({
    queryKey: ['receipts', page],
    queryFn: () => receiptsApi.list(toValue(page)),
  })
}

export function useTransactionsQuery(filter: MaybeRefOrGetter<TransactionFilter>) {
  return useQuery({
    queryKey: ['transactions', filter],
    queryFn: () => transactionsApi.list(toValue(filter)),
  })
}

// Warm the Accounts + Transactions pages (their list data + the meta they need
// to render) so navigating there from the Dashboard paints instantly. Runs in
// the background; failures are ignored (the page will fetch normally on visit).
export function prefetchAccountsAndTransactions(qc: QueryClient) {
  const warm = <T>(p: Promise<T>) => void p.catch(() => {})
  warm(qc.prefetchQuery(accountsDef(false))) // Accounts page
  warm(qc.prefetchQuery(accountsDef(true))) // Transactions needs archived accounts for names
  warm(qc.prefetchQuery(categoriesDef({}))) // Accounts page
  warm(qc.prefetchQuery(categoriesDef({ include_archived: true }))) // Transactions names
  warm(qc.prefetchQuery(transactionsDef({ limit: 200 }))) // Transactions default list
}
