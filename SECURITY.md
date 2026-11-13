# Security

Vantage is a local program. It has no accounts and no telemetry.

## What stays on the machine

The server listens on `127.0.0.1` only. A request whose `Host` is not the loopback name is rejected, which blocks a browser tab on another site from calling the API through DNS rebinding. Do not put the process behind a reverse proxy or bind it to a LAN address: there is no authentication.

The database holds the text of your notes, commit subjects, author names, and changed-file paths. Do not publish `vantage.db`, its `-wal` and `-shm` files, or the `queries/` directory. Saved SQL can contain note text.

## What it changes on disk

`import` and `watch` read the vault and do not modify notes. `report` writes `Statistics.md` at the vault root. It replaces that file only when the existing note contains `generated_by: observatoire`. It does not keep a backup. Copy the note first if you need the previous text.

`start.cmd` remembers the vault path in `%LOCALAPPDATA%\Vantage\vault.txt`. `start.sh` remembers it in `$XDG_DATA_HOME/vantage/vault.txt` on Linux (`~/.local/share/vantage/vault.txt` when `XDG_DATA_HOME` is unset) and in `~/Library/Application Support/Vantage/vault.txt` on macOS. The file contains only that path. If that file is missing, the start script still reads the previous Observatoire location and then writes the Vantage path.

On Windows, `build.ps1` may open an administrator prompt so Windows Defender Controlled Folder Access allows `vantage.exe` to create the database. That changes a machine setting. Decline it and move the repository out of a protected folder if you do not want that change.

## SQL console

The console is read-only: no writes, no `ATTACH`, no `PRAGMA` that changes the connection, and no transaction. A query is limited to 5 seconds, 1000 rows, and two queries at once. It still runs SQL you type against the local database. Do not paste queries from someone else into it.

## Reporting a problem

Open a GitHub issue on [Anarchiejs/Vantage](https://github.com/Anarchiejs/Vantage). Do not include your vault, your database, or commit messages from private repositories.
