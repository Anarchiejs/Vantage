import { Link, useNavigate } from 'react-router'
import type { EChartsOption } from 'echarts'
import { useState } from 'react'
import type { Assistant, ProjectSummary, TechTrend, Technologies as TechData } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Empty, Load } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { series, token } from '../../theme/theme'
import { number } from '../../ui/format'
import { Header } from '../../ui/Header'

const monthLabel = (value: string) => new Date(`${value.slice(0, 7)}-01T12:00:00`).toLocaleDateString('en-US', { month: 'short', year: '2-digit' })

export function filterSankey(data: TechData, name: string): TechData {
  if (!name) return data
  const links = data.sankey.links.filter(link => link.source === name)
  const technologies = new Set(links.map(link => link.target))
  return { ...data, sankey: { links, nodes: data.sankey.nodes.filter(node => node.type === 'project' ? node.name === name : technologies.has(node.name)) } }
}

export function sankeyHeight(data: TechData): number {
  const projects = data.sankey.nodes.filter(node => node.type === 'project').length
  const technologies = data.sankey.nodes.length - projects
  return Math.max(360, Math.max(projects, technologies) * 22)
}

export function sankeyOption(data: TechData): EChartsOption {
  const colors = series()
  return { xAxis: { show: false }, yAxis: { show: false }, tooltip: { trigger: 'item', renderMode: 'richText' }, series: [{ type: 'sankey', left: 12, right: 130, top: 12, bottom: 12, nodeGap: 14, label: { fontSize: 12, color: token('ink'), width: 125, overflow: 'truncate' },
    data: data.sankey.nodes.map((node, index) => ({ name: `${node.type}:${node.name}`, nodeName: node.name, nodeType: node.type,
      depth: node.type === 'project' ? 0 : 1, itemStyle: { color: colors[index % colors.length] }, label: { formatter: node.name } })),
    links: data.sankey.links.map(link => ({ source: `project:${link.source}`, target: `technology:${link.target}`, value: link.value })),
    lineStyle: { color: 'source', opacity: 0.3 }, emphasis: { focus: 'adjacency' } }] }
}

/** Twelve calendar months; missing months count as zero. */
export function trendMonths(points: TechTrend[]): string[] {
  const last = points.map(point => point.month).sort().at(-1)
  if (!last) return []
  const [year, month] = last.split('-').map(Number)
  return Array.from({ length: 12 }, (_, index) => new Date(Date.UTC(year, month - 12 + index, 1)).toISOString().slice(0, 7))
}

export function trends(points: TechTrend[]) {
  const months = trendMonths(points)
  const byTechnology = new Map<string, Map<string, number>>()
  for (const point of points) {
    if (!byTechnology.has(point.technology)) byTechnology.set(point.technology, new Map())
    const values = byTechnology.get(point.technology)!
    const month = point.month.slice(0, 7)
    values.set(month, (values.get(month) ?? 0) + point.commits)
  }
  return [...byTechnology].map(([technology, points]) => {
    const values = months.map(month => points.get(month) ?? 0)
    const average = (values: number[]) => values.reduce((sum, n) => sum + n, 0) / 3
    return { technology, values, total: values.reduce((sum, n) => sum + n, 0), slope: average(values.slice(-3)) - average(values.slice(-6, -3)) }
  }).sort((a, b) => b.total - a.total || a.technology.localeCompare(b.technology))
}

export function trendOption(data: TechData): EChartsOption {
  const rows = trends(data.trend)
  return { color: series(), tooltip: { trigger: 'axis' }, legend: { type: 'scroll', selected: Object.fromEntries(rows.map((row, index) => [row.technology, index < 8])) },
    grid: { top: 50, left: 16, right: 20, bottom: 30 }, xAxis: { type: 'category', data: trendMonths(data.trend), axisLabel: { formatter: monthLabel, hideOverlap: true } }, yAxis: { type: 'value', name: 'Commits', minInterval: 1 },
    series: rows.map(row => ({ type: 'line', name: row.technology, data: row.values, showSymbol: false })) }
}

