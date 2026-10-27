import { expect, test } from './fixture'

test('Ctrl+K puis Entrée navigue vers la première commande', async ({ page }) => {
  await page.goto('/console')
  await expect(page.getByRole('heading', { level: 1, name: 'Console SQL' })).toBeVisible()
  await page.keyboard.press('Control+k')
  const palette = page.getByRole('dialog', { name: 'Palette de commandes' })
  await expect(palette).toBeVisible()
  await expect(palette.getByRole('option', { name: /Accueil/ })).toHaveAttribute('aria-selected', 'true')
  await page.keyboard.press('Enter')
  await expect(palette).toBeHidden()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Accueil' })).toBeVisible()
})
