package requetes_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

func TestRechercheFiltres(t *testing.T) {
	b := preparer(t)
	q := gen.New(b)
	ctx := context.Background()
	cas := []struct {
		nom  string
		p    gen.RechercherParams
		vide bool
	}{
		{"commit", gen.RechercherParams{Termes: "fixture", Source: "commit", Projet: "Atlas", Depuis: "2026-01-01", Jusqua: "2026-03-31", Limite: 3}, false},
		{"note", gen.RechercherParams{Termes: "Atlas", Source: "note", Projet: "Atlas", Dossier: "Projets", Limite: 20}, false},
		{"date", gen.RechercherParams{Termes: "Atlas", Source: "note", Depuis: "2030-01-01", Limite: 20}, true},
		{"dossier", gen.RechercherParams{Termes: "fixture", Source: "commit", Dossier: "Projets", Limite: 20}, true},
		{"projet", gen.RechercherParams{Termes: "echantillonnage", Projet: "Atlas", Limite: 20}, true},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			rows, err := q.Rechercher(ctx, c.p)
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) == 0) != c.vide {
				t.Fatalf("unexpected rows: %+v", rows)
			}
			if int64(len(rows)) > c.p.Limite {
				t.Fatal("limit ignored")
			}
			for _, r := range rows {
				if c.p.Source != nil && r.Source != c.p.Source {
					t.Fatalf("source: %+v", r)
				}
				if c.p.Projet != nil && (r.Projet == nil || *r.Projet != c.p.Projet) {
					t.Fatalf("project: %+v", r)
				}
				if !strings.Contains(r.Titre, "<mark>") && !strings.Contains(r.Extrait, "<mark>") {
					t.Fatalf("highlight: %+v", r)
				}
			}
		})
	}
}

func TestCheminProfondeurEtCycle(t *testing.T) {
	b := preparer(t)
	ctx := context.Background()
	q := gen.New(b)
	var ids []int64
	for i := 0; i < 8; i++ {
		res, err := b.Exec("INSERT INTO note(chemin,dossier,titre,contenu_hash) VALUES (?,'Concepts',?,'test')", fmt.Sprintf("chaine/%d.md", i), fmt.Sprintf("chaine-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		n, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n)
		if i > 0 {
			if _, err := b.Exec("INSERT INTO lien(source_id,cible_titre,cible_id) VALUES (?,?,?)", ids[i-1], fmt.Sprintf("chaine-%d", i), n); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A backward edge creates a cycle and a duplicate undirected connection.
	if _, err := b.Exec("INSERT INTO lien(source_id,cible_titre,cible_id) VALUES (?,'cycle',?)", ids[1], ids[0]); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		cible  int
		taille int
	}{{6, 7}, {7, 0}} {
		rows, err := q.PlusCourtChemin(ctx, gen.PlusCourtCheminParams{Depart: ids[0], Arrivee: ids[c.cible]})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != c.taille {
			t.Fatalf("target %d: %+v", c.cible, rows)
		}
		for i, r := range rows {
			if r.ID != ids[i] {
				t.Fatalf("path order: %+v", rows)
			}
		}
	}
}

func TestFichePrefixeConnu(t *testing.T) {
	b := preparer(t)
	q := gen.New(b)
	ctx := context.Background()
	a := id(t, b, "projet", "nom", "Atlas")
	var sha string
	if err := b.QueryRow("SELECT c.sha FROM depot d JOIN commit_git c ON c.depot_id=d.id WHERE d.projet_id=? ORDER BY c.date_utc,c.sha LIMIT 1", a).Scan(&sha); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Exec("UPDATE projet SET verifie_commit=? WHERE id=?", sha[:10], a); err != nil {
		t.Fatal(err)
	}
	rows, err := q.FichesPerimees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Nom != "Atlas" || rows[0].CommitsDepuis != 20 || rows[1].CommitsDepuis != -1 {
		t.Fatalf("stale: %+v", rows)
	}
	if _, err := b.Exec("UPDATE projet SET verifie_commit=(SELECT substr(dernier_sha_importe,1,10) FROM depot WHERE projet_id=?) WHERE id=?", a, a); err != nil {
		t.Fatal(err)
	}
	rows, err = q.FichesPerimees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Nom != "Boussole" {
		t.Fatalf("current prefix: %+v", rows)
	}
}
