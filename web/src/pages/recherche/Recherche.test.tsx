import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { ProjetResume, ResultatRecherche } from '../../api/types'
import Recherche from './Recherche'
import maquette from '../../../public/mocks/recherche.json'
import projetsMaquette from '../../../public/mocks/projets.json'

const projets = projetsMaquette as ProjetResume[]
const resultats = maquette as ResultatRecherche[]

function Lieu() {
  const lieu = useLocation()
  return <p data-testid="lieu">{lieu.pathname}{lieu.search}</p>
}

function afficher(url = '/recherche') {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <Lieu />
      <Routes>
        <Route path="/recherche" element={<Recherche />} />
        <Route path="/graphe/:id" element={<p>note</p>} />
        <Route path="/projets/:id" element={<p>projet</p>} />
        <Route path="/projets" element={<p>projets</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

function installer(opts: { recherche?: unknown; statut?: number; projets?: ProjetResume[] } = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (entree: RequestInfo | URL) => {
      const url = String(entree)
      if (url.includes('/api/projets')) return Response.json(opts.projets ?? projets)
      if (url.includes('/api/recherche')) {
        if (opts.statut && opts.statut >= 400) {
          return Response.json({ erreur: opts.recherche }, { status: opts.statut, statusText: 'Bad Request' })
        }
        return Response.json(opts.recherche ?? resultats)
      }
      return Response.json({ erreur: url }, { status: 404, statusText: 'Not Found' })
    }),
  )
}

async function chercher(texte: string) {
  fireEvent.change(screen.getByRole('searchbox', { name: 'Recherche' }), { target: { value: texte } })
  return screen.findByRole('listbox', { name: 'Résultats' })
}

beforeEach(() => installer())
afterEach(() => vi.unstubAllGlobals())

test('requête vide : champ focalisé et exemples FTS5', () => {
  afficher()
  expect(document.activeElement).toBe(screen.getByRole('searchbox', { name: 'Recherche' }))
  expect(screen.getByText('"phrase exacte"')).toBeTruthy()
  expect(screen.getByText('go AND sqlite')).toBeTruthy()
  expect(screen.getByText('pref*')).toBeTruthy()
  expect(screen.queryByText(/Aucun résultat/)).toBeNull()
})

test('le surlignage rend mark et affiche un script comme du texte', async () => {
  const piege: ResultatRecherche = {
    source: 'note',
    ref: 'piege.md',
    titre: '<script>alert(1)</script> <mark>sqlite</mark>',
    extrait: 'voir <img onerror="alert(2)"> <mark>sqlite</mark>',
    projet: 'Observatoire',
    date: '2026-10-01',
    id_note: 9,
  }
  installer({ recherche: [piege] })
  afficher()
  const liste = await chercher('sqlite')
  const ligne = within(liste).getAllByRole('option')[0]
  expect(ligne.querySelector('script')).toBeNull()
  expect(ligne.querySelector('img')).toBeNull()
  expect(ligne.innerHTML.toLowerCase()).not.toContain('<script')
  expect(ligne.querySelector('mark')?.textContent).toBe('sqlite')
  expect(ligne.textContent).toContain('<script>alert(1)</script>')
  expect(ligne.textContent).toContain('<img onerror="alert(2)">')
})

test('les filtres et la requête vivent dans l’URL', async () => {
  afficher()
  await screen.findByRole('option', { name: 'Observatoire' })
  fireEvent.change(screen.getByLabelText('Source'), { target: { value: 'note' } })
  fireEvent.change(screen.getByLabelText('Projet'), { target: { value: 'Observatoire' } })
  fireEvent.change(screen.getByLabelText('Dossier'), { target: { value: 'Projets' } })
  fireEvent.change(screen.getByLabelText('Depuis'), { target: { value: '2026-01-01' } })
  fireEvent.change(screen.getByLabelText("Jusqu'à"), { target: { value: '2026-10-26' } })
  fireEvent.change(screen.getByRole('searchbox', { name: 'Recherche' }), { target: { value: 'sqlite' } })
  await waitFor(() => {
    const lieu = screen.getByTestId('lieu').textContent ?? ''
    expect(lieu).toContain('q=sqlite')
    expect(lieu).toContain('source=note')
    expect(lieu).toContain('projet=Observatoire')
    expect(lieu).toContain('dossier=Projets')
    expect(lieu).toContain('depuis=2026-01-01')
    expect(lieu).toContain('jusqua=2026-10-26')
  })
})

test('les flèches puis Entrée ouvrent la note suivante', async () => {
  afficher()
  const liste = await chercher('sqlite')
  fireEvent.keyDown(screen.getByRole('searchbox', { name: 'Recherche' }), { key: 'ArrowDown' })
  fireEvent.keyDown(screen.getByRole('searchbox', { name: 'Recherche' }), { key: 'Enter' })
  expect(screen.getByTestId('lieu').textContent).toBe(`/graphe/${resultats[1].id_note}`)
  expect(within(liste).getAllByRole('option')[1].getAttribute('href')).toBe(`/graphe/${resultats[1].id_note}`)
})

test('un commit ouvre la page du projet', async () => {
  afficher()
  const liste = await chercher('fts5')
  const id = projets.find((p) => p.nom === 'Observatoire')?.id
  const commit = within(liste).getAllByRole('option')[2]
  await waitFor(() => expect(commit.getAttribute('href')).toBe(`/projets/${id}`))
  fireEvent.click(commit)
  expect(screen.getByTestId('lieu').textContent).toBe(`/projets/${id}`)
})

test('aucun résultat', async () => {
  installer({ recherche: [] })
  afficher()
  fireEvent.change(screen.getByRole('searchbox', { name: 'Recherche' }), { target: { value: 'rien' } })
  expect(await screen.findByText(/Aucun résultat pour « rien »/)).toBeTruthy()
})

test('erreur de syntaxe FTS5 affichée', async () => {
  installer({ statut: 400, recherche: 'fts5: syntaxe invalide près de « AND »' })
  afficher()
  fireEvent.change(screen.getByRole('searchbox', { name: 'Recherche' }), { target: { value: 'AND' } })
  expect((await screen.findByRole('alert')).textContent).toBe('fts5: syntaxe invalide près de « AND »')
})
