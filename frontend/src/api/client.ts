import createClient from 'openapi-fetch'
import type { paths } from './schema'

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

// call unwraps an openapi-fetch result, returning the typed body or throwing an
// ApiError carrying the backend's { error } message.
export async function call<T>(
  promise: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await promise
  if (!response.ok || error) {
    const message = (error as { error?: string } | undefined)?.error ?? `HTTP ${response.status}`
    throw new ApiError(message, response.status)
  }
  return data as T
}

export const errMessage = (e: unknown): string => (e instanceof Error ? e.message : String(e))
