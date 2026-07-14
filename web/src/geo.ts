import { feature } from 'topojson-client'
import type { FeatureCollection, Geometry, MultiPolygon, Polygon } from 'geojson'
import countriesUrl from 'world-atlas/countries-110m.json?url'
import worldCountriesUrl from 'world-countries/countries.json?url'

export type CountryFeatures = FeatureCollection<Geometry, { name: string; continent: string }>

// Natural Earth territories without ISO numeric codes in world-countries.
const DISPUTED_CONTINENTS: Record<string, string> = {
  Kosovo: 'Europe',
  'N. Cyprus': 'Asia',
  Somaliland: 'Africa',
}

/**
 * Country polygons (world-atlas 110m) with continent names attached —
 * shared by the stats map fills and the trip search's country matching.
 * Cache with queryKey ['countries-geo'] and staleTime Infinity.
 */
export async function loadCountries(): Promise<CountryFeatures> {
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
}

export function continentOf(region: string, subregion?: string): string {
  if (region === 'Americas') {
    return subregion === 'South America' ? 'South America' : 'North America'
  }
  if (region === 'Antarctic') return 'Antarctica'
  return region || 'Other'
}

/** Ray-casting point-in-polygon over GeoJSON Polygon/MultiPolygon. */
export function geometryContains(geom: Geometry, lon: number, lat: number): boolean {
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
