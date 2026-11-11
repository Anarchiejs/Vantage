import { expect, test } from './fixture'

test('home: calendar, key figures, and a link to a project', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Your work is taking shape.' })).toBeVisible()
  const calendar = page.getByRole('img', { name: 'Commit calendar over 365 days' })
  await expect(calendar.locator('canvas, svg').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Key figures' })).toBeVisible()
  await expect(page.locator('.home-stats').getByText('Commits', { exact: true })).toBeVisible()
  await expect(page.getByText('Lines added', { exact: true })).toBeVisible()
  await expect(page.locator('.home-stats').getByText('Sessions', { exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Atlas' }).first().click()
  await expect(page.getByRole('heading', { level: 1, name: 'Atlas' })).toBeVisible()
})
