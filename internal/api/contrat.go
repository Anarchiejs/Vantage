// Package api sert l'interface et l'API JSON.
//
// contrat.go est le miroir exact de web/src/api/types.ts : toute modification se fait des deux côtés.
// Les champs nullables sont des pointeurs ; les listes ne sont jamais nil (renvoyer []T{}) pour que le JSON
// contienne [] et non null.
package api

// Erreur est le corps de toute réponse en erreur.
type Erreur struct {
	Erreur string `json:"erreur"`
}

// ---- GET /api/accueil

type JourActivite struct {
	Jour         string   `json:"jour"`
	Commits      int64    `json:"commits"`
	Ajouts       int64    `json:"ajouts"`
	Suppressions int64    `json:"suppressions"`
	Sessions     int64    `json:"sessions"`
	Projets      []string `json:"projets"`
}

type Chiffres struct {
	Commits       int64 `json:"commits"`
	Ajouts        int64 `json:"ajouts"`
	Suppressions  int64 `json:"suppressions"`
	Sessions      int64 `json:"sessions"`
	ProjetsActifs int64 `json:"projets_actifs"`
}

type ProjetActif struct {
	ID            int64   `json:"id"`
	Nom           string  `json:"nom"`
	DernierCommit *string `json:"dernier_commit"`
	Commits30j    int64   `json:"commits_30j"`
}

type ProjetDormant struct {
	ID            int64   `json:"id"`
	Nom           string  `json:"nom"`
	DernierCommit *string `json:"dernier_commit"`
	Jours         int64   `json:"jours"`
}

type FichePerimee struct {
	ID            int64   `json:"id"`
	Nom           string  `json:"nom"`
	VerifieCommit *string `json:"verifie_commit"`
	DernierSHA    *string `json:"dernier_sha"`
	CommitsDepuis int64   `json:"commits_depuis"`
}

type Accueil struct {
	Calendrier        []JourActivite  `json:"calendrier"`
	Semaine           Chiffres        `json:"semaine"`
	SemainePrecedente Chiffres        `json:"semaine_precedente"`
	Mois              Chiffres        `json:"mois"`
	MoisPrecedent     Chiffres        `json:"mois_precedent"`
	Actifs            []ProjetActif   `json:"actifs"`
	Dormants          []ProjetDormant `json:"dormants"`
	FichesPerimees    []FichePerimee  `json:"fiches_perimees"`
}

// ---- GET /api/projets et /api/projets/{id}

type ProjetResume struct {
	ID               int64   `json:"id"`
	Nom              string  `json:"nom"`
	Statut           *string `json:"statut"`
	Commits          int64   `json:"commits"`
	DernierCommit    *string `json:"dernier_commit"`
	LangagePrincipal *string `json:"langage_principal"`
}

type EvenementFrise struct {
	Date  string  `json:"date"`
	Type  string  `json:"type"` // commit | session | journal
	Titre string  `json:"titre"`
	Ref   string  `json:"ref"`
	IA    *string `json:"ia"`
}

type VolumeSemaine struct {
	Semaine      string `json:"semaine"`
	Commits      int64  `json:"commits"`
	Ajouts       int64  `json:"ajouts"`
	Suppressions int64  `json:"suppressions"`
}

type PartLangage struct {
	Langage    string  `json:"langage"`
	LignesCode int64   `json:"lignes_code"`
	Part       float64 `json:"part"`
}

type PointLangage struct {
	DateMesure string `json:"date_mesure"`
	Langage    string `json:"langage"`
	LignesCode int64  `json:"lignes_code"`
}

type PointChaud struct {
	Chemin        string `json:"chemin"`
	Modifications int64  `json:"modifications"`
	Ajouts        int64  `json:"ajouts"`
	Suppressions  int64  `json:"suppressions"`
}

type Sante struct {
	VerifieCommit   *string  `json:"verifie_commit"`
	VerifieLe       *string  `json:"verifie_le"`
	DernierSHA      *string  `json:"dernier_sha"`
	AJour           bool     `json:"a_jour"`
	CommitsDepuis   int64    `json:"commits_depuis"`
	NotesManquantes []string `json:"notes_manquantes"`
}

type ActiviteIA struct {
	IA       string `json:"ia"`
	Sessions int64  `json:"sessions"`
	Premiere string `json:"premiere"`
	Derniere string `json:"derniere"`
}

