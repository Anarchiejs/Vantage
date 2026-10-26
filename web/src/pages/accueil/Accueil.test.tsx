import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import maquette from '../../../public/mocks/accueil.json'
import Accueil, { optionCalendrier } from './Accueil'
import type { JourActivite } from '../../api/types'
vi.mock('../../ui/Graphique', () => ({ Graphique: () => <div aria-label="Graphique" /> }))
beforeEach(() => { vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => maquette })) })
afterEach(() => vi.unstubAllGlobals())
test('calendrier : 365 points et cinq paliers, zéro puis quantiles positifs', () => {
  const jours: JourActivite[] = maquette.calendrier.map((j, i) => ({ ...j, commits: i < 5 ? i : 0 }))
  const option = optionCalendrier(jours)
  const serie = (option.series as {data: [string, number][]}[])[0]
  expect(serie.data).toHaveLength(365)
  expect(serie.data.slice(0, 5).map(p => p[1])).toEqual([0, 1, 2, 3, 4])
  expect((option.visualMap as {pieces: unknown[]}).pieces).toHaveLength(5)
  expect((option.calendar as {range: string[]}).range).toEqual([jours[0].jour, jours[364].jour])
})
test('quantiles : les ex æquo restent dans le même palier et les jours vides restent à zéro', () => {
  const jours = maquette.calendrier.map(j => ({ ...j, commits: 8 }))
  const data = ((optionCalendrier(jours).series as {data: [string, number][]}[])[0]).data
  expect(new Set(data.map(p => p[1]))).toEqual(new Set([1]))
  expect(((optionCalendrier(jours.map(j => ({...j, commits: 0}))).series as {data: [string, number][]}[])[0]).data.every(p => p[1] === 0)).toBe(true)
})
test('chiffres, comparaison et commutateur semaine/mois', async () => {
  render(<MemoryRouter><Accueil /></MemoryRouter>)
  await screen.findByText('Chiffres clés')
  const avant = maquette.semaine_precedente.commits.toLocaleString('fr-FR')
  expect(screen.getByTitle(`Période précédente : ${avant}`)).toBeTruthy()
  expect(screen.getByRole('button', {name: 'Semaine'}).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', {name: 'Mois'}))
  expect(screen.getByText('30 derniers jours / 30 jours précédents')).toBeTruthy()
  expect(screen.getByTitle(`Période précédente : ${maquette.mois_precedent.commits.toLocaleString('fr-FR')}`)).toBeTruthy()
  expect(screen.getAllByRole('link', {name: 'Compass'})[0].getAttribute('href')).toBe('/projets/2')
})
test('listes et calendrier vides explicités', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok:true,json:async()=>({...maquette,calendrier:[],actifs:[],dormants:[],fiches_perimees:[]})}))
  render(<MemoryRouter><Accueil /></MemoryRouter>)
  expect(await screen.findByText('Aucune activité enregistrée.')).toBeTruthy()
  for (const texte of ['Aucun projet actif.', 'Aucun projet dormant.', 'Aucune fiche périmée.']) expect(screen.getByText(texte)).toBeTruthy()
})
test('erreur API lisible', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok:false,status:503,json:async()=>({erreur:'Import indisponible'})}))
  render(<MemoryRouter><Accueil /></MemoryRouter>)
  expect((await screen.findByRole('alert')).textContent).toBe('Import indisponible')
})
