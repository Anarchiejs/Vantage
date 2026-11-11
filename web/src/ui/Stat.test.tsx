import { render, screen } from '@testing-library/react'
import { Stat } from './Stat'

test('shows an increase as a percentage', () => {
  render(<Stat label="Commits" value={15} previous={10} />)
  expect(screen.getByText('↑ 50%')).toBeTruthy()
})

test('previous period at zero: absolute difference', () => {
  render(<Stat label="Commits" value={4} previous={0} />)
  expect(screen.getByText('↑ 4')).toBeTruthy()
})
