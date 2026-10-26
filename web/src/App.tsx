import { lazy, Suspense, useEffect, useState } from 'react'
import { BrowserRouter, NavLink, Route, Routes, useNavigate } from 'react-router'
import { useTheme } from './theme/theme'
import { Chargement } from './ui/Etat'

// Une page = un dossier de src/pages, propriété d'un lot (voir .orchestra/PLAN.md).
const Accueil = lazy(() => import('./pages/accueil/Accueil'))
const Projets = lazy(() => import('./pages/projet/Projets'))
const Projet = lazy(() => import('./pages/projet/Projet'))
const Technos = lazy(() => import('./pages/technos/Technos'))
const Graphe = lazy(() => import('./pages/graphe/Graphe'))
const Recherche = lazy(() => import('./pages/recherche/Recherche'))
const Console = lazy(() => import('./pages/console/Console'))
const Palette = lazy(() => import('./pages/palette/Palette'))

const NAV = [
  { a: '/', libelle: 'Accueil', icone: '◎' },
  { a: '/projets', libelle: 'Projets', icone: '▤' },
  { a: '/technos', libelle: 'Technos & IA', icone: '⋈' },
  { a: '/graphe', libelle: 'Graphe', icone: '✦' },
  { a: '/recherche', libelle: 'Recherche', icone: '⌕' },
  { a: '/console', libelle: 'Console SQL', icone: '›_' },
]

/** Relaie les événements SSE du serveur en « observatoire:maj » (écoutés par useApi). */
function useMisesAJour() {
  useEffect(() => {
    if (import.meta.env.VITE_MOCK === '1' || typeof EventSource === 'undefined') return
    const es = new EventSource('/api/evenements')
    es.addEventListener('maj', (e) => {
      window.dispatchEvent(new CustomEvent('observatoire:maj', { detail: JSON.parse((e as MessageEvent).data) }))
    })
    return () => es.close()
  }, [])
}

/** Raccourcis globaux : Ctrl+K ouvre la palette, « / » va à la recherche. */
function useRaccourcis(ouvrirPalette: () => void) {
  const naviguer = useNavigate()
  useEffect(() => {
    const surTouche = (e: KeyboardEvent) => {
      const cible = e.target as HTMLElement
      const saisie = cible.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(cible.tagName)
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        ouvrirPalette()
      } else if (e.key === '/' && !saisie) {
        e.preventDefault()
        naviguer('/recherche')
      }
    }
    window.addEventListener('keydown', surTouche)
    return () => window.removeEventListener('keydown', surTouche)
  }, [naviguer, ouvrirPalette])
}

function Cadre() {
  const [theme, basculer] = useTheme()
  const [palette, setPalette] = useState(false)
  useMisesAJour()
  useRaccourcis(() => setPalette(true))
  // La palette (ou toute page) bascule le thème en émettant cet événement.
  useEffect(() => {
    window.addEventListener('observatoire:basculer-theme', basculer)
    return () => window.removeEventListener('observatoire:basculer-theme', basculer)
  }, [basculer])

  return (
    <div className="flex min-h-screen bg-fond text-texte">
      <nav className="sticky top-0 flex h-screen w-52 shrink-0 flex-col gap-1 border-r border-bordure bg-surface p-3">
        <div className="mb-4 flex items-center gap-2 px-2 py-1">
          <span className="text-xl text-accent">◉</span>
          <span className="font-semibold">Observatoire</span>
        </div>
        {NAV.map((n) => (
          <NavLink
            key={n.a}
            to={n.a}
            end={n.a === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                isActive ? 'bg-accent-doux font-medium text-texte' : 'text-doux hover:bg-surface-2 hover:text-texte'
              }`
            }
          >
            <span className="w-5 text-center font-mono text-xs">{n.icone}</span>
            {n.libelle}
          </NavLink>
        ))}
        <div className="mt-auto flex flex-col gap-2 px-2 text-xs text-doux">
          <button
            type="button"
            onClick={() => setPalette(true)}
            className="flex items-center justify-between rounded-md border border-bordure px-2 py-1 hover:text-texte"
          >
            Commandes <kbd className="font-mono">Ctrl K</kbd>
          </button>
          <button type="button" onClick={basculer} className="rounded-md border border-bordure px-2 py-1 hover:text-texte">
            Thème {theme === 'clair' ? 'sombre ☾' : 'clair ☀'}
          </button>
        </div>
      </nav>
      <main className="min-w-0 flex-1 p-6">
        <Suspense fallback={<Chargement lignes={6} />}>
          <Routes>
            <Route path="/" element={<Accueil />} />
            <Route path="/projets" element={<Projets />} />
            <Route path="/projets/:id" element={<Projet />} />
            <Route path="/technos" element={<Technos />} />
            <Route path="/graphe" element={<Graphe />} />
            <Route path="/graphe/:id" element={<Graphe />} />
            <Route path="/recherche" element={<Recherche />} />
            <Route path="/console" element={<Console />} />
          </Routes>
        </Suspense>
      </main>
      {palette && (
        <Suspense fallback={null}>
          <Palette fermer={() => setPalette(false)} />
        </Suspense>
      )}
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Cadre />
    </BrowserRouter>
  )
}
