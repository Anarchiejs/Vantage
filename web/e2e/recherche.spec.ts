import { expect, test } from './fixture'

test('la touche / ouvre la recherche et surligne un terme des fixtures', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Votre travail prend forme.' })).toBeVisible()
  await page.keyboard.press('/')
  await expect(page).toHaveURL(/\/recherche/)
  await expect(page.getByRole('heading', { level: 1, name: 'Recherche' })).toBeVisible()
  await page.getByLabel('Recherche').fill('sentiers')
  const marque = page.locator('#resultats mark').filter({ hasText: /sentiers/i }).first()
  await expect(marque).toBeVisible()
})
