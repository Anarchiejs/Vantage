// Commande observatoire : tableau de bord local du travail de développement.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
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
	case "serve", "rapport", "watch":
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
	fs.Parse(args)
	base, err := db.Ouvrir(*chemin)
	if err != nil {
		return err
	}
	defer base.Close()
	if err := db.Migrer(base); err != nil {
		return err
	}
	b, err := ingest.Importer(context.Background(), base, ingest.Options{Coffre: *coffre})
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
