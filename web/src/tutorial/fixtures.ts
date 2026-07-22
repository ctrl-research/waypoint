import type { ItineraryItem, ItineraryLayer, Stop, Trip, TripDetail } from '../api'

/**
 * Demo data served while the tour runs (#96): a small in-progress trip so
 * every screen has something to show. Reads are answered from here and
 * writes are refused, so the tour is stateless — nothing touches live data.
 */
export const DEMO_TRIP_ID = 'tour-demo'

const day = (offset: number): string => {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function demoTrip(): Trip {
  return {
    id: DEMO_TRIP_ID,
    title: 'Japan (demo)',
    description: 'A sample trip that exists only during the tour.',
    status: 'active',
    startDate: day(-2),
    endDate: day(5),
    coverPhoto: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    role: 'owner',
    effectiveStatus: 'in-progress',
    cities: [
      { name: 'Tokyo', lat: 35.6762, lon: 139.6503 },
      { name: 'Kyoto', lat: 35.0116, lon: 135.7681 },
    ],
  }
}

const areas: Stop[] = [
  {
    id: 'tour-area-tokyo', name: 'Tokyo', lat: 35.6762, lon: 139.6503,
    arrivalDate: day(-2), departureDate: day(1), position: 0, notes: '', kind: 'city',
  },
  {
    id: 'tour-area-kyoto', name: 'Kyoto', lat: 35.0116, lon: 135.7681,
    arrivalDate: day(1), departureDate: day(5), position: 1, notes: '', kind: 'city',
  },
]

const layers: ItineraryLayer[] = [
  { id: 'tour-layer-main', name: 'Main', color: '#2a78d6', ownerId: null, visible: true },
  { id: 'tour-layer-food', name: 'Food ideas', color: '#d97706', ownerId: 'tour-user', visible: true },
]

const baseItem = {
  destinationStopId: null, originHomeId: null, destinationHomeId: null,
  endTime: null, notes: '', costCents: null, currency: null,
  destinationAddress: '', destinationLat: null, destinationLon: null,
  confirmationCode: null,
} as const

const items: ItineraryItem[] = [
  {
    ...baseItem, id: 'tour-item-teamlab', stopId: 'tour-area-tokyo', day: day(-1),
    startTime: '10:00', title: 'teamLab Planets', category: 'activity',
    address: 'Toyosu, Tokyo', lat: 35.6493, lon: 139.7891, layerId: 'tour-layer-main', position: 0,
  },
  {
    ...baseItem, id: 'tour-item-train', stopId: 'tour-area-tokyo', destinationStopId: 'tour-area-kyoto',
    day: day(1), startTime: '09:30', endTime: '11:45', title: 'Shinkansen to Kyoto', category: 'train',
    address: 'Tokyo Station', lat: 35.6812, lon: 139.7671, layerId: 'tour-layer-main', position: 0,
    destinationAddress: 'Kyoto Station', destinationLat: 34.9855, destinationLon: 135.7585,
  },
  {
    ...baseItem, id: 'tour-item-inari', stopId: 'tour-area-kyoto', day: day(2),
    startTime: '09:00', title: 'Fushimi Inari shrine', category: 'activity',
    address: 'Fushimi Ward, Kyoto', lat: 34.9671, lon: 135.7727, layerId: 'tour-layer-main', position: 0,
  },
  {
    ...baseItem, id: 'tour-item-ramen', stopId: 'tour-area-kyoto', day: day(2),
    startTime: '12:30', title: 'Ramen alley lunch', category: 'food',
    address: 'Kyoto Station 10F', lat: 34.9858, lon: 135.7588, layerId: 'tour-layer-food', position: 1,
  },
]

function demoDetail(): TripDetail {
  return { trip: demoTrip(), stops: areas, items, homes: [], layers }
}

/**
 * Answers an /api/v1 request from the demo data. Returning undefined lets
 * the request pass through (config, session); non-GET writes are refused.
 */
export function tutorialResponse(path: string, method: string): unknown {
  if (!path.startsWith('/api/v1/')) return undefined
  if (path.startsWith('/api/v1/config')) return undefined
  if (method !== 'GET') {
    throw new Error('The tour is read-only — finish or exit it to make changes.')
  }
  if (path === '/api/v1/trips') return { trips: [demoTrip()] }
  if (path.startsWith(`/api/v1/trips/${DEMO_TRIP_ID}/journal`)) return { entries: [] }
  if (path.startsWith(`/api/v1/trips/${DEMO_TRIP_ID}/members`)) return { members: [] }
  if (path.startsWith(`/api/v1/trips/${DEMO_TRIP_ID}/shares`)) return { shares: [] }
  if (path.startsWith(`/api/v1/trips/${DEMO_TRIP_ID}`)) return demoDetail()
  if (path.startsWith('/api/v1/homes')) return { homes: [] }
  if (path.startsWith('/api/v1/geocode')) return { results: [] }
  if (path.startsWith('/api/v1/calendar/token')) return { token: null }
  if (path.startsWith('/api/v1/mcp/token')) return { token: null }
  if (path.startsWith('/api/v1/stats')) {
    return {
      totals: {
        trips: 1, planning: 0, active: 1, completed: 0,
        daysOnRoad: 3, daysOnRoadPlanned: 5,
        traveledDistanceKm: 370, plannedDistanceKm: 0,
        cities: 2, citiesPlanned: 0,
      },
      flights: { count: 0, distanceKm: 0, minutes: 0 },
      trains: { count: 1, distanceKm: 370, minutes: 135 },
      tripsPerYear: [{ year: new Date().getFullYear(), travelled: 1, planned: 0 }],
      stops: [
        { name: 'Tokyo', lat: 35.6762, lon: 139.6503, tripTitle: 'Japan (demo)', travelled: true },
        { name: 'Kyoto', lat: 35.0116, lon: 135.7681, tripTitle: 'Japan (demo)', travelled: true },
      ],
    }
  }
  // Any other read during the tour has no demo answer; an empty object is
  // safer than leaking live data into the sandboxed view.
  return {}
}
