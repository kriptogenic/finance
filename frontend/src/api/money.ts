// Money value object — the openapi-typescript analog of the backend's
// `x-go-type: money.Money`. The generator (scripts/gen-api.mjs) maps every schema
// carrying `x-go-type: money.Money` to this class, and the API client hydrates
// money-shaped JSON ({ amount, currency }) into instances. Backed by dinero.js;
// the value stays integer minor units (never floating point).
//
// The class keeps only public fields/methods (no private members) so it stays
// structurally compatible with openapi-fetch's mapped response types.
import * as dn from 'dinero.js'
import type { Dinero, DineroCurrency } from 'dinero.js'
import { hideMinorUnits } from '../lib/settings'

const registry = dn as unknown as Record<string, DineroCurrency<number> | undefined>

function currencyOf(code: string): DineroCurrency<number> {
  return registry[code] ?? { code, base: 10, exponent: 2 }
}

/** Number of fractional digits for a currency (e.g. 2 for USD, 0 for JPY). */
export function exponentOf(code: string): number {
  return currencyOf(code).exponent
}

// dinero instance for a Money, and back — kept as module helpers so the Money
// class exposes no private members.
function toDinero(m: Money): Dinero<number> {
  return dn.dinero({ amount: m.amount, currency: currencyOf(m.currency) })
}

function fromDinero(d: Dinero<number>): Money {
  const snap = dn.toSnapshot(d)
  return new Money(snap.amount, snap.currency.code)
}

// "1234.56" -> "1 234,56" (space thousands, comma decimal)
function styleDecimal(decimal: string): string {
  const negative = decimal.startsWith('-')
  const abs = negative ? decimal.slice(1) : decimal
  const [int, frac] = abs.split('.')
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

  return (negative ? '-' : '') + grouped + (frac ? ',' + frac : '')
}

export class Money {
  constructor(
    readonly amount: number,
    readonly currency: string,
  ) {}

  static of(amount: number, currency: string): Money {
    return new Money(amount, currency)
  }

  /** Hydrate a money-shaped JSON value into a Money instance. */
  static from(value: { amount: number; currency: string }): Money {
    return new Money(value.amount, value.currency)
  }

  // --- arithmetic (same currency) ---
  add(other: Money): Money {
    return fromDinero(dn.add(toDinero(this), toDinero(other)))
  }

  sub(other: Money): Money {
    return fromDinero(dn.subtract(toDinero(this), toDinero(other)))
  }

  negate(): Money {
    return fromDinero(dn.multiply(toDinero(this), -1))
  }

  abs(): Money {
    return this.isNegative() ? this.negate() : this
  }

  // --- comparisons ---
  isNegative(): boolean {
    return dn.isNegative(toDinero(this))
  }

  isZero(): boolean {
    return dn.isZero(toDinero(this))
  }

  isPositive(): boolean {
    return dn.isPositive(toDinero(this))
  }

  lessThan(other: Money): boolean {
    return dn.lessThan(toDinero(this), toDinero(other))
  }

  greaterThan(other: Money): boolean {
    return dn.greaterThan(toDinero(this), toDinero(other))
  }

  equals(other: Money): boolean {
    return dn.equal(toDinero(this), toDinero(other))
  }

  // --- conversion / formatting ---
  /** Major units as a number, for form inputs. */
  toMajor(): number {
    return this.amount / 10 ** exponentOf(this.currency)
  }

  /** "1 000,12 UZS", or "1 000 UZS" when the hide-minor-units pref is on. */
  format(): string {
    if (hideMinorUnits.value) return this.formatShort()
    return styleDecimal(dn.toDecimal(toDinero(this))) + ' ' + this.currency
  }

  /** Rounded to whole units: "1 000 UZS" */
  formatShort(): string {
    const whole = dn.transformScale(toDinero(this), 0, dn.halfAwayFromZero)

    return styleDecimal(dn.toDecimal(whole)) + ' ' + this.currency
  }

  toJSON(): { amount: number; currency: string } {
    return { amount: this.amount, currency: this.currency }
  }
}
