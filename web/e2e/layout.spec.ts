import { test, expect } from './fixture'

test('activity: periods, values by date, and measure switch', async ({ page }) => {
  const data = await (await page.request.get('/api/home')).json()
  await page.goto('/')
  await page.getByRole('button', { name: '30 days', exact: true }).click()
  await page.getByLabel('Activity measure').selectOption('sessions')
  await expect(page.getByRole('img', { name: 'Sessions per day over 30 days' }).locator('canvas').first()).toBeVisible()
  await page.getByText('View the values by date').click()
  await expect(page.locator('details tbody tr')).toHaveCount(30)
  const last = data.calendar.at(-1)
  const row = page.locator('details tbody tr').first()
  await expect(row.locator('td').nth(1)).toHaveText(last.commits.toLocaleString('en-US'))
  await expect(row.locator('td').nth(2)).toHaveText(last.sessions.toLocaleString('en-US'))
  await page.getByRole('button', { name: 'Dark theme' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.getByRole('button', { name: '90 days', exact: true }).click()
  await expect(page.locator('details tbody tr')).toHaveCount(90)
})

test('tech: filter the Sankey, then restore every project', async ({ page }) => {
  await page.goto('/tech')
  const filter = page.getByLabel('Diagram project')
  await filter.selectOption({ label: 'Atlas' })
  await expect(page.getByRole('img', { name: 'Relationships between projects and technologies' }).locator('canvas').first()).toBeVisible()
  await filter.selectOption('')
  await expect(filter).toHaveValue('')
  await expect(page.getByRole('img', { name: 'Commits per technology over twelve months' }).locator('canvas').first()).toBeVisible()
})

test('mobile: every page stays within the screen width', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  for (const route of ['/', '/projects', '/tech', '/graph', '/search', '/console']) {
    await page.goto(route)
    await expect(page.locator('main h1')).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Main' })).toBeVisible()
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  }
})
