import { useState } from 'react'
import { Link } from 'react-router'
import type { Home as HomeData, Stats } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Load, Empty } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { Stat } from '../../ui/Stat'
import { ago, date, number, shortSha } from '../../ui/format'
import { Header } from '../../ui/Header'
import { activityOption } from './activity'
import { calendarOption } from './calendar'
import './home.css'
export { calendarOption } from './calendar'

type Period = 'week' | 'month' | 'year'
const indicators: [keyof Stats, string][] = [['commits', 'Commits'], ['additions', 'Lines added'], ['deletions', 'Lines deleted'], ['sessions', 'Sessions'], ['active_projects', 'Active projects']]
const periods: Record<Period, { label: string; previous: 'previous_week' | 'previous_month' | 'previous_year'; caption: string }> = {
  week: { label: 'Week', previous: 'previous_week', caption: 'Last 7 days / previous 7 days' },
  month: { label: 'Month', previous: 'previous_month', caption: 'Last 30 days / previous 30 days' },
  year: { label: 'Year', previous: 'previous_year', caption: 'Last 365 days / previous 365 days' },
}
const longDate = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' })

/** When nothing was committed this week, say when and where the last commit happened instead of a row of zeros. */
export function QuietWeek({ data }: { data: HomeData }) {
  if (data.week.commits > 0) return null
  const last = [...data.calendar].reverse().find(day => day.commits > 0)
  if (!last) return null
  const name = last.projects[0]
  const project = name ? [...data.active, ...data.dormant, ...data.stale_cards].find(item => item.name === name) : undefined
  return <section className="callout" aria-label="Quiet week">
    <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-surface-2 text-muted" aria-hidden="true"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg></span>
    <p><strong className="block font-semibold">A quiet week: nothing committed in the last 7 days.</strong><span className="text-muted">Your last commit landed {ago(last.day)}, on {date(last.day)}{name ? <> in <b className="font-semibold text-ink">{name}</b></> : null}. {number(data.year.commits)} commits over the past 12 months.</span></p>
    {project && <Link className="btn btn-primary" to={`/projects/${project.id}`}>Resume {project.name} →</Link>}
  </section>
}

