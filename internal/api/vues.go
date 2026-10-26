package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

// routesVues branche les vues de l'interface sur les requêtes de internal/sql/requetes.
func (s *Serveur) routesVues(m *http.ServeMux) {
	m.HandleFunc("GET /api/accueil", s.accueil)
	m.HandleFunc("GET /api/projets", s.projets)
	m.HandleFunc("GET /api/projets/{id}", s.projet)
	m.HandleFunc("GET /api/technos", s.technos)
	m.HandleFunc("GET /api/ia", s.ia)
	m.HandleFunc("GET /api/graphe", s.graphe)
	m.HandleFunc("GET /api/graphe/chemin", s.chemin)
	m.HandleFunc("GET /api/notes/{id}", s.note)
	m.HandleFunc("GET /api/recherche", s.recherche)
}

// echec répond 500 (ou 404 pour une ligne absente) et indique si le handler doit s'arrêter.
func echec(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		erreur(w, http.StatusNotFound, "introuvable")
	} else {
		erreur(w, http.StatusInternalServerError, err.Error())
	}
	return true
}

func idChemin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		erreur(w, http.StatusBadRequest, "identifiant invalide")
		return 0, false
	}
	return id, true
}

// texte convertit les valeurs calculées que sqlc type en interface{} (texte ou NULL).
func texte(v any) *string {
	switch x := v.(type) {
	case string:
		return &x
	case []byte:
		s := string(x)
		return &s
	case nil:
		return nil
	default:
		s := fmt.Sprint(x)
		return &s
	}
}

func texteOuVide(v any) string {
	if p := texte(v); p != nil {
		return *p
	}
	return ""
}

func decaler(jour string, jours int) string {
	t, err := time.Parse("2006-01-02", jour)
	if err != nil {
		return jour
	}
	return t.AddDate(0, 0, jours).Format("2006-01-02")
}

// ---- Accueil

func (s *Serveur) chiffres(r *http.Request, debut, fin string) (Chiffres, error) {
	c, err := s.q.ChiffresPeriode(r.Context(), gen.ChiffresPeriodeParams{Debut: debut, Fin: fin})
	return Chiffres{Commits: c.Commits, Ajouts: c.Ajouts, Suppressions: c.Suppressions, Sessions: c.Sessions, ProjetsActifs: c.ProjetsActifs}, err
}

func (s *Serveur) accueil(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auj := s.cfg.Aujourdhui()
	res := Accueil{Calendrier: []JourActivite{}, Actifs: []ProjetActif{}, Dormants: []ProjetDormant{}, FichesPerimees: []FichePerimee{}}

	cal, err := s.q.Calendrier(ctx, auj)
	if echec(w, err) {
		return
	}
	for _, j := range cal {
		projets := []string{}
		if j.Projets != "" {
			projets = strings.Split(j.Projets, "\x1f")
		}
		res.Calendrier = append(res.Calendrier, JourActivite{Jour: j.Jour, Commits: j.Commits, Ajouts: j.Ajouts, Suppressions: j.Suppressions, Sessions: j.Sessions, Projets: projets})
	}

	periodes := []struct {
		dest       *Chiffres
		debut, fin int
	}{
		{&res.Semaine, -6, 0}, {&res.SemainePrecedente, -13, -7}, {&res.Mois, -29, 0}, {&res.MoisPrecedent, -59, -30},
	}
	for _, p := range periodes {
		if *p.dest, err = s.chiffres(r, decaler(auj, p.debut), decaler(auj, p.fin)); echec(w, err) {
			return
		}
	}

	actifs, err := s.q.ProjetsActifs(ctx, auj)
	if echec(w, err) {
		return
	}
	for _, a := range actifs {
		res.Actifs = append(res.Actifs, ProjetActif{ID: a.ID, Nom: a.Nom, DernierCommit: texte(a.DernierCommit), Commits30j: a.Commits30j})
	}
	dormants, err := s.q.ProjetsDormants(ctx, auj)
	if echec(w, err) {
		return
	}
	for _, d := range dormants {
		res.Dormants = append(res.Dormants, ProjetDormant{ID: d.ID, Nom: d.Nom, DernierCommit: texte(d.DernierCommit), Jours: d.Jours})
	}
	perimees, err := s.q.FichesPerimees(ctx)
	if echec(w, err) {
		return
	}
	for _, f := range perimees {
		res.FichesPerimees = append(res.FichesPerimees, FichePerimee{ID: f.ID, Nom: f.Nom, VerifieCommit: f.VerifieCommit, DernierSHA: f.DernierSha, CommitsDepuis: f.CommitsDepuis})
	}
	ecrireJSON(w, res)
}

