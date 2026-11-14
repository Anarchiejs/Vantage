import { lazy, Suspense, useEffect, useState } from 'react'
import { BrowserRouter, NavLink, Route, Routes, useNavigate, useLocation } from 'react-router'
import { Icon } from './ui/Icon'
import { useTheme } from './theme/theme'
import { Loading } from './ui/Status'
import { useApi } from './api/useApi'
import type { Status } from './api/types'
import { BrandMark } from './ui/BrandMark'

const Home = lazy(() => import('./pages/home/Home'))
const Projects = lazy(() => import('./pages/project/Projects'))
const Project = lazy(() => import('./pages/project/Project'))
const Tech = lazy(() => import('./pages/tech/Tech'))
const Graph = lazy(() => import('./pages/graph/Graph'))
const Search = lazy(() => import('./pages/search/Search'))
const Console = lazy(() => import('./pages/console/Console'))
const Repos = lazy(() => import('./pages/repos/Repos'))
const Palette = lazy(() => import('./pages/palette/Palette'))

const NAV = [
  { to: '/', label: 'Home', icon: 'home' },
  { to: '/projects', label: 'Projects', icon: 'projects' },
  { to: '/repos', label: 'Repos', icon: 'repos' },
  { to: '/tech', label: 'Tech & AI', icon: 'tech' },
  { to: '/graph', label: 'Graph', icon: 'graph' },
  { to: '/search', label: 'Search', icon: 'search' },
  { to: '/console', label: 'SQL console', icon: 'console' },
]

type Live = 'live' | 'offline' | 'static'

/** Relays the server's SSE events as "vantage:update" (listened to by useApi), and reports the connection. */
function useLiveUpdates(): Live {
  const [live, setLive] = useState<Live>('static')
  useEffect(() => {
    if (import.meta.env.VITE_MOCK === '1' || typeof EventSource === 'undefined') return
    const source = new EventSource('/api/events')
    source.onopen = () => setLive('live')
    source.onerror = () => setLive('offline')
    source.addEventListener('update', (event) => {
      setLive('live')
      window.dispatchEvent(new CustomEvent('vantage:update', { detail: JSON.parse((event as MessageEvent).data) }))
    })
    return () => source.close()
  }, [])
  return live
}

const plural = (n: number, word: string) => `${n.toLocaleString('en-US')} ${word}${n === 1 ? '' : 's'}`

/** Sidebar line: what the import tracks, and whether live updates are flowing. */
function SourceStatus({ live }: { live: Live }) {
  const status = useApi<Status>('/api/status')
  if (status.status !== 'ok') return null
  const { repositories, notes } = status.data
  const label = live === 'live' ? 'Watching' : live === 'offline' ? 'Disconnected from' : 'Tracking'
  return <div className="source-status" role="status"><span className="live-dot" data-state={live} aria-hidden="true" /><span>{label} {plural(repositories, 'repo')} &amp; {plural(notes, 'note')}</span></div>
}

/** Global shortcuts: Ctrl+K opens the palette, "/" goes to search. */
function useShortcuts(openPalette: () => void) {
  const navigate = useNavigate()
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement
      const typing = target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        openPalette()
      } else if (event.key === '/' && !typing) {
        event.preventDefault()
        navigate('/search')
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate, openPalette])
}

function Frame() {
  const { pathname } = useLocation()
  useEffect(() => { window.scrollTo(0, 0) }, [pathname])
  const [theme, toggle] = useTheme()
  const [palette, setPalette] = useState(false)
  const live = useLiveUpdates()
  useShortcuts(() => setPalette(true))
  // The palette (or any page) toggles the theme by emitting this event.
  useEffect(() => {
    window.addEventListener('vantage:toggle-theme', toggle)
    return () => window.removeEventListener('vantage:toggle-theme', toggle)
  }, [toggle])

  const current = NAV.find(item => item.to === '/' ? pathname === '/' : pathname.startsWith(item.to))?.label ?? 'Page'

  return (
    <div className="app-shell">
      <a className="skip-link" href="#content">Skip to content</a>
      <nav aria-label="Main" className="app-sidebar">
        <div className="brand">
          <BrandMark />
          <span>Vantage</span>
        </div>
        <div className="nav-links">
        {NAV.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'is-active' : ''}`
            }
          >
            <Icon name={item.icon} /><span>{item.label}</span>
          </NavLink>
        ))}
        </div>
        <div className="sidebar-bottom">
          <SourceStatus live={live} />
          <button
            type="button"
            onClick={() => setPalette(true)}
            className="sidebar-command"
          >
            Commands <kbd>Ctrl K</kbd>
          </button>
          <button type="button" onClick={toggle} className="sidebar-command">
            <Icon name={theme === 'light' ? 'moon' : 'sun'} />
            {theme === 'light' ? 'Dark theme' : 'Light theme'}
          </button>
        </div>
      </nav>
      <div className="app-workspace">
      <div className="app-topbar"><span className="crumbs">Vantage <span aria-hidden="true">/</span> <b>{current}</b></span><button type="button" aria-label="Command palette (Ctrl K)" onClick={() => setPalette(true)} className="top-search"><Icon name="search" /><span>Search notes, projects, commits…</span><kbd>Ctrl K</kbd></button></div>
      <main id="content" className="app-main">
        <Suspense fallback={<Loading lines={6} />}>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/projects" element={<Projects />} />
            <Route path="/projects/:id" element={<Project />} />
            <Route path="/repos" element={<Repos />} />
            <Route path="/tech" element={<Tech />} />
            <Route path="/graph" element={<Graph />} />
            <Route path="/graph/:id" element={<Graph />} />
            <Route path="/search" element={<Search />} />
            <Route path="/console" element={<Console />} />
            <Route path="*" element={<p>This page does not exist. <NavLink to="/">Back to home</NavLink></p>} />
          </Routes>
        </Suspense>
      </main>
      </div>
      {palette && (
        <Suspense fallback={null}>
          <Palette close={() => setPalette(false)} />
        </Suspense>
      )}
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Frame />
    </BrowserRouter>
  )
}