type ProjetComplet struct {
	ProjetResume
	CheminSource *string  `json:"chemin_source"`
	Technos      []string `json:"technos"`
}

type ProjetDetail struct {
	Projet            ProjetComplet    `json:"projet"`
	Frise             []EvenementFrise `json:"frise"`
	Volume            []VolumeSemaine  `json:"volume"`
	Langages          []PartLangage    `json:"langages"`
	LangagesEvolution []PointLangage   `json:"langages_evolution"`
	PointsChauds      []PointChaud     `json:"points_chauds"`
	Sante             Sante            `json:"sante"`
	IA                []ActiviteIA     `json:"ia"`
}

// ---- GET /api/technos et /api/ia

type NoeudSankey struct {
	Nom  string `json:"nom"`
	Type string `json:"type"` // projet | techno
}

type LienSankey struct {
	Source string `json:"source"`
	Cible  string `json:"cible"`
	Valeur int64  `json:"valeur"` // 1 par lien projet → techno
}

type Sankey struct {
	Noeuds []NoeudSankey `json:"noeuds"`
	Liens  []LienSankey  `json:"liens"`
}

type TendanceTechno struct {
	Mois    string `json:"mois"`
	Techno  string `json:"techno"`
	Commits int64  `json:"commits"`
}

type Technos struct {
	Sankey     Sankey           `json:"sankey"`
	Tendance   []TendanceTechno `json:"tendance"`
	SansProjet []string         `json:"sans_projet"`
}

type SessionsMois struct {
	Mois     string `json:"mois"`
	Sessions int64  `json:"sessions"`
}

type IA struct {
	IA       string         `json:"ia"`
	Sessions int64          `json:"sessions"`
	Projets  []string       `json:"projets"`
	ParMois  []SessionsMois `json:"par_mois"`
}

// ---- GET /api/graphe, /api/graphe/chemin, /api/notes/{id}

type NoeudGraphe struct {
	ID      int64  `json:"id"`
	Titre   string `json:"titre"`
	Dossier string `json:"dossier"`
	Degre   int64  `json:"degre"`
}

type LienGraphe struct {
	Source int64 `json:"source"`
	Cible  int64 `json:"cible"`
}

type LienCasse struct {
	Source      int64  `json:"source"`
	SourceTitre string `json:"source_titre"`
	CibleTitre  string `json:"cible_titre"`
}

type Graphe struct {
	Noeuds     []NoeudGraphe `json:"noeuds"`
	Liens      []LienGraphe  `json:"liens"`
	Orphelines []NoeudGraphe `json:"orphelines"`
	Centrales  []NoeudGraphe `json:"centrales"`
	Casses     []LienCasse   `json:"casses"`
}

type Chemin struct {
	Chemin []NoeudGraphe `json:"chemin"`
}

type NoteDetail struct {
	ID      int64         `json:"id"`
	Titre   string        `json:"titre"`
	Chemin  string        `json:"chemin"`
	Dossier string        `json:"dossier"`
	Maj     *string       `json:"maj"`
	Contenu string        `json:"contenu"`
	Voisins []NoeudGraphe `json:"voisins"`
}

// ---- GET /api/recherche

type ResultatRecherche struct {
	Source  string  `json:"source"` // note | commit
	Ref     string  `json:"ref"`
	Titre   string  `json:"titre"`
	Extrait string  `json:"extrait"`
	Projet  *string `json:"projet"`
	Date    *string `json:"date"`
	IDNote  *int64  `json:"id_note"`
}

// ---- Console SQL

type DemandeSQL struct {
	SQL string `json:"sql"`
}

type ResultatSQL struct {
	Colonnes []string `json:"colonnes"`
	Lignes   [][]any  `json:"lignes"`
	DureeMs  float64  `json:"duree_ms"`
	Tronque  bool     `json:"tronque"`
}

type ColonneSchema struct {
	Nom  string `json:"nom"`
	Type string `json:"type"`
}

type TableSchema struct {
	Table    string          `json:"table"`
	Colonnes []ColonneSchema `json:"colonnes"`
}

type RequeteEnregistree struct {
	Nom string `json:"nom"`
	SQL string `json:"sql"`
}

// ---- GET /api/evenements (SSE), événement « maj »

type EvenementMaj struct {
	Tables []string `json:"tables"`
}
