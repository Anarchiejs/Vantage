# Changelog

## Unreleased

Public-release cleanup. Observatoire is a local application built for Obsidian. It runs on Windows, Linux, and macOS.

### Added

- Linux and macOS launchers `install.sh`, `build.sh`, and `start.sh`, next to the Windows `.cmd` scripts.
- `start.sh` and `start.cmd` take a port (`--port` / `-Port`, default 7878) and a repository remap (`--repos` / `-Repos`). `<vault>/.observatoire/repos` supplies the same remap, with paths relative to the vault. The sample vault maps Atlas and Compass onto `testdata/repos`, so that vault imports Git history on Windows, Linux, and macOS. The start scripts create those repositories when they are missing.
- `start.sh` remembers the vault path (`$XDG_DATA_HOME/observatoire/vault.txt` on Linux, `~/Library/Application Support/Observatoire/vault.txt` on macOS).
- CI runs on Ubuntu, macOS, and Windows. Web unit tests run on all three. Playwright end-to-end tests run on Ubuntu.
- `go test ./...` creates `testdata/repos` when the fixture repositories are missing.

### Fixed

- The UI build no longer reports a JavaScript chunk larger than 500 kB. Charts load the ECharts series this app draws, and CodeMirror is its own chunk.
- macOS filenames are stored as Unicode NFC, so accented note titles and Git paths match Linux and Windows. On Linux and Windows the path bytes are unchanged.
- The server stops on SIGTERM (the signal Linux and macOS use to ask a process to exit). Ctrl+C is unchanged.

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
