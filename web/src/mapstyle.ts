import type maplibregl from 'maplibre-gl'

export type MapSourceConfig = {
  tileUrl: string
  mapStyleUrl: string
  language: string
}

/** The style to hand MapLibre: the configured vector style URL, or a raster
 * style built from the tile template (#66). */
export function mapStyle(cfg: MapSourceConfig): string | maplibregl.StyleSpecification {
  if (cfg.mapStyleUrl) return cfg.mapStyleUrl
  return {
    version: 8,
    sources: {
      raster: {
        type: 'raster',
        tiles: [cfg.tileUrl],
        tileSize: 256,
        attribution: '© OpenStreetMap contributors',
      },
    },
    layers: [{ id: 'raster', type: 'raster', source: 'raster' }],
  }
}

/**
 * Rewrites vector-style symbol layers to prefer the configured language:
 * name:{lang} → name:latin → name. Raster styles have labels baked into the
 * tiles, so this only applies when a vector style is active.
 */
export function localizeMapLabels(map: maplibregl.Map, cfg: MapSourceConfig) {
  if (!cfg.mapStyleUrl || !cfg.language) return
  const field = [
    'coalesce',
    ['get', `name:${cfg.language}`],
    ['get', 'name:latin'],
    ['get', 'name'],
  ]
  for (const layer of map.getStyle().layers ?? []) {
    if (layer.type !== 'symbol') continue
    const current = map.getLayoutProperty(layer.id, 'text-field')
    // Only rewrite label layers that render place names.
    if (current && JSON.stringify(current).includes('name')) {
      map.setLayoutProperty(layer.id, 'text-field', field)
    }
  }
}
