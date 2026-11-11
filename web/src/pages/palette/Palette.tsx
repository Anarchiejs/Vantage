import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'
import type { ProjectSummary } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Empty } from '../../ui/Status'
import { rank, type Labeled } from './fuzzy'

interface Command extends Labeled {
  run: () => void
}

const PAGES: { id: string; label: string; detail: string; to: string }[] = [
  { id: 'home', label: 'Home', detail: 'Dashboard', to: '/' },
  { id: 'projects', label: 'Projects', detail: 'Project list', to: '/projects' },
  { id: 'tech', label: 'Tech & AI', detail: 'Technologies', to: '/tech' },
  { id: 'graph', label: 'Graph', detail: 'Linked notes', to: '/graph' },
  { id: 'search', label: 'Search', detail: 'Notes and commits', to: '/search' },
  { id: 'console', label: 'SQL console', detail: 'SQLite queries', to: '/console' },
]

function focusable(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>('button, [href], input, select, textarea')].filter(
    (el) => !el.hasAttribute('disabled') && el.tabIndex >= 0 && !el.hidden,
  )
}

export default function Palette({ close }: { close: () => void }) {
  const navigate = useNavigate()
  const projects = useApi<ProjectSummary[]>('/api/projects')
  const projectData = projects.status === 'ok' ? projects.data : null
  const [query, setQuery] = useState('')
  const [index, setIndex] = useState(0)
  const field = useRef<HTMLInputElement>(null)

  useEffect(() => { field.current?.focus() }, [])
  useEffect(() => { setIndex(0) }, [query])

  const commands = useMemo(() => {
    const go = (path: string) => { navigate(path); close() }
    const pages: Command[] = PAGES.map((page) => ({ id: `page-${page.id}`, label: page.label, detail: page.detail, run: () => go(page.to) }))
    const openProjects: Command[] = (projectData ?? []).map((project) => ({
      id: `project-${project.id}`, label: project.name, detail: 'Open the project', run: () => go(`/projects/${project.id}`),
    }))
    const theme: Command = {
      id: 'theme', label: 'Toggle theme', detail: 'Light or dark',
      run: () => { window.dispatchEvent(new Event('observatoire:toggle-theme')); close() },
    }
    return [...pages, ...openProjects, theme]
  }, [projectData, navigate, close])

  const visible = useMemo(() => {
    const ranked = rank(commands, query)
    const text = query.trim()
    if (!text) return ranked
    const search: Command = {
      id: 'search', label: `Search “${text}”`, detail: 'Notes and commits',
      run: () => { navigate(`/search?q=${encodeURIComponent(text)}`); close() },
    }
    return [...ranked, search]
  }, [commands, query, navigate, close])

  const current = visible.length === 0 ? -1 : Math.min(index, visible.length - 1)

  useEffect(() => {
    if (current < 0) return
    const el = document.getElementById(`palette-${visible[current].id}`)
    try { el?.scrollIntoView?.({ block: 'nearest' }) } catch { /* jsdom has no scrolling */ }
  }, [current, visible])

  function onKey(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') { event.preventDefault(); close(); return }
    if (event.key === 'Tab') {
      const items = focusable(event.currentTarget)
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement
      if (event.shiftKey && (active === first || !event.currentTarget.contains(active))) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && active === last) { event.preventDefault(); first.focus() }
      return
    }
    if (visible.length === 0) return
    if (event.key === 'ArrowDown') { event.preventDefault(); setIndex((i) => Math.min(Math.min(Math.max(i, 0), visible.length - 1) + 1, visible.length - 1)) }
    else if (event.key === 'ArrowUp') { event.preventDefault(); setIndex((i) => Math.max(Math.min(i, visible.length - 1) - 1, 0)) }
    else if (event.key === 'Enter' && (event.target as HTMLElement).tagName !== 'BUTTON') { event.preventDefault(); if (current >= 0) visible[current].run() }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-canvas/80 px-4 pt-[12vh]" onClick={close}>
      <div role="dialog" aria-modal="true" aria-label="Command palette" onClick={(e) => e.stopPropagation()} onKeyDown={onKey} className="flex max-h-[70vh] w-full max-w-xl flex-col overflow-hidden rounded-xl border border-line bg-surface">
        <input ref={field} autoFocus aria-label="Command" aria-expanded="true" aria-controls="palette-list" aria-activedescendant={current >= 0 ? `palette-${visible[current].id}` : undefined} autoComplete="off" placeholder="Go to a page, a project, a search…" value={query} onChange={(e) => setQuery(e.target.value)} className="w-full border-b border-line bg-transparent px-4 py-3 text-ink outline-none" />
        {visible.length === 0 ? <Empty>No command.</Empty> : (
          <ul id="palette-list" role="listbox" aria-label="Commands" className="overflow-auto">
            {visible.map((command, i) => (
              <li key={command.id}>
                <button id={`palette-${command.id}`} type="button" role="option" aria-selected={i === current} onMouseEnter={() => setIndex(i)} onClick={command.run} className={`flex w-full items-baseline justify-between gap-3 px-4 py-2 text-left text-sm ${i === current ? 'bg-accent-soft text-ink' : 'hover:bg-surface-2'}`}>
                  <span>{command.label}</span>
                  <span className="shrink-0 text-xs text-muted">{command.detail}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
