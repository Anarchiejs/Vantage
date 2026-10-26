// Package model contient les types échangés entre les lecteurs (git, coffre, code) et l'écriture en base.
// Les lecteurs produisent ces valeurs sans toucher à la base ; internal/ingest les écrit via internal/sql/gen.
package model

import "time"

// Commit est une entrée de `git log --numstat`.
type Commit struct {
	SHA       string
	Auteur    string
	Date      time.Time // avec le fuseau de l'auteur ; DateUTC et DecalageMin en dérivent
	Message   string    // ligne de sujet + corps, sans espaces finaux
	Fichiers  []FichierModifie
	EstFusion bool
}

// DateUTC renvoie la date au format stocké en base : « 2026-10-26T14:03:00Z ».
func (c Commit) DateUTC() string { return c.Date.UTC().Format("2006-01-02T15:04:05Z") }

// DecalageMin renvoie le décalage du fuseau de l'auteur en minutes.
func (c Commit) DecalageMin() int64 {
	_, s := c.Date.Zone()
	return int64(s / 60)
}

// FichierModifie : Ajouts et Suppressions valent nil pour un fichier binaire.
type FichierModifie struct {
	Chemin       string // chemin final, séparateur « / » (pour un renommage : le nouveau chemin)
	Langage      string // "" si inconnu
	Ajouts       *int64
	Suppressions *int64
}

// Fiche est la note `Projets/<Nom>/<Nom>.md` d'un projet.
type Fiche struct {
	Nom           string
	Statut        string
	CheminSource  string
	Maj           string
	VerifieLe     string
	VerifieCommit string
	Technos       []string
}

// Note est une note du coffre.
type Note struct {
	Chemin  string // relatif au coffre, séparateur « / »
	Dossier string // premier segment du chemin, "" à la racine
	Titre   string // nom du fichier sans « .md »
	Type    string
	Maj     string
	Hash    string   // sha256 hexadécimal du contenu brut
	Contenu string   // texte sans le front matter, pour la recherche plein texte
	Liens   []string // cibles des [[...]], sans alias ni « #section », dédoublonnées
}

// Session est une note `Sessions/AAAA-MM-JJ - <Projet> - sujet.md`.
type Session struct {
	CheminNote string
	Date       string
	Projet     string // nom du projet, "" si inconnu
	IA         string
}

// EntreeJournal est une ligne datée de Journal.md.
type EntreeJournal struct {
	Date  string
	IA    string
	Texte string
}

// MesureLangage est le comptage scc d'un langage dans un dépôt.
type MesureLangage struct {
	Langage            string
	Fichiers           int64
	LignesCode         int64
	LignesCommentaires int64
}
