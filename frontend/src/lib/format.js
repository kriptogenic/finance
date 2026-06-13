// Money values come from the API as { amount: <int64 minor units>, currency }.
// We render major units (minor / 100) with the currency code.
const MINOR = 100

export function formatMoney(money) {
  if (!money) return ''
  return formatMinor(money.amount, money.currency)
}

// formatMoneyShort drops the fractional part — for big summary numbers where
// cents are just noise.
export function formatMoneyShort(money) {
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

export function formatMinor(amount, currency) {
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

export function formatDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString()
}

// major (user-facing) <-> minor (API) units
export function toMinor(major) {
  return Math.round(Number(major || 0) * MINOR)
}

export function toMajor(minor) {
  return (minor ?? 0) / MINOR
}

// ISO timestamp -> value for <input type="datetime-local">
export function toLocalInput(iso) {
  const d = iso ? new Date(iso) : new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
