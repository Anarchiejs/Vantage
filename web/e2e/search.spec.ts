import { expect, test } from './fixture'

test('the / key opens search and highlights a fixture term', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Your work is taking shape.' })).toBeVisible()
  await page.keyboard.press('/')
  await expect(page).toHaveURL(/\/search/)
  await expect(page.getByRole('heading', { level: 1, name: 'Search' })).toBeVisible()
  await page.getByLabel('Search').fill('trail')
  const mark = page.locator('#results mark').filter({ hasText: /trail/i }).first()
  await expect(mark).toBeVisible()
})
