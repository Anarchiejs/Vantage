# Observatoire

Un tableau de bord **local** de ton travail de développement : il lit ton coffre Obsidian et les dépôts Git de tes projets, puis affiche tout sur **http://127.0.0.1:7878** (historique, langages, graphe des notes, recherche, console SQL). Rien ne quitte ta machine.

## Démarrage rapide (Windows)

Ouvre **PowerShell** et lance ces commandes une par une.

**0. Récupérer le projet** (une seule fois)

```powershell
git clone https://github.com/Anarchiejs/Observatoire.git
cd Observatoire
```

> Pas encore Git ? `winget install --id Git.Git -e`, puis ferme et rouvre PowerShell.
> Toutes les commandes suivantes se lancent **depuis ce dossier `Observatoire`**.

**1. Installer** — git, Go et Node (via winget s'ils manquent), puis les dépendances

```powershell
.\installer.cmd
```

→ `Installation terminée.`

**2. Construire** — l'interface puis `observatoire.exe`

```powershell
.\construire.cmd
```

→ `Build terminé : observatoire.exe`

Si l'*Accès contrôlé aux dossiers* de Windows Defender est actif (fréquent quand le dépôt est dans `Documents`), une fenêtre administrateur s'ouvre pour autoriser `observatoire.exe` : **accepte-la**, sinon il ne pourra pas créer sa base.

**3. Lancer** — importe ton coffre Obsidian puis ouvre l'interface dans le navigateur. Remplace le chemin par celui de **ton** coffre (le dossier que tu ouvres dans Obsidian, pas celui du projet), entre guillemets

```powershell
.\lancer.cmd "C:\Users\toi\Documents\Mon Coffre"
```

→ le navigateur s'ouvre sur **http://127.0.0.1:7878**. `Ctrl+C` arrête le serveur. Le chemin est mémorisé : les fois suivantes, `.\lancer.cmd` suffit.

**Ensuite**

- Tant que `lancer.cmd` tourne, une note modifiée ou un nouveau commit apparaît dans l'interface en quelques secondes, sans recharger la page.
- Relancer plus tard : `.\lancer.cmd` (seuls les nouveaux commits et les notes modifiées sont relus).
- Après un `git pull` : `.\construire.cmd`.

## Préparer ton coffre

Toutes les notes `.md` sont importées (recherche, graphe, liens cassés). Le coffre n'est **jamais modifié** par l'import.

Un **projet** est une fiche placée exactement en `Projets\<Nom>\<Nom>.md`. Pour qu'il ait ses commits, elle doit contenir `type: projet` et **`chemin_source` avec le chemin absolu de son dépôt Git** :

```markdown
---
type: projet
statut: en cours
chemin_source: C:\dev\mon-projet
technos:
  - "[[Go]]"
---
# Mon projet
```

Facultatif : `verifie_commit` / `verifie_le` (repérer les fiches périmées), des sessions (notes du dossier `Sessions\` avec `type: session`, `projet: "[[Nom]]"`, `date: AAAA-MM-JJ`, `ia: "[[Nom]]"`) et un `Journal.md` à la racine (lignes `- AAAA-MM-JJ - [[IA]] - texte`). Exemple complet : `testdata\coffre`.

## Problèmes fréquents

| Message | Solution |
|---|---|
| `unable to open database file (14)`, ou le serveur s'arrête aussitôt | Defender bloque `observatoire.exe` : relance `construire.ps1` et accepte la fenêtre administrateur, ou autorise-le dans *Sécurité Windows › Protection contre les virus et menaces › Gérer la protection contre les ransomware › Autoriser une application*. Autre solution : mettre le dépôt hors de `Documents` (ex. `C:\dev`). |
| `… est introuvable` / `vient d'être installé mais reste introuvable` | Ferme et rouvre PowerShell, puis relance le script. |
| `winget est introuvable` | Installe « App Installer » depuis le Microsoft Store. |
| `Some chunks are larger than 500 kB` | Simple avertissement de Vite, à ignorer. |
| `dossier introuvable : …` | Vérifie le chemin du coffre et mets-le entre guillemets. |
| `aucun dépôt Git importé` / `0 dépôts` | Aucune fiche `Projets\<Nom>\<Nom>.md` avec `type: projet` et un `chemin_source` absolu. |
| `attention : <Projet> (<chemin>) : git rev-parse: …` | Ce `chemin_source` n'est pas un dépôt Git : corrige la fiche. Les autres projets sont quand même importés. |
| `erreur : listen tcp 127.0.0.1:7878: bind: …` | Le port est pris (un autre `serve` tourne ?) : ferme-le, ou `.\observatoire.exe serve -port 8080`. |

---

## Référence

### Commandes de `observatoire.exe`

| Commande | Rôle |
|---|---|
| `import` | importe le coffre et les dépôts Git (ce que fait `lancer.cmd`) |
| `serve` | lance l'interface sur 127.0.0.1 |
| `rapport` | écrit `Statistiques.md` (bilan de la semaine) à la racine du coffre |
| `migrate` | crée ou met à jour le schéma (fait aussi automatiquement par les autres commandes) |
| `watch` | importe, lance l'interface et réimporte dès qu'une note ou un dépôt change ; la page ouverte se met à jour seule (ce que fait `lancer.cmd`) |

| Option | Commandes | Défaut |
|---|---|---|
| `-db <fichier>` | toutes | `observatoire.db` (dossier courant) |
| `-coffre <dossier>` | `import`, `watch`, `rapport` | `C:\dev\Obsidian Vault` |
| `-depots "Nom=dossier,…"` | `import`, `watch` | remplace le `chemin_source` des fiches nommées |
| `-port <n>` | `serve`, `watch` | `7878` |
| `-stdout` | `rapport` | affiche le rapport au lieu de l'écrire |

Exemple : `.\observatoire.exe rapport -coffre "C:\Users\toi\Documents\Mon Coffre"`. Le rapport refuse d'écraser un `Statistiques.md` qui n'a pas été généré par Observatoire.

**Données** : la base `observatoire.db` (+ `-wal`, `-shm`) et les requêtes enregistrées de la console (`requetes\`) sont à la racine du dépôt. Tu peux supprimer la base : `lancer.cmd` la reconstruit.

**Console SQL** : lecture seule (pas d'écriture, d'`ATTACH`, de `PRAGMA` d'écriture ni de transaction), 5 s, 1000 lignes et 2 requêtes simultanées au maximum. Le serveur n'accepte que les requêtes locales.

### Développer le front

```powershell
npm --prefix web run dev        # Vite ; /api relayé vers http://127.0.0.1:7878 (laisse « serve » tourner)
npm --prefix web run dev:mock   # sans serveur Go : réponses lues dans web\public\mocks
npm --prefix web run typecheck
```

`construire.ps1` compile avec `-tags embed` : l'interface est dans l'exécutable. Un `go build ./cmd/observatoire` sans ce tag lit `web\dist` sur le disque et ne marche que lancé depuis la racine du dépôt.

### Migrations et sqlc

- **Migrations** (goose) : `migrations\*.sql`, embarquées et appliquées automatiquement. Nouveau changement de schéma : `0003_….sql` avec `-- +goose Up` / `-- +goose Down`, puis `construire.ps1`.
- **Code SQL** : les requêtes de `internal\sql\import` et `internal\sql\requetes` sont compilées par sqlc 1.31.1 (`sqlc.yaml`) vers `internal\sql\gen` (ne pas modifier à la main) :

  ```powershell
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
  sqlc generate
  ```

### Tests

Les tests Go ont besoin de deux petits dépôts Git générés dans `testdata\depots` :

```powershell
go build -o make-repos.exe ./scripts/fixtures   # hors Windows : go run ./scripts/fixtures/make-repos.go
.\make-repos.exe                                # dans Documents, à autoriser dans Defender comme observatoire.exe
go test ./...
npm --prefix web test                           # Node 22.12 ou plus
```

Mise à jour des résultats attendus des requêtes, après relecture : `go test ./internal/sql/requetes_test -count=1 -args -maj`. `make-repos.exe` n'est pas dans `.gitignore` (contrairement à `observatoire.exe` et `*.db`).

### Architecture

```
cmd/observatoire/   point d'entrée (import, serve, watch, rapport, migrate)
internal/api/       serveur HTTP : vues JSON, console SQL, SSE, interface
internal/db/        SQLite et migrations
internal/ingest/    import : git/, code/ (langages via scc), vault/ (coffre)
internal/report/    Statistiques.md
internal/sql/       requêtes sqlc et code généré (gen/)
internal/watch/     surveillance du coffre et des dépôts (fsnotify)
migrations/         schéma SQL (goose)
scripts/            installer.ps1, construire.ps1, lancer.ps1 (appelés par les .cmd de la racine), fixtures/
testdata/           coffre fictif et dépôts de test
web/                interface React + Vite
```
