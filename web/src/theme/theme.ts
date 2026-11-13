import { useCallback, useEffect, useState } from 'react'

export type Theme = 'light' | 'dark'

const KEY = 'vantage.theme'

function initialTheme(): Theme {
  try {
    const stored = localStorage.getItem(KEY)
    if (stored === 'light' || stored === 'dark') return stored
    if (stored === 'clair') return 'light'
    if (stored === 'sombre') return 'dark'
  } catch {
    /* storage unavailable */
  }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Current theme, applied on <html data-theme> and remembered in the browser. */
export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(initialTheme)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem(KEY, theme)
    } catch {
      /* ignore */
    }
    window.dispatchEvent(new CustomEvent('vantage:theme', { detail: theme }))
  }, [theme])
  const toggle = useCallback(() => setTheme((current) => (current === 'light' ? 'dark' : 'light')), [])
  return [theme, toggle]
}

/** Current value of a CSS token (for example token('series-1')). */
export function token(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(`--${name}`).trim()
}

/** Chart series colors, in order. */
export function series(): string[] {
  return [1, 2, 3, 4, 5, 6, 7, 8].map((i) => token(`series-${i}`))
}

/** Activity calendar scale, from 0 to 4. */
export function heat(): string[] {
  return [0, 1, 2, 3, 4].map((i) => token(`heat-${i}`))
}
