// Command observatoire: a local dashboard for development work.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/api"
	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
	"github.com/Anarchiejs/Observatoire/internal/report"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
	"github.com/Anarchiejs/Observatoire/internal/watch"
	"github.com/Anarchiejs/Observatoire/web"
)

const usage = `usage: observatoire <command> [options]

commands:
  migrate   create or update the database schema
  import    import Git repositories and the vault
  serve     serve the UI on http://127.0.0.1
  report    write Statistics.md in the vault
  watch     import, serve the UI, and reimport on every change
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	command, args := os.Args[1], os.Args[2:]
	var err error
	switch command {
	case "migrate":
		err = migrate(args)
	case "import":
		err = runImport(args)
	case "serve":
		err = serve(args)
	case "report":
		err = runReport(args)
	case "watch":
		err = runWatch(args)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func migrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	path := fs.String("db", "observatoire.db", "database file")
	fs.Parse(args)
	base, err := db.Open(*path)
	if err != nil {
		return err
	}
	defer base.Close()
	return db.Migrate(base)
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	path := fs.String("db", "observatoire.db", "database file")
	vault := fs.String("vault", "", "Obsidian vault root (required)")
	repos := fs.String("repos", "", `replaces source_path for named cards: "Atlas=testdata/repos/alpha,Compass=..."`)
	fs.Parse(args)
	if err := requireVault(*vault); err != nil {
		return err
	}
	replacements, err := readRepos(*repos)
	if err != nil {
		return err
	}
	base, err := db.Open(*path)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrate(base); err != nil {
		return err
	}
	summary, err := ingest.Import(context.Background(), base, ingest.Options{Vault: *vault, Repos: replacements})
	if err != nil {
		return err
	}
	printSummary(summary)
	return nil
}

func requireVault(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("-vault is required: pass the Obsidian vault folder (the directory that contains .obsidian)")
	}
	return nil
}

func readRepos(repos string) (map[string]string, error) {
	replacements := map[string]string{}
	for _, pair := range strings.Split(repos, ",") {
		if name, dir, ok := strings.Cut(strings.TrimSpace(pair), "="); ok {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return nil, err
			}
			replacements[name] = abs
		}
	}
	return replacements, nil
}

func printSummary(summary ingest.Summary) {
	for _, warning := range summary.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	fmt.Printf("%d repositories, %d commits added; notes: %d written, %d unchanged, %d deleted; %s\n",
		summary.Repos, summary.Commits, summary.NotesWritten, summary.NotesUnchanged, summary.NotesDeleted, summary.Duration.Round(time.Millisecond))
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	path := fs.String("db", "observatoire.db", "database file")
	port := fs.Int("port", 7878, "port on 127.0.0.1")
	fs.Parse(args)

	server, _, closeDBs, err := prepareServer(*path, *port)
	if err != nil {
		return err
	}
	defer closeDBs()
	ctx, stop := cancelOnSignal()
	defer stop()
	return server.Listen(ctx)
}

// prepareServer opens the database (read-write and read-only) and builds the server.
func prepareServer(path string, port int) (*api.Server, *sql.DB, func(), error) {
	base, err := db.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := db.Migrate(base); err != nil {
		base.Close()
		return nil, nil, nil, err
	}
	readDB, err := db.OpenReadOnly(path)
	if err != nil {
		base.Close()
		return nil, nil, nil, err
	}
	closeDBs := func() { readDB.Close(); base.Close() }
	ui, err := web.Interface()
	if err != nil {
		closeDBs()
		return nil, nil, nil, err
	}
	if ui == nil {
		fmt.Fprintln(os.Stderr, "UI missing: API only (build it with npm --prefix web run build)")
	}
	server, err := api.New(api.Config{
		Address: fmt.Sprintf("127.0.0.1:%d", port),
		DB:      base,
		ReadDB:  readDB,
		UI:      ui,
		Queries: filepath.Join(filepath.Dir(path), "queries"),
	})
	if err != nil {
		closeDBs()
		return nil, nil, nil, err
	}
	return server, base, closeDBs, nil
}

// runWatch imports, serves the UI, and reimports whenever the vault or a repository changes.
// The UI is notified by the SSE "update" event.
func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	path := fs.String("db", "observatoire.db", "database file")
	vault := fs.String("vault", "", "Obsidian vault root (required)")
	repos := fs.String("repos", "", `replaces source_path for named cards: "Atlas=testdata/repos/alpha,Compass=..."`)
	port := fs.Int("port", 7878, "port on 127.0.0.1")
	fs.Parse(args)
	if err := requireVault(*vault); err != nil {
		return err
	}
	replacements, err := readRepos(*repos)
	if err != nil {
		return err
	}

	server, base, closeDBs, err := prepareServer(*path, *port)
	if err != nil {
		return err
	}
	defer closeDBs()
	ctx, stop := cancelOnSignal()
	defer stop()

	opt := ingest.Options{Vault: *vault, Repos: replacements}
	summary, err := ingest.Import(ctx, base, opt)
	if err != nil {
		return err
	}
	printSummary(summary)

	go func() {
		err := watch.Watch(ctx, watch.Options{
			Vault: *vault,
			Repos: func() []string { return repositoryPaths(ctx, base) },
			Import: func(ctx context.Context) (watch.Changes, error) {
				summary, err := ingest.Import(ctx, base, opt)
				if err != nil {
					return watch.Changes{}, err
				}
				printSummary(summary)
				return watch.Changes{Notes: summary.NotesWritten + summary.NotesDeleted, Commits: summary.Commits}, nil
			},
			Announce: server.Broadcaster().Announce,
			Log:      log.Printf,
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("watch stopped: %v", err)
		}
	}()
	return server.Listen(ctx)
}

// cancelOnSignal stops the server on Ctrl+C, and on SIGTERM on Linux and macOS.
func cancelOnSignal() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func repositoryPaths(ctx context.Context, base *sql.DB) []string {
	rows, err := base.QueryContext(ctx, `SELECT path FROM repository ORDER BY path`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if rows.Scan(&path) == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	path := fs.String("db", "observatoire.db", "database file")
	vault := fs.String("vault", "", "Obsidian vault root (required)")
	stdout := fs.Bool("stdout", false, "print the report instead of writing it into the vault")
	fs.Parse(args)
	if !*stdout {
		if err := requireVault(*vault); err != nil {
			return err
		}
	}
	base, err := db.Open(*path)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrate(base); err != nil {
		return err
	}
	content, err := report.Generate(context.Background(), gen.New(base), time.Now().Format("2006-01-02"))
	if err != nil {
		return err
	}
	if *stdout {
		_, err = os.Stdout.Write(content)
		return err
	}
	if err := report.Write(*vault, content); err != nil {
		return err
	}
	fmt.Println("Statistics.md written in", *vault)
	return nil
}
