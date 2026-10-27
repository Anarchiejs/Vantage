import { chromium } from '@playwright/test'
const browser = await chromium.launch({headless:true})
const page = await browser.newPage({viewport:{width:1440,height:1000},deviceScaleFactor:1})
const errors=[]
page.on('pageerror', e=>errors.push(e.message))
const projets=await (await page.request.get('http://127.0.0.1:5173/api/projets')).json()
const id=projets.find(p=>p.nom==='Observatoire').id
for (const [nom,route] of [['accueil','/'],['projets','/projets'],['projet',`/projets/${id}`],['technos','/technos'],['graphe','/graphe'],['recherche','/recherche'],['console','/console']]) {
 await page.goto(`http://127.0.0.1:5173${route}`)
 await page.locator('h1').waitFor()
 await page.waitForTimeout(700)
 await page.screenshot({path:`.orchestra/refonte-${nom}.png`,fullPage:true})
 console.log(nom,JSON.stringify({overflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),charts:await page.locator('[role=img] canvas').count()}))
}
await page.goto('http://127.0.0.1:5173')
await page.getByRole('button',{name:/Thème sombre/}).click()
await page.waitForTimeout(400)
await page.screenshot({path:'.orchestra/refonte-sombre.png',fullPage:true})
await page.getByRole('button',{name:/Thème clair/}).click()
await page.setViewportSize({width:390,height:844})
for (const [nom,route] of [['accueil','/'],['projets','/projets'],['projet',`/projets/${id}`],['technos','/technos'],['graphe','/graphe'],['recherche','/recherche'],['console','/console']]) {
 await page.goto(`http://127.0.0.1:5173${route}`)
 await page.locator('h1').waitFor()
 await page.waitForTimeout(400)
 await page.screenshot({path:`.orchestra/mobile-${nom}.png`,fullPage:true})
 console.log('mobile',nom,JSON.stringify({overflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)}))
}
console.log(JSON.stringify({errors}))
await browser.close()
