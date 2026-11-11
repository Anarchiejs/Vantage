import { expect, test } from './fixture'

test('Ctrl+K then Enter navigates to the first command', async ({ page }) => {
  await page.goto('/console')
  await expect(page.getByRole('heading', { level: 1, name: 'SQL console' })).toBeVisible()
  await page.keyboard.press('Control+k')
  const palette = page.getByRole('dialog', { name: 'Command palette' })
  await expect(palette).toBeVisible()
  await expect(palette.getByRole('option', { name: /Home/ })).toHaveAttribute('aria-selected', 'true')
  await page.keyboard.press('Enter')
  await expect(palette).toBeHidden()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Your work is taking shape.' })).toBeVisible()
})
