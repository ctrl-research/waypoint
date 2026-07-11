import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { fetchConfig, type Stop } from './api'

const ROUTE_SOURCE = 'route'

/**
 * TripMap renders the trip's stops as numbered markers connected by a route
 * line. When `picking` is set, the next map click reports coordinates via
 * onPick (used to place a stop, #14).
 */
export function TripMap({
  stops,
  picking,
  onPick,
}: {
  stops: Stop[]
  picking: boolean
  onPick: (lat: number, lon: number) => void
}) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const markersRef = useRef<maplibregl.Marker[]>([])
  // Refs so the single click handler always sees current props.
  const pickingRef = useRef(picking)
  const onPickRef = useRef(onPick)
  pickingRef.current = picking
  onPickRef.current = onPick

  const { data: config } = useQuery({
    queryKey: ['config'],
    queryFn: fetchConfig,
    staleTime: Infinity,
  })

  // Create the map once the tile URL is known.
  useEffect(() => {
    if (!config || !container.current || mapRef.current) return

    const map = new maplibregl.Map({
      container: container.current,
      style: {
        version: 8,
        sources: {
          raster: {
            type: 'raster',
            tiles: [config.tileUrl],
            tileSize: 256,
            attribution: '© OpenStreetMap contributors',
          },
        },
        layers: [{ id: 'raster', type: 'raster', source: 'raster' }],
      },
      center: [0, 20],
      zoom: 1,
    })
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }))

    map.on('load', () => {
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
      syncMap(map, stopsRef.current, markersRef)
    })

    map.on('click', (e) => {
      if (pickingRef.current) onPickRef.current(e.lngLat.lat, e.lngLat.lng)
    })

    return () => {
      mapRef.current = null
      map.remove()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config])

  // Keep markers and route in sync with the stops.
  const stopsRef = useRef(stops)
  stopsRef.current = stops
  useEffect(() => {
    if (mapRef.current) syncMap(mapRef.current, stops, markersRef)
  }, [stops])

  // Picking mode: crosshair cursor.
  useEffect(() => {
    const canvas = mapRef.current?.getCanvas()
    if (canvas) canvas.style.cursor = picking ? 'crosshair' : ''
  }, [picking])

  return (
    <div className="relative">
      <div ref={container} className="h-80 w-full rounded-xl border border-slate-200" />
      {picking && (
        <div className="absolute left-1/2 top-3 -translate-x-1/2 rounded-full bg-slate-900/90 px-4 py-1.5 text-sm text-white shadow">
          Click the map to place the stop
        </div>
      )}
    </div>
  )
}

function syncMap(
  map: maplibregl.Map,
  stops: Stop[],
  markersRef: React.MutableRefObject<maplibregl.Marker[]>,
) {
  const located = stops.filter((s) => s.lat !== null && s.lon !== null)

  for (const m of markersRef.current) m.remove()
  markersRef.current = located.map((stop, i) => {
    const el = document.createElement('div')
    el.className =
      'flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white shadow-md'
    el.textContent = String(i + 1)
    return new maplibregl.Marker({ element: el })
      .setLngLat([stop.lon!, stop.lat!])
      .setPopup(new maplibregl.Popup({ closeButton: false }).setText(stop.name))
      .addTo(map)
  })

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
