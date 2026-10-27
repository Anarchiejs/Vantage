import { expect, test } from './fixture'

test('projet Atlas : frise, langages et santé de la fiche', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Atlas' }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: 'Atlas' })).toBeVisible()
  await expect(page.getByText('Go', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('TypeScript', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Frise chronologique' })).toBeVisible()
  await expect(page.getByText(/fixture alpha change/).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Santé de la fiche' })).toBeVisible()
  await expect(page.getByText('Périmée')).toBeVisible()
  await expect(page.getByText('Référence inconnue du dépôt')).toBeVisible()
  const langages = page.getByRole('img', { name: 'Répartition des lignes de code par langage' })
  await expect(langages.locator('canvas')).toBeVisible()
})
