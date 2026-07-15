import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import type { FeatureCollection, Geometry } from 'geojson'
import { fetchConfig, type StatsPayload } from './api'
import { geometryContains, loadCountries } from './geo'
import { localizeMapLabels, mapStyle } from './mapstyle'

// Travelled/planned hues (#53) — validated as a pair on the light surface.
const TRAVELLED = '#059669'
const PLANNED = '#d97706'

export type StatsMapMode = 'countries' | 'continents' | 'cities'
export type StatsMapProjection = 'mercator' | 'globe'

export type VisitedPlaces = {
  countries: string[]
  plannedCountries: string[]
  continents: string[]
  plannedContinents: string[]
  countryTotal: number
}

type CountryProps = {
  name: string
  continent: string
  /** Reached by a trip that has started. */
  visited: boolean
  /** Only reached by future trips. */
  planned: boolean
  continentVisited: boolean
  continentPlanned: boolean
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
  const countries = useQuery({ queryKey: ['countries-geo'], queryFn: loadCountries, staleTime: Infinity })

  // Mark visited/planned countries (point-in-polygon) and their continents.
  const visitedGeo = useRef<FeatureCollection<Geometry, CountryProps> | null>(null)
  useEffect(() => {
    if (!countries.data) return
    const withVisited = countries.data.features.map((f) => {
      const visited = stops.some((s) => s.travelled && geometryContains(f.geometry, s.lon, s.lat))
      const planned =
        !visited && stops.some((s) => !s.travelled && geometryContains(f.geometry, s.lon, s.lat))
      return { ...f, properties: { ...f.properties, visited, planned } }
    })
    const visitedContinents = new Set(
      withVisited.filter((f) => f.properties.visited).map((f) => f.properties.continent),
    )
    const plannedContinents = new Set(
      withVisited
        .filter((f) => f.properties.planned)
        .map((f) => f.properties.continent)
        .filter((c) => !visitedContinents.has(c)),
    )
    visitedGeo.current = {
      type: 'FeatureCollection',
      features: withVisited.map((f) => ({
        ...f,
        properties: {
          ...f.properties,
          continentVisited: visitedContinents.has(f.properties.continent),
          continentPlanned: plannedContinents.has(f.properties.continent),
        },
      })),
    }
    onVisited({
      countries: withVisited.filter((f) => f.properties.visited).map((f) => f.properties.name).sort(),
      plannedCountries: withVisited.filter((f) => f.properties.planned).map((f) => f.properties.name).sort(),
      continents: [...visitedContinents].filter((c) => c !== 'Other').sort(),
      plannedContinents: [...plannedContinents].filter((c) => c !== 'Other').sort(),
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
        properties: { name: s.name, trip: s.tripTitle, travelled: s.travelled },
      })),
    })
  }

  useEffect(() => {
    if (!config || !container.current || mapRef.current) return

    const map = new maplibregl.Map({
      container: container.current,
      style: mapStyle(config),
      center: [10, 25],
      zoom: 1.2,
      attributionControl: false,
    })
    map.addControl(new maplibregl.AttributionControl({ compact: true }))
    map.once('idle', () => {
      const attrib = container.current?.querySelector<HTMLElement>('.maplibregl-ctrl-attrib')
      attrib?.classList.remove('maplibregl-compact-show')
      attrib?.removeAttribute('open')
    })
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }))

    map.on('load', () => {
      localizeMapLabels(map, config)
      map.addSource('countries', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
      const reached: maplibregl.FilterSpecification = [
        'any',
        ['==', ['get', 'visited'], true],
        ['==', ['get', 'planned'], true],
      ]
      const fillByVisited = ['case', ['get', 'visited'], TRAVELLED, PLANNED] as unknown as string
      map.addLayer({
        id: 'countries-fill',
        type: 'fill',
        source: 'countries',
        filter: reached,
        paint: { 'fill-color': fillByVisited, 'fill-opacity': 0.45 },
      })
      map.addLayer({
        id: 'countries-line',
        type: 'line',
        source: 'countries',
        filter: reached,
        paint: { 'line-color': fillByVisited, 'line-width': 1 },
      })
      // Continents reuse the country polygons, filtered by continent flag.
      map.addLayer({
        id: 'continents-fill',
        type: 'fill',
        source: 'countries',
        filter: [
          'any',
          ['==', ['get', 'continentVisited'], true],
          ['==', ['get', 'continentPlanned'], true],
        ],
        paint: {
          'fill-color': ['case', ['get', 'continentVisited'], TRAVELLED, PLANNED] as unknown as string,
          'fill-opacity': 0.35,
        },
      })

      map.addSource('cities', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
      map.addLayer({
        id: 'cities-dots',
        type: 'circle',
        source: 'cities',
        paint: {
          'circle-radius': 5,
          'circle-color': ['case', ['get', 'travelled'], TRAVELLED, PLANNED] as unknown as string,
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

  return <div ref={container} className="h-[28rem] w-full rounded-xl border border-slate-200 dark:border-slate-700" />
}

