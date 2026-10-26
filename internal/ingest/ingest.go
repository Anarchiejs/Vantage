// Package ingest écrit en base ce que lisent les lecteurs git, vault et code.
// Une transaction pour le coffre, puis une par dépôt ; tout est idempotent.
package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	icode "github.com/Anarchiejs/Observatoire/internal/ingest/code"
	igit "github.com/Anarchiejs/Observatoire/internal/ingest/git"
	ivault "github.com/Anarchiejs/Observatoire/internal/ingest/vault"
	"github.com/Anarchiejs/Observatoire/internal/model"
	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

// Options de l'import.
type Options struct {
	Coffre string
	// Depots remplace le chemin_source d'une fiche (nom du projet → dossier) ; sert aux tests.
	Depots map[string]string
	// Aujourdhui date les mesures de langage ; vide = date du jour.
	Aujourdhui string
}

// Bilan de l'import.
type Bilan struct {
	NotesEcrites    int
	NotesInchangees int
	NotesSupprimees int
	Depots          int
	Commits         int
	Duree           time.Duration
	Avertissements  []string
}

// Importer lit le coffre puis chaque dépôt cité par une fiche et met la base à jour.
func Importer(ctx context.Context, base *sql.DB, opt Options) (Bilan, error) {
	debut := time.Now()
	var b Bilan
	if opt.Aujourdhui == "" {
		opt.Aujourdhui = time.Now().Format("2006-01-02")
	}

	res, err := ivault.Lire(opt.Coffre)
	if err != nil {
		return b, fmt.Errorf("lecture du coffre : %w", err)
	}
	projets, err := ecrireCoffre(ctx, base, res, &b)
	if err != nil {
		return b, fmt.Errorf("écriture du coffre : %w", err)
	}

	// Lecture des dépôts en parallèle (git et scc dominent le temps), écriture une par une :
	// SQLite n'accepte qu'un écrivain à la fois.
	var lectures []*lecture
	for _, f := range res.Fiches {
		chemin := f.CheminSource
		if c, ok := opt.Depots[f.Nom]; ok {
			chemin = c
		}
		if chemin != "" {
			lectures = append(lectures, &lecture{projet: f.Nom, chemin: chemin})
		}
	}
	q := gen.New(base)
	var wg sync.WaitGroup
	place := make(chan struct{}, runtime.NumCPU())
	for _, l := range lectures {
		wg.Add(1)
		go func() {
			defer wg.Done()
			place <- struct{}{}
			defer func() { <-place }()
			l.lire(ctx, q)
		}()
	}
	wg.Wait()

	for _, l := range lectures {
		ajoutes, err := 0, l.err
		if err == nil {
			ajoutes, err = ecrireDepot(ctx, base, projets[l.projet], l, opt.Aujourdhui)
		}
		if err != nil {
			// Un dépôt absent ou cassé ne doit pas bloquer les autres.
			b.Avertissements = append(b.Avertissements, fmt.Sprintf("%s (%s) : %v", l.projet, l.chemin, err))
			continue
		}
		b.Depots++
		b.Commits += ajoutes
	}
	b.Duree = time.Since(debut)
	return b, nil
}

func ecrireCoffre(ctx context.Context, base *sql.DB, res ivault.Resultat, b *Bilan) (map[string]int64, error) {
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Requêtes préparées une fois par transaction : SQLite ne recompile pas le SQL à chaque ligne.
	q, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	projets := map[string]int64{}
	for _, f := range res.Fiches {
		id, err := q.UpsertProjet(ctx, gen.UpsertProjetParams{
			Nom:           f.Nom,
			Statut:        vide(f.Statut),
			CheminSource:  vide(f.CheminSource),
			FicheMaj:      vide(f.Maj),
			VerifieCommit: vide(f.VerifieCommit),
			VerifieLe:     vide(f.VerifieLe),
		})
		if err != nil {
			return nil, fmt.Errorf("projet %s : %w", f.Nom, err)
		}
		projets[f.Nom] = id
		for _, t := range f.Technos {
			tid, err := q.UpsertTechno(ctx, gen.UpsertTechnoParams{Nom: t})
			if err != nil {
				return nil, err
			}
			if err := q.LierProjetTechno(ctx, gen.LierProjetTechnoParams{ProjetID: id, TechnoID: tid}); err != nil {
				return nil, err
			}
		}
	}

	existantes, err := q.ListerNotes(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(existantes))
	for _, n := range existantes {
		ids[n.Chemin] = n.ID
	}
	vues := make(map[string]bool, len(res.Notes))
	for _, n := range res.Notes {
		vues[n.Chemin] = true
		id, err := ecrireNote(ctx, q, n)
		if err != nil {
			return nil, fmt.Errorf("note %s : %w", n.Chemin, err)
		}
		if id == 0 {
			b.NotesInchangees++
			continue
		}
		ids[n.Chemin] = id
		b.NotesEcrites++
	}
	for _, n := range existantes {
		if vues[n.Chemin] {
			continue
		}
		if err := q.SupprimerRecherche(ctx, gen.SupprimerRechercheParams{Source: "note", Ref: n.Chemin}); err != nil {
			return nil, err
		}
		if err := q.SupprimerNote(ctx, n.ID); err != nil {
			return nil, err
		}
		b.NotesSupprimees++
	}
	if err := q.ResoudreLiens(ctx); err != nil {
		return nil, err
	}

	for _, s := range res.Sessions {
		id, ok := ids[s.CheminNote]
		if !ok {
			continue
		}
		var projet *int64
		if p, ok := projets[s.Projet]; ok {
			projet = &p
		}
		if err := q.UpsertSession(ctx, gen.UpsertSessionParams{NoteID: id, Date: s.Date, ProjetID: projet, Ia: vide(s.IA)}); err != nil {
			return nil, err
		}
	}
	for _, e := range res.Journal {
		if err := q.InsererJournal(ctx, gen.InsererJournalParams{Date: e.Date, Ia: vide(e.IA), Texte: e.Texte}); err != nil {
			return nil, err
		}
	}
	return projets, tx.Commit()
}