export function aiOption(assistants: Assistant[]): EChartsOption {
  const months = [...new Set(assistants.flatMap(assistant => assistant.by_month.map(point => point.month)))].sort()
  return { color: series(), tooltip: { trigger: 'axis' }, legend: { type: 'scroll' }, grid: { top: 50, left: 50, right: 20, bottom: 30 },
    xAxis: { type: 'category', data: months, axisLabel: { formatter: monthLabel, hideOverlap: true } }, yAxis: { type: 'value', minInterval: 1 },
    series: assistants.map(assistant => ({ name: assistant.ai, type: 'bar', stack: 'sessions', data: months.map(month => assistant.by_month.find(point => point.month === month)?.sessions ?? 0) })) }
}

export default function Tech() {
  const technologies = useApi<TechData>('/api/technologies')
  const assistants = useApi<Assistant[]>('/api/ai')
  const projects = useApi<ProjectSummary[]>('/api/projects')
  const navigate = useNavigate()
  const [projectFilter, setProjectFilter] = useState('')
  const project = (name: string) => projects.data?.find(item => item.name === name)
  return <div className="space-y-5">
    <Header title="Tech & AI" description="The tools that shape your projects, and the assistants that work with you." />
    <Load state={technologies}>{data => <>
      <Card title="Change over 12 months"><p className="chart-footnote mb-4">Commits in the repositories tied to each technology, including upstream history. One commit can count for several technologies.</p>{data.trend.length ? <Chart aria-label="Commits per technology over twelve months" option={() => trendOption(data)} height={300} /> : <Empty>No technology activity.</Empty>}
        <div className="grid grid-cols-2 gap-4 text-sm">{[true, false].map(rising => <div key={String(rising)}><h3 className={rising ? 'text-success' : 'text-danger'}>{rising ? 'Rising' : 'Falling'}</h3>
          <p className="mb-2 text-xs text-muted">Average of the last 3 months minus the 3 before</p>
          <ul>{trends(data.trend).filter(item => rising ? item.slope > 0 : item.slope < 0).map(item => <li key={item.technology}>{item.technology} <span className="tabular-nums text-muted">{item.slope > 0 ? '+' : ''}{number(Math.round(item.slope * 10) / 10)} commits/month</span></li>)}</ul>
          {!trends(data.trend).some(item => rising ? item.slope > 0 : item.slope < 0) && <p className="text-muted">No technology is {rising ? 'rising' : 'falling'}.</p>}</div>)}</div>
      </Card>
      <Card title="Projects → technologies" action={<select aria-label="Diagram project" className="border border-line bg-surface px-3 text-xs" value={projectFilter} onChange={e => setProjectFilter(e.target.value)}><option value="">All projects</option>{data.sankey.nodes.filter(node => node.type === 'project').map(node => <option key={node.name}>{node.name}</option>)}</select>}><p className="chart-footnote mb-4">A link means a technology is used. Filter by project or scroll the diagram to explore every relationship.</p>{data.sankey.nodes.length ? <div className="sankey-scroll" tabIndex={0} role="region" aria-label="Scrollable technology diagram"><Chart aria-label="Relationships between projects and technologies" height={sankeyHeight(filterSankey(data, projectFilter))} option={() => sankeyOption(filterSankey(data, projectFilter))} onClick={event => {
        const node = event.data as { nodeName?: string; nodeType?: string }
        const match = node?.nodeType === 'project' && node.nodeName ? project(node.nodeName) : undefined
        if (match) navigate(`/projects/${match.id}`)
      }} /></div> : <Empty>No relationship between a project and a technology.</Empty>}</Card>
      <p className="text-sm text-muted">Technologies without a project: {data.unassigned.join(', ') || 'none'}.</p>
    </>}</Load>
    <Load state={assistants}>{data => data.length ? <>
      <div className="grid grid-cols-3 gap-4">{data.map(assistant => <Card key={assistant.ai} title={assistant.ai}><p className="mb-3 text-xl tabular-nums">{number(assistant.sessions)} <span className="text-sm text-muted">sessions</span></p>
        <div className="flex flex-wrap gap-2">{assistant.projects.map(name => <span className="rounded bg-accent-soft px-2 py-1 text-xs" key={name}>{project(name) ? <Link to={`/projects/${project(name)!.id}`}>{name}</Link> : name}</span>)}</div>
        {!assistant.projects.length && <p className="text-sm text-muted">No linked project.</p>}</Card>)}</div>
      <Card title="Sessions per month and per AI"><Chart aria-label="Monthly sessions per assistant" option={() => aiOption(data)} height={300} /></Card>
    </> : <Empty>No AI session.</Empty>}</Load>
  </div>
}
