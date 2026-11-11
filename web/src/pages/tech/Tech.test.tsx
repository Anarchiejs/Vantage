import { expect, test } from 'vitest'
import type { Technologies } from '../../api/types'
import { filterSankey, sankeyHeight, trends } from './Tech'

test('SQL months YYYY-MM-01 keep the real commits and the slope', () => {
  const points = [1, 2, 3, 4, 5, 6].map(month => ({
    month: `2026-0${month}-01`,
    technology: 'Go',
    commits: month <= 3 ? 3 : 9,
  }))
  const row = trends(points)[0]
  expect(row.total).toBe(36)
  expect(row.values.slice(-6)).toEqual([3, 3, 3, 9, 9, 9])
  expect(row.slope).toBe(6)
})

test('the sankey filter keeps one project and the technologies it uses', () => {
  const data: Technologies = {
    trend: [],
    unassigned: [],
    sankey: {
      nodes: [
        { name: 'Atlas', type: 'project' },
        { name: 'Compass', type: 'project' },
        { name: 'Go', type: 'technology' },
        { name: 'Rust', type: 'technology' },
      ],
      links: [
        { source: 'Atlas', target: 'Go', value: 1 },
        { source: 'Compass', target: 'Rust', value: 1 },
      ],
    },
  }
  const filtered = filterSankey(data, 'Atlas')
  expect(filtered.sankey.links).toEqual([{ source: 'Atlas', target: 'Go', value: 1 }])
  expect(filtered.sankey.nodes.map(node => node.name).sort()).toEqual(['Atlas', 'Go'])
  expect(sankeyHeight(data)).toBeGreaterThanOrEqual(360)
})
