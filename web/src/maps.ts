import type { ItineraryItem } from './api'

/**
 * External map-app link for an item's venue (Google Maps universal URLs —
 * they hand off to the native app on iOS/Android when installed). The
 * address is the query when present so the app opens a named place;
 * otherwise the exact coordinates.
 */
export function mapsLink(item: Pick<ItineraryItem, 'address' | 'lat' | 'lon'>): string | null {
  if (item.address) {
    return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(item.address)}`
  }
  if (item.lat !== null && item.lon !== null) {
    return `https://www.google.com/maps/search/?api=1&query=${item.lat}%2C${item.lon}`
  }
  return null
}
