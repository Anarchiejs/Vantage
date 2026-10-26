import { fireEvent, render, screen, waitFor } from '@testing-library/react'

const rectVide = () =>
  ({ x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0, toJSON() { return {} } }) as DOMRect
const listeVide = () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList
Range.prototype.getClientRects = listeVide
Range.prototype.getBoundingClientRect = rectVide
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { RequeteEnregistree, ResultatSQL, TableSchema } from '../../api/types'
import Console, { nomValide, optionGraphique } from './Console'
import schemaMaquette from '../../../public/mocks/schema.json'
import requetesMaquette from '../../../public/mocks/requetes.json'
import requeteCreeeMaquette from '../../../public/mocks/requetes.post.json'
import sqlMaquette from '../../../public/mocks/sql.post.json'

const schema = schemaMaquette as TableSchema[]
const requetes = requetesMaquette as RequeteEnregistree[]
const requeteCreee = requeteCreeeMaquette as RequeteEnregistree
const sqlMaquetteTypee = sqlMaquette as ResultatSQL

function installer(resultat: ResultatSQL = { ...sqlMaquetteTypee, tronque: true }) {
  const fetchMock = vi.fn(async (entree: RequestInfo | URL, init?: RequestInit) => {
    const url = String(entree)
    if (url.includes('/api/schema')) return Response.json(schema)
    if (url.includes('/api/requetes')) {
      if (init?.method === 'POST') return Response.json(requeteCreee)
      return Response.json(requetes)
    }
    if (url.includes('/api/sql')) return Response.json(resultat)
    return Response.json({ erreur: url }, { status: 404, statusText: 'Not Found' })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

beforeEach(() => {
  localStorage.clear()
  installer()
})
afterEach(() => {
  localStorage.clear()
  vi.unstubAllGlobals()
})

test('graphique : axe X, séries numériques, secteurs, aucune série', () => {
  const resultat: ResultatSQL = {
    colonnes: ['mois', 'commits', 'notes'],
    lignes: [
      ['2026-09', 10, 2],
      ['2026-10', 4, null],
    ],
    duree_ms: 5,
    tronque: false,
  }
  const barres = optionGraphique(resultat, 'barres')
  expect(barres?.xAxis).toMatchObject({ type: 'category', data: ['2026-09', '2026-10'] })
  expect(barres?.series).toEqual([
    { name: 'commits', type: 'bar', data: [10, 4] },
    { name: 'notes', type: 'bar', data: [2, null] },
  ])
  expect((optionGraphique(resultat, 'courbe')?.series as { type: string }[])[0].type).toBe('line')
  expect((optionGraphique(resultat, 'secteurs')?.series as { type: string }[])[0].type).toBe('pie')
  expect(
    optionGraphique(
      { colonnes: ['nom', 'statut'], lignes: [['Observatoire', 'en cours']], duree_ms: 1, tronque: false },
      'barres',
    ),
  ).toBeNull()

  const depuisMaquette = optionGraphique(sqlMaquetteTypee, 'barres')
  const series = depuisMaquette?.series as { name: string; type: string }[]
  expect(series.map((s) => s.name)).toEqual(['commits', 'ajouts'])
  expect(series.every((s) => s.type === 'bar')).toBe(true)
  expect((depuisMaquette?.xAxis as { data: string[] }).data[0]).toBe('Observatoire')
})

test('nom de requête : minuscules, chiffres et tirets', () => {
  expect(nomValide('commits-par-projet')).toBe(true)
  expect(nomValide('a1')).toBe(true)
  expect(nomValide('')).toBe(false)
  expect(nomValide('Commits')).toBe(false)
  expect(nomValide('ma_requete')).toBe(false)
  expect(nomValide('ma requete')).toBe(false)
})

test('résultat tronqué, NULL et graphique proposé s’il y a des nombres', async () => {
  render(<Console />)
  expect(screen.getByText(/Lecture seule/)).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Exécuter' }))
  expect((await screen.findByRole('status')).textContent).toMatch(/tronqué/)
  expect(screen.getByText('NULL').className).toContain('text-doux')
  expect(screen.getByText('Compass')).toBeTruthy()
  expect(screen.getByText(/12 ms/)).toBeTruthy()
  expect(screen.getByText(/7 lignes/)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Barres' }).hasAttribute('disabled')).toBe(false)
})

test('le graphique est désactivé sans colonne numérique', async () => {
  installer({
    colonnes: ['nom', 'statut'],
    lignes: [['Observatoire', 'en cours']],
    duree_ms: 4,
    tronque: false,
  })
  render(<Console />)
  fireEvent.click(screen.getByRole('button', { name: 'Exécuter' }))
  expect(await screen.findByText('Observatoire')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Barres' }).hasAttribute('disabled')).toBe(true)
  expect(screen.getByRole('button', { name: 'Courbe' }).hasAttribute('disabled')).toBe(true)
  expect(screen.getByRole('button', { name: 'Secteurs' }).hasAttribute('disabled')).toBe(true)
})

test('validation du nom, enregistrement et chargement d’une requête', async () => {
  const fetchMock = installer()
  render(<Console />)
  expect(await screen.findByRole('button', { name: /commits-par-projet/ })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Insérer fichier_modifie' }))
  await waitFor(() => expect(document.querySelector('.cm-content')?.textContent).toContain('fichier_modifie'))

  fireEvent.click(screen.getByRole('button', { name: /commits-par-projet/ }))
  await waitFor(() => expect(document.querySelector('.cm-content')?.textContent).toContain('commit_git'))

  fireEvent.click(screen.getByRole('button', { name: 'Enregistrer sous…' }))
  const champ = screen.getByLabelText('Nom de la requête')
  fireEvent.change(champ, { target: { value: 'Nom Invalide' } })
  fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
  expect(screen.getByRole('alert').textContent).toMatch(/minuscules/)
  const posts = () => fetchMock.mock.calls.filter((appel) => String(appel[0]).includes('/api/requetes') && appel[1]?.method === 'POST')
  expect(posts()).toHaveLength(0)

  fireEvent.change(champ, { target: { value: 'ma-requete' } })
  fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
  await waitFor(() => expect(posts()).toHaveLength(1))
  const corps = JSON.parse(String(posts()[0][1]?.body)) as { nom: string; sql: string }
  expect(corps.nom).toBe('ma-requete')
  expect(corps.sql).toContain('commit_git')
  expect(await screen.findByText('ma-requete')).toBeTruthy()
})
