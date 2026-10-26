package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/db"
	"github.com/Anarchiejs/Observatoire/internal/ingest"
)

func racine() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// serveurFixtures : base remplie par l'import des fixtures, « aujourd'hui » figé au 2026-03-31.
func serveurFixtures(t *testing.T) http.Handler {
	t.Helper()
	r := racine()
	if _, err := os.Stat(filepath.Join(r, "testdata", "depots", "alpha", ".git")); err != nil {
		t.Fatal("fixtures absentes : lancer d'abord go run ./scripts/fixtures/make-repos.go")
	}
	chemin := filepath.Join(t.TempDir(), "test.db")
	rw, err := db.Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rw.Close() })
	if err := db.Migrer(rw); err != nil {
		t.Fatal(err)
	}
	b, err := ingest.Importer(context.Background(), rw, ingest.Options{
		Coffre:     filepath.Join(r, "testdata", "coffre"),
		Depots:     map[string]string{"Atlas": filepath.Join(r, "testdata", "depots", "alpha"), "Boussole": filepath.Join(r, "testdata", "depots", "beta")},
		Aujourdhui: "2026-03-31",
	})
	if err != nil || len(b.Avertissements) > 0 {
		t.Fatalf("import : %v %v", err, b.Avertissements)
	}
	ro, err := db.OuvrirLectureSeule(chemin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ro.Close() })
	s, err := Nouveau(Config{Adresse: "127.0.0.1:0", Base: rw, BaseLecture: ro, Aujourdhui: func() string { return "2026-03-31" }})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

// lire appelle la route et décode strictement : un champ hors contrat fait échouer le test.
func lire[T any](t *testing.T, h http.Handler, url string, statut int) T {
	t.Helper()
	w := appeler(h, "GET", url, "127.0.0.1", "")
	if w.Code != statut {
		t.Fatalf("%s : %d %s", url, w.Code, w.Body)
	}
	var v T
	dec := json.NewDecoder(w.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("%s : %v", url, err)
	}
	return v
}

func pasDeListeNulle(t *testing.T, url string, v any) {
	t.Helper()
	var visiter func(chemin string, x reflect.Value)
	visiter = func(chemin string, x reflect.Value) {
		switch x.Kind() {
		case reflect.Pointer:
			if !x.IsNil() {
				visiter(chemin, x.Elem())
			}
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				visiter(chemin+"."+x.Type().Field(i).Name, x.Field(i))
			}
		case reflect.Slice:
			if x.IsNil() {
				t.Errorf("%s : liste nulle %s (doit être [])", url, chemin)
			}
			for i := 0; i < x.Len(); i++ {
				visiter(chemin, x.Index(i))
			}
		}
	}
	visiter("", reflect.ValueOf(v))
}

