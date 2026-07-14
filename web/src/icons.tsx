import type { ItineraryCategory } from './api'

/** One emoji per itinerary category — board rows, print, and map pins. */
export const categoryIcons: Record<ItineraryCategory, string> = {
  activity: '🎟️',
  food: '🍜',
  lodging: '🛏️',
  transport: '🚌',
  flight: '✈️',
  train: '🚆',
  other: '📌',
}

/** Shared inline SVG icons; stroke follows currentColor. */
export function EyeIcon({ open, className = 'h-3.5 w-3.5' }: { open: boolean; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <path d="M2 12s3.5-6.5 10-6.5S22 12 22 12s-3.5 6.5-10 6.5S2 12 2 12Z" />
      <circle cx="12" cy="12" r="2.5" />
      {!open && <line x1="4" y1="20" x2="20" y2="4" />}
    </svg>
  )
}

/** Edit-pencil, tip to the lower left (mirrors the usual ✎ direction). */
export function PencilIcon({ className = 'h-3.5 w-3.5' }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497Z" />
    </svg>
  )
}
