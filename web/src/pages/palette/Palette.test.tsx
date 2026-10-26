import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { ProjetResume } from '../../api/types'
import Palette from './Palette'
import projetsMaquette from '../../../public/mocks/projets.json'

const projets = projetsMaquette as ProjetResume[]

function Lieu() {
  const lieu = useLocation()
  return <p data-testid="lieu">{lieu.pathname}{lieu.search}</p>
}

function afficher() {
  const fermer = vi.fn()
  render(
    <MemoryRouter initialEntries={['/']}>
      <Lieu />
      <Routes>
        <Route path="/" element={<Palette fermer={fermer} />} />
        <Route path="/console" element={<h1>Console</h1>} />
        <Route path="/recherche" element={<h1>Recherche</h1>} />
        <Route path="/graphe" element={<h1>Graphe</h1>} />
        <Route path="/projets/:id" element={<h1>Projet</h1>} />
      </Routes>
    </MemoryRouter>,
  )
  return fermer
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async () => Response.json(projets)))
})
afterEach(() => vi.unstubAllGlobals())

test('le filtrage ne garde que les commandes qui correspondent', async () => {
  afficher()
  await screen.findByRole('option', { name: /^Observatoire/ })
  fireEvent.change(screen.getByRole('textbox', { name: 'Commande' }), { target: { value: 'graphe' } })
  expect(screen.getByRole('option', { name: /^Graphe/ })).toBeTruthy()
  expect(screen.queryByRole('option', { name: /^Accueil/ })).toBeNull()
  expect(screen.queryByRole('option', { name: /^Observatoire/ })).toBeNull()
  expect(screen.getByRole('option', { name: /Rechercher « graphe »/ })).toBeTruthy()
})

test('Entrée ouvre la commande active', async () => {
  const fermer = afficher()
  const champ = screen.getByRole('textbox', { name: 'Commande' })
  fireEvent.change(champ, { target: { value: 'console' } })
  fireEvent.keyDown(champ, { key: 'Enter' })
  expect(screen.getByTestId('lieu').textContent).toBe('/console')
  expect(fermer).toHaveBeenCalled()
})

test('Entrée sur Rechercher ouvre la recherche', () => {
  const fermer = afficher()
  const champ = screen.getByRole('textbox', { name: 'Commande' })
  fireEvent.change(champ, { target: { value: 'zzz' } })
  fireEvent.keyDown(champ, { key: 'Enter' })
  expect(screen.getByTestId('lieu').textContent).toBe('/recherche?q=zzz')
  expect(fermer).toHaveBeenCalled()
})

test('Échap ferme et un clic hors du dialogue ferme', () => {
  const fermer = afficher()
  const champ = screen.getByRole('textbox', { name: 'Commande' })
  fireEvent.keyDown(champ, { key: 'Escape' })
  expect(fermer).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('dialog', { name: 'Palette de commandes' }))
  expect(fermer).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('dialog', { name: 'Palette de commandes' }).parentElement!)
  expect(fermer).toHaveBeenCalledTimes(2)
})

test('le focus reste dans le dialogue', async () => {
  afficher()
  await screen.findByRole('option', { name: /^Observatoire/ })
  const dialogue = screen.getByRole('dialog', { name: 'Palette de commandes' })
  const champ = screen.getByRole('textbox', { name: 'Commande' })
  const options = screen.getAllByRole('option')
  options[options.length - 1].focus()
  fireEvent.keyDown(dialogue, { key: 'Tab' })
  expect(document.activeElement).toBe(champ)
  fireEvent.keyDown(dialogue, { key: 'Tab', shiftKey: true })
  expect(document.activeElement).toBe(options[options.length - 1])
})

test('basculer le thème émet l’événement déjà écouté par l’application', async () => {
  const fermer = afficher()
  const espion = vi.fn()
  window.addEventListener('observatoire:basculer-theme', espion)
  await screen.findByRole('option', { name: /Basculer le thème/ })
  fireEvent.click(screen.getByRole('option', { name: /Basculer le thème/ }))
  expect(espion).toHaveBeenCalledTimes(1)
  expect(fermer).toHaveBeenCalled()
  window.removeEventListener('observatoire:basculer-theme', espion)
})
