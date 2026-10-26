import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import detail from '../../../public/mocks/projets/1.json'
import liste from '../../../public/mocks/projets.json'
import type { EvenementFrise, ProjetDetail } from '../../api/types'
import Projet, { Frise, SanteFiche, filtrerFrise } from './Projet'
import Projets, { trierProjets } from './Projets'
import { optionEvolution, optionVolume } from './graphiques'
vi.mock('../../ui/Graphique', () => ({ Graphique: () => <div aria-label="Graphique" /> }))
afterEach(() => vi.unstubAllGlobals())
test('santé à jour, périmée et référence inconnue', () => {
  const {rerender} = render(<SanteFiche sante={detail.sante} />)
  expect(screen.getByText('À jour')).toBeTruthy()
  rerender(<SanteFiche sante={{...detail.sante,a_jour:false,commits_depuis:4,notes_manquantes:['Résumé']}} />)
  expect(screen.getByText('Périmée')).toBeTruthy()
  expect(screen.getByText('Résumé')).toBeTruthy()
  expect(screen.getByText('4')).toBeTruthy()
  rerender(<SanteFiche sante={{...detail.sante,a_jour:false,commits_depuis:-1}} />)
  expect(screen.getByText('Référence inconnue du dépôt')).toBeTruthy()
})
test('frise : filtre par type et pagination par 50', () => {
  render(<Frise evenements={detail.frise as EvenementFrise[]} />)
  expect(screen.getAllByRole('listitem')).toHaveLength(50)
  fireEvent.click(screen.getByRole('button', {name:'Afficher plus'}))
  expect(screen.getAllByRole('listitem')).toHaveLength(78)
  fireEvent.click(screen.getByRole('checkbox',{name:'Commits'}))
  expect(screen.getAllByRole('listitem')).toHaveLength(50)
  expect(screen.queryByText('Corrige la validation des données')).toBeNull()
  fireEvent.click(screen.getByRole('checkbox',{name:'Sessions'}))
  expect(screen.getAllByRole('listitem')).toHaveLength(26)
  fireEvent.click(screen.getByRole('checkbox',{name:'Journal'}))
  expect(screen.getByText('Aucun événement pour ces filtres.')).toBeTruthy()
})
test('frise : limite de 500 avant filtre', () => {
  const events: EvenementFrise[] = Array.from({length:600},(_,i)=>({date:'2026-10-26',type:'commit',titre:String(i),ref:String(i),ia:null}))
  expect(filtrerFrise(events,['commit'])).toHaveLength(500)
  expect(filtrerFrise(events,['journal'])).toHaveLength(0)
})
test('tri numérique et textuel sans modifier les données ; valeurs nulles en dernier', () => {
  const copie = [...liste]
  expect(trierProjets(liste,'commits',true)[0].commits).toBe(Math.max(...liste.map(p=>p.commits)))
  expect(trierProjets(liste,'nom')[0].nom).toBe('Northwind')
  expect(trierProjets([{...liste[0],dernier_commit:null},liste[1]],'dernier_commit',true).at(-1)?.id).toBe(liste[0].id)
  expect(liste).toEqual(copie)
})
test('liste : maquette, tri interactif, filtre et lien clavier', async () => {
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>liste}))
  render(<MemoryRouter><Projets /></MemoryRouter>)
  await screen.findByRole('link',{name:'Observatoire'})
  fireEvent.click(screen.getByRole('button',{name:'Commits'}))
  const lignes = screen.getAllByRole('row')
  expect(within(lignes[1]).getByRole('link').textContent).toBe('Observatoire')
  fireEvent.change(screen.getByRole('searchbox'),{target:{value:'reviplan'}})
  expect(screen.getAllByRole('row')).toHaveLength(2)
  expect(screen.getByRole('link',{name:'Compass'}).getAttribute('href')).toBe('/projets/2')
  fireEvent.change(screen.getByRole('searchbox'),{target:{value:'introuvable'}})
  expect(screen.getByText('Aucun projet ne correspond au filtre.')).toBeTruthy()
})
test('page projet : endpoint, santé, graphe et données de la maquette', async () => {
  const fetchMock = vi.fn().mockResolvedValue({ok:true,json:async()=>detail})
  vi.stubGlobal('fetch',fetchMock)
  render(<MemoryRouter initialEntries={['/projets/1']}><Routes><Route path="/projets/:id" element={<Projet />} /></Routes></MemoryRouter>)
  expect(await screen.findByRole('heading',{name:'Observatoire'})).toBeTruthy()
  expect(fetchMock.mock.calls[0][0]).toBe('/api/projets/1')
  expect(screen.getByRole('link',{name:'Voir dans le graphe →'}).getAttribute('href')).toBe('/graphe?note=Observatoire')
  expect(screen.getByText('src/main.go')).toBeTruthy()
  expect(screen.getByText('À jour')).toBeTruthy()
})
test('projet vide : messages explicites', async () => {
  const vide: ProjetDetail = {...detail,frise:[],volume:[],langages:[],langages_evolution:[],points_chauds:[],ia:[]}
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>vide}))
  render(<MemoryRouter initialEntries={['/projets/1']}><Routes><Route path="/projets/:id" element={<Projet />} /></Routes></MemoryRouter>)
  expect(await screen.findByText('Aucun événement pour ces filtres.')).toBeTruthy()
  for(const t of ['Aucune session IA enregistrée.','Aucun volume de code enregistré.','Aucune mesure de langage.','Aucun fichier modifié.']) expect(screen.getByText(t)).toBeTruthy()
})
test('volume : suppressions négatives ; évolution alignée par date', () => {
  const v=optionVolume([{semaine:'2026-10-19',commits:1,ajouts:12,suppressions:4}])
  expect((v.series as {data:number[]}[]).map(s=>s.data)).toEqual([[12],[-4]])
  const evolution=optionEvolution([{date_mesure:'2026-10-02',langage:'Go',lignes_code:20},{date_mesure:'2026-10-01',langage:'Go',lignes_code:10}])
  expect((evolution.series as {data:number[]}[])[0].data).toEqual([10,20])
})
