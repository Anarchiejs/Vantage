// Commande observatoire : tableau de bord local du travail de développement.
package main

import (
	"context"
	"flag"
	"fmt"
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
	"github.com/Anarchiejs/Observatoire/web"
)

const usage = `usage : observatoire <commande> [options]

commandes :
  migrate   crée ou met à jour le schéma de la base
  import    importe les dépôts Git et le coffre
  serve     lance l'interface sur http://127.0.0.1
  rapport   écrit Statistiques.md dans le coffre
  watch     importe en continu à chaque changement
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
		err = fmt.Errorf("%s : pas encore implémenté", cmd)
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
	remplacements := map[string]string{}
	for _, paire := range strings.Split(*depots, ",") {
		if nom, dossier, ok := strings.Cut(strings.TrimSpace(paire), "="); ok {
			abs, err := filepath.Abs(dossier)
			if err != nil {
				return err
			}
			remplacements[nom] = abs
		}
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
	for _, a := range b.Avertissements {
		fmt.Fprintln(os.Stderr, "attention :", a)
	}
	fmt.Printf("%d dépôts, %d commits ajoutés ; notes : %d écrites, %d inchangées, %d supprimées ; %s\n",
		b.Depots, b.Commits, b.NotesEcrites, b.NotesInchangees, b.NotesSupprimees, b.Duree.Round(time.Millisecond))
	return nil
}

func servir(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	chemin := fs.String("db", "observatoire.db", "fichier de la base")
	port := fs.Int("port", 7878, "port sur 127.0.0.1")
	fs.Parse(args)

	base, err := db.Ouvrir(*chemin)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrer(base); err != nil {
		return err
	}
	lecture, err := db.OuvrirLectureSeule(*chemin)
	if err != nil {
		return err
	}
	defer lecture.Close()
	ui, err := web.Interface()
	if err != nil {
		return err
	}
	if ui == nil {
		fmt.Fprintln(os.Stderr, "interface absente : API seule (construire avec npm --prefix web run build)")
	}
	s, err := api.Nouveau(api.Config{
		Adresse:     fmt.Sprintf("127.0.0.1:%d", *port),
		Base:        base,
		BaseLecture: lecture,
		Interface:   ui,
		Requetes:    filepath.Join(filepath.Dir(*chemin), "requetes"),
	})
	if err != nil {
		return err
	}
	ctx, fin := signal.NotifyContext(context.Background(), os.Interrupt)
	defer fin()
	return s.Ecouter(ctx)
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