// ---- Projets

func (s *Serveur) projets(w http.ResponseWriter, r *http.Request) {
	lignes, err := s.q.ListerProjets(r.Context())
	if echec(w, err) {
		return
	}
	res := make([]ProjetResume, 0, len(lignes))
	for _, p := range lignes {
		res = append(res, ProjetResume{ID: p.ID, Nom: p.Nom, Statut: p.Statut, Commits: p.Commits, DernierCommit: texte(p.DernierCommit), LangagePrincipal: p.LangagePrincipal})
	}
	ecrireJSON(w, res)
}

func (s *Serveur) projet(w http.ResponseWriter, r *http.Request) {
	id, ok := idChemin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	p, err := s.q.ProjetParID(ctx, id)
	if echec(w, err) {
		return
	}
	res := ProjetDetail{Frise: []EvenementFrise{}, Volume: []VolumeSemaine{}, Langages: []PartLangage{}, LangagesEvolution: []PointLangage{}, PointsChauds: []PointChaud{}, IA: []ActiviteIA{}}

	// Résumé : réutilise la ligne de la liste (commits, dernier commit, langage principal).
	res.Projet = ProjetComplet{ProjetResume: ProjetResume{ID: p.ID, Nom: p.Nom, Statut: p.Statut}, CheminSource: p.CheminSource}
	liste, err := s.q.ListerProjets(ctx)
	if echec(w, err) {
		return
	}
	for _, l := range liste {
		if l.ID == id {
			res.Projet.Commits, res.Projet.DernierCommit, res.Projet.LangagePrincipal = l.Commits, texte(l.DernierCommit), l.LangagePrincipal
		}
	}
	if res.Projet.Technos, err = s.q.TechnosDuProjet(ctx, id); echec(w, err) {
		return
	}
	if res.Projet.Technos == nil {
		res.Projet.Technos = []string{}
	}

	frise, err := s.q.Frise(ctx, id)
	if echec(w, err) {
		return
	}
	for _, e := range frise {
		res.Frise = append(res.Frise, EvenementFrise{Date: e.Date, Type: e.Type, Titre: texteOuVide(e.Titre), Ref: e.Ref, IA: e.Ia})
	}
	vol, err := s.q.VolumeHebdo(ctx, gen.VolumeHebdoParams{ProjetID: id, Aujourdhui: s.cfg.Aujourdhui()})
	if echec(w, err) {
		return
	}
	for _, v := range vol {
		res.Volume = append(res.Volume, VolumeSemaine{Semaine: v.Semaine, Commits: v.Commits, Ajouts: v.Ajouts, Suppressions: v.Suppressions})
	}
	langs, err := s.q.LangagesDerniereMesure(ctx, id)
	if echec(w, err) {
		return
	}
	for _, l := range langs {
		res.Langages = append(res.Langages, PartLangage{Langage: l.Langage, LignesCode: l.LignesCode, Part: l.Part})
	}
	evo, err := s.q.LangagesEvolution(ctx, id)
	if echec(w, err) {
		return
	}
	for _, l := range evo {
		res.LangagesEvolution = append(res.LangagesEvolution, PointLangage{DateMesure: l.DateMesure, Langage: l.Langage, LignesCode: l.LignesCode})
	}
	chauds, err := s.q.PointsChauds(ctx, id)
	if echec(w, err) {
		return
	}
	for _, c := range chauds {
		res.PointsChauds = append(res.PointsChauds, PointChaud{Chemin: c.Chemin, Modifications: c.Modifications, Ajouts: c.Ajouts, Suppressions: c.Suppressions})
	}
	ias, err := s.q.IAduProjet(ctx, &id)
	if echec(w, err) {
		return
	}
	for _, a := range ias {
		if a.Ia == nil {
			continue
		}
		res.IA = append(res.IA, ActiviteIA{IA: *a.Ia, Sessions: a.Sessions, Premiere: texteOuVide(a.Premiere), Derniere: texteOuVide(a.Derniere)})
	}

	if res.Sante, err = s.sante(r, p); echec(w, err) {
		return
	}
	ecrireJSON(w, res)
}

