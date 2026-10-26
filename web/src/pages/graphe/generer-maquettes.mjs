// Maquettes synth?tiques : aucun acc?s au coffre, aucune date courante ni source al?atoire.
// R?g?n?rer depuis la racine : node web/src/pages/graphe/generer-maquettes.mjs
import { writeFileSync } from 'node:fs'

const base = {
  "noeuds": [
    {
      "id": 1,
      "titre": "Observatoire",
      "dossier": "Projets"
    },
    {
      "id": 2,
      "titre": "Compass",
      "dossier": "Projets"
    },
    {
      "id": 3,
      "titre": "Harbor",
      "dossier": "Projets"
    },
    {
      "id": 4,
      "titre": "Northwind",
      "dossier": "Projets"
    },
    {
      "id": 5,
      "titre": "Lantern",
      "dossier": "Projets"
    },
    {
      "id": 6,
      "titre": "Keel",
      "dossier": "Projets"
    },
    {
      "id": 7,
      "titre": "Tidepool",
      "dossier": "Projets"
    },
    {
      "id": 8,
      "titre": "Quartz",
      "dossier": "Projets"
    },
    {
      "id": 9,
      "titre": "Sable",
      "dossier": "Projets"
    },
    {
      "id": 10,
      "titre": "Waypoint",
      "dossier": "Projets"
    },
    {
      "id": 20,
      "titre": "Go",
      "dossier": "Technos"
    },
    {
      "id": 21,
      "titre": "React",
      "dossier": "Technos"
    },
    {
      "id": 22,
      "titre": "TypeScript",
      "dossier": "Technos"
    },
    {
      "id": 23,
      "titre": "SQLite",
      "dossier": "Technos"
    },
    {
      "id": 24,
      "titre": "Expo",
      "dossier": "Technos"
    },
    {
      "id": 25,
      "titre": "Python",
      "dossier": "Technos"
    },
    {
      "id": 26,
      "titre": "MongoDB",
      "dossier": "Technos"
    },
    {
      "id": 27,
      "titre": "Luau",
      "dossier": "Technos"
    },
    {
      "id": 28,
      "titre": "Supabase",
      "dossier": "Technos"
    },
    {
      "id": 29,
      "titre": "Rust",
      "dossier": "Technos"
    },
    {
      "id": 40,
      "titre": "Claude Code",
      "dossier": "IA"
    },
    {
      "id": 41,
      "titre": "Codex",
      "dossier": "IA"
    },
    {
      "id": 42,
      "titre": "DeepSeek",
      "dossier": "IA"
    },
    {
      "id": 43,
      "titre": "IA locale",
      "dossier": "IA"
    },
    {
      "id": 50,
      "titre": "Architecture Observatoire",
      "dossier": "Concepts"
    },
    {
      "id": 51,
      "titre": "Idée à explorer",
      "dossier": "Inbox"
    },
    {
      "id": 52,
      "titre": "2026-10-26 - Observatoire",
      "dossier": "Sessions"
    }
  ],
  "liens": [
    {
      "source": 1,
      "cible": 20
    },
    {
      "source": 1,
      "cible": 21
    },
    {
      "source": 1,
      "cible": 22
    },
    {
      "source": 1,
      "cible": 23
    },
    {
      "source": 2,
      "cible": 21
    },
    {
      "source": 2,
      "cible": 22
    },
    {
      "source": 2,
      "cible": 24
    },
    {
      "source": 3,
      "cible": 25
    },
    {
      "source": 3,
      "cible": 23
    },
    {
      "source": 4,
      "cible": 22
    },
    {
      "source": 4,
      "cible": 26
    },
    {
      "source": 5,
      "cible": 22
    },
    {
      "source": 6,
      "cible": 27
    },
    {
      "source": 7,
      "cible": 21
    },
    {
      "source": 7,
      "cible": 28
    },
    {
      "source": 8,
      "cible": 29
    },
    {
      "source": 9,
      "cible": 25
    },
    {
      "source": 10,
      "cible": 25
    },
    {
      "source": 10,
      "cible": 22
    },
    {
      "source": 1,
      "cible": 40
    },
    {
      "source": 2,
      "cible": 41
    },
    {
      "source": 3,
      "cible": 42
    },
    {
      "source": 4,
      "cible": 43
    },
    {
      "source": 5,
      "cible": 40
    },
    {
      "source": 6,
      "cible": 41
    },
    {
      "source": 7,
      "cible": 42
    },
    {
      "source": 8,
      "cible": 43
    },
    {
      "source": 9,
      "cible": 40
    },
    {
      "source": 10,
      "cible": 41
    },
    {
      "source": 50,
      "cible": 1
    },
    {
      "source": 52,
      "cible": 1
    },
    {
      "source": 52,
      "cible": 41
    }
  ],
  "casses": [
    {
      "source": 50,
      "source_titre": "Architecture Observatoire",
      "cible_titre": "Plan de déploiement"
    }
  ]
}
const projets = ["Observatoire","Compass","Harbor","Northwind","Lantern","Keel","Tidepool","Quartz","Sable","Waypoint","Atlas m?t?o","Atelier recettes","Sentier v?lo","Inventaire jardin","Studio podcast","Agenda associatif","Cartes du ciel"]
const technos = ["Go","React","TypeScript","SQLite","Expo","Python","MongoDB","Luau","Supabase","Rust","JavaScript","HTML","CSS","Vite","Vitest","Playwright","Tailwind CSS","ECharts","Node.js","Express","Fastify","PostgreSQL","MySQL","MariaDB","Prisma","Drizzle","GraphQL","REST","WebSocket","Svelte","Vue","Angular","Next.js","Astro","Electron","Tauri","C#",".NET","Java","Kotlin","Swift","Dart","Flutter","NumPy","Pandas"]
if (projets.length !== 17 || technos.length !== 45) throw new Error('Volume Sankey incorrect')
const liensTechnos = technos.flatMap((cible, i) => [0, 5, 11].map(decalage => ({ source: projets[(i + decalage) % projets.length], cible, valeur: 1 })))
const tendance = technos.flatMap((techno, i) => Array.from({ length: 12 }, (_, m) => ({
  mois: new Date(Date.UTC(2025, 10 + m, 1)).toISOString().slice(0, 7), techno,
  commits: Math.max(0, 12 + (i * 13 % 70) + (i % 2 ? -1 : 1) * m * (1 + i % 5)),
})))
const donneesTechnos = { sankey: { noeuds: [...projets.map(nom => ({ nom, type: 'projet' })), ...technos.map(nom => ({ nom, type: 'techno' }))], liens: liensTechnos }, tendance, sans_projet: ['Docker', 'Redis'] }

