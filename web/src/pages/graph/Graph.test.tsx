import { expect, test } from 'vitest'
import type { Graph as GraphData } from '../../api/types'
import { mockPath, nodeSize, selectedNote } from './Graph'

const node = (id: number, title: string): GraphData['nodes'][number] => ({ id, title, path: `Projects/${title}.md`, folder: 'Projects', degree: 1 })

const graph: GraphData = {
  nodes: [node(1, 'Atlas'), node(2, 'Compass'), node(3, 'Harbor')],
  links: [{ source: 1, target: 2 }, { source: 2, target: 3 }],
  orphans: [],
  central: [],
  broken: [],
  folders: ['Projects'],
}

test('node size stays inside the drawing bounds', () => {
  expect(nodeSize(0)).toBe(9)
  expect(nodeSize(10_000)).toBe(44)
})

test('a note is selected by title or by id', () => {
  expect(selectedNote(graph, undefined, 'Compass')?.id).toBe(2)
  expect(selectedNote(graph, '3')?.title).toBe('Harbor')
  expect(selectedNote(graph, '9')).toBeUndefined()
})

test('mock path walks links in either direction', () => {
  expect(mockPath(graph, 1, 3).path.map(item => item.title)).toEqual(['Atlas', 'Compass', 'Harbor'])
  expect(mockPath(graph, 3, 1).path.map(item => item.id)).toEqual([3, 2, 1])
  expect(mockPath(graph, 1, 99).path).toEqual([])
})
