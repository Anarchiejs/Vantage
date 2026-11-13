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

const indicators: [keyof Stats, string][] = [['commits', 'Commits'], ['additions', 'Lines added'], ['deletions', 'Lines deleted'], ['sessions', 'Sessions'], ['active_projects', 'Active projects']]
export default function Home() {
  const state = useApi<HomeData>('/api/home')
  const [period, setPeriod] = useState<'week' | 'month'>('week')
  const [span, setSpan] = useState(90)
  const [measure, setMeasure] = useState<'commits' | 'sessions'>('commits')
  return <div className="space-y-5"><Header title="Your work is taking shape." description="An overview of your projects, your ideas, and how they are progressing." action={<span className="date-chip">{date(new Date().toLocaleDateString('sv-SE'))}</span>} /><Load state={state}>{data => <>
    <Card title="Key figures" action={<div className="segmented" role="group" aria-label="Period">{(['week', 'month'] as const).map(item => <button key={item} type="button" aria-pressed={period === item} onClick={() => setPeriod(item)}>{item === 'week' ? 'Week' : 'Month'}</button>)}</div>}>
      <p className="mb-4 text-xs text-muted">{period === 'week' ? 'Last 7 days / previous 7 days' : 'Last 30 days / previous 30 days'}</p>
      <div className="home-stats grid grid-cols-5 gap-4">{indicators.map(([key, label]) => <Stat key={key} label={label} value={data[period][key]} previous={data[period === 'week' ? 'previous_week' : 'previous_month'][key]} />)}</div>
    </Card>
    <Card title="The rhythm of your projects" action={<div className="flex flex-wrap gap-3"><select aria-label="Activity measure" value={measure} onChange={e => setMeasure(e.target.value as 'commits' | 'sessions')} className="border border-line bg-surface px-2 text-xs"><option value="commits">Commits</option><option value="sessions">Sessions</option></select><div className="segmented" role="group" aria-label="Chart range">{[30, 90, 365].map(n => <button key={n} aria-pressed={span === n} onClick={() => setSpan(n)}>{n === 365 ? '1 year' : `${n} days`}</button>)}</div></div>}>
      {data.calendar.length ? <><Chart height={270} aria-label={`${measure === 'commits' ? 'Commits' : 'Sessions'} per day over ${span} days`} option={() => activityOption(data.calendar.slice(-span), measure)} /><p className="chart-footnote">From {date(data.calendar.slice(-span)[0]?.day)} to {date(data.calendar.at(-1)?.day)} · {number(data.calendar.slice(-span).reduce((sum, day) => sum + day[measure], 0))} {measure}. Days with no activity are included.</p>
      <details className="data-details"><summary>View the values by date</summary><div><table><thead><tr><th>Date</th><th>Commits</th><th>Sessions</th><th>Additions</th><th>Deletions</th></tr></thead><tbody>{data.calendar.slice(-span).slice().reverse().map(day => <tr key={day.day}><td>{date(day.day)}</td><td>{number(day.commits)}</td><td>{number(day.sessions)}</td><td>{number(day.additions)}</td><td>{number(day.deletions)}</td></tr>)}</tbody></table></div></details></> : <Empty>No data for this period.</Empty>}
    </Card>
    <Card title="Activity over 12 months" className="min-w-0">{data.calendar.length ? <><div className="chart-scroll"><div className="chart-wide"><Chart height={210} aria-label="Commit calendar over 365 days" option={() => calendarOption(data.calendar)} /></div></div><p className="chart-footnote">Imported Git history, including upstream commits from cloned repositories. Each cell is one day. Hover it to see the commits, sessions, and projects involved.</p></> : <Empty>No activity recorded.</Empty>}</Card>
    <h2 className="font-semibold">Right now</h2><div className="current-projects grid grid-cols-3 items-start gap-4">
      <Card title="Active projects">{data.active.length ? <ul className="divide-y divide-line">{data.active.map(project => <li className="py-3" key={project.id}><Link className="text-accent hover:underline" to={`/projects/${project.id}`}>{project.name}</Link><p className="text-xs text-muted">{number(project.commits_30d)} commits over 30 days · {ago(project.last_commit)}</p></li>)}</ul> : <Empty>No active project.</Empty>}</Card>
      <Card title="Dormant projects">{data.dormant.length ? <ul className="divide-y divide-line">{data.dormant.map(project => <li className="py-3" key={project.id}><Link className="text-accent hover:underline" to={`/projects/${project.id}`}>{project.name}</Link><p className="text-xs text-muted">{project.last_commit ? `${number(project.days)} days without a commit` : 'No commits'}</p></li>)}</ul> : <Empty>No dormant project.</Empty>}{data.quiet_hidden > 0 && <p className="mt-3 text-xs text-muted">Other quiet cards stay off this list because their status is not in progress, active, or ongoing.</p>}</Card>
      <Card title="Stale cards">{data.stale_cards.length ? <ul className="divide-y divide-line">{data.stale_cards.map(project => <li className="py-3" key={project.id}><Link className="text-accent hover:underline" to={`/projects/${project.id}`}>{project.name}</Link><p className="font-mono text-xs text-muted">{shortSha(project.verified_commit)} → {shortSha(project.last_sha)}</p><p className="text-xs text-warning">{project.commits_since < 0 ? 'Reference unknown in the repository' : `${number(project.commits_since)} commits since verification`}</p></li>)}</ul> : <Empty>No stale card.</Empty>}</Card>
    </div>
  </>}</Load></div>
}
