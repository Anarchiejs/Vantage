import { expect, test } from './fixture'

test('graphe : la recherche d’une note ouvre son panneau', async ({ page }) => {
  await page.goto('/graphe')
  await expect(page.getByRole('heading', { level: 1, name: 'Graphe du cerveau' })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Graphe des liens du coffre' }).locator('canvas, svg')).toBeVisible()
  await page.getByLabel('Rechercher un nœud par titre').fill('Cartographie')
  await page.getByRole('button', { name: 'Ouvrir' }).click()
  await expect(page.getByRole('heading', { name: 'Note sélectionnée' })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Cartographie' })).toBeVisible()
  await expect(page.getByText('Concept fictif, référencé avec un alias dans la fiche Atlas.')).toBeVisible()
})
