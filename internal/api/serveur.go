package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

// Config du serveur.
type Config struct {
	Adresse     string  // « 127.0.0.1:7878 » ; toute autre interface que la boucle locale est refusée
	Base        *sql.DB // lecture-écriture, pour les vues
	BaseLecture *sql.DB // ouverte en mode=ro, pour la console SQL
	Interface   fs.FS   // fichiers de web/dist ; nil = pas d'interface (API seule)
	Requetes    string  // dossier des requêtes enregistrées (*.sql)
	Aujourdhui  func() string
}

// Serveur HTTP de l'Observatoire.
type Serveur struct {
	cfg  Config
	q    *gen.Queries
	diff *Diffuseur
	// console limite les requêtes SQL simultanées (jeton pris pendant l'exécution).
	console chan struct{}
}

// Nouveau prépare le serveur ; Diffuseur() permet d'annoncer les mises à jour.
func Nouveau(cfg Config) (*Serveur, error) {
	if cfg.Aujourdhui == nil {
		cfg.Aujourdhui = func() string { return time.Now().Format("2006-01-02") }
	}
	if err := verifierBoucleLocale(cfg.Adresse); err != nil {
		return nil, err
	}
	return &Serveur{cfg: cfg, q: gen.New(cfg.Base), diff: NouveauDiffuseur(),
		console: make(chan struct{}, requetesSimultanees)}, nil
}

// Diffuseur renvoie le canal SSE des mises à jour.
func (s *Serveur) Diffuseur() *Diffuseur { return s.diff }

func verifierBoucleLocale(adresse string) error {
	hote, _, err := net.SplitHostPort(adresse)
	if err != nil {
		return err
	}
	ip := net.ParseIP(hote)
	if hote == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("le serveur n'écoute que sur la boucle locale (127.0.0.1)")
}

// Handler renvoie toutes les routes, protégées par la vérification de l'en-tête Host.
func (s *Serveur) Handler() http.Handler {
	m := http.NewServeMux()
	s.routesVues(m)
	m.HandleFunc("POST /api/sql", s.executerSQL)
	m.HandleFunc("GET /api/schema", s.schema)
	m.HandleFunc("GET /api/requetes", s.listerRequetes)
	m.HandleFunc("POST /api/requetes", s.enregistrerRequete)
	m.HandleFunc("GET /api/evenements", s.diff.ServeHTTP)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		erreur(w, http.StatusNotFound, "route inconnue : "+r.URL.Path)
	})
	if s.cfg.Interface != nil {
		m.Handle("/", interfaceSPA(s.cfg.Interface))
	}
	return gardeLocale(m)
}

// Ecouter sert jusqu'à l'annulation du contexte.
func (s *Serveur) Ecouter(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Adresse,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		arret, fin := context.WithTimeout(context.Background(), 3*time.Second)
		defer fin()
		s.diff.Fermer()
		srv.Shutdown(arret)
	}()
	log.Printf("Observatoire sur http://%s", s.cfg.Adresse)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// gardeLocale refuse toute requête dont l'en-tête Host n'est pas la boucle locale : sans cela, une page web
// malveillante pourrait lire l'API par « DNS rebinding » malgré l'écoute sur 127.0.0.1.
// Les écritures (POST) exigent en plus un corps JSON, ce qui force un contrôle CORS préalable côté navigateur
// et bloque les formulaires d'un autre site.
func gardeLocale(suivant http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hote := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			hote = h
		}
		if hote != "127.0.0.1" && hote != "localhost" && hote != "[::1]" && hote != "::1" {
			erreur(w, http.StatusForbidden, "hôte refusé")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				erreur(w, http.StatusUnsupportedMediaType, "corps JSON attendu")
				return
			}
			if o := r.Header.Get("Origin"); o != "" && !origineLocale(o) {
				erreur(w, http.StatusForbidden, "origine refusée")
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		suivant.ServeHTTP(w, r)
	})
}

func origineLocale(o string) bool {
	for _, p := range []string{"http://127.0.0.1:", "http://localhost:", "http://[::1]:"} {
		if strings.HasPrefix(o, p) {
			return true
		}
	}
	return o == "http://127.0.0.1" || o == "http://localhost"
}

// interfaceSPA sert les fichiers de l'interface ; toute route inconnue renvoie index.html (routage côté client).
func interfaceSPA(f fs.FS) http.Handler {
	fichiers := http.FileServerFS(f)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chemin := strings.TrimPrefix(r.URL.Path, "/")
		if chemin != "" {
			if _, err := fs.Stat(f, chemin); err == nil {
				if strings.HasPrefix(chemin, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fichiers.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		http.ServeFileFS(w, r, f, "index.html")
	})
}

func ecrireJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encodage JSON : %v", err)
	}
}

func erreur(w http.ResponseWriter, statut int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	json.NewEncoder(w).Encode(Erreur{Erreur: message})
}

// lireJSON décode un corps limité à 1 Mo.
func lireJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	// Délai de lecture du corps seulement : un ReadTimeout global couperait aussi le flux SSE.
	http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Second))
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		erreur(w, http.StatusBadRequest, "JSON invalide : "+err.Error())
		return false
	}
	return true
}
