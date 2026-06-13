import type { Money } from '../api/types'

// Money values come from the API as { amount: <int64 minor units>, currency }.
// We render major units (minor / 100) with the currency code.
const MINOR = 100

export function formatMoney(money: Money | null | undefined): string {
  if (!money) return ''
  return formatMinor(money.amount, money.currency)
}

// formatMoneyShort drops the fractional part — for big summary numbers where
// cents are just noise.
export function formatMoneyShort(money: Money | null | undefined): string {
  if (!money) return ''
  const value = (money.amount ?? 0) / MINOR
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: money.currency,
      maximumFractionDigits: 0,
    }).format(value)
  } catch {
    return `${Math.round(value).toLocaleString()} ${money.currency}`
  }
}

export function formatMinor(amount: number, currency: string): string {
  const value = (amount ?? 0) / MINOR
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      maximumFractionDigits: 2,
    }).format(value)
  } catch {
    return `${value.toLocaleString()} ${currency}`
  }
}

export function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString()
}

// major (user-facing) <-> minor (API) units
export function toMinor(major: number | string | null | undefined): number {
  return Math.round(Number(major || 0) * MINOR)
}

export function toMajor(minor: number | null | undefined): number {
  return (minor ?? 0) / MINOR
}

// ISO timestamp -> value for <input type="datetime-local">
export function toLocalInput(iso?: string): string {
  const d = iso ? new Date(iso) : new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
