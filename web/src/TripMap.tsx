import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { fetchConfig, type ItineraryItem, type Stop } from './api'
import { EyeIcon, categoryIcons } from './icons'
import { mapsLink } from './maps'
import { localizeMapLabels, mapStyle, type MapSourceConfig } from './mapstyle'

const ROUTE_SOURCE = 'route'
const REPLAY_SOURCE = 'replay'

/** Key for map markers and hover-highlighting: stop:<id> or item:<id>. */
export type MarkerKey = `stop:${string}` | `item:${string}`

/** One chronological point of the trip's replay animation (#62). */
export type ReplayPoint = { lat: number; lon: number; label: string; day: string }

/**
 * TripMap renders the trip's stops as S-numbered markers connected by a
 * route line, plus category-icon pins for itinerary items at their venue
 * or stop (#72, #73).
 * `highlightKey` enlarges the hovered list row's marker (#71).
 */
export function TripMap({
  stops,
  items = [],
  highlightKey = null,
  mapConfig,
  layerColors,
  replay,
}: {
  stops: Stop[]
  items?: ItineraryItem[]
  highlightKey?: MarkerKey | null
  /** Item pin color per layerId (#73 slice 2); unlisted layers stay indigo. */
  layerColors?: Record<string, string>
  /** Overrides the authed /api/v1/config lookup (used by the public page). */
  mapConfig?: MapSourceConfig
  /** Chronological points enabling the ▶ Replay control (#62). */
  replay?: ReplayPoint[]
}) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const markersRef = useRef<Map<string, maplibregl.Marker>>(new Map())

  const { data: fetched } = useQuery({
    queryKey: ['config'],
    queryFn: fetchConfig,
    staleTime: Infinity,
    enabled: !mapConfig,
  })
  const cfg = mapConfig ?? fetched

  // Create the map once the source config is known.
  useEffect(() => {
    if (!cfg || !container.current || mapRef.current) return

    const map = new maplibregl.Map({
      container: container.current,
      style: mapStyle(cfg),
      center: [0, 20],
      zoom: 1,
    })
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }))

    map.on('load', () => {
      localizeMapLabels(map, cfg)
      map.addSource(ROUTE_SOURCE, {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      })
      map.addLayer({
        id: ROUTE_SOURCE,
        type: 'line',
        source: ROUTE_SOURCE,
        paint: { 'line-color': '#0f172a', 'line-width': 2, 'line-dasharray': [2, 1.5] },
      })
      map.addSource(REPLAY_SOURCE, {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      })
      map.addLayer({
        id: REPLAY_SOURCE,
        type: 'line',
        source: REPLAY_SOURCE,
        paint: { 'line-color': '#2a78d6', 'line-width': 3 },
      })
      mapRef.current = map
      syncMap(map, stopsRef.current, itemsRef.current, markersRef, layerColorsRef.current)
    })

    return () => {
      mapRef.current = null
      map.remove()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cfg])

  // Keep markers and route in sync with the data.
  const stopsRef = useRef(stops)
  const itemsRef = useRef(items)
  const layerColorsRef = useRef(layerColors)
  stopsRef.current = stops
  itemsRef.current = items
  layerColorsRef.current = layerColors
  useEffect(() => {
    if (mapRef.current) syncMap(mapRef.current, stops, items, markersRef, layerColors)
  }, [stops, items, layerColors])

  // Hover-highlight from the lists (#71): the hovered marker pops, every
  // other POI fades back so the emphasis is unmistakable.
  useEffect(() => {
    for (const [key, marker] of markersRef.current) {
      const el = marker.getElement().firstElementChild as HTMLElement | null
      if (!el) continue
      const active = key === highlightKey
      el.classList.toggle('scale-125', active)
      el.classList.toggle('ring-2', active)
      el.classList.toggle('ring-sky-400', active)
      el.classList.toggle('z-10', active)
      el.classList.toggle('opacity-40', highlightKey !== null && !active)
    }
  }, [highlightKey])

  // Layer visibility (default: everything shown).
  const [showStops, setShowStops] = useState(true)
  const [showItems, setShowItems] = useState(true)
  useEffect(() => {
    for (const [key, marker] of markersRef.current) {
      const visible = key.startsWith('stop:') ? showStops : showItems
      marker.getElement().style.display = visible ? '' : 'none'
    }
    const map = mapRef.current
    if (map?.getLayer(ROUTE_SOURCE)) {
      map.setLayoutProperty(ROUTE_SOURCE, 'visibility', showStops ? 'visible' : 'none')
    }
  }, [showStops, showItems, stops, items])

  // ---- Replay (#62): fly the camera point to point while the path draws.
  const [replaying, setReplaying] = useState(false)
  const [caption, setCaption] = useState<ReplayPoint | null>(null)
  const cancelRef = useRef(false)
  const replayMarker = useRef<maplibregl.Marker | null>(null)

  const stopReplay = () => {
    cancelRef.current = true
    setReplaying(false)
    setCaption(null)
    replayMarker.current?.remove()
    replayMarker.current = null
    const src = mapRef.current?.getSource(REPLAY_SOURCE) as maplibregl.GeoJSONSource | undefined
    src?.setData({ type: 'FeatureCollection', features: [] })
  }
  useEffect(() => () => stopReplay(), []) // eslint-disable-line react-hooks/exhaustive-deps

  async function startReplay() {
    const map = mapRef.current
    if (!map || !replay || replay.length < 2 || replaying) return
    // Consecutive items at the same spot draw nothing — collapse them.
    const pts = replay.filter(
      (p, i) => i === 0 || p.lat !== replay[i - 1].lat || p.lon !== replay[i - 1].lon,
    )
    if (pts.length < 2) return
    cancelRef.current = false
    setReplaying(true)

    const el = document.createElement('div')
    el.className = 'h-3.5 w-3.5 rounded-full bg-[#2a78d6] ring-2 ring-white shadow'
    replayMarker.current = new maplibregl.Marker({ element: el })
      .setLngLat([pts[0].lon, pts[0].lat])
      .addTo(map)

    const src = map.getSource(REPLAY_SOURCE) as maplibregl.GeoJSONSource
    const done: [number, number][] = [[pts[0].lon, pts[0].lat]]
    setCaption(pts[0])
    map.easeTo({ center: [pts[0].lon, pts[0].lat], zoom: Math.max(map.getZoom(), 5), duration: 900 })
    await wait(1000)

    for (let i = 1; i < pts.length && !cancelRef.current; i++) {
      const from = pts[i - 1]
      const to = pts[i]
      setCaption(to)
      const km = haversineKm(from.lat, from.lon, to.lat, to.lon)
      const duration = Math.min(2600, Math.max(700, km * 6))
      map.easeTo({ center: [to.lon, to.lat], duration, easing: (t) => t })
      await animate(duration, (t) => {
        const lon = from.lon + (to.lon - from.lon) * t
        const lat = from.lat + (to.lat - from.lat) * t
        replayMarker.current?.setLngLat([lon, lat])
        src.setData({
          type: 'Feature',
          properties: {},
          geometry: { type: 'LineString', coordinates: [...done, [lon, lat]] },
        })
      })
      done.push([to.lon, to.lat])
      await wait(350)
    }
    if (!cancelRef.current) {
      await wait(1600)
      stopReplay()
    }
  }

  function animate(duration: number, frame: (t: number) => void): Promise<void> {
    return new Promise((resolve) => {
      const start = performance.now()
      const tick = (now: number) => {
        if (cancelRef.current) return resolve()
        const t = Math.min(1, (now - start) / duration)
        frame(t)
        if (t < 1) requestAnimationFrame(tick)
        else resolve()
      }
      requestAnimationFrame(tick)
    })
  }
  const wait = (ms: number) =>
    new Promise<void>((resolve) => {
      if (cancelRef.current) return resolve()
      window.setTimeout(resolve, ms)
    })

  return (
    <div className="relative">
      <div ref={container} className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-700" />
      {replay && replay.length >= 2 && (
        <button
          type="button"
          onClick={() => (replaying ? stopReplay() : startReplay())}
          className="absolute bottom-3 left-3 rounded-lg bg-white/90 px-3 py-1.5 text-xs font-medium text-slate-900 shadow hover:bg-white"
        >
          {replaying ? '⏹ Stop' : '▶ Replay'}
        </button>
      )}
      {caption && (
        <div className="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full bg-slate-900/90 px-4 py-1.5 text-sm text-white shadow">
          {caption.day
            ? `${new Date(caption.day + 'T00:00:00').toLocaleDateString(undefined, {
                month: 'short',
                day: 'numeric',
              })} · ${caption.label}`
            : caption.label}
        </div>
      )}
      <div className="absolute left-3 top-3 flex gap-1 rounded-lg bg-white/90 p-1 text-xs shadow">
        {(
          [
            ['Stops', showStops, setShowStops],
            ['Items', showItems, setShowItems],
          ] as const
        ).map(([label, on, set]) => (
          <button
            key={label}
            type="button"
            onClick={() => set(!on)}
            className={`flex items-center gap-1 rounded-md px-2 py-1 hover:bg-slate-100 ${on ? 'text-slate-900' : 'text-slate-400'}`}
            title={on ? `Hide ${label.toLowerCase()}` : `Show ${label.toLowerCase()}`}
            aria-pressed={on}
          >
            <EyeIcon open={on} />
            {label}
          </button>
        ))}
      </div>
    </div>
  )
}

