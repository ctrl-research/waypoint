export type Me = {
  id: string
  email: string
  displayName: string
  avatarUrl: string | null
  isAdmin: boolean
}

export class ApiError extends Error {
  code: string

  constructor(code: string, message: string) {
    super(message)
    this.code = code
  }
}

async function throwApiError(res: Response): Promise<never> {
  let code = 'unknown'
  let message = `request failed (${res.status})`
  try {
    const body = await res.json()
    if (body?.error) {
      code = body.error.code ?? code
      message = body.error.message ?? message
    }
  } catch {
    // non-JSON error body; keep defaults
  }
  throw new ApiError(code, message)
}

/** Returns the signed-in user, or null when there is no session. */
export async function fetchMe(): Promise<Me | null> {
  const res = await fetch('/api/v1/me')
  if (res.status === 401) return null
  if (!res.ok) await throwApiError(res)
  return res.json()
}

export async function fetchProviders(): Promise<string[]> {
  const res = await fetch('/auth/providers')
  if (!res.ok) await throwApiError(res)
  const body = await res.json()
  return body.providers
}

export async function login(email: string, password: string): Promise<void> {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) await throwApiError(res)
}

export async function logout(): Promise<void> {
  const res = await fetch('/auth/logout', { method: 'POST' })
  if (!res.ok) await throwApiError(res)
}
