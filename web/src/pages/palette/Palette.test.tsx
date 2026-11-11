import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { ProjectSummary } from '../../api/types'
import projects from '../../../public/mocks/projects.json'
import Palette from './Palette'
import { fuzzyScore, rank } from './fuzzy'

const cards = projects as ProjectSummary[]

function Location() {
  const location = useLocation()
  return <p data-testid="location">{location.pathname}{location.search}</p>
}

function show() {
  const close = vi.fn()
  render(
    <MemoryRouter initialEntries={['/']}>
      <Location />
      <Routes>
        <Route path="/" element={<Palette close={close} />} />
        <Route path="/console" element={<h1>Console</h1>} />
        <Route path="/search" element={<h1>Search</h1>} />
        <Route path="/projects/:id" element={<h1>Project</h1>} />
      </Routes>
    </MemoryRouter>,
  )
  return close
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async () => Response.json(cards)))
})
afterEach(() => vi.unstubAllGlobals())

test('fuzzy rank keeps characters in order and prefers a word start', () => {
  expect(fuzzyScore('Atlas', 'at')).toBeGreaterThan(0)
  expect(fuzzyScore('Atlas', 'zz')).toBeNull()
  const ranked = rank([
    { id: 'a', label: 'Harbor', detail: 'Open the project' },
    { id: 'b', label: 'Quartz', detail: 'Open the project' },
  ], 'har')
  expect(ranked.map(item => item.id)).toEqual(['a'])
})

test('the palette lists fictional projects and opens one', async () => {
  show()
  await screen.findByRole('option', { name: /^Atlas/ })
  fireEvent.change(screen.getByRole('textbox', { name: 'Command' }), { target: { value: 'compass' } })
  fireEvent.click(await screen.findByRole('option', { name: /^Compass/ }))
  expect(screen.getByTestId('location').textContent).toBe('/projects/2')
})

test('typing a query offers search', async () => {
  show()
  await screen.findByRole('option', { name: /^Atlas/ })
  fireEvent.change(screen.getByRole('textbox', { name: 'Command' }), { target: { value: 'sqlite' } })
  fireEvent.click(screen.getByRole('option', { name: /Search/ }))
  expect(screen.getByTestId('location').textContent).toBe('/search?q=sqlite')
})

test('Escape closes the palette', async () => {
  const close = show()
  const dialog = await screen.findByRole('dialog', { name: 'Command palette' })
  fireEvent.keyDown(dialog, { key: 'Escape' })
  expect(close).toHaveBeenCalled()
})
