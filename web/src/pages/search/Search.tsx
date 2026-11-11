import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import type { ProjectSummary, SearchHit } from '../../api/types'
import { withParams } from '../../api/client'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Empty, ErrorMessage, Loading } from '../../ui/Status'
import { date } from '../../ui/format'
import { Header } from '../../ui/Header'
import { highlightedHtml } from './highlight'

const EXAMPLES = ['"exact phrase"', 'go AND sqlite', 'pref*']

function apiSource(value: string): string {
  return value === 'note' || value === 'commit' ? value : ''
}

function target(hit: SearchHit, projects: ProjectSummary[]): string {
  if (hit.source === 'note') return hit.note_id != null ? `/graph/${hit.note_id}` : '/graph'
  const project = projects.find((item) => item.name === hit.project)
  return project ? `/projects/${project.id}` : '/projects'
}

function Highlighted({ text }: { text: string }) {
  return <span dangerouslySetInnerHTML={{ __html: highlightedHtml(text) }} />
}

export default function Search() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const qUrl = params.get('q') ?? ''
  const source = params.get('source') ?? ''
  const project = params.get('project') ?? ''
  const folder = params.get('folder') ?? ''
  const since = params.get('since') ?? ''
  const until = params.get('until') ?? ''

  const [draft, setDraft] = useState(qUrl)
  const written = useRef(qUrl)
  const field = useRef<HTMLInputElement>(null)
  const [active, setActive] = useState(0)

  useEffect(() => { field.current?.focus() }, [])
  useEffect(() => {
    if (qUrl === written.current) return
    written.current = qUrl
    setDraft(qUrl)
  }, [qUrl])
  useEffect(() => {
    if (draft === qUrl) return
    const timer = window.setTimeout(() => {
      written.current = draft
      setParams((current) => {
        const next = new URLSearchParams(current)
        if (draft) next.set('q', draft)
        else next.delete('q')
        return next
      }, { replace: true })
    }, 150)
    return () => window.clearTimeout(timer)
  }, [draft, qUrl, setParams])

  function update(partial: Record<string, string>) {
    setParams((current) => {
      const next = new URLSearchParams(current)
      for (const [key, value] of Object.entries(partial)) {
        if (value) next.set(key, value)
        else next.delete(key)
      }
      return next
    }, { replace: true })
  }

  const path = qUrl.trim()
    ? withParams('/api/search', { q: qUrl, source: apiSource(source), project, folder, since, until })
    : null
  const state = useApi<SearchHit[]>(path)
  const projectState = useApi<ProjectSummary[]>('/api/projects')
  const projects = projectState.status === 'ok' ? projectState.data : []
  const hits = qUrl.trim() && state.status === 'ok' ? state.data : []
  const index = hits.length === 0 ? -1 : Math.min(active, hits.length - 1)

  useEffect(() => { setActive(0) }, [path])

  const projectNames = projects.map((item) => item.name)
  const projectOptions = project && !projectNames.includes(project) ? [project, ...projectNames] : projectNames

  function open(hit: SearchHit) { navigate(target(hit, projects)) }

  function onKey(event: KeyboardEvent<HTMLInputElement>) {
    if (hits.length === 0) return
    if (event.key === 'ArrowDown') { event.preventDefault(); setActive((i) => Math.min(Math.max(i, 0) + 1, hits.length - 1)) }
    else if (event.key === 'ArrowUp') { event.preventDefault(); setActive((i) => Math.max(Math.min(i, hits.length - 1) - 1, 0)) }
    else if (event.key === 'Enter' && index >= 0) { event.preventDefault(); open(hits[index]) }
  }

  return (
    <div className="space-y-4">
      <Header title="Search" description="Find an idea, a note, or a change across all of your work." />
      <form role="search" className="space-y-3" onSubmit={(e) => e.preventDefault()}>
        <input ref={field} type="search" aria-label="Search" aria-controls="results" aria-activedescendant={index >= 0 ? `result-${index}` : undefined} autoComplete="off" placeholder="Notes, commits, projects…" value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={onKey} className="w-full rounded-lg border border-line bg-surface px-3 py-2 text-ink outline-none focus:border-accent" />
        <div className="grid grid-cols-5 gap-3">
          <label className="flex flex-col gap-1 text-xs text-muted">Source
            <select aria-label="Source" value={apiSource(source)} onChange={(e) => update({ source: e.target.value })} className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-ink">
              <option value="">All</option><option value="note">Notes</option><option value="commit">Commits</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">Project
            <select aria-label="Project" value={project} onChange={(e) => update({ project: e.target.value })} className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-ink">
              <option value="">All projects</option>
              {projectOptions.map((name) => <option key={name} value={name}>{name}</option>)}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">Folder
            <input aria-label="Folder" value={folder} placeholder="Projects, Journal…" onChange={(e) => update({ folder: e.target.value })} className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-ink" />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">From
            <input type="date" aria-label="From" value={since} onChange={(e) => update({ since: e.target.value })} className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-ink" />
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted">Until
            <input type="date" aria-label="Until" value={until} onChange={(e) => update({ until: e.target.value })} className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-ink" />
          </label>
        </div>
      </form>
      {!qUrl.trim() && (
        <Card title="FTS5 syntax">
          <p className="mb-2 text-sm text-muted">Search covers vault notes and commit messages. A few examples:</p>
          <ul className="flex flex-col gap-1 font-mono text-sm">{EXAMPLES.map((example) => <li key={example}><code>{example}</code></li>)}</ul>
        </Card>
      )}
      {qUrl.trim() && state.status === 'loading' && <Loading lines={5} />}
      {qUrl.trim() && state.status === 'error' && <ErrorMessage error={state.error} />}
      {qUrl.trim() && state.status === 'ok' && hits.length === 0 && <Empty>No results for “{qUrl}”.</Empty>}
      {hits.length > 0 && (
        <div id="results" role="listbox" aria-label="Results" className="overflow-hidden rounded-xl border border-line bg-surface">
          {hits.map((hit, i) => (
            <Link key={`${hit.source}-${hit.ref}-${i}`} id={`result-${i}`} to={target(hit, projects)} role="option" aria-selected={i === index} onMouseEnter={() => setActive(i)} className={`flex items-start gap-3 border-b border-line px-3 py-2 last:border-b-0 ${i === index ? 'bg-accent-soft' : 'hover:bg-surface-2'}`}>
              <span className="sr-only">{hit.source === 'note' ? 'Note' : 'Commit'}</span>
              <span aria-hidden="true" className="mt-0.5 w-5 text-center font-mono text-xs text-accent">{hit.source === 'note' ? '✦' : '›'}</span>
              <span className="min-w-0 flex-1">
                <span className="block font-medium"><Highlighted text={hit.title} /></span>
                <span className="block truncate text-sm text-muted"><Highlighted text={hit.excerpt} /></span>
              </span>
              <span className="shrink-0 text-sm text-muted">{hit.project ?? '—'}</span>
              <span className="w-28 shrink-0 text-right text-sm text-muted tabular-nums">{date(hit.date)}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
