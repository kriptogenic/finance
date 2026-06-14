import createClient from 'openapi-fetch'
import type { paths } from './schema'
import { Money } from './money'

export const api = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_URL || 'http://localhost:8080',
})

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

// reviveMoney walks a parsed response and replaces every money-shaped object
// ({ amount: number, currency: string }) with a Money value object, so callers
// get instances with .format()/.isNegative()/.add()/... rather than plain data.
function reviveMoney(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(reviveMoney)
  }
  if (value !== null && typeof value === 'object') {
    const obj = value as Record<string, unknown>
    const keys = Object.keys(obj)
    if (keys.length === 2 && typeof obj.amount === 'number' && typeof obj.currency === 'string') {
      return Money.from({ amount: obj.amount, currency: obj.currency })
    }
    const out: Record<string, unknown> = {}
    for (const key of keys) {
      out[key] = reviveMoney(obj[key])
    }
    return out
  }

  return value
}

// call unwraps an openapi-fetch result, hydrating money fields and throwing an
// ApiError carrying the backend's { error } message.
export async function call<T>(
  promise: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await promise
  if (!response.ok || error) {
    const message = (error as { error?: string } | undefined)?.error ?? `HTTP ${response.status}`
    throw new ApiError(message, response.status)
  }

  return reviveMoney(data) as T
}

export const errMessage = (e: unknown): string => (e instanceof Error ? e.message : String(e))
