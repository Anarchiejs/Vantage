import { expect, test } from './fixture'

test('the theme button switches between light and dark', async ({ page }) => {
  await page.goto('/')
  const html = page.locator('html')
  await expect(html).toHaveAttribute('data-theme', /light|dark/)
  const before = await html.getAttribute('data-theme')
  const button = page.getByRole('button', { name: /theme/i })
  await button.click()
  const after = before === 'light' ? 'dark' : 'light'
  await expect(html).toHaveAttribute('data-theme', after)
  await expect(button).toContainText(before === 'light' ? 'Light theme' : 'Dark theme')
  await button.click()
  await expect(html).toHaveAttribute('data-theme', before!)
})
