import { expect, test } from './fixture'

test('tech and AI: the Sankey is drawn', async ({ page }) => {
  await page.goto('/tech')
  await expect(page.getByRole('heading', { level: 1, name: 'Tech & AI' })).toBeVisible()
  const sankey = page.getByRole('img', { name: 'Relationships between projects and technologies' })
  await expect(sankey.locator('canvas, svg')).toBeVisible()
})
