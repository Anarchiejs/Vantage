import { expect, test } from './fixture'

test('une requête SELECT affiche des lignes', async ({ page }) => {
  await page.goto('/console')
  await expect(page.getByRole('heading', { level: 1, name: 'Console SQL' })).toBeVisible()
  await expect(page.getByLabel('Requête SQL')).toContainText('SELECT')
  await page.getByRole('button', { name: 'Exécuter' }).click()
  await expect(page.getByRole('cell', { name: 'Atlas' })).toBeVisible()
  await expect(page.getByRole('cell', { name: 'Boussole' })).toBeVisible()
})

test('ATTACH affiche le message « commande interdite »', async ({ page }) => {
  await page.goto('/console')
  await page.getByLabel('Requête SQL').click()
  await page.keyboard.press('Control+A')
  await page.keyboard.insertText("ATTACH DATABASE 'interdit.db' AS ext")
  await page.getByRole('button', { name: 'Exécuter' }).click()
  await expect(page.getByRole('alert')).toContainText('commande interdite')
})
