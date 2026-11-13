# Changelog

## Unreleased

The project is now named Vantage. It is a local application built for Obsidian. It runs on Windows, Linux, and macOS.

### Added

- Linux and macOS launchers `install.sh`, `build.sh`, and `start.sh`, next to the Windows `.cmd` scripts.
- `start.sh` and `start.cmd` take a port (`--port` / `-Port`, default 7878) and a repository remap (`--repos` / `-Repos`). `<vault>/.vantage/repos` supplies the same remap, with paths relative to the vault. If only `.observatoire/repos` exists, it is still read and a warning is printed. The sample vault maps Atlas and Compass onto `testdata/repos`, so that vault imports Git history on Windows, Linux, and macOS. The start scripts create those repositories when they are missing.
- `start.sh` remembers the vault path (`$XDG_DATA_HOME/vantage/vault.txt` on Linux, `~/Library/Application Support/Vantage/vault.txt` on macOS). If a path was stored under the previous Observatoire directory, the start script still reads it, warns, and then writes the Vantage location.
- CI runs on Ubuntu, macOS, and Windows. Web unit tests run on all three. Playwright end-to-end tests run on Ubuntu.
- `go test ./...` creates `testdata/repos` when the fixture repositories are missing.

### Fixed

- `-repos` keeps the backslashes in a Windows path. A comma still separates pairs; write `\,` when the path itself contains a comma.
- The UI build no longer reports a JavaScript chunk larger than 500 kB. Charts load the ECharts series this app draws, and CodeMirror is its own chunk.
- macOS filenames are stored as Unicode NFC, so accented note titles and Git paths match Linux and Windows. On Linux and Windows the path bytes are unchanged.
- The server stops on SIGTERM (the signal Linux and macOS use to ask a process to exit). Ctrl+C is unchanged.

### Breaking

- The command is `vantage`, the Go module is `github.com/Anarchiejs/Vantage`, and the default database file is `vantage.db`. A file still named `observatoire.db`, or an `observatoire.db` sitting next to a missing `vantage.db`, is refused with instructions to rename it (and the `-wal` and `-shm` files). You can also pass `-db` after renaming.
- The remap file is `<vault>/.vantage/repos`. If only `.observatoire/repos` exists, Vantage still reads it and prints a warning. `generated_by: observatoire` in report front matter is still accepted so an old `Statistics.md` can be replaced.
- Commit identity is `(repository_id, sha)`. Two clones of the same history both keep their commits. Reimporting a vault deletes projects, journal lines, sessions, technologies, and repository paths that are no longer in the vault.
- `-vault` is required for `import` and `watch`, and for `report` unless `-stdout` is set. There is no default vault path. `report` does not create the vault directory.
- A database created before the English schema rename (it still has a table named `projet`) is refused. Goose cannot rebuild tables whose migration version is already applied. Delete `vantage.db`, `vantage.db-wal`, and `vantage.db-shm`, then import the vault again.
- `Statistics.md` no longer links to a fixed hub note, and it no longer rewrites the `Claude Code` wikilink.
- The remembered vault path moved from the Observatoire application-data directory to the Vantage directory (`%LOCALAPPDATA%\Vantage\vault.txt` on Windows, `$XDG_DATA_HOME/vantage/vault.txt` on Linux, `~/Library/Application Support/Vantage/vault.txt` on macOS). The start scripts still read the previous file when the new one is missing.

### Changed

- Go module path is `github.com/Anarchiejs/Vantage`.
- Fixture repositories are generated in `testdata/repos` (`go run ./scripts/fixtures/make-repos.go`). `make-repos.exe` is not part of the repository.
- The weekly report is still `Statistics.md` at the vault root, with front matter `generated_by: vantage`. Its activity table is the last 7 days against the previous 7 days.
- `vantage version` prints the build version (`dev` unless the release build sets it).

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
| `vantage rapport -coffre` | `vantage report -vault` |
| `vantage import -depots` | `vantage import -repos` |
| `%LOCALAPPDATA%\Vantage\coffre.txt` | `%LOCALAPPDATA%\Vantage\vault.txt` |
| `installer.cmd`, `construire.cmd`, `lancer.cmd` | `install.cmd`, `build.cmd`, `start.cmd` |

Dormant projects are cards whose `status` is exactly `in progress`.
