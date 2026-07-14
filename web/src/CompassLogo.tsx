import { useEffect, useRef, useState } from 'react'
import spriteUrl from './assets/compass-sprite.png'

/**
 * Per-frame needle bearings, degrees clockwise from north, measured from
 * public/compass.gif by taking the red-pixel centroid of each frame
 * against the needle pivot. The sprite sheet is those 29 frames sampled
 * back down to their native 16×16 pixel grid.
 */
const ANGLES = [
  164.1, 191.1, 202.2, 229.3, 237.6, 248.7, 252.5, 269.0, 269.0, 269.0, 285.2, 292.3, 300.2,
  308.7, 336.6, 348.1, 17.1, 39.6, 43.3, 61.8, 71.6, 73.5, 78.6, 90.9, 103.0, 110.6, 119.2,
  138.4, 142.5,
]
const SOURCE_PX = 16
const NORTH_FRAME = 15 // closest to 0° — the resting pose

/**
 * The compass logo, with an easter egg: the red needle swings to whichever
 * frame best matches the mouse's bearing from the icon (#89). Sits still,
 * pointing north, until a pointer moves.
 */
export function CompassLogo({ size = 32, className = '' }: { size?: number; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null)
  const raf = useRef(0)
  const [frame, setFrame] = useState(NORTH_FRAME)

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      cancelAnimationFrame(raf.current)
      raf.current = requestAnimationFrame(() => {
        const el = ref.current
        if (!el) return
        const rect = el.getBoundingClientRect()
        const dx = e.clientX - (rect.left + rect.width / 2)
        const dy = e.clientY - (rect.top + rect.height / 2)
        // Directly over the icon there is no meaningful bearing.
        if (dx * dx + dy * dy < 64) return
        const deg = ((Math.atan2(dx, -dy) * 180) / Math.PI + 360) % 360
        let best = NORTH_FRAME
        let bestDist = Infinity
        for (let i = 0; i < ANGLES.length; i++) {
          const d = Math.abs(((ANGLES[i] - deg + 540) % 360) - 180)
          if (d < bestDist) {
            bestDist = d
            best = i
          }
        }
        setFrame(best)
      })
    }
    window.addEventListener('mousemove', onMove)
    return () => {
      window.removeEventListener('mousemove', onMove)
      cancelAnimationFrame(raf.current)
    }
  }, [])

  const scale = size / SOURCE_PX
  return (
    <span
      ref={ref}
      aria-hidden="true"
      className={`inline-block shrink-0 ${className}`}
      style={{
        width: size,
        height: size,
        backgroundImage: `url(${spriteUrl})`,
        backgroundPosition: `${-frame * size}px 0`,
        backgroundSize: `${ANGLES.length * SOURCE_PX * scale}px ${SOURCE_PX * scale}px`,
        imageRendering: 'pixelated',
      }}
    />
  )
}
