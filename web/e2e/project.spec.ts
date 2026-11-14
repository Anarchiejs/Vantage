import { expect, test } from './fixture'

test('Atlas project: timeline, languages, and card health', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Atlas' }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: 'Atlas' })).toBeVisible()
  await expect(page.getByText('Go', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('TypeScript', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Timeline' })).toBeVisible()
  await expect(page.getByText(/fixture alpha change/).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Card health' })).toBeVisible()
  await expect(page.getByText('Stale')).toBeVisible()
  await expect(page.getByText('Reference unknown in the repository')).toBeVisible()
  await expect(page.getByRole('img', { name: 'Share of lines of code by language' })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Lines added and deleted per week' }).locator('canvas')).toBeVisible()
})
