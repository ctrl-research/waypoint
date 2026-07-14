import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { fetchConfig, type ItineraryItem, type Stop } from './api'
import { EyeIcon, categoryIcons } from './icons'
import { localizeMapLabels, mapStyle, type MapSourceConfig } from './mapstyle'

const ROUTE_SOURCE = 'route'

/** Key for map markers and hover-highlighting: stop:<id> or item:<id>. */
export type MarkerKey = `stop:${string}` | `item:${string}`

/**
 * TripMap renders the trip's stops as S-numbered markers connected by a
 * route line, plus category-icon pins for itinerary items at their venue
 * or stop (#72, #73).
 * `highlightKey` enlarges the hovered list row's marker (#71). When
 * `picking` is set, the next map click reports coordinates via onPick (#14).
 */
export function TripMap({
  stops,
  items = [],
  picking,
  onPick,
  highlightKey = null,
  mapConfig,
  layerColors,
}: {
  stops: Stop[]
  items?: ItineraryItem[]
  picking: boolean
  onPick: (lat: number, lon: number) => void
  highlightKey?: MarkerKey | null
  /** Item pin color per layerId (#73 slice 2); unlisted layers stay indigo. */
  layerColors?: Record<string, string>
  /** Overrides the authed /api/v1/config lookup (used by the public page). */
  mapConfig?: MapSourceConfig
}) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const markersRef = useRef<Map<string, maplibregl.Marker>>(new Map())
  // Refs so the single click handler always sees current props.
  const pickingRef = useRef(picking)
  const onPickRef = useRef(onPick)
  pickingRef.current = picking
  onPickRef.current = onPick

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
      mapRef.current = map
      syncMap(map, stopsRef.current, itemsRef.current, markersRef, layerColorsRef.current)
    })

    map.on('click', (e) => {
      if (pickingRef.current) onPickRef.current(e.lngLat.lat, e.lngLat.lng)
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

  // Picking mode: crosshair cursor.
  useEffect(() => {
    const canvas = mapRef.current?.getCanvas()
    if (canvas) canvas.style.cursor = picking ? 'crosshair' : ''
  }, [picking])

  return (
    <div className="relative">
      <div ref={container} className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-700" />
      {picking && (
        <div className="absolute left-1/2 top-3 -translate-x-1/2 rounded-full bg-slate-900/90 px-4 py-1.5 text-sm text-white shadow">
          Click the map to place the stop
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
      .setPopup(
        new maplibregl.Popup({ closeButton: false }).setText(
          where ? `${item.title} — ${where}` : item.title,
        ),
      )
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