func (s *Serveur) sante(r *http.Request, p gen.Projet) (Sante, error) {
	ctx := r.Context()
	st := Sante{VerifieCommit: p.VerifieCommit, VerifieLe: p.VerifieLe, AJour: true, NotesManquantes: []string{}}
	depots, err := s.q.DepotsDuProjet(ctx, p.ID)
	if err != nil {
		return st, err
	}
	if len(depots) > 0 {
		st.DernierSHA = depots[0].DernierShaImporte
	}
	perimees, err := s.q.FichesPerimees(ctx)
	if err != nil {
		return st, err
	}
	for _, f := range perimees {
		if f.ID == p.ID {
			st.AJour = false
			st.CommitsDepuis = max(st.CommitsDepuis, f.CommitsDepuis)
			if f.CommitsDepuis < 0 {
				st.CommitsDepuis = f.CommitsDepuis
			}
		}
	}
	presentes, err := s.q.NotesDuProjet(ctx, p.ID)
	if err != nil {
		return st, err
	}
	for _, attendue := range []string{p.Nom + " - Resume", p.Nom + " - Detaille"} {
		trouvee := false
		for _, n := range presentes {
			trouvee = trouvee || n == attendue
		}
		if !trouvee {
			st.NotesManquantes = append(st.NotesManquantes, attendue)
		}
	}
	return st, nil
}

// ---- Technos et IA

func (s *Serveur) technos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := Technos{Sankey: Sankey{Noeuds: []NoeudSankey{}, Liens: []LienSankey{}}, Tendance: []TendanceTechno{}}
	liens, err := s.q.SankeyLiens(ctx)
	if echec(w, err) {
		return
	}
	// ECharts exige des noms de nœuds uniques : une techno qui porte le nom d'un projet est suffixée.
	projets := map[string]bool{}
	for _, l := range liens {
		projets[l.Source] = true
	}
	vus := map[string]bool{}
	for _, l := range liens {
		cible := l.Cible
		if projets[cible] {
			cible += " (techno)"
		}
		for _, n := range []NoeudSankey{{l.Source, "projet"}, {cible, "techno"}} {
			if !vus[n.Nom] {
				vus[n.Nom] = true
				res.Sankey.Noeuds = append(res.Sankey.Noeuds, n)
			}
		}
		res.Sankey.Liens = append(res.Sankey.Liens, LienSankey{Source: l.Source, Cible: cible, Valeur: l.Valeur})
	}
	tend, err := s.q.TendanceTechnos(ctx, s.cfg.Aujourdhui())
	if echec(w, err) {
		return
	}
	for _, t := range tend {
		res.Tendance = append(res.Tendance, TendanceTechno{Mois: t.Mois, Techno: t.Techno, Commits: t.Commits})
	}
	if res.SansProjet, err = s.q.TechnosSansProjet(ctx); echec(w, err) {
		return
	}
	if res.SansProjet == nil {
		res.SansProjet = []string{}
	}
	ecrireJSON(w, res)
}

