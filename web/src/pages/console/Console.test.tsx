import { expect, test } from 'vitest'
import type { SQLResult } from '../../api/types'
import { chartOption, numericColumns, validName } from './chart'
import sample from '../../../public/mocks/sql.post.json'

const result = sample as SQLResult

test('numeric columns skip the first column and ignore text', () => {
  const rows: SQLResult = {
    columns: ['name', 'commits', 'note'],
    rows: [['Atlas', 10, 'x'], ['Compass', 4, null]],
    duration_ms: 1,
    truncated: false,
  }
  expect(numericColumns(rows)).toEqual([1])
  expect(numericColumns({ ...rows, rows: [['Atlas', 'n', 'x']] })).toEqual([])
})

test('chart options: bars, line, pie, and no series', () => {
  const data: SQLResult = {
    columns: ['month', 'commits', 'notes'],
    rows: [['2026-09', 10, 2], ['2026-10', 4, null]],
    duration_ms: 5,
    truncated: false,
  }
  const bars = chartOption(data, 'bars')
  expect(bars?.xAxis).toMatchObject({ type: 'category', data: ['2026-09', '2026-10'] })
  expect(bars?.series).toEqual([
    { name: 'commits', type: 'bar', data: [10, 4] },
    { name: 'notes', type: 'bar', data: [2, null] },
  ])
  expect((chartOption(data, 'line')?.series as { type: string }[])[0].type).toBe('line')
  expect((chartOption(data, 'pie')?.series as { type: string }[])[0].type).toBe('pie')
  expect(chartOption({ columns: ['name', 'status'], rows: [['Atlas', 'in progress']], duration_ms: 1, truncated: false }, 'bars')).toBeNull()
  const fromSample = chartOption(result, 'bars')
  const series = fromSample?.series as { name: string }[]
  expect(series.map(item => item.name)).toEqual(['commits', 'additions'])
  expect((fromSample?.xAxis as { data: string[] }).data[0]).toBe('Atlas')
})

test('a saved query name is lowercase letters, digits, and dashes', () => {
  expect(validName('commits-by-project')).toBe(true)
  expect(validName('a1')).toBe(true)
  expect(validName('')).toBe(false)
  expect(validName('Notlower')).toBe(false)
  expect(validName('../evil')).toBe(false)
})
