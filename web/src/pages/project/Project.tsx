import { useState } from 'react'
import { Link, useParams } from 'react-router'
import type { Health, ProjectDetail, TimelineEvent, EventType } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Load, Empty } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { date, number, shortSha } from '../../ui/format'
import { evolutionOption, languagesOption, volumeOption } from './charts'

export function CardHealth({ health }: { health: Health }) {
  return <Card title="Card health"><p className={`mb-3 font-semibold ${health.up_to_date ? 'text-success' : 'text-warning'}`}>{health.up_to_date ? 'Up to date' : 'Stale'}</p>
    <dl className="grid grid-cols-2 gap-2 text-sm"><dt className="text-muted">Verified SHA</dt><dd className="font-mono" title={health.verified_commit ?? undefined}>{shortSha(health.verified_commit)}</dd><dt className="text-muted">Latest SHA</dt><dd className="font-mono" title={health.last_sha ?? undefined}>{shortSha(health.last_sha)}</dd><dt className="text-muted">Verified on</dt><dd>{date(health.verified_on)}</dd><dt className="text-muted">Commits since</dt><dd>{health.commits_since < 0 ? 'Reference unknown in the repository' : number(health.commits_since)}</dd></dl>
    {health.missing_notes.length ? <div className="mt-3 text-sm text-warning">Missing notes:<ul className="list-inside list-disc">{health.missing_notes.map(note => <li key={note}>{note}</li>)}</ul></div> : <p className="mt-3 text-xs text-muted">No missing notes.</p>}
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
function ProjectBody({ detail }: { detail: ProjectDetail }) {
  const project = detail.project
  const [weeks, setWeeks] = useState(13)
  return <>
    <header className="space-y-2"><div className="flex items-center gap-3"><h1 className="text-xl font-semibold">{project.name}</h1><span className="rounded bg-surface-2 px-2 py-1 text-xs">{project.status || 'Unknown status'}</span></div><p className="font-mono text-xs text-muted break-all">{project.source_path || 'Source path not set'}</p><div className="flex flex-wrap gap-2">{project.technologies.map(item => <span key={item} className="rounded bg-accent-soft px-2 py-1 text-xs text-accent">{item}</span>)}</div><Link className="inline-block text-sm text-accent hover:underline" to={`/graph?note=${encodeURIComponent(project.name)}`}>View in the graph →</Link></header>
    <Card title="Code volume per week" action={<div className="segmented" role="group" aria-label="Project history">{[13, 26, 52].map(n => <button key={n} aria-pressed={weeks === n} onClick={() => setWeeks(n)}>{n} weeks</button>)}</div>}>{detail.volume.length ? <Chart aria-label="Lines added and deleted per week" option={() => volumeOption(detail.volume.slice(-weeks))} /> : <Empty>No code volume recorded.</Empty>}</Card>
    <div className="grid grid-cols-2 gap-4"><Card title="Languages — latest measurement">{detail.languages.length ? <Chart aria-label="Share of lines of code by language" option={() => languagesOption(detail.languages)} /> : <Empty>No language measurement.</Empty>}</Card><Card title="Language evolution">{new Set(detail.language_evolution.map(item => item.measured_on)).size > 1 ? <Chart aria-label="Evolution of lines of code by language" option={() => evolutionOption(detail.language_evolution)} /> : <Empty>Two measurements are needed to show the evolution.</Empty>}</Card></div>
    <div className="grid grid-cols-[minmax(0,2fr)_minmax(280px,1fr)] gap-4"><Timeline key={project.id} events={detail.timeline} /><div className="space-y-4"><CardHealth health={detail.health} /><Card title="Project AI">{detail.ai.length ? <ul className="divide-y divide-line">{detail.ai.map(item => <li key={item.ai} className="py-3"><p className="flex justify-between text-sm"><span>{item.ai}</span><span className="tabular-nums">{number(item.sessions)} sessions</span></p><p className="text-xs text-muted">From {date(item.first)} to {date(item.last)}</p></li>)}</ul> : <Empty>No AI session recorded.</Empty>}</Card></div></div>
    <Card title="Hotspots — 20 files">{detail.hotspots.length ? <table className="w-full text-left text-sm"><thead><tr>{['File', 'Modifications', 'Additions', 'Deletions'].map(title => <th className="border-b border-line p-2" key={title}>{title}</th>)}</tr></thead><tbody>{detail.hotspots.slice(0, 20).map(file => <tr key={file.path} className="border-b border-line"><td className="p-2 font-mono text-xs break-all">{file.path}</td><td className="p-2 tabular-nums">{number(file.modifications)}</td><td className="p-2 text-success tabular-nums">+{number(file.additions)}</td><td className="p-2 text-danger tabular-nums">−{number(file.deletions)}</td></tr>)}</tbody></table> : <Empty>No modified file.</Empty>}</Card>
  </>
}
export default function Project() {
  const { id } = useParams()
  const state = useApi<ProjectDetail>(`/api/projects/${encodeURIComponent(id ?? '')}`)
  return <div className="space-y-5"><Link to="/projects" className="text-sm text-accent hover:underline">← All projects</Link><Load state={state}>{detail => <ProjectBody key={detail.project.id} detail={detail} />}</Load></div>
}
