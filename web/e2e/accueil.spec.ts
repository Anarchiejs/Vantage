import { expect, test } from './fixture'

test('accueil : calendrier, chiffres clés et lien vers un projet', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Accueil' })).toBeVisible()
  const calendrier = page.getByRole('img', { name: 'Calendrier des commits sur 365 jours' })
  await expect(calendrier.locator('canvas, svg').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Chiffres clés' })).toBeVisible()
  await expect(page.getByText('Commits', { exact: true })).toBeVisible()
  await expect(page.getByText('Lignes ajoutées', { exact: true })).toBeVisible()
  await expect(page.getByText('Sessions', { exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Atlas' }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: 'Atlas' })).toBeVisible()
})