func (s *Serveur) ia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	act, err := s.q.ActiviteIA(ctx)
	if echec(w, err) {
		return
	}
	parIA := map[string]*IA{}
	var ordre []string
	for _, a := range act {
		if a.Ia == nil {
			continue
		}
		parIA[*a.Ia] = &IA{IA: *a.Ia, Sessions: a.Sessions, Projets: []string{}, ParMois: []SessionsMois{}}
		ordre = append(ordre, *a.Ia)
	}
	projets, err := s.q.ProjetsParIA(ctx)
	if echec(w, err) {
		return
	}
	for _, p := range projets {
		if p.Ia != nil && parIA[*p.Ia] != nil {
			parIA[*p.Ia].Projets = append(parIA[*p.Ia].Projets, p.Nom)
		}
	}
	mois, err := s.q.SessionsIAParMois(ctx, s.cfg.Aujourdhui())
	if echec(w, err) {
		return
	}
	for _, m := range mois {
		if m.Ia != nil && parIA[*m.Ia] != nil {
			parIA[*m.Ia].ParMois = append(parIA[*m.Ia].ParMois, SessionsMois{Mois: m.Mois, Sessions: m.Sessions})
		}
	}
	res := make([]IA, 0, len(ordre))
	for _, n := range ordre {
		res = append(res, *parIA[n])
	}
	ecrireJSON(w, res)
}

// ---- Graphe

func noeud(id int64, titre, dossier string, degre int64) NoeudGraphe {
	return NoeudGraphe{ID: id, Titre: titre, Dossier: dossier, Degre: degre}
}

func (s *Serveur) graphe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := Graphe{Noeuds: []NoeudGraphe{}, Liens: []LienGraphe{}, Orphelines: []NoeudGraphe{}, Centrales: []NoeudGraphe{}, Casses: []LienCasse{}}
	noeuds, err := s.q.NoeudsGraphe(ctx)
	if echec(w, err) {
		return
	}
	for _, n := range noeuds {
		g := noeud(n.ID, n.Titre, n.Dossier, n.Degre)
		res.Noeuds = append(res.Noeuds, g)
		if n.Degre == 0 {
			res.Orphelines = append(res.Orphelines, g)
		}
	}
	centrales := append([]NoeudGraphe(nil), res.Noeuds...)
	sort.SliceStable(centrales, func(i, j int) bool { return centrales[i].Degre > centrales[j].Degre })
	for _, n := range centrales[:min(10, len(centrales))] {
		if n.Degre > 0 {
			res.Centrales = append(res.Centrales, n)
		}
	}
	liens, err := s.q.LiensGraphe(ctx)
	if echec(w, err) {
		return
	}
	for _, l := range liens {
		if l.Cible != nil {
			res.Liens = append(res.Liens, LienGraphe{Source: l.Source, Cible: *l.Cible})
		}
	}
	casses, err := s.q.LiensCasses(ctx)
	if echec(w, err) {
		return
	}
	for _, c := range casses {
		res.Casses = append(res.Casses, LienCasse{Source: c.Source, SourceTitre: c.SourceTitre, CibleTitre: c.CibleTitre})
	}
	ecrireJSON(w, res)
}

