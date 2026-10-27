import { expect, test } from 'vitest'
import { tendances } from './Technos'

test('les mois SQL AAAA-MM-01 conservent les commits réels et la pente', () => {
  const points = [1, 2, 3, 4, 5, 6].map(m => ({ mois: `2026-0${m}-01`, techno: 'Go', commits: m <= 3 ? 3 : 9 }))
  const ligne = tendances(points)[0]
  expect(ligne.total).toBe(36)
  expect(ligne.valeurs.slice(-6)).toEqual([3, 3, 3, 9, 9, 9])
  expect(ligne.pente).toBe(6)
})
