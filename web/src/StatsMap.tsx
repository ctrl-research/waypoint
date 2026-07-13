import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { feature } from 'topojson-client'
import type { FeatureCollection, Geometry, MultiPolygon, Polygon } from 'geojson'
import countriesUrl from 'world-atlas/countries-110m.json?url'
import { fetchConfig, type StatsPayload } from './api'

// Validated data hue (dataviz palette slot 1, passes on the light surface).
const VISITED = '#2a78d6'

export type StatsMapMode = 'countries' | 'cities'
export type StatsMapProjection = 'mercator' | 'globe'

type CountryProps = { name: string; visited: boolean }

/**
 * Visited-places map (#26): countries mode fills every country containing a
 * stop; cities mode dots each located stop. Projection toggles between a 2D
 * world map and a 3D globe.
 */
export function StatsMap({
  stops,
  mode,
  projection,
  onVisitedCountries,
}: {
  stops: StatsPayload['stops']
  mode: StatsMapMode
  projection: StatsMapProjection
  onVisitedCountries: (visited: string[]) => void
}) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const [ready, setReady] = useState(false)

  const { data: config } = useQuery({ queryKey: ['config'], queryFn: fetchConfig, staleTime: Infinity })
  const countries = useQuery({
    queryKey: ['countries-geo'],
    queryFn: async (): Promise<FeatureCollection<Geometry, { name: string }>> => {
      const topo = await (await fetch(countriesUrl)).json()
      return feature(topo, topo.objects.countries) as unknown as FeatureCollection<
        Geometry,
        { name: string }
      >
    },
    staleTime: Infinity,
  })

  // Countries containing at least one stop, via point-in-polygon.
  const visitedGeo = useRef<FeatureCollection<Geometry, CountryProps> | null>(null)
  useEffect(() => {
    if (!countries.data) return
    const features = countries.data.features.map((f) => {
      const visited = stops.some((s) => geometryContains(f.geometry, s.lon, s.lat))
      return { ...f, properties: { name: f.properties.name, visited } }
    })
    visitedGeo.current = { type: 'FeatureCollection', features }
    onVisitedCountries(
      features.filter((f) => f.properties.visited).map((f) => f.properties.name).sort(),
    )
    syncSources()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [countries.data, stops])

  function syncSources() {
    const map = mapRef.current
    if (!map || !ready) return
    if (visitedGeo.current) {
      ;(map.getSource('countries') as maplibregl.GeoJSONSource | undefined)?.setData(visitedGeo.current)
    }
    ;(map.getSource('cities') as maplibregl.GeoJSONSource | undefined)?.setData({
      type: 'FeatureCollection',
      features: stops.map((s) => ({
        type: 'Feature',
        geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
        properties: { name: s.name, trip: s.tripTitle },
      })),
    })
  }

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
      center: [10, 25],
      zoom: 1.2,
    })
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }))

    map.on('load', () => {
      map.addSource('countries', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
      map.addLayer({
        id: 'countries-fill',
        type: 'fill',
        source: 'countries',
        filter: ['==', ['get', 'visited'], true],
        paint: { 'fill-color': VISITED, 'fill-opacity': 0.45 },
      })
      map.addLayer({
        id: 'countries-line',
        type: 'line',
        source: 'countries',
        filter: ['==', ['get', 'visited'], true],
        paint: { 'line-color': VISITED, 'line-width': 1 },
      })

      map.addSource('cities', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
      map.addLayer({
        id: 'cities-dots',
        type: 'circle',
        source: 'cities',
        paint: {
          'circle-radius': 5,
          'circle-color': VISITED,
          // 2px surface ring so overlapping dots stay separable (mark spec).
          'circle-stroke-width': 2,
          'circle-stroke-color': '#ffffff',
        },
      })

      // Hover tooltips for both modes.
      const popup = new maplibregl.Popup({ closeButton: false, closeOnClick: false })
      map.on('mousemove', 'countries-fill', (e) => {
        const f = e.features?.[0]
        if (!f) return
        map.getCanvas().style.cursor = 'default'
        popup.setLngLat(e.lngLat).setText((f.properties as CountryProps).name).addTo(map)
      })
      map.on('mouseleave', 'countries-fill', () => {
        map.getCanvas().style.cursor = ''
        popup.remove()
      })
      map.on('mousemove', 'cities-dots', (e) => {
        const f = e.features?.[0]
        if (!f) return
        map.getCanvas().style.cursor = 'default'
        const p = f.properties as { name: string; trip: string }
        popup.setLngLat(e.lngLat).setText(`${p.name} — ${p.trip}`).addTo(map)
      })
      map.on('mouseleave', 'cities-dots', () => {
        map.getCanvas().style.cursor = ''
        popup.remove()
      })

      mapRef.current = map
      setReady(true)
    })

    return () => {
      mapRef.current = null
      setReady(false)
      map.remove()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config])

  // Push data and visibility whenever anything changes.
  useEffect(() => {
    syncSources()
    const map = mapRef.current
    if (!map || !ready) return
    map.setLayoutProperty('countries-fill', 'visibility', mode === 'countries' ? 'visible' : 'none')
    map.setLayoutProperty('countries-line', 'visibility', mode === 'countries' ? 'visible' : 'none')
    map.setLayoutProperty('cities-dots', 'visibility', mode === 'cities' ? 'visible' : 'none')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, mode, stops])

  useEffect(() => {
    if (mapRef.current && ready) mapRef.current.setProjection({ type: projection })
  }, [ready, projection])

  return <div ref={container} className="h-[28rem] w-full rounded-xl border border-slate-200" />
}

/** Ray-casting point-in-polygon over GeoJSON Polygon/MultiPolygon. */
function geometryContains(geom: Geometry, lon: number, lat: number): boolean {
  if (geom.type === 'Polygon') return polygonContains((geom as Polygon).coordinates, lon, lat)
  if (geom.type === 'MultiPolygon') {
    return (geom as MultiPolygon).coordinates.some((poly) => polygonContains(poly, lon, lat))
  }
  return false
}

function polygonContains(rings: number[][][], lon: number, lat: number): boolean {
  if (rings.length === 0 || !ringContains(rings[0], lon, lat)) return false
  // Inside the outer ring; holes subtract.
  for (let i = 1; i < rings.length; i++) {
    if (ringContains(rings[i], lon, lat)) return false
  }
  return true
}

function ringContains(ring: number[][], lon: number, lat: number): boolean {
  let inside = false
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const [xi, yi] = ring[i]
    const [xj, yj] = ring[j]
    if (yi > lat !== yj > lat && lon < ((xj - xi) * (lat - yi)) / (yj - yi) + xi) {
      inside = !inside
    }
  }
  return inside
}
