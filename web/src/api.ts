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

// ---- trips ------------------------------------------------------------------

export type TripStatus = 'planning' | 'active' | 'completed'

export type Trip = {
  id: string
  title: string
  description: string
  status: TripStatus
  startDate: string | null
  endDate: string | null
  coverPhoto: string | null
  createdAt: string
  updatedAt: string
}

export type Stop = {
  id: string
  name: string
  lat: number | null
  lon: number | null
  arrivalDate: string | null
  departureDate: string | null
  position: number
  notes: string
}

export type ItineraryCategory = 'activity' | 'food' | 'lodging' | 'transport' | 'other'

export type ItineraryItem = {
  id: string
  stopId: string | null
  day: string
  startTime: string | null
  title: string
  category: ItineraryCategory
  notes: string
  costCents: number | null
  currency: string | null
  position: number
}

export type TripDetail = { trip: Trip; stops: Stop[]; items: ItineraryItem[] }

/** Fields for creating/patching; dates are "YYYY-MM-DD", "" clears. */
export type TripInput = Partial<{
  title: string
  description: string
  status: TripStatus
  startDate: string
  endDate: string
}>

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  })
  if (!res.ok) await throwApiError(res)
  return res.status === 204 ? (undefined as T) : res.json()
}

export async function listTrips(): Promise<Trip[]> {
  const body = await requestJSON<{ trips: Trip[] }>('/api/v1/trips')
  return body.trips
}

export function getTrip(id: string): Promise<TripDetail> {
  return requestJSON(`/api/v1/trips/${id}`)
}

export function createTrip(input: TripInput): Promise<Trip> {
  return requestJSON('/api/v1/trips', { method: 'POST', body: JSON.stringify(input) })
}

export function updateTrip(id: string, input: TripInput): Promise<Trip> {
  return requestJSON(`/api/v1/trips/${id}`, { method: 'PATCH', body: JSON.stringify(input) })
}

export function deleteTrip(id: string): Promise<void> {
  return requestJSON(`/api/v1/trips/${id}`, { method: 'DELETE' })
}

export type StopInput = Partial<{
  name: string
  lat: number
  lon: number
  arrivalDate: string
  departureDate: string
  notes: string
}>

export function createStop(tripId: string, input: StopInput): Promise<Stop> {
  return requestJSON(`/api/v1/trips/${tripId}/stops`, { method: 'POST', body: JSON.stringify(input) })
}

export function deleteStop(tripId: string, stopId: string): Promise<void> {
  return requestJSON(`/api/v1/trips/${tripId}/stops/${stopId}`, { method: 'DELETE' })
}

export type ItemInput = Partial<{
  stopId: string
  day: string
  startTime: string
  title: string
  category: ItineraryCategory
  notes: string
  costCents: number
  currency: string
}>

export function createItem(tripId: string, input: ItemInput): Promise<ItineraryItem> {
  return requestJSON(`/api/v1/trips/${tripId}/items`, { method: 'POST', body: JSON.stringify(input) })
}

export function deleteItem(tripId: string, itemId: string): Promise<void> {
  return requestJSON(`/api/v1/trips/${tripId}/items/${itemId}`, { method: 'DELETE' })
}

// ---- geocoding ---------------------------------------------------------------

export type GeocodeResult = { name: string; lat: number; lon: number }

export async function geocode(q: string): Promise<GeocodeResult[]> {
  const body = await requestJSON<{ results: GeocodeResult[] }>(
    `/api/v1/geocode?q=${encodeURIComponent(q)}`,
  )
  return body.results
}
