import * as dn from 'dinero.js'
import type { DineroCurrency, Dinero } from 'dinero.js'

import type { Money } from '../api/types'

// dinero.js bundles the ISO-4217 currency objects (code/base/exponent) and all
// money operations. We look a currency up by code, falling back to a 2-decimal
// default for anything dinero doesn't know.
const registry = dn as unknown as Record<string, DineroCurrency<number> | undefined>

function currencyOf(code: string): DineroCurrency<number> {
  return registry[code] ?? { code, base: 10, exponent: 2 }
}

export function exponentOf(code: string): number {
  return currencyOf(code).exponent
}

function toDinero(money: Money): Dinero<number> {
  return dn.dinero({ amount: money.amount, currency: currencyOf(money.currency) })
}

function toMoney(d: Dinero<number>): Money {
  const snap = dn.toSnapshot(d)
  return { amount: snap.amount, currency: snap.currency.code }
}

// --- formatting: "1 000,12 CUR" (space thousands, comma decimal) ----------

function styleDecimal(decimal: string): string {
  const negative = decimal.startsWith('-')
  const abs = negative ? decimal.slice(1) : decimal
  const [int, frac] = abs.split('.')
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

  return (negative ? '-' : '') + grouped + (frac ? ',' + frac : '')
}

export function formatMoney(money: Money | null | undefined): string {
  if (!money) return ''
  return styleDecimal(dn.toDecimal(toDinero(money))) + ' ' + money.currency
}

export function formatMinor(amount: number, currency: string): string {
  return formatMoney({ amount, currency })
}

// formatMoneyShort rounds to whole units (no minor part) for big summary numbers.
export function formatMoneyShort(money: Money | null | undefined): string {
  if (!money) return ''
  const whole = dn.transformScale(toDinero(money), 0, dn.halfAwayFromZero)

  return styleDecimal(dn.toDecimal(whole)) + ' ' + money.currency
}

// --- comparisons / arithmetic via dinero ---------------------------------

export function isNegative(money: Money): boolean {
  return dn.isNegative(toDinero(money))
}

export function absolute(money: Money): Money {
  return isNegative(money) ? toMoney(dn.multiply(toDinero(money), -1)) : money
}

// --- exponent-aware unit conversion for form input -----------------------

export function toMinor(major: number | string | null | undefined, currency: string): number {
  return Math.round(Number(major || 0) * 10 ** exponentOf(currency))
}

export function toMajor(minor: number | null | undefined, currency: string): number {
  return (minor ?? 0) / 10 ** exponentOf(currency)
}

// --- dates ---------------------------------------------------------------

export function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString()
}

export function toLocalInput(iso?: string): string {
  const d = iso ? new Date(iso) : new Date()
  const pad = (n: number) => String(n).padStart(2, '0')

  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
