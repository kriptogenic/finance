// Single-credential HTTP Basic auth for the API. The login/password are the
// core service's AUTH_USERNAME/AUTH_PASSWORD; we keep the base64 token in
// localStorage and send it as `Authorization: Basic …` on every request.

const STORAGE_KEY = 'finance.auth'

export const apiBaseUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export function isAuthenticated(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== null
}

export function authHeader(): string | null {
  const token = localStorage.getItem(STORAGE_KEY)
  return token ? `Basic ${token}` : null
}

export function setCredentials(username: string, password: string): void {
  localStorage.setItem(STORAGE_KEY, btoa(`${username}:${password}`))
}

export function clearCredentials(): void {
  localStorage.removeItem(STORAGE_KEY)
}

// verifyCredentials checks login/password against an authenticated endpoint with
// a raw fetch (bypassing the shared client so the global 401 handler doesn't
// fire mid-login). Returns false on bad credentials, throws on other failures.
export async function verifyCredentials(username: string, password: string): Promise<boolean> {
  const res = await fetch(`${apiBaseUrl}/accounts`, {
    headers: { Authorization: `Basic ${btoa(`${username}:${password}`)}` },
  })
  if (res.ok) return true
  if (res.status === 401) return false
  throw new Error(`HTTP ${res.status}`)
}
