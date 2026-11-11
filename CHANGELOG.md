# Changelog

## Unreleased

Public-release cleanup. Observatoire is a local application built for Obsidian.

### Breaking

- `-vault` is required for `import` and `watch`, and for `report` unless `-stdout` is set. There is no default vault path.
- A database created before the English schema rename (it still has a table named `projet`) is refused. Goose cannot rebuild tables whose migration version is already applied. Delete `observatoire.db`, `observatoire.db-wal`, and `observatoire.db-shm`, then import the vault again.
- `Statistics.md` no longer links to a fixed hub note, and it no longer rewrites the `Claude Code` wikilink.

### Changed

- Go module path is `github.com/Anarchiejs/Observatoire`.
- Fixture repositories are generated in `testdata/repos` (`go run ./scripts/fixtures/make-repos.go`). `make-repos.exe` is not part of the repository.
- The weekly report is still `Statistics.md` at the vault root, with front matter `generated_by: observatoire`.

## 2026-11-11

English rename of the vault contract, the CLI, and the SQLite schema. This was applied by editing `migrations/0001_init.sql` and `migrations/0002_index_sessions.sql` in place, not by a new migration.

If you already imported a vault with the French contract, recreate the database and update the cards:

| Before | After |
|---|---|
| `Projets/<Name>/<Name>.md` | `Projects/<Name>/<Name>.md` |
| `type: projet` | `type: project` |
| `chemin_source` | `source_path` |
| `statut` | `status` |
| `technos` | `technologies` |
| `verifie_commit`, `verifie_le` | `verified_commit`, `verified_on` |
| session fields `projet`, `ia` | `project`, `ai` |
| `Statistiques.md`, `genere_par` | `Statistics.md`, `generated_by` |
| `observatoire rapport -coffre` | `observatoire report -vault` |
| `observatoire import -depots` | `observatoire import -repos` |
| `%LOCALAPPDATA%\Observatoire\coffre.txt` | `%LOCALAPPDATA%\Observatoire\vault.txt` |
| `installer.cmd`, `construire.cmd`, `lancer.cmd` | `install.cmd`, `build.cmd`, `start.cmd` |

Dormant projects are cards whose `status` is exactly `in progress`.
