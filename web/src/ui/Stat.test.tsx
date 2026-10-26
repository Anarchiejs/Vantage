import { render, screen } from '@testing-library/react'
import { Stat } from './Stat'

test('affiche la hausse en pourcentage', () => {
  render(<Stat libelle="Commits" valeur={15} avant={10} />)
  expect(screen.getByText('↑ 50 %')).toBeTruthy()
})

test('période précédente à zéro : écart en valeur absolue', () => {
  render(<Stat libelle="Commits" valeur={4} avant={0} />)
  expect(screen.getByText('↑ 4')).toBeTruthy()
})