function haversineKm(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const rad = (d: number) => (d * Math.PI) / 180
  const dLat = rad(lat2 - lat1)
  const dLon = rad(lon2 - lon1)
  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(rad(lat1)) * Math.cos(rad(lat2)) * Math.sin(dLon / 2) ** 2
  return 2 * 6371 * Math.asin(Math.sqrt(a))
}

function makeMarkerEl(label: string, kind: 'stop' | 'item', color?: string): HTMLElement {
  // Outer wrapper stays untransformed (MapLibre positions it); the inner
  // element carries styling so hover effects can scale it safely.
  const wrap = document.createElement('div')
  const el = document.createElement('div')
  el.className =
    kind === 'stop'
      ? 'flex h-7 min-w-7 items-center justify-center rounded-full bg-slate-900 px-1 text-xs font-semibold text-white shadow-md transition'
      : 'flex h-6 w-6 items-center justify-center rounded-full bg-indigo-600 text-[11px] text-white shadow transition'
  if (color) el.style.backgroundColor = color
  el.textContent = label
  wrap.appendChild(el)
  return wrap
}

/** Popup content: title plus the venue as a maps-app link (addresses are
 * user data — build DOM nodes, never HTML strings). */
function itemPopup(item: ItineraryItem, where: string): HTMLElement {
  const el = document.createElement('div')
  el.appendChild(document.createTextNode(where ? `${item.title} — ` : item.title))
  const href = item.address ? mapsLink(item) : null
  if (where && href) {
    const a = document.createElement('a')
    a.href = href
    a.target = '_blank'
    a.rel = 'noopener noreferrer'
    a.textContent = `📍 ${where}`
    a.style.textDecoration = 'underline'
    el.appendChild(a)
  } else if (where) {
    el.appendChild(document.createTextNode(where))
  }
  return el
}