// Conserver les identifiants des petites fixtures de panneaux et de chemins.
const noeuds = base.noeuds.map(n => ({ ...n, degre: 0 }))
const dossiers = ['Concepts', 'Sessions', 'Guides', 'Archives', 'Inbox']
for (let id = 53; noeuds.length < 300; id++) {
  const dossier = dossiers[(id - 53) % dossiers.length]
  noeuds.push({ id, titre: dossier + ' ? exemple ' + String(id).padStart(3, '0'), dossier, degre: 0 })
}
const liens = [...base.liens]
const cles = new Set(liens.map(l => l.source + ':' + l.cible))
function relier(source, cible) {
  const cle = source + ':' + cible
  if (source !== cible && !cles.has(cle)) { liens.push({ source, cible }); cles.add(cle) }
}
const actifs = noeuds.slice(base.noeuds.length, -8)
relier(50, actifs[0].id)
for (const pas of [1, 7]) for (let i = 0; i < actifs.length; i++) relier(actifs[i].id, actifs[(i + pas) % actifs.length].id)
for (let i = 0; liens.length < 800; i++) relier(actifs[15 + i % (actifs.length - 15)].id, actifs[i % 15].id)
const parID = new Map(noeuds.map(n => [n.id, n]))
for (const l of liens) { parID.get(l.source).degre++; parID.get(l.cible).degre++ }
const graphe = { noeuds, liens, orphelines: noeuds.filter(n => n.degre === 0), centrales: [...noeuds].sort((a, b) => b.degre - a.degre || a.id - b.id).slice(0, 10), casses: base.casses }
for (const [nom, donnees] of [['technos', donneesTechnos], ['graphe', graphe]]) {
  writeFileSync(new URL('../../../public/mocks/' + nom + '.json', import.meta.url), JSON.stringify(donnees, null, 2) + '\n')
}
