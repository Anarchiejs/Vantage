import { test, expect } from './fixture'

test('activité : périodes, valeurs par date et changement de mesure', async ({ page }) => {
  const donnees = await (await page.request.get('/api/accueil')).json()
  await page.goto('/')
  await page.getByRole('button', { name: '30 jours', exact: true }).click()
  await page.getByLabel('Mesure de l’activité').selectOption('sessions')
  await expect(page.getByRole('img', { name: 'Sessions par jour sur 30 jours' }).locator('canvas').first()).toBeVisible()
  await page.getByText('Consulter les valeurs par date').click()
  await expect(page.locator('details tbody tr')).toHaveCount(30)
  const dernier = donnees.calendrier.at(-1)
  const ligne = page.locator('details tbody tr').first()
  await expect(ligne.locator('td').nth(1)).toHaveText(dernier.commits.toLocaleString('fr-FR'))
  await expect(ligne.locator('td').nth(2)).toHaveText(dernier.sessions.toLocaleString('fr-FR'))
  await page.getByRole('button', { name: 'Thème sombre ☾' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'sombre')
  await page.getByRole('button', { name: '90 jours', exact: true }).click()
  await expect(page.locator('details tbody tr')).toHaveCount(90)
})

test('technos : filtrer le Sankey puis restaurer tous les projets', async ({ page }) => {
  await page.goto('/technos')
  const filtre = page.getByLabel('Projet du diagramme')
  await filtre.selectOption({ label: 'Atlas' })
  await expect(page.getByRole('img', { name: 'Relations entre projets et technologies' }).locator('canvas').first()).toBeVisible()
  await filtre.selectOption('')
  await expect(filtre).toHaveValue('')
  await expect(page.getByRole('img', { name: 'Commits par technologie sur douze mois' }).locator('canvas').first()).toBeVisible()
})

test('mobile : toutes les pages restent dans la largeur de l’écran', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  for (const route of ['/', '/projets', '/technos', '/graphe', '/recherche', '/console']) {
    await page.goto(route)
    await expect(page.locator('main h1')).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Navigation principale' })).toBeVisible()
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  }
})
