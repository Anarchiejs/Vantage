package requetes_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

var maj = flag.Bool("maj", false, "Rewrite reviewed JSON expectations and EXPLAIN plans")

const aujourd = "2026-03-31"

func racine() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(f), "../../.."))
}

func preparer(t *testing.T) *sql.DB {
	t.Helper()
	r := racine()
	if _, err := os.Stat(filepath.Join(r, "testdata/depots/alpha/.git")); err != nil {
		t.Fatal("run go run ./scripts/fixtures/make-repos.go first:", err)
	}
	b, err := db.Ouvrir(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if err = db.Migrer(b); err != nil {
		t.Fatal(err)
	}
	bilan, err := ingest.Importer(context.Background(), b, ingest.Options{
		Coffre:     filepath.Join(r, "testdata/coffre"),
		Depots:     map[string]string{"Atlas": filepath.Join(r, "testdata/depots/alpha"), "Boussole": filepath.Join(r, "testdata/depots/beta")},
		Aujourdhui: aujourd,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bilan.Commits != 42 || len(bilan.Avertissements) > 0 {
		t.Fatalf("import: %+v", bilan)
	}
	return b
}

func id(t *testing.T, b *sql.DB, table, champ, valeur string) int64 {
	t.Helper()
	var n int64
	if err := b.QueryRow("SELECT id FROM "+table+" WHERE "+champ+"=?", valeur).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Capture the exact generated SQL and bindings, rather than maintaining a second SQL copy.
type observation struct {
	*sql.DB
	texte string
	args  []any
}

func (o *observation) QueryContext(ctx context.Context, s string, args ...any) (*sql.Rows, error) {
	o.texte = s
	o.args = args
	return o.DB.QueryContext(ctx, s, args...)
}
func (o *observation) QueryRowContext(ctx context.Context, s string, args ...any) *sql.Row {
	o.texte = s
	o.args = args
	return o.DB.QueryRowContext(ctx, s, args...)
}

func plan(t *testing.T, o *observation) string {
	t.Helper()
	rows, err := o.DB.Query("EXPLAIN QUERY PLAN "+o.texte, o.args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var a, b, c int
		var d string
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			t.Fatal(err)
		}
		details = append(details, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(details, "; ")
}

func TestAttendus(t *testing.T) {
	b := preparer(t)
	ctx := context.Background()
	atlas := id(t, b, "projet", "nom", "Atlas")
	note := id(t, b, "note", "titre", "Atlas")
	concept := id(t, b, "note", "titre", "Cartographie")
	o := &observation{DB: b}
	q := gen.New(o)
	cas := []struct {
		nom string
		arg any
	}{
		{"Calendrier", aujourd}, {"ChiffresPeriode", gen.ChiffresPeriodeParams{Debut: "2026-01-01", Fin: aujourd}},
		{"ProjetsActifs", aujourd}, {"ProjetsDormants", aujourd}, {"FichesPerimees", nil},
		{"ListerProjets", nil}, {"ProjetParID", atlas}, {"TechnosDuProjet", atlas}, {"Frise", atlas},
		{"VolumeHebdo", gen.VolumeHebdoParams{ProjetID: atlas, Aujourdhui: aujourd}},
		{"LangagesDerniereMesure", atlas}, {"LangagesEvolution", atlas}, {"PointsChauds", atlas}, {"IAduProjet", &atlas}, {"NotesDuProjet", atlas},
		{"SankeyLiens", nil}, {"TendanceTechnos", aujourd}, {"TechnosSansProjet", nil}, {"ActiviteIA", nil}, {"ProjetsParIA", nil}, {"SessionsIAParMois", aujourd},
		{"NoeudsGraphe", nil}, {"LiensGraphe", nil}, {"LiensCasses", nil}, {"NoteParID", note}, {"Voisins", note},
		{"PlusCourtChemin", gen.PlusCourtCheminParams{Depart: note, Arrivee: concept}},
		{"Rechercher", gen.RechercherParams{Termes: "echantillonnage", Limite: 20}},
	}
	var plans []string
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			args := []reflect.Value{reflect.ValueOf(ctx)}
			if c.arg != nil {
				args = append(args, reflect.ValueOf(c.arg))
			}
			ret := reflect.ValueOf(q).MethodByName(c.nom).Call(args)
			if !ret[1].IsNil() {
				t.Fatal(ret[1].Interface())
			}
			v := ret[0]
			if v.Kind() == reflect.Slice && v.IsNil() {
				v = reflect.MakeSlice(v.Type(), 0, 0)
			}
			data, err := json.MarshalIndent(v.Interface(), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
			path := filepath.Join("attendu", c.nom+".json")
			if *maj {
				if err := os.MkdirAll("attendu", 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			attendu, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(attendu) != string(data) {
				t.Fatalf("result differs from %s (review before -maj)", path)
			}
			plans = append(plans, "- "+c.nom+": "+plan(t, o))
		})
	}
	if *maj && !t.Failed() {
		texte := "# EXPLAIN QUERY PLAN\n\nCaptured on the migrated fixture database, without proposed indexes.\nFull scans for whole-graph/history aggregates are intentional; see INDEX_PROPOSES.md for selective scans.\n\n" + strings.Join(plans, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(racine(), "internal/sql/requetes/PLANS.md"), []byte(texte), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInvariants(t *testing.T) {
	b := preparer(t)
	q := gen.New(b)
	ctx := context.Background()
	calendrier, err := q.Calendrier(ctx, aujourd)
	if err != nil {
		t.Fatal(err)
	}
	if len(calendrier) != 365 || calendrier[0].Jour != "2025-04-01" || calendrier[364].Jour != aujourd {
		t.Fatalf("calendar bounds: %+v", calendrier)
	}
	var total, vides int64
	for _, j := range calendrier {
		total += j.Commits
		if j.Commits == 0 {
			vides++
		}
	}
	// Each fixture has 20 changes plus one extra merge commit: 21 * 2.
	if total != 42 || vides == 0 {
		t.Fatalf("calendar commits=%d empty=%d", total, vides)
	}
	chiffres, err := q.ChiffresPeriode(ctx, gen.ChiffresPeriodeParams{Debut: "2026-01-01", Fin: aujourd})
	if err != nil {
		t.Fatal(err)
	}
	if chiffres.Commits != 42 || chiffres.ProjetsActifs != 2 || chiffres.Sessions != 0 {
		t.Fatalf("period: %+v", chiffres)
	}
	// Changes 15..20 fall on March 5,8,11,14,17,20 in each repository: 12.
	actifs, err := q.ProjetsActifs(ctx, aujourd)
	if err != nil {
		t.Fatal(err)
	}
	if len(actifs) != 2 || actifs[0].Commits30j != 6 || actifs[1].Commits30j != 6 {
		t.Fatalf("active: %+v", actifs)
	}
	a := id(t, b, "projet", "nom", "Atlas")
	// Fixtures say actif/archive, so add test-only records for the specified 'en cours' status.
	if _, err := b.Exec("UPDATE projet SET statut='en cours' WHERE id=?", a); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Exec("INSERT INTO projet(nom,statut) VALUES ('Jamais','en cours')"); err != nil {
		t.Fatal(err)
	}
	dormants, err := q.ProjetsDormants(ctx, "2026-05-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(dormants) != 2 || dormants[0].Nom != "Atlas" || dormants[1].Jours != -1 {
		t.Fatalf("dormants: %+v", dormants)
	}
	depart := id(t, b, "note", "titre", "Atlas")
	arrivee := id(t, b, "note", "titre", "Cartographie")
	orpheline := id(t, b, "note", "titre", "Échantillonnage")
	for _, pair := range [][2]int64{{depart, arrivee}, {arrivee, depart}, {depart, depart}, {depart, orpheline}} {
		chemin, err := q.PlusCourtChemin(ctx, gen.PlusCourtCheminParams{Depart: pair[0], Arrivee: pair[1]})
		if err != nil {
			t.Fatal(err)
		}
		attendu := 2
		if pair[0] == pair[1] {
			attendu = 1
		}
		if pair[1] == orpheline {
			attendu = 0
		}
		if len(chemin) != attendu || (attendu > 0 && (chemin[0].ID != pair[0] || chemin[attendu-1].ID != pair[1])) {
			t.Fatalf("path %v: %+v", pair, chemin)
		}
	}
	resultats, err := q.Rechercher(ctx, gen.RechercherParams{Termes: "echantillonnage", Limite: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(resultats) != 1 || resultats[0].IDNote == nil || *resultats[0].IDNote != orpheline {
		t.Fatalf("accent search: %+v", resultats)
	}
	volume, err := q.VolumeHebdo(ctx, gen.VolumeHebdoParams{ProjetID: a, Aujourdhui: aujourd})
	if err != nil {
		t.Fatal(err)
	}
	if len(volume) != 52 || volume[51].Semaine != "2026-03-30" {
		t.Fatalf("weeks: %+v", volume)
	}
}

func TestSQLASCII(t *testing.T) {
	fichiers, err := filepath.Glob(filepath.Join(racine(), "internal/sql/requetes/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fichiers {
		contenu, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range contenu {
			if c > 127 {
				t.Error(fmt.Sprintf("%s: non-ASCII at byte %d", f, i))
			}
		}
	}
}
