import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { fetchConfig, type ItineraryCategory, type ItineraryItem, type Stop } from './api'
import { EyeIcon, categoryIcons } from './icons'
import { mapsLink } from './maps'
import { localizeMapLabels, mapStyle, type MapSourceConfig } from './mapstyle'

const ITEMS_PATH_SOURCE = 'items-path'
const REPLAY_SOURCE = 'replay'
const PATH_LAYERS = ['items-path-line', 'items-path-arrows'] as const

/** Key for map markers and hover-highlighting: stop:<id> or item:<id>. */
export type MarkerKey = `stop:${string}` | `item:${string}`

/** One chronological point of the itinerary path / replay (#62). The leg
 * LEAVING a point curves when the point is a flight, and its replay pace
 * follows the item's own start→end duration when one is set. */
type PathPoint = {
  lat: number
  lon: number
  label: string
  day: string
  category?: ItineraryCategory
  minutes?: number
  /** Transport whose origin is unknown: the PREVIOUS point lends the leg. */
  inbound?: { category: ItineraryCategory; minutes: number }
}

/** Emoji that rides the replay dot during a transportation leg. */
const TRANSPORT_EMOJI: Partial<Record<ItineraryCategory, string>> = {
  flight: categoryIcons.flight,
  train: categoryIcons.train,
  transport: categoryIcons.transport,
}
type LngLat = [number, number]

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
  replayable = false,
}: {
  stops: Stop[]
  items?: ItineraryItem[]
  highlightKey?: MarkerKey | null
  /** Item pin color per layerId (#73 slice 2); unlisted layers stay indigo. */
  layerColors?: Record<string, string>
  /** Overrides the authed /api/v1/config lookup (used by the public page). */
  mapConfig?: MapSourceConfig
  /** Enables the ▶ Replay control (#62). */
  replayable?: boolean
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
      // One small direction chevron per leg, at constant screen size.
      map.addImage('chevron', chevronImage(), { pixelRatio: 2 })
      const arrows = { 'symbol-placement': 'line-center', 'icon-image': 'chevron', 'icon-allow-overlap': true, 'icon-ignore-placement': true } as const
      // Sequential itinerary items: dashed directed links; flights curve.
      map.addSource(ITEMS_PATH_SOURCE, {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      })
      map.addLayer({
        id: 'items-path-line',
        type: 'line',
        source: ITEMS_PATH_SOURCE,
        paint: { 'line-color': '#0f172a', 'line-width': 2, 'line-dasharray': [2, 1.5] },
      })
      map.addLayer({ id: 'items-path-arrows', type: 'symbol', source: ITEMS_PATH_SOURCE, layout: { ...arrows } })
      // Replay redraws the itinerary path in blue.
      map.addSource(REPLAY_SOURCE, {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
      })
      map.addLayer({
        id: REPLAY_SOURCE,
        type: 'line',
        source: REPLAY_SOURCE,
        paint: { 'line-color': '#2a78d6', 'line-width': 3, 'line-dasharray': [2, 1.5] },
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

  // ---- Replay (#62): fly the camera point to point while the path draws.
  const [replaying, setReplaying] = useState(false)
  const [caption, setCaption] = useState<PathPoint | null>(null)

  // Layer visibility (default: everything shown). Markers and their
  // connecting lines toggle independently; replays hide the static lines.
  const [showStops, setShowStops] = useState(true)
  const [showItems, setShowItems] = useState(true)
  const [showPath, setShowPath] = useState(true)
  const [legendOpen, setLegendOpen] = useState(false)
  useEffect(() => {
    for (const [key, marker] of markersRef.current) {
      const visible = key.startsWith('stop:') ? showStops : showItems
      marker.getElement().style.display = visible ? '' : 'none'
    }
    const map = mapRef.current
    if (!map) return
    const visible: Record<(typeof PATH_LAYERS)[number], boolean> = {
      'items-path-line': showPath && !replaying,
      'items-path-arrows': showPath && !replaying,
    }
    for (const layer of PATH_LAYERS) {
      if (map.getLayer(layer)) {
        map.setLayoutProperty(layer, 'visibility', visible[layer] ? 'visible' : 'none')
      }
    }
  }, [showStops, showItems, showPath, stops, items, replaying])
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
    const pts = replayPath(items, stops)
    if (!map || pts.length < 2 || replaying) return
    cancelRef.current = false
    setReplaying(true)

    const el = document.createElement('div')
    el.className = 'relative h-3.5 w-3.5 rounded-full bg-[#2a78d6] ring-2 ring-white shadow'
    // The vehicle trails just left of the dot — travelling together. Inline
    // styles, not utility classes: the marker element is MapLibre-managed.
    const vehicle = document.createElement('div')
    vehicle.style.cssText =
      'position:absolute;right:100%;top:50%;transform:translateY(-50%);margin-right:3px;' +
      'font-size:18px;line-height:1;pointer-events:none;filter:drop-shadow(0 1px 1px rgb(0 0 0 / .4));'
    el.appendChild(vehicle)
    replayMarker.current = new maplibregl.Marker({ element: el })
      .setLngLat([pts[0].lon, pts[0].lat])
      .addTo(map)

    const src = map.getSource(REPLAY_SOURCE) as maplibregl.GeoJSONSource
    const done: [number, number][] = [[pts[0].lon, pts[0].lat]]
    setCaption(pts[0])
    const firstKm = haversineKm(pts[0].lat, pts[0].lon, pts[1].lat, pts[1].lon)
    map.easeTo({ center: [pts[0].lon, pts[0].lat], zoom: zoomForKm(firstKm), duration: 900 })
    await wait(1000)

    for (let i = 1; i < pts.length && !cancelRef.current; i++) {
      const from = pts[i - 1]
      const to = pts[i]
      setCaption(to)
      const km = haversineKm(from.lat, from.lon, to.lat, to.lon)
      // Pace by the item's own start→end duration when it has one (a 5h
      // flight plays longer than a 1h hop); distance decides otherwise.
      const duration = from.minutes
        ? Math.min(5200, Math.max(1400, from.minutes * 14))
        : Math.min(4200, Math.max(1400, km * 12))
      // The dot follows the leg's actual geometry (flights arc), and the
      // camera rides the dot every frame while the zoom glides to the
      // leg's scale, so the dot never outruns the view.
      const line = legLine(from, to)
      vehicle.textContent = (from.category && TRANSPORT_EMOJI[from.category]) || ''
      // Settle the zoom at the departure point BEFORE the dot moves — mixing
      // the two made the dot race across the screen while still zoomed in.
      const zoomTo = zoomForKm(km)
      const zoomDelta = Math.abs(zoomTo - map.getZoom())
      if (zoomDelta > 0.3) {
        const zoomDur = Math.min(1300, 250 + zoomDelta * 160)
        map.easeTo({ center: [from.lon, from.lat], zoom: zoomTo, duration: zoomDur })
        await wait(zoomDur + 80)
      }
      await animate(duration, (t) => {
        const [lon, lat] = pointAlong(line, t)
        replayMarker.current?.setLngLat([lon, lat])
        map.jumpTo({ center: [lon, lat], zoom: zoomTo })
        src.setData({
          type: 'Feature',
          properties: {},
          geometry: {
            type: 'LineString',
            coordinates: [...done, ...line.slice(1, Math.floor(t * (line.length - 1)) + 1), [lon, lat]],
          },
        })
      })
      done.push(...line.slice(1))
      await wait(450)
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
      {replayable && replayPath(items, stops).length >= 2 && (
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
      <div className="absolute left-3 top-3 rounded-lg bg-white/90 p-1 text-xs shadow">
        <button
          type="button"
          onClick={() => setLegendOpen((v) => !v)}
          className="flex w-full items-center gap-1 rounded-md px-2 py-1 text-slate-900 hover:bg-slate-100"
          aria-expanded={legendOpen}
        >
          <span className={`text-[9px] transition-transform ${legendOpen ? 'rotate-90' : ''}`}>▶</span>
          Layers
        </button>
        {legendOpen && (
          <div className="mt-0.5 flex flex-col">
            {(
              [
                ['Stops', showStops, setShowStops],
                ['Items', showItems, setShowItems],
                ['Path', showPath, setShowPath],
              ] as const
            ).map(([label, on, set]) => (
              <button
                key={label}
                type="button"
                onClick={() => set(!on)}
                className={`flex items-center gap-1.5 rounded-md px-2 py-1 text-left hover:bg-slate-100 ${on ? 'text-slate-900' : 'text-slate-400'}`}
                title={on ? `Hide ${label.toLowerCase()}` : `Show ${label.toLowerCase()}`}
                aria-pressed={on}
              >
                <EyeIcon open={on} />
                {label}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/** Chronological located itinerary points: day/time order, venue coords
 * with stop fallback, consecutive same-spot items collapsed. */
function itineraryPath(items: ItineraryItem[], stops: Stop[]): PathPoint[] {
  const locate = (it: ItineraryItem): { lat: number; lon: number } | null => {
    if (it.lat !== null && it.lon !== null) return { lat: it.lat, lon: it.lon }
    const stop = stops.find((s) => s.id === it.stopId)
    return stop && stop.lat !== null && stop.lon !== null ? { lat: stop.lat, lon: stop.lon } : null
  }
  const ordered = [...items].sort((a, b) => {
    if (a.day !== b.day) return a.day < b.day ? -1 : 1
    if (a.startTime && b.startTime) return a.startTime < b.startTime ? -1 : 1
    if (a.startTime) return -1
    if (b.startTime) return 1
    return a.position - b.position
  })
  const stopCoords = (id: string | null): { lat: number; lon: number } | null => {
    const stop = stops.find((s) => s.id === id)
    return stop && stop.lat !== null && stop.lon !== null ? { lat: stop.lat, lon: stop.lon } : null
  }
  const pts: PathPoint[] = ordered.flatMap((it): PathPoint[] => {
    const base = { label: it.title, day: it.day }
    const isTransport = it.category === 'flight' || it.category === 'train' || it.category === 'transport'
    if (isTransport) {
      // A transport item IS a leg: the journey runs from its origin to its
      // destination for exactly its departure→arrival duration — that's
      // where the vehicle emoji, the arc, and the pacing belong (#62).
      const origin = stopCoords(it.stopId) ?? locate(it)
      const dest = stopCoords(it.destinationStopId)
      const travel = { category: it.category, minutes: legMinutes(it.startTime, it.endTime) }
      if (origin && dest && (origin.lat !== dest.lat || origin.lon !== dest.lon)) {
        return [
          { ...origin, ...base, ...travel },
          { ...dest, ...base },
        ]
      }
      if (origin) return [{ ...origin, ...base, ...travel }]
      if (dest) return [{ ...dest, ...base, inbound: travel }]
      return []
    }
    const at = locate(it)
    return at ? [{ ...at, ...base }] : []
  })
  // A destination-only transport point can't describe its own leg — the
  // point before it does.
  for (let i = 1; i < pts.length; i++) {
    const inbound = pts[i].inbound
    if (inbound) {
      pts[i - 1] = { ...pts[i - 1], category: inbound.category, minutes: inbound.minutes }
      delete pts[i].inbound
    }
  }
  // Collapse consecutive same-spot points, but let the survivor describe
  // the DEPARTURE: a flight sits at its origin stop — same coordinates as
  // the activity before it — and the leg leaving must carry the flight's
  // category (arc, emoji) and duration (pace), not the activity's.
  const collapsed: PathPoint[] = []
  for (const p of pts) {
    const prev = collapsed[collapsed.length - 1]
    if (prev && prev.lat === p.lat && prev.lon === p.lon) {
      prev.category = p.category
      prev.minutes = p.minutes
    } else {
      collapsed.push({ ...p })
    }
  }
  return collapsed
}

/** Replay path: the itinerary path, or the stop route when it's too short. */
function replayPath(items: ItineraryItem[], stops: Stop[]): PathPoint[] {
  const pts = itineraryPath(items, stops)
  if (pts.length >= 2) return pts
  return stops.flatMap((s) =>
    s.lat !== null && s.lon !== null
      ? [{ lat: s.lat, lon: s.lon, label: s.name, day: s.arrivalDate ?? '' }]
      : [],
  )
}

/** Quadratic-bezier arc between two points, bulging left of travel. Lon is
 * scaled by cos(lat) so the curve looks round on screen. */
function arcCoords(a: LngLat, b: LngLat, curvature = 0.18, steps = 24): LngLat[] {
  const k = Math.cos((((a[1] + b[1]) / 2) * Math.PI) / 180) || 1e-6
  const x1 = a[0] * k
  const x2 = b[0] * k
  const dx = x2 - x1
  const dy = b[1] - a[1]
  const cx = (x1 + x2) / 2 - dy * curvature
  const cy = (a[1] + b[1]) / 2 + dx * curvature
  const out: LngLat[] = []
  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    const x = (1 - t) ** 2 * x1 + 2 * (1 - t) * t * cx + t ** 2 * x2
    const y = (1 - t) ** 2 * a[1] + 2 * (1 - t) * t * cy + t ** 2 * b[1]
    out.push([x / k, y])
  }
  return out
}

/** A small right-pointing chevron sprite; symbol layers rotate it along
 * each leg (~2× the line width on screen, at every zoom). */
function chevronImage(size = 16): ImageData {
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const g = canvas.getContext('2d')!
  g.strokeStyle = '#0f172a'
  g.lineWidth = 3.5
  g.lineCap = 'round'
  g.lineJoin = 'round'
  g.beginPath()
  g.moveTo(size * 0.32, size * 0.18)
  g.lineTo(size * 0.72, size * 0.5)
  g.lineTo(size * 0.32, size * 0.82)
  g.stroke()
  return g.getImageData(0, 0, size, size)
}

/** Coordinates for the leg leaving `from`: flight legs arc, others are
 * straight. Shared by the static path and the replay so they overlap. */
function legLine(from: PathPoint, to: PathPoint): LngLat[] {
  const a: LngLat = [from.lon, from.lat]
  const b: LngLat = [to.lon, to.lat]
  return from.category === 'flight' ? arcCoords(a, b) : [a, b]
}

/** Position at parameter t along a polyline (per-vertex interpolation). */
function pointAlong(line: LngLat[], t: number): LngLat {
  const seg = Math.min(0.999999, Math.max(0, t)) * (line.length - 1)
  const i = Math.floor(seg)
  const f = seg - i
  const [ax, ay] = line[i]
  const [bx, by] = line[i + 1]
  return [ax + (bx - ax) * f, ay + (by - ay) * f]
}

/** Minutes between "HH:MM" strings; overnight wraps, unset gives 0. */
function legMinutes(start: string | null, end: string | null): number {
  if (!start || !end) return 0
  const mins = (s: string) => Number(s.slice(0, 2)) * 60 + Number(s.slice(3, 5))
  const d = mins(end) - mins(start)
  return d <= 0 ? d + 24 * 60 : d
}

/** One LineString per consecutive point pair (line-center symbols put a
 * chevron on each). */
function directedFeatures(points: PathPoint[]): GeoJSON.Feature[] {
  const features: GeoJSON.Feature[] = []
  for (let i = 1; i < points.length; i++) {
    features.push({
      type: 'Feature',
      properties: {},
      geometry: { type: 'LineString', coordinates: legLine(points[i - 1], points[i]) },
    })
  }
  return features
}

/** A zoom where a leg of this length reads well — street level between
 * venues, a regional view for trains, wide (but not global) for flights.
 * The camera rides the dot, so the whole leg never needs to fit on screen. */
function zoomForKm(km: number): number {
  return Math.min(14.5, Math.max(6, 14 - 2 * Math.log10(Math.max(km, 1))))
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

  const itemsPath = map.getSource(ITEMS_PATH_SOURCE) as maplibregl.GeoJSONSource | undefined
  itemsPath?.setData({
    type: 'FeatureCollection',
    features: directedFeatures(itineraryPath(items, stops)),
  })

  if (located.length === 1) {
    map.easeTo({ center: [located[0].lon!, located[0].lat!], zoom: 9 })
  } else if (located.length > 1) {
    const bounds = new maplibregl.LngLatBounds()
    for (const s of located) bounds.extend([s.lon!, s.lat!])
    map.fitBounds(bounds, { padding: 48, maxZoom: 11 })
  }
}
