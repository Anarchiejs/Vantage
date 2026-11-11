# Observatoire

Observatoire is a **local dashboard built for [Obsidian](https://obsidian.md)**. It is not an Obsidian plugin and it does not run inside Obsidian. It reads the vault folder on disk, reads the Git repositories named by that vault's project cards, and shows the result at **http://127.0.0.1:7878**: history, languages, the note graph, search, and a SQL console.

It is for someone who already keeps project notes in Obsidian and wants those notes next to the commits of the repositories they describe.

![Home, fictional vault](docs/home.png)
![Note graph, fictional vault](docs/graph.png)
![Atlas project, fictional vault](docs/project.png)

The pictures are `testdata/vault` (Atlas and Compass), not a personal vault.

Nothing is uploaded. There is no telemetry. The network behavior, and the few files the program writes, are listed under [What it reads and writes](#what-it-reads-and-writes).

## What Obsidian features it uses

Observatoire understands a vault the way Obsidian stores it on disk:

- A vault is a folder that contains `.obsidian`. Hidden directories, including `.obsidian`, are skipped.
- Notes are Markdown files. Properties are YAML between the opening `---` lines. [Dataview](https://github.com/blacksmithgu/obsidian-dataview) inline fields (`key:: value`) are not read. Templater, the core graph, and community plugins are not required and are not consulted.
- Links are Obsidian wikilinks (`[[Note]]`, `[[Note|alias]]`, `[[Note#heading]]`). Links inside fenced code are ignored. An embed of a non-Markdown file is ignored.
- The graph in the UI is Observatoire's graph of those wikilinks. It is not `graph.json`.

Every `.md` note is imported for search, the graph, and broken links. A **project** is a card at exactly `Projects/<Name>/<Name>.md`:

```markdown
---
type: project
status: in progress
source_path: C:\dev\my-project
technologies:
  - "[[Go]]"
---
# My project
```

`source_path` must be an absolute path to a Git repository. Without it, the card is still a project and still has notes, and it has no commits. `status: in progress` is the exact string used to decide that a quiet project is dormant.

Optional:

- `verified_commit` and `verified_on`, to spot a card that has fallen behind the repository. A value may start with a SHA (`0df64eb2 (upstream)`).
- Sessions: notes in `Sessions/` with `type: session`, `project: "[[Name]]"`, `date: YYYY-MM-DD`, and `ai: "[[Name]]"`.
- A `Journal.md` at the vault root. A line is kept when it matches `- YYYY-MM-DD - [[AI]] - text`.

A full fictional vault is in `testdata/vault` (Atlas and Compass). `import` does not modify notes. `report` does write one file; see below.

Git import follows **HEAD** of each repository (the current branch only). It is incremental. If that history is rewritten, the repository is read again from the start. Author **names** are stored. Author emails are not. Changed paths and line counts come from `git log --numstat`. Language totals come from [scc](https://github.com/boyter/scc), which counts lines and does not store source text.

## What it reads and writes

**Reads**

- Every non-hidden `.md` file in the vault (symlink notes are refused).
- `git log` of `HEAD` for each absolute `source_path`.
- A line count of the repository working tree, for language statistics.

**Writes**

- `observatoire.db`, plus `observatoire.db-wal` and `observatoire.db-shm`, in the current directory (or `-db`).
- `queries/*.sql` next to that database, when you save a query in the SQL console.
- `Statistics.md` at the vault root, only when you run `report` (or the report command). The file is replaced atomically. An existing `Statistics.md` is overwritten only when it contains `generated_by: observatoire`. There is no backup: copy the note first if you need the previous text. `report` can create the vault directory.
- On Windows, `start.cmd` stores the vault path in `%LOCALAPPDATA%\Observatoire\vault.txt`.

`build.ps1` on Windows may open an administrator prompt so Controlled Folder Access allows `observatoire.exe` to create the database. That is a machine setting. On Linux and macOS that step does not exist.

## Quick start (Windows)

Open **PowerShell** in this folder.

**0. Get the project** (once)

```powershell
git clone https://github.com/Anarchiejs/Observatoire.git
cd Observatoire
```

> No Git yet? `winget install --id Git.Git -e`, then close and reopen PowerShell.

**1. Install** — Git, Go, and Node (via winget if they are missing), then the dependencies

```powershell
.\install.cmd
```

**2. Build** — the UI, then `observatoire.exe`

```powershell
.\build.cmd
```

If Windows Defender **Controlled folder access** is on, an administrator window may open to allow `observatoire.exe`. Accept it, or the program cannot create its database. You can also keep the repository outside a protected folder, for example `C:\dev\Observatoire`.

**3. Start** — import your Obsidian vault and open the UI. The path is the folder you open in Obsidian, not this repository.

```powershell
.\start.cmd "C:\Users\you\Documents\My Vault"
```

The browser opens **http://127.0.0.1:7878**. `Ctrl+C` stops the server. The path is remembered: next time, `.\start.cmd` is enough.

While `start.cmd` is running, an edited note or a new commit shows up within a few seconds. After a `git pull`, run `.\build.cmd` again.

## Linux and macOS

The server is the same program. CI runs it on Ubuntu and on Windows. You need Go 1.26.4, Node.js 22, and Git.

```bash
git clone https://github.com/Anarchiejs/Observatoire.git
cd Observatoire
go mod download
npm --prefix web ci
npm --prefix web run build
go build -tags embed -o observatoire ./cmd/observatoire
./observatoire import -vault "$HOME/Documents/My Vault"
./observatoire watch -vault "$HOME/Documents/My Vault"
```

Open http://127.0.0.1:7878. `-vault` is required. There is no remembered path outside Windows: pass `-vault` each time, or wrap the two commands in a shell script.

`go build` without `-tags embed` reads `web/dist` from disk and only works when the current directory is the repository root.

## Commands

| Command | Role |
|---|---|
| `import -vault <folder>` | import the vault and the Git repositories |
| `serve` | serve the UI on 127.0.0.1 (no vault; uses the existing database) |
| `report -vault <folder>` | write `Statistics.md` at the vault root |
| `migrate` | create or update the schema (also done by the other commands) |
| `watch -vault <folder>` | import, serve, and reimport when a note or a repository changes |

| Option | Commands | Default |
|---|---|---|
| `-db <file>` | all | `observatoire.db` in the current directory |
| `-vault <folder>` | `import`, `watch`, and `report` (unless `-stdout`) | required; no default |
| `-repos "Name=folder,…"` | `import`, `watch` | replaces `source_path` for the named cards |
| `-port <n>` | `serve`, `watch` | `7878` |
| `-stdout` | `report` | print the report instead of writing it |

```bash
./observatoire report -vault "$HOME/Documents/My Vault"
./observatoire report -stdout
```

**SQL console:** read-only (no writes, no `ATTACH`, no `PRAGMA` that changes the connection, no transaction), 5 seconds, 1000 rows, and at most two queries at once. The server accepts only local requests. Details are in [SECURITY.md](SECURITY.md).

Deleting `observatoire.db` is safe. The next import rebuilds it from the vault and the repositories. Do not delete the vault.

## Upgrading an older database

The schema, the vault keys, and the CLI were renamed from French to English by editing the first two migration files in place. Goose will not rebuild a database that already applied those versions, and Observatoire refuses to open a file that still contains a table named `projet`.

Delete `observatoire.db`, `observatoire.db-wal`, and `observatoire.db-shm`, update the cards using the table in [CHANGELOG.md](CHANGELOG.md), then import again. The remembered Windows path also moved from `coffre.txt` to `vault.txt`; `start.cmd` will ask for the vault once.

## Common problems

| Message | What to do |
|---|---|
| `-vault is required` | Pass the folder you open in Obsidian. |
| `retired French schema` | Delete the database and its `-wal` and `-shm` files, then import again. See [CHANGELOG.md](CHANGELOG.md). |
| `unable to open database file (14)`, or the server stops immediately | On Windows, Defender is blocking `observatoire.exe`. Run `.\build.cmd` again and accept the administrator window, or allow the program in *Windows Security › Virus & threat protection › Manage ransomware protection › Allow an app*. Or move the repository out of a protected folder. |
| `… was not found` / `was just installed but is still not found` | Close and reopen PowerShell, then run the script again. |
| `winget was not found` | Install "App Installer" from the Microsoft Store. |
| `Some chunks are larger than 500 kB` | A Vite warning. Ignore it. |
| `folder not found` | Check the vault path and quote it. |
| `no Git repository imported` / `0 repositories` | No `Projects/<Name>/<Name>.md` card with `type: project` and an absolute `source_path`. |
| `warning: <Project> (<path>): git rev-parse` | That `source_path` is not a Git repository. The other projects are still imported. |
| `listen tcp 127.0.0.1:7878: bind` | The port is taken. Stop the other process, or `observatoire serve -port 8080`. |

## Develop

See [CONTRIBUTING.md](CONTRIBUTING.md). Short form:

```bash
npm --prefix web run dev          # Vite; /api is proxied to http://127.0.0.1:7878
npm --prefix web run dev:mock     # no Go server; responses come from web/public/mocks
go run ./scripts/fixtures/make-repos.go
go test ./...
```

Schema changes are new files in `migrations/`. SQL is compiled with sqlc 1.31.1. Do not edit `internal/sql/gen` by hand.

## Layout

```
cmd/observatoire/   entry point (import, serve, watch, report, migrate)
internal/api/       HTTP server: JSON views, SQL console, SSE, UI
internal/db/        SQLite and migrations
internal/ingest/    import: git/, code/ (languages via scc), vault/
internal/report/    Statistics.md
internal/sql/       sqlc queries and generated code (gen/)
internal/watch/     vault and repository watching (fsnotify)
migrations/         SQL schema (goose)
scripts/            install.ps1, build.ps1, start.ps1, fixtures/
testdata/           fictional vault; repos/ is generated and gitignored
web/                React + Vite UI
```

## License

[MIT](LICENSE). Copyright (c) 2026 [Anarchy](https://github.com/Anarchiejs). Third-party licenses are listed in [NOTICE](NOTICE).
