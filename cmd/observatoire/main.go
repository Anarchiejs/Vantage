// Commande observatoire : tableau de bord local du travail de développement.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/api"
	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
	"github.com/Anarchiejs/Observatoire/internal/report"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
	"github.com/Anarchiejs/Observatoire/internal/watch"
	"github.com/Anarchiejs/Observatoire/web"
)

const usage = `usage : observatoire <commande> [options]

commandes :
  migrate   crée ou met à jour le schéma de la base
  import    importe les dépôts Git et le coffre
  serve     lance l'interface sur http://127.0.0.1
  rapport   écrit Statistiques.md dans le coffre
  watch     importe, lance l'interface et réimporte à chaque changement
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "migrate":
		err = migrer(args)
	case "import":
		err = importer(args)
	case "serve":
		err = servir(args)
	case "rapport":
		err = rapport(args)
	case "watch":
		err = surveiller(args)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "erreur :", err)
		os.Exit(1)
	}
}

func migrer(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	fs.Parse(args)
	base, err := db.Ouvrir(*chemin)
	if err != nil {
		return err
	}
	defer base.Close()
	return db.Migrer(base)
}

func importer(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	coffre := fs.String("coffre", `C:\dev\Obsidian Vault`, "racine du coffre Obsidian")
	depots := fs.String("depots", "", `remplace le chemin_source de fiches : "Atlas=testdata/depots/alpha,Boussole=..."`)
	fs.Parse(args)
	remplacements, err := lireDepots(*depots)
	if err != nil {
		return err
	}
	base, err := db.Ouvrir(*chemin)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrer(base); err != nil {
		return err
	}
	b, err := ingest.Importer(context.Background(), base, ingest.Options{Coffre: *coffre, Depots: remplacements})
	if err != nil {
		return err
	}
	afficherBilan(b)
	return nil
}

func lireDepots(depots string) (map[string]string, error) {
	remplacements := map[string]string{}
	for _, paire := range strings.Split(depots, ",") {
		if nom, dossier, ok := strings.Cut(strings.TrimSpace(paire), "="); ok {
			abs, err := filepath.Abs(dossier)
			if err != nil {
				return nil, err
			}
			remplacements[nom] = abs
		}
	}
	return remplacements, nil
}

func afficherBilan(b ingest.Bilan) {
	for _, a := range b.Avertissements {
		fmt.Fprintln(os.Stderr, "attention :", a)
	}
	fmt.Printf("%d dépôts, %d commits ajoutés ; notes : %d écrites, %d inchangées, %d supprimées ; %s\n",
		b.Depots, b.Commits, b.NotesEcrites, b.NotesInchangees, b.NotesSupprimees, b.Duree.Round(time.Millisecond))
}

func servir(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	port := fs.Int("port", 7878, "port sur 127.0.0.1")
	fs.Parse(args)

	s, _, fermer, err := preparerServeur(*chemin, *port)
	if err != nil {
		return err
	}
	defer fermer()
	ctx, fin := signal.NotifyContext(context.Background(), os.Interrupt)
	defer fin()
	return s.Ecouter(ctx)
}

// preparerServeur ouvre la base (lecture-écriture et lecture seule) et construit le serveur.
func preparerServeur(chemin string, port int) (*api.Serveur, *sql.DB, func(), error) {
	base, err := db.Ouvrir(chemin)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := db.Migrer(base); err != nil {
		base.Close()
		return nil, nil, nil, err
	}
	lecture, err := db.OuvrirLectureSeule(chemin)
	if err != nil {
		base.Close()
		return nil, nil, nil, err
	}
	fermer := func() { lecture.Close(); base.Close() }
	ui, err := web.Interface()
	if err != nil {
		fermer()
		return nil, nil, nil, err
	}
	if ui == nil {
		fmt.Fprintln(os.Stderr, "interface absente : API seule (construire avec npm --prefix web run build)")
	}
	s, err := api.Nouveau(api.Config{
		Adresse:     fmt.Sprintf("127.0.0.1:%d", port),
		Base:        base,
		BaseLecture: lecture,
		Interface:   ui,
		Requetes:    filepath.Join(filepath.Dir(chemin), "requetes"),
	})
	if err != nil {
		fermer()
		return nil, nil, nil, err
	}
	return s, base, fermer, nil
}

// surveiller importe, sert l'interface et réimporte à chaque changement du coffre ou d'un dépôt ;
// l'interface est prévenue par l'événement SSE « maj ».
func surveiller(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	coffre := fs.String("coffre", `C:\dev\Obsidian Vault`, "racine du coffre Obsidian")
	depots := fs.String("depots", "", `remplace le chemin_source de fiches : "Atlas=testdata/depots/alpha,Boussole=..."`)
	port := fs.Int("port", 7878, "port sur 127.0.0.1")
	fs.Parse(args)
	remplacements, err := lireDepots(*depots)
	if err != nil {
		return err
	}

	s, base, fermer, err := preparerServeur(*chemin, *port)
	if err != nil {
		return err
	}
	defer fermer()
	ctx, fin := signal.NotifyContext(context.Background(), os.Interrupt)
	defer fin()

	opt := ingest.Options{Coffre: *coffre, Depots: remplacements}
	b, err := ingest.Importer(ctx, base, opt)
	if err != nil {
		return err
	}
	afficherBilan(b)

	go func() {
		err := watch.Surveiller(ctx, watch.Options{
			Coffre: *coffre,
			Depots: func() []string { return cheminsDepots(ctx, base) },
			Importer: func(ctx context.Context) (watch.Changements, error) {
				b, err := ingest.Importer(ctx, base, opt)
				if err != nil {
					return watch.Changements{}, err
				}
				afficherBilan(b)
				return watch.Changements{Notes: b.NotesEcrites + b.NotesSupprimees, Commits: b.Commits}, nil
			},
			Annoncer: s.Diffuseur().Annoncer,
			Journal:  log.Printf,
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("surveillance arrêtée : %v", err)
		}
	}()
	return s.Ecouter(ctx)
}

func cheminsDepots(ctx context.Context, base *sql.DB) []string {
	lignes, err := base.QueryContext(ctx, `SELECT chemin FROM depot ORDER BY chemin`)
	if err != nil {
		return nil
	}
	defer lignes.Close()
	var res []string
	for lignes.Next() {
		var c string
		if lignes.Scan(&c) == nil {
			res = append(res, c)
		}
	}
	return res
}

func rapport(args []string) error {
	fs := flag.NewFlagSet("rapport", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	coffre := fs.String("coffre", `C:\dev\Obsidian Vault`, "racine du coffre Obsidian")
	sortie := fs.Bool("stdout", false, "afficher le rapport au lieu de l'écrire dans le coffre")
	fs.Parse(args)
	base, err := db.Ouvrir(*chemin)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrer(base); err != nil {
		return err
	}
	contenu, err := report.Generer(context.Background(), gen.New(base), time.Now().Format("2006-01-02"))
	if err != nil {
		return err
	}
	if *sortie {
		_, err = os.Stdout.Write(contenu)
		return err
	}
	if err := report.Ecrire(*coffre, contenu); err != nil {
		return err
	}
	fmt.Println("Statistiques.md écrit dans", *coffre)
	return nil
}
