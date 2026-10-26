package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Anarchiejs/Observatoire/internal/db"
)

func serveurTest(t *testing.T) (*Serveur, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	chemin := filepath.Join(dir, "test.db")
	rw, err := db.Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rw.Close() })
	if err := db.Migrer(rw); err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`INSERT INTO projet (nom) VALUES ('Alpha'), ('Beta')`); err != nil {
		t.Fatal(err)
	}
	ro, err := db.OuvrirLectureSeule(chemin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ro.Close() })
	s, err := Nouveau(Config{
		Adresse: "127.0.0.1:0", Base: rw, BaseLecture: ro,
		Requetes:  filepath.Join(dir, "requetes"),
		Interface: fstest.MapFS{"index.html": {Data: []byte("<html>accueil</html>")}, "assets/a.js": {Data: []byte("x")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Handler()
}

func appeler(h http.Handler, methode, url, hote, corps string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(methode, url, strings.NewReader(corps))
	r.Host = hote
	if corps != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestBoucleLocaleSeulement(t *testing.T) {
	if _, err := Nouveau(Config{Adresse: "0.0.0.0:7878"}); err == nil {
		t.Fatal("écoute sur 0.0.0.0 acceptée")
	}
	if _, err := Nouveau(Config{Adresse: "192.168.1.10:7878"}); err == nil {
		t.Fatal("écoute sur le réseau local acceptée")
	}
}

func TestGardeHost(t *testing.T) {
	_, h := serveurTest(t)
	if w := appeler(h, "GET", "/api/schema", "evil.example:7878", ""); w.Code != http.StatusForbidden {
		t.Fatalf("Host étranger : %d", w.Code)
	}
	if w := appeler(h, "GET", "/api/schema", "127.0.0.1:7878", ""); w.Code != http.StatusOK {
		t.Fatalf("Host local : %d %s", w.Code, w.Body)
	}
	// POST sans JSON (formulaire d'un autre site) refusé.
	r := httptest.NewRequest("POST", "/api/sql", strings.NewReader("sql=select+1"))
	r.Host = "127.0.0.1:7878"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("POST formulaire : %d", w.Code)
	}
	// Origine étrangère refusée.
	r = httptest.NewRequest("POST", "/api/sql", strings.NewReader(`{"sql":"select 1"}`))
	r.Host = "127.0.0.1:7878"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Origin étrangère : %d", w.Code)
	}
}

func TestConsoleLectureSeule(t *testing.T) {
	_, h := serveurTest(t)
	w := appeler(h, "POST", "/api/sql", "127.0.0.1", `{"sql":"SELECT nom FROM projet ORDER BY nom"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var res ResultatSQL
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Lignes) != 2 || res.Lignes[0][0] != "Alpha" || res.Colonnes[0] != "nom" {
		t.Fatalf("résultat : %+v", res)
	}

	for _, sql := range []string{
		`DELETE FROM projet`,
		`INSERT INTO projet (nom) VALUES ('x')`,
		`DROP TABLE projet`,
		`SELECT 1; DELETE FROM projet`,
		`WITH x AS (SELECT 1) DELETE FROM projet`,
		`ATTACH DATABASE 'autre.db' AS a`,
		`PRAGMA query_only = 0`,
		`PRAGMA query_only(0)`,
	} {
		corps, _ := json.Marshal(DemandeSQL{SQL: sql})
		if w := appeler(h, "POST", "/api/sql", "127.0.0.1", string(corps)); w.Code == http.StatusOK {
			t.Errorf("accepté : %s", sql)
		}
	}
	w = appeler(h, "POST", "/api/sql", "127.0.0.1", `{"sql":"SELECT count(*) FROM projet"}`)
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Lignes[0][0] != float64(2) {
		t.Fatalf("la base a changé : %v", res.Lignes)
	}
}

func TestConsoleTronque(t *testing.T) {
	_, h := serveurTest(t)
	w := appeler(h, "POST", "/api/sql", "127.0.0.1",
		`{"sql":"WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 5000) SELECT i FROM n"}`)
	var res ResultatSQL
	json.Unmarshal(w.Body.Bytes(), &res)
	if !res.Tronque || len(res.Lignes) != maxLignesConsole {
		t.Fatalf("tronque=%v lignes=%d", res.Tronque, len(res.Lignes))
	}
}

func TestRequetesEnregistrees(t *testing.T) {
	s, h := serveurTest(t)
	for _, nom := range []string{"../evil", "a/b", `a\b`, "Majuscule", "", strings.Repeat("a", 65)} {
		corps, _ := json.Marshal(RequeteEnregistree{Nom: nom, SQL: "select 1"})
		if w := appeler(h, "POST", "/api/requetes", "127.0.0.1", string(corps)); w.Code != http.StatusBadRequest {
			t.Errorf("nom %q accepté (%d)", nom, w.Code)
		}
	}
	if w := appeler(h, "POST", "/api/requetes", "127.0.0.1", `{"nom":"top-projets","sql":"select 1"}`); w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(s.cfg.Requetes, "top-projets.sql")); err != nil {
		t.Fatal(err)
	}
	w := appeler(h, "GET", "/api/requetes", "127.0.0.1", "")
	var liste []RequeteEnregistree
	json.Unmarshal(w.Body.Bytes(), &liste)
	if len(liste) != 1 || liste[0].Nom != "top-projets" {
		t.Fatalf("%+v", liste)
	}
}

func TestInterfaceSPA(t *testing.T) {
	_, h := serveurTest(t)
	if w := appeler(h, "GET", "/projets/3", "127.0.0.1", ""); !strings.Contains(w.Body.String(), "accueil") {
		t.Fatalf("route client : %d %s", w.Code, w.Body)
	}
	if w := appeler(h, "GET", "/api/inconnue", "127.0.0.1", ""); w.Code != http.StatusNotFound {
		t.Fatalf("API inconnue : %d", w.Code)
	}
}