// ecrireNote renvoie 0 si la note est inchangée depuis le dernier import.
func ecrireNote(ctx context.Context, q *gen.Queries, n model.Note) (int64, error) {
	if ancien, err := q.HashNote(ctx, n.Chemin); err == nil && ancien.ContenuHash == n.Hash {
		return 0, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	id, err := q.UpsertNote(ctx, gen.UpsertNoteParams{
		Chemin: n.Chemin, Dossier: n.Dossier, Titre: n.Titre,
		Type: vide(n.Type), Maj: vide(n.Maj), ContenuHash: n.Hash,
	})
	if err != nil {
		return 0, err
	}
	if err := q.SupprimerLiensDeNote(ctx, id); err != nil {
		return 0, err
	}
	for _, l := range n.Liens {
		if err := q.InsererLien(ctx, gen.InsererLienParams{SourceID: id, CibleTitre: l}); err != nil {
			return 0, err
		}
	}
	if err := q.SupprimerRecherche(ctx, gen.SupprimerRechercheParams{Source: "note", Ref: n.Chemin}); err != nil {
		return 0, err
	}
	err = q.InsererRecherche(ctx, gen.InsererRechercheParams{Titre: n.Titre, Contenu: n.Contenu, Source: "note", Ref: n.Chemin})
	return id, err
}

// lecture est ce qu'on sait d'un dépôt avant de l'écrire.
type lecture struct {
	projet, chemin string
	tete, branche  string
	inchange       bool // HEAD est déjà importé : rien à écrire
	reecrit        bool // l'historique a changé sous dernier_sha_importe
	commits        []model.Commit
	mesures        []model.MesureLangage
	err            error
}

func (l *lecture) lire(ctx context.Context, q *gen.Queries) {
	l.tete, l.branche, l.err = igit.Tete(ctx, l.chemin)
	if l.err != nil {
		return
	}
	depuis := ""
	if sha, err := q.DernierShaDepot(ctx, l.chemin); err == nil && sha != nil {
		depuis = *sha
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		l.err = err
		return
	}
	if depuis == l.tete {
		l.inchange = true
		return
	}
	l.commits, l.err = igit.Lire(ctx, l.chemin, depuis)
	if errors.Is(l.err, igit.ErrHistoriqueReecrit) {
		l.reecrit, l.err = true, nil
	}
	if l.err != nil {
		return
	}
	if l.mesures, l.err = icode.Mesurer(ctx, l.chemin); l.err != nil {
		l.err = fmt.Errorf("mesure des langages : %w", l.err)
	}
}

func ecrireDepot(ctx context.Context, base *sql.DB, projetID int64, l *lecture, aujourdhui string) (int, error) {
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q, err := gen.Prepare(ctx, tx)
	if err != nil {
		return 0, err
	}
	defer q.Close()

	d, err := q.UpsertDepot(ctx, gen.UpsertDepotParams{ProjetID: projetID, Chemin: l.chemin, Branche: vide(l.branche)})
	if err != nil {
		return 0, err
	}
	if l.inchange {
		return 0, tx.Commit()
	}
	if l.reecrit {
		if err := q.SupprimerRechercheCommitsDepot(ctx, d.ID); err != nil {
			return 0, err
		}
		if err := q.SupprimerCommitsDepot(ctx, d.ID); err != nil {
			return 0, err
		}
	}

	n := 0
	for _, c := range l.commits {
		ajoute, err := q.InsererCommit(ctx, gen.InsererCommitParams{
			Sha: c.SHA, DepotID: d.ID, Auteur: c.Auteur,
			DateUtc: c.DateUTC(), DecalageMin: c.DecalageMin(), Message: c.Message,
		})
		if err != nil {
			return 0, fmt.Errorf("commit %s : %w", c.SHA, err)
		}
		if ajoute == 0 {
			continue // déjà connu (même commit dans un autre dépôt)
		}
		n++
		for _, f := range c.Fichiers {
			if err := q.InsererFichierModifie(ctx, gen.InsererFichierModifieParams{
				CommitSha: c.SHA, Chemin: f.Chemin, Langage: vide(f.Langage),
				Ajouts: f.Ajouts, Suppressions: f.Suppressions,
			}); err != nil {
				return 0, err
			}
		}
		titre, _, _ := strings.Cut(c.Message, "\n")
		if err := q.InsererRecherche(ctx, gen.InsererRechercheParams{Titre: titre, Contenu: c.Message, Source: "commit", Ref: c.SHA}); err != nil {
			return 0, err
		}
	}

	for _, m := range l.mesures {
		if err := q.InsererMesureLangage(ctx, gen.InsererMesureLangageParams{
			DepotID: d.ID, DateMesure: aujourdhui, Langage: m.Langage,
			Fichiers: m.Fichiers, LignesCode: m.LignesCode, LignesCommentaires: m.LignesCommentaires,
		}); err != nil {
			return 0, err
		}
	}

	if err := q.MarquerDepotImporte(ctx, gen.MarquerDepotImporteParams{DernierShaImporte: &l.tete, ID: d.ID}); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func vide(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
