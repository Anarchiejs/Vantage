# Observatoire

Observatoire est un tableau de bord **local** de ton travail de développement. Il lit tes dépôts Git et ton coffre Obsidian, range tout dans une base SQLite, puis affiche le résultat dans une interface web sur `http://127.0.0.1:7878`. Rien ne quitte ta machine.

Au menu : historique Git (commits, fichiers, renommages), lignes de code par langage, notes et liens Obsidian (y compris les liens cassés), graphe des notes, recherche plein texte, console SQL en lecture seule et rapport hebdomadaire `Statistiques.md` écrit dans le coffre.

> **Toutes les commandes se lancent depuis la racine du dépôt** (le dossier qui contient `go.mod`), dans PowerShell.

---

## Démarrage rapide (Windows)

Objectif : voir l'interface avec les **données de démo** fournies (un coffre fictif et deux petits dépôts Git générés).

**Prérequis** : [Git](https://git-scm.com/), [Go](https://go.dev/dl/) 1.26.4 ou plus, [Node.js](https://nodejs.org/) 22.12 ou plus (avec npm). Vérifie :

```powershell
git --version
go version
node --version
```

### 1. Cloner le projet

```powershell
git clone https://github.com/Anarchiejs/Observatoire.git
cd Observatoire
```

### 2. Installer les dépendances

```powershell
go mod download
npm --prefix web install
```

Tu dois voir à la fin `added … packages`.

### 3. Construire l'interface, puis l'exécutable

L'ordre compte : l'exécutable embarque le dossier `web\dist` produit par la première commande.

```powershell
npm --prefix web run build
go build -tags embed ./cmd/observatoire
go build -o make-repos.exe ./scripts/fixtures
```

- La première affiche `✓ built in …`. Un avertissement `Some chunks are larger than 500 kB` peut apparaître **en rouge** (PowerShell l'affiche comme `NativeCommandError`) : il est **sans gravité**, le build a bien réussi.
- Les deux suivantes n'affichent rien quand tout va bien. Tu as maintenant `observatoire.exe` (l'application) et `make-repos.exe` (le générateur des dépôts de démo).

### 4. Autoriser les exécutables dans Windows Defender (si le dépôt est dans `Documents`)

Si l'**Accès contrôlé aux dossiers** de Windows Defender est actif, il empêche les programmes inconnus d'écrire dans `Documents`, `Bureau`, etc. Sans cette étape, l'application ne peut pas créer sa base (voir [Problèmes fréquents](#problèmes-fréquents)).

**Option A — autoriser les deux exécutables.** Récupère leurs chemins complets :

```powershell
(Resolve-Path .\observatoire.exe).Path
(Resolve-Path .\make-repos.exe).Path
```

Puis, dans un **PowerShell lancé en administrateur**, avec tes chemins :

```powershell
Add-MpPreference -ControlledFolderAccessAllowedApplications "C:\Users\<toi>\Documents\Observatoire\observatoire.exe"
Add-MpPreference -ControlledFolderAccessAllowedApplications "C:\Users\<toi>\Documents\Observatoire\make-repos.exe"
```

Sans ligne de commande : *Sécurité Windows › Protection contre les virus et menaces › Gérer la protection contre les ransomware › Autoriser une application via l'Accès contrôlé aux dossiers*.

Recompiler au même endroit continue en général de fonctionner ; si Windows bloque de nouveau, refais l'autorisation.

**Option B — placer le dépôt hors des dossiers protégés**, par exemple `C:\dev\Observatoire`, puis reprendre à l'étape 1.

### 5. Générer les dépôts de démo

```powershell
.\make-repos.exe
```

Rien ne s'affiche ; le dossier `testdata\depots\` contient maintenant `alpha` et `beta`.

### 6. Importer la démo

```powershell
.\observatoire.exe import -db demo.db -coffre testdata/coffre -depots "Atlas=testdata/depots/alpha,Boussole=testdata/depots/beta"
```

Tu dois voir :

```
2 dépôts, 42 commits ajoutés ; notes : 14 écrites, 0 inchangées, 0 supprimées ; …ms
```

### 7. Lancer le serveur et ouvrir l'interface

```powershell
.\observatoire.exe serve -db demo.db
```

Il affiche `Observatoire sur http://127.0.0.1:7878`. Ouvre **http://127.0.0.1:7878** dans ton navigateur : les projets Atlas et Boussole apparaissent. `Ctrl+C` arrête le serveur.

---

## Utiliser avec ton propre coffre

### 1. Repérer ton coffre Obsidian

C'est le dossier que tu ouvres dans Obsidian (il contient un sous-dossier caché `.obsidian`). Par défaut l'application cherche `C:\dev\Obsidian Vault` ; si ton coffre est ailleurs (c'est presque toujours le cas), **il faut passer `-coffre`** à chaque `import` et `rapport`.

Tu n'as pas de coffre ? Tu peux partir d'une copie de `testdata\coffre` comme modèle.

### 2. Décrire tes projets avec une fiche

Un projet n'est reconnu que par une **fiche** placée exactement en `Projets\<Nom>\<Nom>.md` avec `type: projet`. Pour qu'il reçoive les données Git, la fiche doit contenir **`chemin_source` avec le chemin absolu** du dépôt :

```markdown
---
type: projet
statut: en cours
chemin_source: C:\dev\mon-projet
technos:
  - "[[Go]]"
  - "[[TypeScript]]"
---
# Mon projet
```

- Sans `chemin_source`, ou avec une valeur qui n'est pas un chemin absolu (`C:\…` ou `/…`), le projet existe mais **n'a aucun commit**.
- `verifie_commit: <sha>` et `verifie_le: <date>` (facultatifs) servent à repérer les fiches périmées.
- Le rapport classe en « projets qui dorment » ceux dont le `statut` est exactement `en cours`.

Facultatif aussi :

- **Sessions** : notes du dossier `Sessions\` avec `type: session`, `projet: "[[Nom]]"`, `date: AAAA-MM-JJ`, `ia: "[[Nom de l'IA]]"`.
- **Journal** : `Journal.md` à la racine du coffre, lignes `- AAAA-MM-JJ - [[IA]] - texte`.

Toutes les autres notes `.md` sont importées pour la recherche et le graphe (les dossiers cachés et les liens symboliques sont ignorés). L'import **ne modifie jamais** le coffre.

### 3. Importer

```powershell
.\observatoire.exe import -coffre "C:\Users\<toi>\Documents\Mon Coffre"
```

La base `observatoire.db` est créée dans le dossier courant. Le bilan indique combien de dépôts et de commits ont été importés ; un dépôt introuvable affiche `attention : <Projet> (<chemin>) : …` sans bloquer les autres.

Relance cette commande quand tu veux : seuls les nouveaux commits et les notes modifiées sont réécrits.

### 4. Ouvrir l'interface

```powershell
.\observatoire.exe serve
```

Puis **http://127.0.0.1:7878**.

### 5. (Facultatif) Écrire le rapport de la semaine dans le coffre

```powershell
.\observatoire.exe rapport -coffre "C:\Users\<toi>\Documents\Mon Coffre"
.\observatoire.exe rapport -stdout      # pour juste l'afficher
```

Il crée ou remplace `Statistiques.md` à la racine du coffre.

---

## Problèmes fréquents

| Ce que tu vois | Cause | Solution |
|---|---|---|
| `erreur : ouverture de demo.db : unable to open database file (14)` (ou le serveur s'arrête aussitôt) | L'Accès contrôlé aux dossiers bloque `observatoire.exe` | [Étape 4](#4-autoriser-les-exécutables-dans-windows-defender-si-le-dépôt-est-dans-documents) du démarrage rapide, ou dépôt hors de `Documents` |
| `mkdir testdata\depots: The system cannot find the file specified` avec `go run ./scripts/fixtures/make-repos.go` | `go run` crée un exécutable temporaire au nom aléatoire, que Defender bloque | Utiliser `go build -o make-repos.exe ./scripts/fixtures`, autoriser `make-repos.exe`, puis `.\make-repos.exe` |
| `erreur : lecture du coffre : lstat C:\dev\Obsidian Vault: …` | Le coffre par défaut n'existe pas chez toi | Ajouter `-coffre "<chemin de ton coffre>"` |
| `0 dépôts, 0 commits ajoutés` | Aucune fiche `Projets\<Nom>\<Nom>.md` avec `type: projet` et un `chemin_source` absolu | Voir [Décrire tes projets](#2-décrire-tes-projets-avec-une-fiche) |
| `attention : <Projet> (<chemin>) : git rev-parse: …` | Le `chemin_source` n'est pas un dépôt Git, ou Git n'est pas dans le `PATH` | Corriger le chemin ; vérifier `git --version` |
| `Some chunks are larger than 500 kB` en rouge / `NativeCommandError` | Simple avertissement de Vite | À ignorer |
| `npm error … ENOENT … web\package.json` | Commande lancée hors de la racine du dépôt | `cd` à la racine (dossier qui contient `go.mod`) |
| `interface absente : API seule (construire avec npm --prefix web run build)` | Exécutable compilé **sans** `-tags embed` et lancé hors de la racine, ou `web\dist` pas construit | Relancer l'étape 3 (`npm --prefix web run build` puis `go build -tags embed ./cmd/observatoire`) |
| `erreur : listen tcp 127.0.0.1:7878: bind: …` | Le port est déjà pris (un autre `serve` tourne ?) | Fermer l'autre serveur, ou `serve -port 8080` |
| `refus d'ecraser …\Statistiques.md: front matter genere_par: observatoire absent` | Un `Statistiques.md` écrit à la main existe déjà dans le coffre | Le renommer, ou utiliser `rapport -stdout` |
| `fixtures absentes : lancer d'abord go run ./scripts/fixtures/make-repos.go` (tests Go) | Dépôts de démo non générés | `.\make-repos.exe` |

---

## Référence

### Commandes et options

```
observatoire <commande> [options]
```

| Commande | Rôle |
|---|---|
| `migrate` | crée ou met à jour le schéma de la base |
| `import` | importe les dépôts Git et le coffre |
| `serve` | lance l'interface sur http://127.0.0.1 |
| `rapport` | écrit `Statistiques.md` dans le coffre |
| `watch` | prévu, **pas encore implémenté** |

| Option | Commandes | Défaut | Rôle |
|---|---|---|---|
| `-db <fichier>` | toutes | `observatoire.db` | fichier de la base |
| `-coffre <dossier>` | `import`, `rapport` | `C:\dev\Obsidian Vault` | racine du coffre Obsidian |
| `-depots "Nom=dossier,…"` | `import` | — | remplace le `chemin_source` des fiches nommées (chemins relatifs acceptés) |
| `-port <n>` | `serve` | `7878` | port sur 127.0.0.1 |
| `-stdout` | `rapport` | désactivé | affiche le rapport au lieu de l'écrire |

`import`, `serve`, `rapport` et `migrate` appliquent eux-mêmes les migrations : `migrate` n'est jamais obligatoire.

À savoir sur `serve` :

- il n'écoute que sur `127.0.0.1` et refuse toute requête dont l'en-tête `Host` n'est pas local ;
- la **console SQL** est en lecture seule (pas d'écriture, d'`ATTACH`, de `PRAGMA` d'écriture ni de transaction), limitée à 5 s, 1000 lignes et 2 requêtes simultanées ;
- les **requêtes enregistrées** sont des fichiers `*.sql` dans un dossier `requetes\` à côté de la base.

### Où sont les données

| Quoi | Où |
|---|---|
| Base SQLite | `observatoire.db` dans le dossier courant, ou le fichier de `-db` (plus ses fichiers `-wal` et `-shm`) |
| Requêtes enregistrées | `requetes\` à côté de la base |
| Rapport | `Statistiques.md` à la racine du coffre |

Tu peux supprimer la base à tout moment : un nouvel `import` la reconstruit.

`.gitignore` ignore déjà `*.db`, `*.db-wal`, `*.db-shm`, `/observatoire.exe`, `testdata/depots/` et `web/dist/`, mais **pas `make-repos.exe`** : pense à l'ajouter si tu ne veux pas le committer.

### Compiler sans l'interface intégrée

`go build ./cmd/observatoire` (ou `go run ./cmd/observatoire <commande>`) produit un exécutable qui lit l'interface sur le disque dans `web\dist`, **relativement au dossier courant** : il ne marche donc que lancé depuis la racine du dépôt. Avec `-tags embed`, l'interface est dans l'exécutable et il marche de n'importe où (il faut alors avoir construit `web\dist` avant de compiler).

### Développer le front

```powershell
cd web
npm run dev          # Vite ; les appels /api partent vers http://127.0.0.1:7878
npm run dev:mock     # Vite sans serveur Go : réponses lues dans public/mocks/*.json
npm run typecheck
npm test
```

Pour `npm run dev`, laisse `observatoire.exe serve` tourner dans un autre terminal (port 7878), puis ouvre l'adresse affichée par Vite. `dev:mock` charge `web\.env.mock` (`VITE_MOCK=1`).

### Base, migrations et sqlc

- **Migrations** : `migrations\*.sql` (goose), embarquées dans l'exécutable et appliquées automatiquement. Pour changer le schéma, ajoute `0003_….sql` avec `-- +goose Up` / `-- +goose Down`, puis recompile.
- **Code SQL** : les requêtes de `internal\sql\import` et `internal\sql\requetes` sont compilées par sqlc (`sqlc.yaml`) vers `internal\sql\gen`, à ne pas modifier à la main. Pour régénérer (sqlc 1.31.1) :

  ```powershell
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
  sqlc generate
  ```

  Particularités des requêtes : `internal\sql\requetes\README.md`.

### Tests

```powershell
.\make-repos.exe                 # une fois (hors Windows : go run ./scripts/fixtures/make-repos.go)
go test ./...
npm --prefix web test
```

Après relecture, mettre à jour les résultats attendus des requêtes SQL :

```powershell
go test ./internal/sql/requetes_test -count=1 -args -maj
```

### Architecture

```
.
├── cmd/observatoire/   point d'entrée (migrate, import, serve, rapport)
├── internal/
│   ├── api/            serveur HTTP : vues JSON, console SQL, SSE, interface
│   ├── db/             ouverture SQLite et migrations
│   ├── ingest/         import : git/ (historique), code/ (langages, scc), vault/ (coffre)
│   ├── model/          types partagés
│   ├── report/         génération de Statistiques.md
│   └── sql/            requêtes sqlc (import/, requetes/), code généré (gen/), tests
├── migrations/         schéma SQL (goose)
├── scripts/fixtures/   générateur des dépôts de démo
├── testdata/           coffre fictif + dépôts générés
├── web/                interface React + Vite (src/, public/mocks/, embed.go, disque.go)
└── sqlc.yaml
```
