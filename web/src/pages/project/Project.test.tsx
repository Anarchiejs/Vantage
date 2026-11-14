import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import detail from '../../../public/mocks/projects/1.json'
import list from '../../../public/mocks/projects.json'
import type { ProjectDetail, ProjectSummary, TimelineEvent } from '../../api/types'
import Project, { CardHealth, LanguageBar, LinkFolder, Timeline, filterTimeline, projectFigures } from './Project'
import Projects, { sortProjects } from './Projects'

vi.mock('../../ui/Chart', () => ({ Chart: () => <div aria-label="Chart" /> }))
afterEach(() => vi.unstubAllGlobals())

const project = detail as ProjectDetail
const projects = list as ProjectSummary[]

test('card health: up to date, stale, and an unknown reference', () => {
  const { rerender } = render(<CardHealth health={project.health} />)
  expect(screen.getByText('Up to date')).toBeTruthy()
  rerender(<CardHealth health={{ ...project.health, up_to_date: false, commits_since: 4, missing_notes: ['Summary'] }} />)
  expect(screen.getByText('Stale')).toBeTruthy()
  expect(screen.getByText('Summary')).toBeTruthy()
  expect(screen.getByText('4')).toBeTruthy()
  rerender(<CardHealth health={{ ...project.health, up_to_date: false, commits_since: -1 }} />)
  expect(screen.getByText('Reference unknown in the repository')).toBeTruthy()
})

test('timeline filters by type and pages by 50', () => {
  render(<Timeline events={project.timeline} />)
  expect(screen.getAllByRole('listitem')).toHaveLength(50)
  fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
  expect(screen.getAllByRole('listitem')).toHaveLength(78)
  fireEvent.click(screen.getByRole('checkbox', { name: 'Commits' }))
  expect(screen.getAllByRole('listitem')).toHaveLength(50)
  expect(screen.queryByText('Fix data validation')).toBeNull()
  fireEvent.click(screen.getByRole('checkbox', { name: 'Sessions' }))
  expect(screen.getAllByRole('listitem')).toHaveLength(26)
  fireEvent.click(screen.getByRole('checkbox', { name: 'Journal' }))
  expect(screen.getByText('No events for these filters.')).toBeTruthy()
})

test('timeline keeps at most 500 events before filtering', () => {
  const events: TimelineEvent[] = Array.from({ length: 600 }, (_, i) => ({
    date: '2026-10-26', type: 'commit', title: String(i), ref: String(i), ai: null,
  }))
  expect(filterTimeline(events, ['commit'])).toHaveLength(500)
  expect(filterTimeline(events, ['journal'])).toHaveLength(0)
})

test('sort is numeric and textual and does not mutate; nulls stay last', () => {
  const copy = projects.map(item => ({ ...item }))
  expect(sortProjects(projects, 'commits', true)[0].commits).toBe(Math.max(...projects.map(item => item.commits)))
  expect(sortProjects(projects, 'name')[0].name).toBe('Atlas')
  const withNull = [{ ...projects[0], last_commit: null }, projects[1]]
  expect(sortProjects(withNull, 'last_commit', true).at(-1)?.id).toBe(projects[0].id)
  expect(projects.map(item => item.name)).toEqual(copy.map(item => item.name))
})

test('project list renders the fictional cards and filters them', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => projects }))
  render(<MemoryRouter><Projects /></MemoryRouter>)
  expect(await screen.findByRole('link', { name: 'Atlas' })).toBeTruthy()
  expect(screen.getByRole('link', { name: 'Compass' })).toBeTruthy()
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'harbor' } })
  expect(screen.getByRole('link', { name: 'Harbor' })).toBeTruthy()
  expect(screen.queryByRole('link', { name: 'Atlas' })).toBeNull()
})

test('project page shows the card and its source path', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => project }))
  render(
    <MemoryRouter initialEntries={['/projects/1']}>
      <Routes><Route path="/projects/:id" element={<Project />} /></Routes>
    </MemoryRouter>,
  )
  expect(await screen.findByRole('heading', { name: 'Atlas' })).toBeTruthy()
  expect(screen.getAllByText('C:/dev/Atlas').length).toBeGreaterThan(0)
  expect(screen.getByText('Up to date')).toBeTruthy()
})

test('figures come from the weekly volume', () => {
  const figures = projectFigures({ ...project, volume: [
    { week: '2026-01-05', commits: 0, additions: 0, deletions: 0 },
    { week: '2026-01-12', commits: 2, additions: 9, deletions: 1 },
    { week: '2026-01-19', commits: 3, additions: 15, deletions: 0 },
  ] })
  expect(figures.activeWeeks).toBe(2)
  expect(figures.totalWeeks).toBe(3)
  expect(figures.peak?.week).toBe('2026-01-19')
})

test('languages show as a labelled bar, largest first', () => {
  render(<LanguageBar languages={[{ language: 'TypeScript', code_lines: 10, share: 0.1 }, { language: 'Go', code_lines: 90, share: 0.9 }]} />)
  expect(screen.getByRole('img', { name: 'Share of lines of code by language' })).toBeTruthy()
  expect(screen.getAllByRole('listitem').map(item => item.textContent)).toEqual(['Go90 lines90%', 'TypeScript10 lines10%'])
})

test('a missing source path explains how to link the folder', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ repositories: 1, notes: 1, vault: '', can_link_repos: false }) }))
  render(<LinkFolder project={{ ...project.project, source_path: null }} onLinked={() => {}} />)
  expect(screen.getByText('No source folder linked.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Link a folder' })).toBeNull()
})

test('a folder can be linked under vantage watch', async () => {
  const fetch = vi.fn(async (url: string) => ({ ok: true, json: async () => url === '/api/status' ? { repositories: 1, notes: 1, vault: '', can_link_repos: true } : { source_path: '/code/atlas' } }))
  vi.stubGlobal('fetch', fetch)
  const linked = vi.fn()
  render(<LinkFolder project={{ ...project.project, source_path: null }} onLinked={linked} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Link a folder' }))
  fireEvent.change(screen.getByLabelText('Repository folder'), { target: { value: '/code/atlas' } })
  fireEvent.click(screen.getByRole('button', { name: 'Link and import' }))
  await vi.waitFor(() => expect(linked).toHaveBeenCalled())
  expect(fetch).toHaveBeenCalledWith(`/api/projects/${project.project.id}/repo`, expect.objectContaining({ method: 'POST', body: JSON.stringify({ path: '/code/atlas' }) }))
})
