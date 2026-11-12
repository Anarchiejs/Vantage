# Contributing

Observatoire is maintained by [Anarchy](https://github.com/Anarchiejs). Issues and pull requests are welcome.

## Setup

Windows, Linux, and macOS. CI runs the Go tests on all three, and runs the install and build scripts there too.

- Go 1.26.4 (`go.mod`)
- Node.js 22.12 or newer
- Git

Windows, from PowerShell:

```powershell
.\install.cmd
.\build.cmd
```

Linux and macOS:

```bash
./install.sh
./build.sh
```

`install.sh` installs Git through the system package manager when it is missing (Homebrew on macOS), and downloads official Go and Node.js builds into `~/.local/opt` when the copies on `PATH` are missing or too old.

Then, on every platform:

```bash
go run ./scripts/fixtures/make-repos.go
```

The fixture command creates `testdata/repos/alpha` and `testdata/repos/beta`. Those directories are gitignored. `go test ./...` fails without them. It forces `commit.gpgsign=false` so a machine that signs commits still produces the hashes checked in under `internal/sql/queries_test/expected/`.

## Checks

```bash
go vet ./...
go test ./...
npm --prefix web run typecheck
npm --prefix web test
npm --prefix web run build
npm --prefix web run e2e
```

End-to-end tests build the UI, import `testdata/vault`, and serve it on `127.0.0.1:7879`. On Windows the same build runs through `install.cmd` and `build.cmd`. On Linux and macOS it runs through `./install.sh` and `./build.sh`. `scripts/smoke-start.sh` checks that `start.sh` remembers a vault and that the server stops on SIGTERM.

## SQL

Queries in `internal/sql/import` and `internal/sql/queries` are compiled with sqlc 1.31.1 into `internal/sql/gen`. Do not edit `internal/sql/gen` by hand.

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
sqlc generate
```

CI runs `sqlc generate` on Linux and fails if `internal/sql/gen` drifts.

To refresh the checked-in query results after you have read the diff:

```bash
go test ./internal/sql/queries_test -count=1 -args -update
```

A new schema change is a new file `migrations/0003_….sql` with `-- +goose Up` and `-- +goose Down`. Do not edit a migration that has already been applied: goose records the version and will not re-run it.

## Vault fixtures

`testdata/vault` is a fictional Obsidian vault. Keep personal names, real repository paths, and private note titles out of it. The sample cards are Atlas and Compass.

## UI unit tests

Vitest covers the home page, the project list and timeline, the command palette, search highlighting, the graph helpers, the technology trends, and the SQL console chart helpers. Chart components are mocked: tests assert the options passed to ECharts, not the canvas.
