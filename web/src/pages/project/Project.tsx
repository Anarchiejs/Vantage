import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router'
import type { Health, LanguageShare, LinkRepoRequest, LinkRepoResponse, ProjectDetail, Status, TimelineEvent, EventType } from '../../api/types'
import { useApi } from '../../api/useApi'
import { post } from '../../api/client'
import { series } from '../../theme/theme'
import { Card } from '../../ui/Card'
import { Load, Empty } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { ago, date, number, shortSha } from '../../ui/format'
import { evolutionOption, volumeOption } from './charts'

export function CardHealth({ health }: { health: Health }) {
  return <Card title="Card health"><p className={`mb-3 font-semibold ${health.up_to_date ? 'text-success' : 'text-warning'}`}>{health.up_to_date ? 'Up to date' : 'Stale'}</p>
    <dl className="grid grid-cols-2 gap-2 text-sm"><dt className="text-muted">Verified SHA</dt><dd className="font-mono" title={health.verified_commit ?? undefined}>{shortSha(health.verified_commit)}</dd><dt className="text-muted">Latest SHA</dt><dd className="font-mono" title={health.last_sha ?? undefined}>{shortSha(health.last_sha)}</dd><dt className="text-muted">Verified on</dt><dd>{date(health.verified_on)}</dd><dt className="text-muted">Commits since</dt><dd>{health.commits_since < 0 ? 'Reference unknown in the repository' : number(health.commits_since)}</dd></dl>
    {health.activity_note && <p className="mt-3 text-sm">{health.activity_note}</p>}
    {health.missing_notes.length ? <div className="mt-3 text-sm text-warning">Missing notes:<ul className="list-inside list-disc">{health.missing_notes.map(note => <li key={note}>{note}</li>)}</ul></div> : <p className="mt-3 text-xs text-muted">No missing notes.</p>}
    <button type="button" className="mt-3 rounded border border-line px-3 py-2 text-sm" disabled={!health.last_sha} onClick={() => { if (health.last_sha) void navigator.clipboard.writeText(`verified_commit: ${health.last_sha}`) }}>Copy verified SHA</button>
  </Card>
}
const labels: Record<EventType, string> = { commit: 'Commits', session: 'Sessions', journal: 'Journal' }
const dots: Record<EventType, string> = { commit: 'bg-accent', session: 'bg-success', journal: 'bg-warning' }
export function filterTimeline(events: TimelineEvent[], types: EventType[]) {
  return [...events].sort((a, b) => b.date.localeCompare(a.date)).slice(0, 500).filter(event => types.includes(event.type))
}
export function Timeline({ events }: { events: TimelineEvent[] }) {
  const [types, setTypes] = useState<EventType[]>(['commit', 'session', 'journal'])
  const [limit, setLimit] = useState(50)
  const filtered = filterTimeline(events, types)
  const groups = new Map<string, TimelineEvent[]>()
  for (const event of filtered.slice(0, limit)) { const month = event.date.slice(0, 7); groups.set(month, [...(groups.get(month) ?? []), event]) }
  return <Card title="Timeline"><div className="mb-4 flex gap-4" role="group" aria-label="Event types">{(Object.keys(labels) as EventType[]).map(type => <label key={type} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={types.includes(type)} onChange={() => { setTypes(types.includes(type) ? types.filter(item => item !== type) : [...types, type]); setLimit(50) }} />{labels[type]}</label>)}</div>
    <p className="mb-3 text-xs text-muted">{number(filtered.length)} events · 500 most recent events at most</p>
    {filtered.length ? [...groups].map(([month, rows]) => <section key={month} className="mb-4"><h3 className="mb-3 text-sm font-semibold">{new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric' }).format(new Date(`${month}-01T12:00:00`))}</h3><ol className="ml-2 border-l border-line">{rows.map((event, index) => <li key={`${event.type}-${event.ref}-${index}`} className="relative mb-3 pl-5"><span aria-hidden="true" className={`absolute -left-1 top-2 h-2 w-2 rounded-full ${dots[event.type]}`} /><div className="flex flex-wrap gap-2 text-xs text-muted"><time dateTime={event.date}>{date(event.date)}</time><span>{labels[event.type]}</span>{event.ai && <span className="text-accent">{event.ai}</span>}</div><p className="text-sm break-words">{event.title}</p><p className="truncate font-mono text-xs text-muted" title={event.ref}>{event.type === 'commit' ? shortSha(event.ref) : event.ref}</p></li>)}</ol></section>) : <Empty>No events for these filters.</Empty>}
    {filtered.length > limit && <button type="button" onClick={() => setLimit(n => n + 50)} className="rounded border border-line px-3 py-2 text-sm text-accent">Show more</button>}
  </Card>
}
const tones: Record<string, 'active' | 'paused'> = { 'in progress': 'active', active: 'active', ongoing: 'active', paused: 'paused', 'on hold': 'paused', blocked: 'paused' }

export function StatusPill({ status }: { status: string | null }) {
  return <span className="status-pill" data-tone={tones[(status ?? '').toLowerCase()] ?? 'neutral'}>{status || 'Unknown status'}</span>
}

/** Links a repository folder to the project (writes <vault>/.vantage/repos under "vantage watch"). */
export function LinkFolder({ project, onLinked }: { project: ProjectDetail['project']; onLinked: () => void }) {
  const status = useApi<Status>('/api/status')
  const [open, setOpen] = useState(false)
  const [path, setPath] = useState('')
  const [state, setState] = useState<{ busy: boolean; error?: string }>({ busy: false })
  const canLink = status.status === 'ok' && status.data.can_link_repos === true
  const missing = !project.source_path
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setState({ busy: true })
    try {
      await post<LinkRepoResponse>(`/api/projects/${project.id}/repo`, { path } satisfies LinkRepoRequest)
      setState({ busy: false }); setOpen(false); setPath(''); onLinked()
    } catch (error) {
      setState({ busy: false, error: (error as Error).message })
    }
  }
  const form = open && canLink && <form className="flex w-full flex-wrap items-center gap-2" onSubmit={submit}>
    <label className="field min-w-0 flex-[1_1_320px]"><span className="sr-only">Repository folder</span><input autoFocus value={path} onChange={e => setPath(e.target.value)} placeholder="C:\Users\you\code\project" aria-label="Repository folder" /></label>
    <button className="btn btn-primary" disabled={state.busy || !path.trim()}>{state.busy ? 'Importing…' : 'Link and import'}</button>
    <button type="button" className="btn btn-secondary" onClick={() => { setOpen(false); setState({ busy: false }) }}>Cancel</button>
    {state.error && <p role="alert" className="w-full text-sm text-danger">{state.error}</p>}
  </form>
  if (!missing) return <div className="flex flex-wrap items-center gap-2"><span className="font-mono text-xs text-muted break-all">{project.source_path}</span>{canLink && !open && <button type="button" className="text-xs text-accent hover:underline" onClick={() => setOpen(true)}>Change</button>}{form}</div>
  return <section className="callout callout-warning" aria-label="Source folder">
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M12 3 2 20h20z" /><path d="M12 10v4M12 17h.01" /></svg>
    <p><strong className="font-semibold">No source folder linked.</strong> Languages and hotspots need the local path of this repository.{!canLink && <> Add <code className="font-mono">{project.name}=C:\path\to\repo</code> to <code className="font-mono">.vantage/repos</code> in your vault, or start <code className="font-mono">vantage watch</code> to link it from here.</>}</p>
    {canLink && !open && <button type="button" className="btn btn-warning" onClick={() => setOpen(true)}>Link a folder</button>}
    {form}
  </section>
}

/** Figures read from the detail already loaded: no extra request. */
export function projectFigures(detail: ProjectDetail) {
  const weeks = detail.volume.filter(week => week.commits > 0 || week.additions > 0 || week.deletions > 0)
  const peak = weeks.reduce<ProjectDetail['volume'][number] | undefined>((best, week) => !best || week.additions > best.additions ? week : best, undefined)
  return { activeWeeks: weeks.length, totalWeeks: detail.volume.length, first: weeks[0]?.week, last: weeks.at(-1)?.week, peak }
}

export function LanguageBar({ languages }: { languages: LanguageShare[] }) {
  const colors = series()
  const rows = [...languages].sort((a, b) => b.share - a.share)
  const pct = (share: number) => `${share < 0.01 && share > 0 ? '<1' : Math.round(share * 100)}%`
  return <>
    <div role="img" aria-label="Share of lines of code by language" className="flex h-3 gap-0.5 overflow-hidden rounded-full bg-surface-2">{rows.map((item, i) => <span key={item.language} title={`${item.language} ${pct(item.share)}`} style={{ width: `${item.share * 100}%`, background: colors[i % colors.length] }} />)}</div>
    <ul className="mt-3">{rows.map((item, i) => <li key={item.language} className="flex items-center gap-3 border-t border-line py-2 text-sm first:border-t-0"><span className="dot" style={{ background: colors[i % colors.length] }} aria-hidden="true" /><span className="flex-1">{item.language}</span><span className="tabular-nums text-muted">{number(item.code_lines)} lines</span><span className="w-12 text-right tabular-nums font-medium">{pct(item.share)}</span></li>)}</ul>
  </>
}

function ProjectBody({ detail, reload }: { detail: ProjectDetail; reload: () => void }) {
  const project = detail.project
  const [weeks, setWeeks] = useState(52)
  const figures = projectFigures(detail)
  const measurements = new Set(detail.language_evolution.map(item => item.measured_on)).size
  return <>
    <header className="flex flex-wrap items-start justify-between gap-5">
      <div className="flex min-w-0 items-center gap-4">
        <span className="avatar" aria-hidden="true">{project.name.slice(0, 1)}</span>
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-3"><h1 className="mb-0!">{project.name}</h1><StatusPill status={project.status} /></div>
          {project.technologies.length > 0 && <div className="flex flex-wrap gap-1.5">{project.technologies.map(item => <span key={item} className="chip">{item}</span>)}</div>}
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {detail.obsidian_url && <a className="btn btn-secondary" href={detail.obsidian_url}>Open in Obsidian</a>}
        <Link className="btn btn-secondary" to={`/graph?note=${encodeURIComponent(project.name)}`}>View in the graph →</Link>
      </div>
    </header>
    {(project.source_path || detail.repo_paths.length === 0) && <LinkFolder project={project} onLinked={reload} />}
    {detail.repo_paths.length > 0 && <ul className="space-y-1">{detail.repo_paths.map(path => <li key={path} className="flex items-center gap-2 font-mono text-xs"><span className="break-all text-muted">{path}</span><button type="button" className="chip" onClick={() => void navigator.clipboard.writeText(path)}>Copy</button></li>)}</ul>}
    <section aria-label="Project summary" className="kpi-grid">
      <div className="kpi"><span className="kpi-label">Last commit</span><span className="kpi-value">{project.last_commit ? date(project.last_commit).replace(/, \d{4}$/, '') : '—'}</span><span className="kpi-note">{ago(project.last_commit)}</span></div>
      <div className="kpi"><span className="kpi-label">Commits</span><span className="kpi-value">{number(project.commits)}</span><span className="kpi-note">{project.primary_language ? `Mostly ${project.primary_language}` : 'All imported history'}</span></div>
      <div className="kpi"><span className="kpi-label">Active weeks</span><span className="kpi-value">{figures.activeWeeks} <span className="text-sm font-medium text-muted">/ {figures.totalWeeks}</span></span><span className="kpi-note">{figures.first ? `${date(figures.first).replace(/, \d{4}$/, '')} → ${date(figures.last).replace(/, \d{4}$/, '')}` : 'No activity in 52 weeks'}</span></div>
      <div className="kpi"><span className="kpi-label">Peak week</span><span className="kpi-value">{figures.peak ? `+${number(figures.peak.additions)}` : '—'}</span><span className="kpi-note">{figures.peak ? `lines, week of ${date(figures.peak.week).replace(/, \d{4}$/, '')}` : 'No lines changed'}</span></div>
    </section>
    <Card title="Commits since the last session">{detail.since_session.length ? <><p className="mb-2 text-xs text-muted">Commits after {date(detail.since_session[0].since)}</p><ul className="space-y-2 text-sm">{detail.since_session.map(commit => <li key={commit.ref}><span className="font-mono text-xs text-muted">{shortSha(commit.ref)}</span> {commit.title}</li>)}</ul></> : <Empty>No session yet, so there is nothing to list after one.</Empty>}</Card>
    <Card title="Code volume per week" action={<div className="segmented" role="group" aria-label="Project history">{[13, 26, 52].map(n => <button key={n} type="button" aria-pressed={weeks === n} onClick={() => setWeeks(n)}>{n} weeks</button>)}</div>}>{detail.volume.length ? <><p className="-mt-2 mb-2 text-xs text-muted">Additions above the line, deletions below</p><Chart aria-label="Lines added and deleted per week" option={() => volumeOption(detail.volume.slice(-weeks))} /></> : <Empty>No code volume recorded.</Empty>}</Card>
    <div className="grid grid-cols-2 gap-4"><Card title="Languages — latest measurement">{detail.languages.length ? <LanguageBar languages={detail.languages} /> : <Empty>No language measurement.</Empty>}</Card><Card title="Language evolution">{measurements > 1 ? <Chart aria-label="Evolution of lines of code by language" option={() => evolutionOption(detail.language_evolution)} /> : <div className="empty-state"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" aria-hidden="true"><path d="M3 20h18M6 16l4-5 3 3 5-7" /></svg><strong>{measurements === 1 ? 'One measurement so far' : 'No measurement yet'}</strong><span>Two measurements are needed to show the evolution. A new one is taken when the repository changes.</span></div>}</Card></div>
    <div className="grid grid-cols-[minmax(0,2fr)_minmax(280px,1fr)] gap-4"><Timeline key={project.id} events={detail.timeline} /><div className="space-y-4"><CardHealth health={detail.health} /><Card title="Project AI">{detail.ai.length ? <ul className="divide-y divide-line">{detail.ai.map(item => <li key={item.ai} className="py-3"><p className="flex justify-between text-sm"><span>{item.ai}</span><span className="tabular-nums">{number(item.sessions)} sessions</span></p><p className="text-xs text-muted">From {date(item.first)} to {date(item.last)}</p></li>)}</ul> : <Empty>No AI session recorded.</Empty>}</Card></div></div>
    <Card title="Hotspots — 20 files">{detail.hotspots.length ? <table className="w-full text-left text-sm"><thead><tr>{['File', 'Modifications', 'Additions', 'Deletions'].map(title => <th className="border-b border-line p-2" key={title}>{title}</th>)}</tr></thead><tbody>{detail.hotspots.slice(0, 20).map(file => <tr key={file.path} className="border-b border-line"><td className="p-2 font-mono text-xs break-all">{file.path}</td><td className="p-2 tabular-nums">{number(file.modifications)}</td><td className="p-2 text-success tabular-nums">+{number(file.additions)}</td><td className="p-2 text-danger tabular-nums">−{number(file.deletions)}</td></tr>)}</tbody></table> : <Empty>No modified file.</Empty>}</Card>
  </>
}
export default function Project() {
  const { id } = useParams()
  const state = useApi<ProjectDetail>(`/api/projects/${encodeURIComponent(id ?? '')}`)
  return <div className="space-y-5"><Link to="/projects" className="text-sm text-accent hover:underline">← All projects</Link><Load state={state}>{detail => <ProjectBody key={detail.project.id} detail={detail} reload={state.reload} />}</Load></div>
}