// chemin cherche le plus court chemin par un parcours en largeur en Go : linéaire en notes et liens,
// là où une récursion SQL sur les chemins simples explose sur un graphe dense. Liens suivis dans les
// deux sens ; à longueur égale, le voisin de plus petit id passe en premier.
func (s *Serveur) chemin(w http.ResponseWriter, r *http.Request) {
	de, err1 := strconv.ParseInt(r.URL.Query().Get("de"), 10, 64)
	a, err2 := strconv.ParseInt(r.URL.Query().Get("a"), 10, 64)
	if err1 != nil || err2 != nil {
		erreur(w, http.StatusBadRequest, "paramètres de et a : identifiants de notes attendus")
		return
	}
	ctx := r.Context()
	noeuds, err := s.q.NoeudsGraphe(ctx)
	if echec(w, err) {
		return
	}
	parID := make(map[int64]NoeudGraphe, len(noeuds))
	for _, n := range noeuds {
		parID[n.ID] = noeud(n.ID, n.Titre, n.Dossier, n.Degre)
	}
	if _, ok := parID[de]; !ok {
		erreur(w, http.StatusNotFound, "note de départ introuvable")
		return
	}
	if _, ok := parID[a]; !ok {
		erreur(w, http.StatusNotFound, "note d'arrivée introuvable")
		return
	}
	liens, err := s.q.LiensGraphe(ctx)
	if echec(w, err) {
		return
	}
	voisins := map[int64][]int64{}
	for _, l := range liens {
		if l.Cible != nil && *l.Cible != l.Source {
			voisins[l.Source] = append(voisins[l.Source], *l.Cible)
			voisins[*l.Cible] = append(voisins[*l.Cible], l.Source)
		}
	}
	for _, v := range voisins {
		slices.Sort(v)
	}

	res := Chemin{Chemin: []NoeudGraphe{}}
	precedent := map[int64]int64{de: de}
	file := []int64{de}
	for len(file) > 0 {
		if _, trouve := precedent[a]; trouve {
			break
		}
		n := file[0]
		file = file[1:]
		for _, v := range voisins[n] {
			if _, vu := precedent[v]; !vu {
				precedent[v] = n
				file = append(file, v)
			}
		}
	}
	if _, trouve := precedent[a]; trouve {
		for n := a; ; n = precedent[n] {
			res.Chemin = append(res.Chemin, parID[n])
			if n == de {
				break
			}
		}
		slices.Reverse(res.Chemin)
	}
	ecrireJSON(w, res)
}

func (s *Serveur) note(w http.ResponseWriter, r *http.Request) {
	id, ok := idChemin(w, r)
	if !ok {
		return
	}
	n, err := s.q.NoteParID(r.Context(), id)
	if echec(w, err) {
		return
	}
	voisins, err := s.q.Voisins(r.Context(), id)
	if echec(w, err) {
		return
	}
	res := NoteDetail{ID: n.ID, Titre: n.Titre, Chemin: n.Chemin, Dossier: n.Dossier, Maj: n.Maj, Contenu: n.Contenu, Voisins: []NoeudGraphe{}}
	for _, v := range voisins {
		res.Voisins = append(res.Voisins, noeud(v.ID, v.Titre, v.Dossier, v.Degre))
	}
	ecrireJSON(w, res)
}

// ---- Recherche

func (s *Serveur) recherche(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	termes := strings.TrimSpace(q.Get("q"))
	if termes == "" {
		ecrireJSON(w, []ResultatRecherche{})
		return
	}
	limite := int64(50)
	if l, err := strconv.ParseInt(q.Get("limite"), 10, 64); err == nil && l > 0 {
		limite = min(l, 200)
	}
	facultatif := func(nom string) any {
		if v := strings.TrimSpace(q.Get(nom)); v != "" {
			return v
		}
		return nil
	}
	lignes, err := s.q.Rechercher(r.Context(), gen.RechercherParams{
		Termes: termes, Source: facultatif("source"), Projet: facultatif("projet"), Dossier: facultatif("dossier"),
		Depuis: facultatif("depuis"), Jusqua: facultatif("jusqua"), Limite: limite,
	})
	if err != nil {
		// Une syntaxe FTS5 invalide (guillemet non fermé, opérateur seul) est une erreur de l'utilisateur.
		erreur(w, http.StatusBadRequest, "recherche invalide : "+err.Error())
		return
	}
	res := make([]ResultatRecherche, 0, len(lignes))
	for _, l := range lignes {
		res = append(res, ResultatRecherche{Source: l.Source, Ref: l.Ref, Titre: l.Titre, Extrait: l.Extrait, Projet: l.Projet, Date: texte(l.Date), IDNote: l.IDNote})
	}
	ecrireJSON(w, res)
}
