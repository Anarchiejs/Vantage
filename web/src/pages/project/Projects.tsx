import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import type { ProjectSummary } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Load, Empty } from '../../ui/Status'
import { date, number } from '../../ui/format'
import { Header } from '../../ui/Header'
export type Column = 'name' | 'status' | 'commits' | 'last_commit' | 'primary_language'
export function sortProjects(projects: ProjectSummary[], column: Column, desc = false): ProjectSummary[] {
  return [...projects].sort((a, b) => {
    const x = a[column], y = b[column]
    if (x === null) return y === null ? a.id - b.id : 1
    if (y === null) return -1
    const result = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), 'en', { sensitivity: 'base' })
    return (desc ? -result : result) || a.id - b.id
  })
}
const columns: [Column, string][] = [['name', 'Name'], ['status', 'Status'], ['commits', 'Commits'], ['last_commit', 'Last commit'], ['primary_language', 'Primary language']]
export default function Projects() {
  const state = useApi<ProjectSummary[]>('/api/projects')
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<Column>('name')
  const [desc, setDesc] = useState(false)
  const navigate = useNavigate()
  return <div className="space-y-5"><Header title="Projects" description="All of your repositories, their activity, and the technologies that keep them going." /><Card>
    <label className="mb-4 block text-sm">Filter projects<input className="ml-3 rounded border border-line bg-canvas px-3 py-2" type="search" value={filter} onChange={e => setFilter(e.target.value)} placeholder="Name, status, language…" /></label>
    <Load state={state}>{projects => {
      const rows = sortProjects(projects.filter(project => [project.name, project.status, project.primary_language].some(value => value?.toLocaleLowerCase('en').includes(filter.trim().toLocaleLowerCase('en')))), sort, desc)
      return rows.length ? <table className="w-full text-left text-sm"><thead><tr>{columns.map(([key, label]) => <th key={key} className="border-b border-line p-3" aria-sort={sort === key ? desc ? 'descending' : 'ascending' : 'none'}><button type="button" onClick={() => { setSort(key); setDesc(sort === key ? !desc : false) }}>{label}{sort === key ? desc ? ' ↓' : ' ↑' : ''}</button></th>)}</tr></thead><tbody>{rows.map(project => <tr key={project.id} className="cursor-pointer border-b border-line hover:bg-surface-2" onClick={() => navigate(`/projects/${project.id}`)}><td className="p-3"><Link className="text-accent hover:underline" to={`/projects/${project.id}`} onClick={e => e.stopPropagation()}>{project.name}</Link></td><td className="p-3">{project.status || '—'}</td><td className="p-3 tabular-nums">{number(project.commits)}</td><td className="p-3">{date(project.last_commit)}</td><td className="p-3">{project.primary_language || '—'}</td></tr>)}</tbody></table> : <Empty>{projects.length ? 'No project matches the filter.' : 'No project recorded.'}</Empty>
    }}</Load>
  </Card></div>
}
