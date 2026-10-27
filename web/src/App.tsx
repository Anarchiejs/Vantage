import { lazy, Suspense, useEffect, useState } from 'react'
import { BrowserRouter, NavLink, Route, Routes, useNavigate, useLocation } from 'react-router'
import { Icone } from './ui/Icone'
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
  { a: '/', libelle: 'Accueil', icone: 'accueil' },
  { a: '/projets', libelle: 'Projets', icone: 'projets' },
  { a: '/technos', libelle: 'Technos & IA', icone: 'technos' },
  { a: '/graphe', libelle: 'Graphe', icone: 'graphe' },
  { a: '/recherche', libelle: 'Recherche', icone: 'recherche' },
  { a: '/console', libelle: 'Console SQL', icone: 'console' },
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
  const { pathname } = useLocation()
  useEffect(() => { window.scrollTo(0, 0) }, [pathname])
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
    <div className="app-shell">
      <a className="skip-link" href="#contenu">Aller au contenu</a>
      <nav aria-label="Navigation principale" className="app-sidebar">
        <div className="brand">
          <span className="brand-mark">◉</span>
          <div><span className="font-semibold">Observatoire</span><small>Le fil de vos projets</small></div>
        </div>
        <p className="nav-caption">ESPACE DE TRAVAIL</p>
        <div className="nav-links">
        {NAV.map((n) => (
          <NavLink
            key={n.a}
            to={n.a}
            end={n.a === '/'}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'is-active' : ''}`
            }
          >
            <Icone nom={n.icone} /><span>{n.libelle}</span>
          </NavLink>
        ))}
        </div>
        <div className="sidebar-bottom">
          <div className="local-note"><span>◎</span><div>Votre activité, réunie.<small>Dépôts Git & coffre Obsidian</small></div></div>
          <button
            type="button"
            onClick={() => setPalette(true)}
            className="sidebar-command"
          >
            Commandes <kbd className="font-mono">Ctrl K</kbd>
          </button>
          <button type="button" onClick={basculer} className="sidebar-command">
            <Icone nom={theme === 'clair' ? 'lune' : 'soleil'} />
            Thème {theme === 'clair' ? 'sombre ☾' : 'clair ☀'}
          </button>
        </div>
      </nav>
      <div className="app-workspace">
      <div className="app-topbar"><span>Mon observatoire <span className="text-doux">/ {NAV.find(n => n.a === '/' ? pathname === '/' : pathname.startsWith(n.a))?.libelle ?? 'Page'}</span></span><button onClick={() => setPalette(true)} className="top-search"><Icone nom="recherche" /> Rechercher… <kbd>Ctrl K</kbd></button></div>
      <main id="contenu" className="app-main">
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
            <Route path="*" element={<p>Cette page n’existe pas. <NavLink to="/">Retour à l’accueil</NavLink></p>} />
          </Routes>
        </Suspense>
      </main>
      </div>
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
