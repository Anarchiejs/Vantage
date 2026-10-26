import { useCallback, useEffect, useState } from 'react'

export type Theme = 'clair' | 'sombre'

const CLE = 'observatoire.theme'

function themeInitial(): Theme {
  try {
    const t = localStorage.getItem(CLE)
    if (t === 'clair' || t === 'sombre') return t
  } catch {
    /* stockage indisponible */
  }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'sombre' : 'clair'
}

/** Thème courant, appliqué sur <html data-theme>, mémorisé dans le navigateur. */
export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(themeInitial)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem(CLE, theme)
    } catch {
      /* rien */
    }
    window.dispatchEvent(new CustomEvent('observatoire:theme', { detail: theme }))
  }, [theme])
  const basculer = useCallback(() => setTheme((t) => (t === 'clair' ? 'sombre' : 'clair')), [])
  return [theme, basculer]
}

/** Valeur actuelle d'un jeton CSS (ex. jeton('serie-1')). */
export function jeton(nom: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(`--${nom}`).trim()
}

/** Couleurs des séries de graphiques, dans l'ordre. */
export function series(): string[] {
  return [1, 2, 3, 4, 5, 6, 7, 8].map((i) => jeton(`serie-${i}`))
}

/** Échelle du calendrier d'activité, de 0 à 4. */
export function chaleur(): string[] {
  return [0, 1, 2, 3, 4].map((i) => jeton(`chaleur-${i}`))
}
