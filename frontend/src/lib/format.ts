import { Money, exponentOf } from '../api/money'

// Money-typed API fields are hydrated into Money instances by the client, so use
// their methods (.format(), .isNegative(), ...). These helpers cover the rest:
// raw minor-unit integers and form input conversion.

// formatMinor formats a bare minor-unit amount + currency code (e.g. the
// per-currency exposure figures, which aren't Money objects on the wire).
export function formatMinor(amount: number, currency: string): string {
  return Money.of(amount, currency).format()
}

// exponent-aware unit conversion for form inputs
export function toMinor(major: number | string | null | undefined, currency: string): number {
  return Math.round(Number(major || 0) * 10 ** exponentOf(currency))
}

export function toMajor(minor: number | null | undefined, currency: string): number {
  return Money.of(minor ?? 0, currency).toMajor()
}

export function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString()
}

export function toLocalInput(iso?: string): string {
  const d = iso ? new Date(iso) : new Date()
  const pad = (n: number) => String(n).padStart(2, '0')

  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
