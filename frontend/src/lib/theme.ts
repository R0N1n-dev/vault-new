import type { Theme } from '../types'

const STORAGE_KEY = 'coffer.theme'

/**
 * The theme is stored locally as well as inside the vault so the unlock screen
 * paints correctly before the vault is open (and on first run, before any
 * settings exist).
 */
export function readStoredTheme(): Theme {
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored === 'dark' || stored === 'light' || stored === 'system') return stored
  return 'dark'
}

export function storeTheme(theme: Theme): void {
  window.localStorage.setItem(STORAGE_KEY, theme)
}

export function resolvedTheme(theme: Theme): 'dark' | 'light' {
  if (theme === 'system') {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  }
  return theme
}

export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = resolvedTheme(theme)
}

/** watchSystemTheme re-applies the theme when the OS preference changes. */
export function watchSystemTheme(getTheme: () => Theme): () => void {
  const media = window.matchMedia('(prefers-color-scheme: light)')
  const handler = (): void => {
    if (getTheme() === 'system') applyTheme('system')
  }
  media.addEventListener('change', handler)
  return () => media.removeEventListener('change', handler)
}
