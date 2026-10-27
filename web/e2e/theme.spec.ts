import { expect, test } from './fixture'

test('le bouton de thème bascule entre clair et sombre', async ({ page }) => {
  await page.goto('/')
  const html = page.locator('html')
  await expect(html).toHaveAttribute('data-theme', /clair|sombre/)
  const avant = await html.getAttribute('data-theme')
  const bouton = page.getByRole('button', { name: /^Thème / })
  await bouton.click()
  const apres = avant === 'clair' ? 'sombre' : 'clair'
  await expect(html).toHaveAttribute('data-theme', apres)
  await expect(bouton).toContainText(avant === 'clair' ? 'clair' : 'sombre')
  await bouton.click()
  await expect(html).toHaveAttribute('data-theme', avant!)
})
