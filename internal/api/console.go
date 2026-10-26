package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxLignesConsole    = 1000
	maxOctetsConsole    = 16 << 20 // budget du résultat avant encodage JSON
	requetesSimultanees = 2
)

// delaiConsole borne chaque requête de la console (variable pour les tests).
var delaiConsole = 5 * time.Second

// executerSQL exécute une requête de la console sur une connexion en lecture seule protégée par
// autoriserConsole (garde_sql.go) : mode=ro empêche les écritures sur le fichier, l'authorizer refuse
// ATTACH, les PRAGMA d'écriture et les transactions, et le délai couvre aussi le parcours des lignes.
func (s *Serveur) executerSQL(w http.ResponseWriter, r *http.Request) {
	var d DemandeSQL
	if !lireJSON(w, r, &d) {
		return
	}
	requete := strings.TrimSpace(d.SQL)
	if requete == "" {
		erreur(w, http.StatusBadRequest, "requête vide")
		return
	}
	select {
	case s.console <- struct{}{}:
		defer func() { <-s.console }()
	default:
		erreur(w, http.StatusTooManyRequests, "une autre requête de la console est en cours")
		return
	}

	ctx, fin := context.WithTimeout(r.Context(), delaiConsole)
	defer fin()
	conn, err := s.cfg.BaseLecture.Conn(ctx)
	if err != nil {
		erreur(w, http.StatusServiceUnavailable, messageSQL(ctx, err))
		return
	}
	defer conn.Close()
	arreter, err := protegerConnexion(ctx, conn)
	if err != nil {
		erreur(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer arreter()

	debut := time.Now()
	lignes, err := conn.QueryContext(ctx, requete)
	if err != nil {
		erreur(w, statutSQL(err), messageSQL(ctx, err))
		return
	}
	defer lignes.Close()

	colonnes, err := lignes.Columns()
	if err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	res := ResultatSQL{Colonnes: colonnes, Lignes: [][]any{}}
	octets := 0
	for lignes.Next() {
		if len(res.Lignes) == maxLignesConsole || octets > maxOctetsConsole {
			res.Tronque = true
			break
		}
		vals := make([]any, len(colonnes))
		ptrs := make([]any, len(colonnes))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := lignes.Scan(ptrs...); err != nil {
			erreur(w, http.StatusBadRequest, err.Error())
			return
		}
		for i, v := range vals {
			octets += 8
			if b, ok := v.([]byte); ok {
				if utf8.Valid(b) {
					vals[i] = string(b)
					// Un caractère de contrôle devient « \u0000 » en JSON : six octets.
					octets += 6 * len(b)
				} else {
					vals[i] = fmt.Sprintf("<blob %d octets>", len(b))
				}
			} else if t, ok := v.(string); ok {
				octets += 6 * len(t)
			}
		}
		res.Lignes = append(res.Lignes, vals)
	}
	if err := lignes.Err(); err != nil {
		erreur(w, statutSQL(err), messageSQL(ctx, err))
		return
	}
	res.DureeMs = float64(time.Since(debut).Microseconds()) / 1000
	ecrireJSON(w, res)
}

func statutSQL(err error) int {
	if strings.Contains(err.Error(), "not authorized") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

func messageSQL(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Sprintf("requête interrompue après %s", delaiConsole)
	}
	if strings.Contains(err.Error(), "not authorized") {
		return "commande interdite dans la console : lecture seule (pas d'ATTACH, de PRAGMA d'écriture ni de transaction)"
	}
	return err.Error()
}

func (s *Serveur) schema(w http.ResponseWriter, r *http.Request) {
	tables, err := s.cfg.BaseLecture.QueryContext(r.Context(),
		`SELECT name FROM sqlite_schema
		 WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'goose_%'
		   AND name NOT LIKE 'recherche_%'
		 ORDER BY name`)
	if err != nil {
		erreur(w, http.StatusInternalServerError, err.Error())
		return
	}
	var noms []string
	for tables.Next() {
		var n string
		if err := tables.Scan(&n); err != nil {
			tables.Close()
			erreur(w, http.StatusInternalServerError, err.Error())
			return
		}
		noms = append(noms, n)
	}
	tables.Close()

	res := []TableSchema{}
	for _, n := range noms {
		t := TableSchema{Table: n, Colonnes: []ColonneSchema{}}
		cols, err := s.cfg.BaseLecture.QueryContext(r.Context(), `SELECT name, type FROM pragma_table_xinfo(?)`, n)
		if err != nil {
			erreur(w, http.StatusInternalServerError, err.Error())
			return
		}
		for cols.Next() {
			var c ColonneSchema
			if err := cols.Scan(&c.Nom, &c.Type); err != nil {
				cols.Close()
				erreur(w, http.StatusInternalServerError, err.Error())
				return
			}
			t.Colonnes = append(t.Colonnes, c)
		}
		cols.Close()
		res = append(res, t)
	}
	ecrireJSON(w, res)
}

var nomRequete = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

func (s *Serveur) listerRequetes(w http.ResponseWriter, r *http.Request) {
	res := []RequeteEnregistree{}
	entrees, err := os.ReadDir(s.cfg.Requetes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		erreur(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, e := range entrees {
		nom, ok := strings.CutSuffix(e.Name(), ".sql")
		if e.IsDir() || !ok || !nomRequete.MatchString(nom) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.cfg.Requetes, e.Name()))
		if err != nil {
			continue
		}
		res = append(res, RequeteEnregistree{Nom: nom, SQL: string(b)})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Nom < res[j].Nom })
	ecrireJSON(w, res)
}

func (s *Serveur) enregistrerRequete(w http.ResponseWriter, r *http.Request) {
	var q RequeteEnregistree
	if !lireJSON(w, r, &q) {
		return
	}
	// Le nom est la seule partie du chemin : la regex interdit « .. », « / » et « \ ».
	if !nomRequete.MatchString(q.Nom) {
		erreur(w, http.StatusBadRequest, "nom invalide : lettres minuscules, chiffres et tirets uniquement")
		return
	}
	if strings.TrimSpace(q.SQL) == "" {
		erreur(w, http.StatusBadRequest, "requête vide")
		return
	}
	if err := os.MkdirAll(s.cfg.Requetes, 0o755); err != nil {
		erreur(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(s.cfg.Requetes, q.Nom+".sql"), []byte(q.SQL), 0o644); err != nil {
		erreur(w, http.StatusInternalServerError, err.Error())
		return
	}
	ecrireJSON(w, q)
}
