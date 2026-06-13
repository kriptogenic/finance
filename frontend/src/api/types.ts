import type { components } from './schema'

type S = components['schemas']

export type Money = S['Money']
export type Account = S['Account']
export type Category = S['Category']
export type Transaction = S['Transaction']
export type NetWorthReport = S['NetWorthReport']
export type SpendingReport = S['SpendingReport']
export type CashFlowReport = S['CashFlowReport']

export type CreateAccountRequest = S['CreateAccountRequest']
export type UpdateAccountRequest = S['UpdateAccountRequest']
export type CreateCategoryRequest = S['CreateCategoryRequest']
export type UpdateCategoryRequest = S['UpdateCategoryRequest']
export type CreateTransactionRequest = S['CreateTransactionRequest']

export type AccountKind = S['AccountKind']
export type AccountType = S['AccountType']
export type CategoryType = S['CategoryType']
export type TransactionType = S['TransactionType']
