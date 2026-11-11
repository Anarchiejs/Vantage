import { expect, test } from './fixture'

test('graph: searching for a note opens its panel', async ({ page }) => {
  await page.goto('/graph')
  await expect(page.getByRole('heading', { level: 1, name: 'Note graph' })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Graph of vault links' }).locator('canvas, svg')).toBeVisible()
  await page.getByLabel('Find a node by title').fill('Mapping')
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page.getByRole('heading', { name: 'Selected note' })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Mapping' })).toBeVisible()
  await expect(page.getByText('Fictional concept, referenced with an alias from the Atlas card.')).toBeVisible()
})
