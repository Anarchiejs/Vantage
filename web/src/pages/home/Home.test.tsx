import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import mock from '../../../public/mocks/home.json'
import Home, { calendarOption } from './Home'
import type { ActivityDay } from '../../api/types'
vi.mock('../../ui/Chart', () => ({ Chart: () => <div aria-label="Chart" /> }))
beforeEach(() => { vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mock })) })
afterEach(() => vi.unstubAllGlobals())
test('calendar: 365 points and five buckets, zero then positive quantiles', () => {
  const days: ActivityDay[] = mock.calendar.map((day, i) => ({ ...day, commits: i < 5 ? i : 0 }))
  const option = calendarOption(days)
  const series = (option.series as {data: [string, number][]}[])[0]
  expect(series.data).toHaveLength(365)
  expect(series.data.slice(0, 5).map(point => point[1])).toEqual([0, 1, 2, 3, 4])
  expect((option.visualMap as {pieces: unknown[]}).pieces).toHaveLength(5)
  expect((option.calendar as {range: string[]}).range).toEqual([days[0].day, days[364].day])
})
test('quantiles: ties stay in the same bucket and empty days stay at zero', () => {
  const days = mock.calendar.map(day => ({ ...day, commits: 8 }))
  const data = ((calendarOption(days).series as {data: [string, number][]}[])[0]).data
  expect(new Set(data.map(point => point[1]))).toEqual(new Set([1]))
  expect(((calendarOption(days.map(day => ({...day, commits: 0}))).series as {data: [string, number][]}[])[0]).data.every(point => point[1] === 0)).toBe(true)
})
test('figures, comparison, and week/month switch', async () => {
  render(<MemoryRouter><Home /></MemoryRouter>)
  await screen.findByText('Key figures')
  const previous = mock.previous_week.commits.toLocaleString('en-US')
  expect(screen.getByTitle(`Previous period: ${previous}`)).toBeTruthy()
  expect(screen.getByRole('button', {name: 'Week'}).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', {name: 'Month'}))
  expect(screen.getByText('Last 30 days / previous 30 days')).toBeTruthy()
  expect(screen.getByTitle(`Previous period: ${mock.previous_month.commits.toLocaleString('en-US')}`)).toBeTruthy()
  expect(screen.getAllByRole('link', {name: /^Compass/})[0].getAttribute('href')).toBe('/projects/2')
})
test('empty lists and calendar are explicit', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok:true,json:async()=>({...mock,calendar:[],active:[],dormant:[],stale_cards:[]})}))
  render(<MemoryRouter><Home /></MemoryRouter>)
  expect(await screen.findByText('No activity recorded.')).toBeTruthy()
  for (const text of ['No active project.', 'No dormant project.', 'No stale card.']) expect(screen.getByText(text)).toBeTruthy()
})
test('API error is readable', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok:false,status:503,json:async()=>({error:'Import unavailable'})}))
  render(<MemoryRouter><Home /></MemoryRouter>)
  expect((await screen.findByRole('alert')).textContent).toBe('Import unavailable')
})
test('a quiet week names the last commit instead of zeros', async () => {
  const zero = { commits: 0, additions: 0, deletions: 0, sessions: 0, active_projects: 0 }
  const calendar = mock.calendar.map((day, i) => ({ ...day, commits: i === 300 ? 3 : 0, projects: i === 300 ? ['Compass'] : [] }))
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ...mock, calendar, week: zero }) }))
  render(<MemoryRouter><Home /></MemoryRouter>)
  expect(await screen.findByText('A quiet week: nothing committed in the last 7 days.')).toBeTruthy()
  expect(screen.getByRole('link', { name: 'Resume Compass →' }).getAttribute('href')).toBe('/projects/2')
})
