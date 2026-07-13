import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { feature } from 'topojson-client'
import type { FeatureCollection, Geometry, MultiPolygon, Polygon } from 'geojson'
import countriesUrl from 'world-atlas/countries-110m.json?url'
import worldCountriesUrl from 'world-countries/countries.json?url'
import { fetchConfig, type StatsPayload } from './api'

// Validated data hue (dataviz palette slot 1, passes on the light surface).
const VISITED = '#2a78d6'

export type StatsMapMode = 'countries' | 'continents' | 'cities'
export type StatsMapProjection = 'mercator' | 'globe'

export type VisitedPlaces = { countries: string[]; continents: string[]; countryTotal: number }

type CountryProps = {
  name: string
  continent: string
  visited: boolean
  continentVisited: boolean
}

/**
 * Visited-places map (#26): countries/continents modes fill everywhere a
 * stop lands; cities mode dots each located stop. The projection toggle
 * morphs between the 2D map and the 3D globe.
 */
export function StatsMap({
  stops,
  mode,
  projection,
  onVisited,
}: {
  stops: StatsPayload['stops']
  mode: StatsMapMode
  projection: StatsMapProjection
  onVisited: (v: VisitedPlaces) => void
}) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const [ready, setReady] = useState(false)

  const { data: config } = useQuery({ queryKey: ['config'], queryFn: fetchConfig, staleTime: Infinity })
  const countries = useQuery({
    queryKey: ['countries-geo'],
    queryFn: async (): Promise<FeatureCollection<Geometry, { name: string; continent: string }>> => {
      const [topo, meta] = await Promise.all([
        fetch(countriesUrl).then((r) => r.json()),
        fetch(worldCountriesUrl).then((r) => r.json()) as Promise<
          { ccn3?: string; region: string; subregion?: string }[]
        >,
      ])
      const continentByID = new Map<string, string>()
      for (const c of meta) {
        if (c.ccn3) continentByID.set(c.ccn3, continentOf(c.region, c.subregion))
      }
      const fc = feature(topo, topo.objects.countries) as unknown as FeatureCollection<
        Geometry,
        { name: string }
      >
      return {
        type: 'FeatureCollection',
        features: fc.features.map((f) => ({
          ...f,
          properties: {
            name: f.properties.name,
            continent:
              continentByID.get(String(f.id).padStart(3, '0')) ??
              DISPUTED_CONTINENTS[f.properties.name] ??
              'Other',
          },
        })),
      }
    },
    staleTime: Infinity,
  })

  // Mark visited countries (point-in-polygon) and their continents.
  const visitedGeo = useRef<FeatureCollection<Geometry, CountryProps> | null>(null)
  useEffect(() => {
    if (!countries.data) return
    const withVisited = countries.data.features.map((f) => ({
      ...f,
      properties: {
        ...f.properties,
        visited: stops.some((s) => geometryContains(f.geometry, s.lon, s.lat)),
      },
    }))
    const visitedContinents = new Set(
      withVisited.filter((f) => f.properties.visited).map((f) => f.properties.continent),
    )
    visitedGeo.current = {
      type: 'FeatureCollection',
      features: withVisited.map((f) => ({
        ...f,
        properties: {
          ...f.properties,
          continentVisited: visitedContinents.has(f.properties.continent),
        },
      })),
    }
    onVisited({
      countries: withVisited.filter((f) => f.properties.visited).map((f) => f.properties.name).sort(),
      continents: [...visitedContinents].filter((c) => c !== 'Other').sort(),
      countryTotal: withVisited.length,
    })
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
      // Continents reuse the country polygons, filtered by continent flag.
      map.addLayer({
        id: 'continents-fill',
        type: 'fill',
        source: 'countries',
        filter: ['==', ['get', 'continentVisited'], true],
        paint: { 'fill-color': VISITED, 'fill-opacity': 0.35 },
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

      // Hover tooltips for all three modes.
      const popup = new maplibregl.Popup({ closeButton: false, closeOnClick: false })
      const hover = (layer: string, text: (p: CountryProps & { trip?: string }) => string) => {
        map.on('mousemove', layer, (e) => {
          const f = e.features?.[0]
          if (!f) return
          map.getCanvas().style.cursor = 'default'
          popup.setLngLat(e.lngLat).setText(text(f.properties as CountryProps & { trip?: string })).addTo(map)
        })
        map.on('mouseleave', layer, () => {
          map.getCanvas().style.cursor = ''
          popup.remove()
        })
      }
      hover('countries-fill', (p) => p.name)
      hover('continents-fill', (p) => p.continent)
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
    const show = (layer: string, on: boolean) =>
      map.setLayoutProperty(layer, 'visibility', on ? 'visible' : 'none')
    show('countries-fill', mode === 'countries')
    show('countries-line', mode === 'countries')
    show('continents-fill', mode === 'continents')
    show('cities-dots', mode === 'cities')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, mode, stops])

  // Projection switches instantly. An animated globe→map morph was tried
  // and reverted — see the "globe unwrap transition" issue for the findings
  // before attempting it again.
  useEffect(() => {
    if (mapRef.current && ready) mapRef.current.setProjection({ type: projection })
  }, [ready, projection])

  return <div ref={container} className="h-[28rem] w-full rounded-xl border border-slate-200" />
}

// Natural Earth territories without ISO numeric codes in world-countries.
const DISPUTED_CONTINENTS: Record<string, string> = {
  Kosovo: 'Europe',
  'N. Cyprus': 'Asia',
  Somaliland: 'Africa',
}

function continentOf(region: string, subregion?: string): string {
  if (region === 'Americas') {
    return subregion === 'South America' ? 'South America' : 'North America'
  }
  if (region === 'Antarctic') return 'Antarctica'
  return region || 'Other'
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
