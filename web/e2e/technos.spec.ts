import { expect, test } from './fixture'

test('technos et IA : le Sankey est dessiné', async ({ page }) => {
  await page.goto('/technos')
  await expect(page.getByRole('heading', { level: 1, name: 'Technos & IA' })).toBeVisible()
  const sankey = page.getByRole('img', { name: 'Relations entre projets et technologies' })
  await expect(sankey.locator('canvas, svg')).toBeVisible()
})