function syncMap(
  map: maplibregl.Map,
  stops: Stop[],
  items: ItineraryItem[],
  markersRef: React.MutableRefObject<Map<string, maplibregl.Marker>>,
  layerColors?: Record<string, string>,
) {
  const located = stops.filter((s) => s.lat !== null && s.lon !== null)

  for (const m of markersRef.current.values()) m.remove()
  markersRef.current = new Map()

  for (const [i, stop] of located.entries()) {
    const marker = new maplibregl.Marker({ element: makeMarkerEl(`S${i + 1}`, 'stop') })
      .setLngLat([stop.lon!, stop.lat!])
      .setPopup(new maplibregl.Popup({ closeButton: false }).setText(stop.name))
      .addTo(map)
    markersRef.current.set(`stop:${stop.id}`, marker)
  }

  // Itinerary pins (#72): the category icon at the item's own venue when it
  // has an address, otherwise stacked below its stop's marker.
  const perStopStack = new Map<string, number>()
  for (const item of items) {
    const label = categoryIcons[item.category]
    const stop = item.stopId ? located.find((s) => s.id === item.stopId) : undefined
    let lngLat: [number, number] | null = null
    let offset: [number, number] = [0, 0]
    let where = ''
    if (item.lat !== null && item.lon !== null) {
      lngLat = [item.lon, item.lat]
      where = item.address
    } else if (stop) {
      const stacked = perStopStack.get(stop.id) ?? 0
      perStopStack.set(stop.id, stacked + 1)
      lngLat = [stop.lon!, stop.lat!]
      offset = [0, 26 + stacked * 22]
      where = stop.name
    }
    if (!lngLat) continue
    const marker = new maplibregl.Marker({
      element: makeMarkerEl(label, 'item', layerColors?.[item.layerId]),
      offset,
    })
      .setLngLat(lngLat)
      .setPopup(new maplibregl.Popup({ closeButton: false }).setDOMContent(itemPopup(item, where)))
      .addTo(map)
    markersRef.current.set(`item:${item.id}`, marker)
  }

  const route = map.getSource(ROUTE_SOURCE) as maplibregl.GeoJSONSource | undefined
  route?.setData({
    type: 'Feature',
    properties: {},
    geometry: {
      type: 'LineString',
      coordinates: located.map((s) => [s.lon!, s.lat!]),
    },
  })

  if (located.length === 1) {
    map.easeTo({ center: [located[0].lon!, located[0].lat!], zoom: 9 })
  } else if (located.length > 1) {
    const bounds = new maplibregl.LngLatBounds()
    for (const s of located) bounds.extend([s.lon!, s.lat!])
    map.fitBounds(bounds, { padding: 48, maxZoom: 11 })
  }
}