func TestVues(t *testing.T) {
	h := serveurFixtures(t)

	acc := lire[Accueil](t, h, "/api/accueil", 200)
	pasDeListeNulle(t, "accueil", acc)
	if len(acc.Calendrier) != 365 || acc.Calendrier[364].Jour != "2026-03-31" {
		t.Fatalf("calendrier : %d jours, dernier %v", len(acc.Calendrier), acc.Calendrier[len(acc.Calendrier)-1])
	}
	var total int64
	for _, j := range acc.Calendrier {
		total += j.Commits
	}
	if total != 42 {
		t.Errorf("calendrier : %d commits, 42 attendus", total)
	}
	if acc.Mois.Commits != 12 { // 6 commits en mars par dépôt (voir internal/sql/requetes/README.md)
		t.Errorf("mois : %d commits", acc.Mois.Commits)
	}

	projets := lire[[]ProjetResume](t, h, "/api/projets", 200)
	if len(projets) < 2 {
		t.Fatalf("projets : %+v", projets)
	}
	var atlas int64
	for _, p := range projets {
		if p.Nom == "Atlas" {
			atlas = p.ID
		}
	}
	det := lire[ProjetDetail](t, h, "/api/projets/"+itoa(atlas), 200)
	pasDeListeNulle(t, "projet", det)
	if det.Projet.Commits != 21 || len(det.Volume) != 52 || len(det.Langages) == 0 || len(det.Projet.Technos) == 0 {
		t.Errorf("projet Atlas : commits=%d semaines=%d langages=%d technos=%v", det.Projet.Commits, len(det.Volume), len(det.Langages), det.Projet.Technos)
	}
	if det.Sante.DernierSHA == nil || det.Sante.AJour {
		t.Errorf("santé Atlas (verifie_commit fictif a1b2c3d) : %+v", det.Sante)
	}
	lire[Erreur](t, h, "/api/projets/999999", 404)
	lire[Erreur](t, h, "/api/projets/abc", 400)

	tech := lire[Technos](t, h, "/api/technos", 200)
	pasDeListeNulle(t, "technos", tech)
	noms := map[string]bool{}
	for _, n := range tech.Sankey.Noeuds {
		if noms[n.Nom] {
			t.Errorf("nœud Sankey en double : %s", n.Nom)
		}
		noms[n.Nom] = true
	}
	for _, l := range tech.Sankey.Liens {
		if !noms[l.Source] || !noms[l.Cible] {
			t.Errorf("lien Sankey vers un nœud absent : %+v", l)
		}
	}
	ias := lire[[]IA](t, h, "/api/ia", 200)
	pasDeListeNulle(t, "ia", ias)

	g := lire[Graphe](t, h, "/api/graphe", 200)
	pasDeListeNulle(t, "graphe", g)
	if len(g.Noeuds) == 0 || len(g.Liens) == 0 || len(g.Casses) != 3 {
		t.Errorf("graphe : %d nœuds, %d liens, %d cassés", len(g.Noeuds), len(g.Liens), len(g.Casses))
	}
	n := lire[NoteDetail](t, h, "/api/notes/"+itoa(g.Noeuds[0].ID), 200)
	pasDeListeNulle(t, "note", n)
	ch := lire[Chemin](t, h, "/api/graphe/chemin?de="+itoa(g.Liens[0].Source)+"&a="+itoa(g.Liens[0].Cible), 200)
	if len(ch.Chemin) != 2 {
		t.Errorf("chemin direct : %+v", ch)
	}
	lire[Erreur](t, h, "/api/graphe/chemin?de=x", 400)

	rech := lire[[]ResultatRecherche](t, h, "/api/recherche?q=echantillonnage", 200)
	if len(rech) == 0 || !strings.Contains(rech[0].Titre+rech[0].Extrait, "<mark>") {
		t.Errorf("recherche : %+v", rech)
	}
	if v := lire[[]ResultatRecherche](t, h, "/api/recherche?q=", 200); len(v) != 0 {
		t.Errorf("recherche vide : %+v", v)
	}
	lire[Erreur](t, h, `/api/recherche?q=%22non+ferme`, 400)
	lire[[]ResultatRecherche](t, h, "/api/recherche?q=fixture&source=commit&limite=5", 200)
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// TestContratMiroir vérifie que chaque interface de web/src/api/types.ts a exactement les champs JSON
// du struct Go du même nom (contrat.go).
func TestContratMiroir(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(racine(), "web", "src", "api", "types.ts"))
	if err != nil {
		t.Fatal(err)
	}
	blocs := regexp.MustCompile(`(?s)export interface (\w+)(?: extends [^{]+)? \{(.*?)\n\}`).FindAllStringSubmatch(string(src), -1)
	champTS := regexp.MustCompile(`(?m)^  (\w+)\??:`)
	types := map[string]reflect.Type{}
	for _, v := range []any{Erreur{}, JourActivite{}, Chiffres{}, ProjetActif{}, ProjetDormant{}, FichePerimee{}, Accueil{},
		ProjetResume{}, EvenementFrise{}, VolumeSemaine{}, PartLangage{}, PointLangage{}, PointChaud{}, Sante{}, ActiviteIA{},
		ProjetDetail{}, NoeudSankey{}, LienSankey{}, TendanceTechno{}, Technos{}, IA{}, NoeudGraphe{}, Graphe{}, Chemin{},
		NoteDetail{}, ResultatRecherche{}, DemandeSQL{}, ResultatSQL{}, TableSchema{}, RequeteEnregistree{}, EvenementMaj{}} {
		types[reflect.TypeOf(v).Name()] = reflect.TypeOf(v)
	}
	if len(blocs) < 25 {
		t.Fatalf("seulement %d interfaces lues dans types.ts", len(blocs))
	}
	for _, b := range blocs {
		typ, ok := types[b[1]]
		if !ok {
			t.Errorf("interface TS %s sans struct Go", b[1])
			continue
		}
		var ts []string
		for _, m := range champTS.FindAllStringSubmatch(b[2], -1) {
			ts = append(ts, m[1])
		}
		var goChamps []string
		for i := 0; i < typ.NumField(); i++ {
			if tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
				goChamps = append(goChamps, tag)
			}
		}
		sort.Strings(ts)
		sort.Strings(goChamps)
		if !reflect.DeepEqual(ts, goChamps) {
			t.Errorf("%s : TS %v ≠ Go %v", b[1], ts, goChamps)
		}
	}
}