export default function Home() {
  const state = useApi<HomeData>('/api/home')
  const [period, setPeriod] = useState<Period>('week')
  const [span, setSpan] = useState(90)
  const [measure, setMeasure] = useState<'commits' | 'sessions'>('commits')
  return <div className="space-y-5"><Header eyebrow={longDate.format(new Date())} title="Your work is taking shape." description="An overview of your projects, your ideas, and how they are progressing." /><Load state={state}>{data => <>
    <QuietWeek data={data} />
    <Card title="Key figures" action={<div className="segmented" role="group" aria-label="Period">{(Object.keys(periods) as Period[]).map(item => <button key={item} type="button" aria-pressed={period === item} onClick={() => setPeriod(item)}>{periods[item].label}</button>)}</div>}>
      <p className="mb-4 text-xs text-muted">{periods[period].caption}</p>
      <div className="home-stats grid grid-cols-5 gap-4">{indicators.map(([key, label]) => <Stat key={key} label={label} value={data[period][key]} previous={data[periods[period].previous][key]} note={period === 'year' ? undefined : `${number(data.year[key])} over 12 months`} />)}</div>
    </Card>
    <Card title="Activity over 12 months" className="min-w-0">{data.calendar.length ? <><div className="chart-scroll"><div className="chart-wide"><Chart height={210} aria-label="Commit calendar over 365 days" option={() => calendarOption(data.calendar)} /></div></div><p className="chart-footnote">Imported Git history, including upstream commits from cloned repositories. Each cell is one day. Hover it to see the commits, sessions, and projects involved.</p></> : <Empty>No activity recorded.</Empty>}</Card>
    <Card title="The rhythm of your projects" action={<div className="flex flex-wrap gap-3"><select aria-label="Activity measure" value={measure} onChange={e => setMeasure(e.target.value as 'commits' | 'sessions')}><option value="commits">Commits</option><option value="sessions">Sessions</option></select><div className="segmented" role="group" aria-label="Chart range">{[30, 90, 365].map(n => <button key={n} type="button" aria-pressed={span === n} onClick={() => setSpan(n)}>{n === 365 ? '1 year' : `${n} days`}</button>)}</div></div>}>
      {data.calendar.length ? <><Chart height={240} aria-label={`${measure === 'commits' ? 'Commits' : 'Sessions'} per day over ${span} days`} option={() => activityOption(data.calendar.slice(-span), measure)} /><p className="chart-footnote">From {date(data.calendar.slice(-span)[0]?.day)} to {date(data.calendar.at(-1)?.day)} · {number(data.calendar.slice(-span).reduce((sum, day) => sum + day[measure], 0))} {measure}. Days with no activity are included.</p>
      <details className="data-details"><summary>View the values by date</summary><div><table><thead><tr><th>Date</th><th>Commits</th><th>Sessions</th><th>Additions</th><th>Deletions</th></tr></thead><tbody>{data.calendar.slice(-span).slice().reverse().map(day => <tr key={day.day}><td>{date(day.day)}</td><td>{number(day.commits)}</td><td>{number(day.sessions)}</td><td>{number(day.additions)}</td><td>{number(day.deletions)}</td></tr>)}</tbody></table></div></details></> : <Empty>No data for this period.</Empty>}
    </Card>
    <h2 className="pt-2 text-base font-semibold">Right now</h2><div className="current-projects grid grid-cols-2 items-start gap-4">
      <Card title="Active projects">{data.active.length ? <ul>{data.active.map(project => <li key={project.id}><Link className="link-row" to={`/projects/${project.id}`}><span className="avatar avatar-sm" aria-hidden="true">{project.name.slice(0, 1)}</span><span className="min-w-0 flex-1"><span className="block font-semibold">{project.name}</span><span className="block text-xs text-muted">{number(project.commits_30d)} commits over 30 days · {ago(project.last_commit)}</span></span></Link></li>)}</ul> : <Empty>No active project.</Empty>}</Card>
      <Card title="Dormant projects">{data.dormant.length ? <ul>{data.dormant.map(project => <li key={project.id}><Link className="link-row" to={`/projects/${project.id}`}><span className="avatar avatar-sm bg-surface-2 text-muted" aria-hidden="true">{project.name.slice(0, 1)}</span><span className="min-w-0 flex-1"><span className="block font-semibold">{project.name}</span><span className="block text-xs text-muted">{project.last_commit ? `${number(project.days)} days without a commit` : 'No commits'}</span></span></Link></li>)}</ul> : <Empty>No dormant project.</Empty>}{data.quiet_hidden > 0 && <p className="mt-3 text-xs text-muted">Other quiet cards stay off this list because their status is not in progress, active, or ongoing.</p>}</Card>
      <Card title="Stale cards">{data.stale_cards.length ? <ul>{data.stale_cards.map(project => <li key={project.id}><Link className="link-row py-2" to={`/projects/${project.id}`}><span className="min-w-0 flex-1"><span className="block font-semibold">{project.name}</span><span className="block font-mono text-xs text-muted">{shortSha(project.verified_commit)} → {shortSha(project.last_sha)}</span><span className="block text-xs text-warning">{project.commits_since < 0 ? 'Reference unknown in the repository' : `${number(project.commits_since)} commits since verification`}</span></span></Link></li>)}</ul> : <Empty>No stale card.</Empty>}</Card>
      <Card title="Latest notes & sessions">{data.recent_notes.length ? <ul>{data.recent_notes.map(note => <li key={note.id}><Link className="link-row" to={`/graph/${note.id}`}><span className="w-14 shrink-0 font-mono text-xs text-muted">{date(note.day).replace(/, \d{4}$/, '')}</span><span className="min-w-0 flex-1"><span className="block truncate font-medium">{note.title}</span><span className="block text-xs text-muted">{note.folder || 'Vault root'}</span></span></Link></li>)}</ul> : <Empty>No dated note yet.</Empty>}</Card>
    </div>
  </>}</Load></div>
}
