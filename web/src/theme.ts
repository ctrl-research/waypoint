/**
 * Theme state lives at module level so the OS-follow listener runs for the
 * app's whole lifetime, not just while some component is mounted. The
 * pre-paint script in index.html reads the same localStorage key to avoid
 * a flash before this module loads.
 */
export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'waypoint-theme'
let current: Theme = (localStorage.getItem(STORAGE_KEY) as Theme) ?? 'system'
const listeners = new Set<() => void>()

function apply() {
  const dark =
    current === 'dark' ||
    (current === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

export function getTheme(): Theme {
  return current
}

export function setTheme(theme: Theme) {
  current = theme
  localStorage.setItem(STORAGE_KEY, theme)
  apply()
  for (const notify of listeners) notify()
}

/** For useSyncExternalStore. */
export function subscribeTheme(onChange: () => void): () => void {
  listeners.add(onChange)
  return () => listeners.delete(onChange)
}

apply()
// Follow OS changes live while in system mode.
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (current === 'system') apply()
})
