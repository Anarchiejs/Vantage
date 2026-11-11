import { expect, test } from './fixture'

test('a SELECT query shows rows', async ({ page }) => {
  await page.goto('/console')
  await expect(page.getByRole('heading', { level: 1, name: 'SQL console' })).toBeVisible()
  await expect(page.getByLabel('SQL query')).toContainText('SELECT')
  await page.getByRole('button', { name: 'Run' }).click()
  await expect(page.getByRole('cell', { name: 'Atlas' })).toBeVisible()
  await expect(page.getByRole('cell', { name: 'Compass' })).toBeVisible()
})

test('ATTACH shows the "command not allowed" message', async ({ page }) => {
  await page.goto('/console')
  await page.getByLabel('SQL query').click()
  await page.keyboard.press('Control+A')
  await page.keyboard.insertText("ATTACH DATABASE 'forbidden.db' AS ext")
  await page.getByRole('button', { name: 'Run' }).click()
  await expect(page.getByRole('alert')).toContainText('command not allowed')
})
