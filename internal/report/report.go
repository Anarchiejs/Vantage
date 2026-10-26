// Package report renders the weekly Obsidian statistics note.
package report

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/sql/gen"
)

const jour = "2006-01-02"

// Generer writes the Markdown report for the week ending at aujourdhui.
func Generer(ctx context.Context, q *gen.Queries, aujourdhui string) ([]byte, error) {
	fin, err := time.Parse(jour, aujourdhui)
	if err != nil {
		return nil, fmt.Errorf("date aujourdhui invalide: %w", err)
	}
	lundi := fin.AddDate(0, 0, -((int(fin.Weekday()) + 6) % 7))
	precedentFin := lundi.AddDate(0, 0, -1)
	precedentDebut := precedentFin.AddDate(0, 0, -6)

	semaine, err := q.ChiffresPeriode(ctx, gen.ChiffresPeriodeParams{Debut: lundi.Format(jour), Fin: aujourdhui})
	if err != nil {
		return nil, fmt.Errorf("chiffres semaine: %w", err)
	}
	precedente, err := q.ChiffresPeriode(ctx, gen.ChiffresPeriodeParams{Debut: precedentDebut.Format(jour), Fin: precedentFin.Format(jour)})
	if err != nil {
		return nil, fmt.Errorf("chiffres semaine precedente: %w", err)
	}
	actifs, err := q.ProjetsActifs(ctx, aujourdhui)
	if err != nil {
		return nil, fmt.Errorf("projets actifs: %w", err)
	}
	dormants, err := q.ProjetsDormants(ctx, aujourdhui)
	if err != nil {
		return nil, fmt.Errorf("projets dormants: %w", err)
	}
	perimees, err := q.FichesPerimees(ctx)
	if err != nil {
		return nil, fmt.Errorf("fiches perimees: %w", err)
	}
	ias, err := q.SessionsIAPeriode(ctx, gen.SessionsIAPeriodeParams{Debut: lundi.Format(jour), Fin: aujourdhui})
	if err != nil {
		return nil, fmt.Errorf("sessions IA semaine: %w", err)
	}
	calendrier, err := q.Calendrier(ctx, aujourdhui)
	if err != nil {
		return nil, fmt.Errorf("calendrier: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntype: statistiques\ngenere_par: observatoire\ngenere_le: %s\n---\n# Statistiques — semaine du %s au %s\n\n", aujourdhui, lundi.Format(jour), aujourdhui)
	b.WriteString("Note générée automatiquement par [[Observatoire]] : ne pas modifier à la main, elle est réécrite chaque semaine.\nRetour au centre : [[Statistics]]\n\n")
	b.WriteString("## Activité\n\n| | Cette semaine | Semaine précédente |\n|---|---:|---:|\n")
	metric := func(n string, a, z int64) { fmt.Fprintf(&b, "| %s | %s | %s |\n", n, nombre(a), nombre(z)) }
	metric("Commits", semaine.Commits, precedente.Commits)
	metric("Lignes ajoutées", semaine.Ajouts, precedente.Ajouts)
	metric("Lignes supprimées", semaine.Suppressions, precedente.Suppressions)
	metric("Sessions", semaine.Sessions, precedente.Sessions)
	metric("Projets actifs", semaine.ProjetsActifs, precedente.ProjetsActifs)
	b.WriteString("\n## Projets actifs\n\n")
	if len(actifs) == 0 {
		b.WriteString("Aucun.\n")
	} else {
		for _, p := range actifs {
			fmt.Fprintf(&b, "- [[%s]] — %s, dernier commit le %s\n", lien(p.Nom), pluriel(p.Commits30j, "commit"), premiers(valeur(p.DernierCommit, "inconnu"), 10))
		}
	}
	b.WriteString("\n## Projets qui dorment\n\n")
	if len(dormants) == 0 {
		b.WriteString("Aucun.\n")
	} else {
		for _, p := range dormants {
			etat := "aucun commit importé"
			if p.Jours >= 0 {
				etat = "sans commit depuis " + pluriel(p.Jours, "jour")
			}
			fmt.Fprintf(&b, "- [[%s]] — %s\n", lien(p.Nom), etat)
		}
	}
	b.WriteString("\n## Fiches périmées\n\n")
	if len(perimees) == 0 {
		b.WriteString("Aucun.\n")
	} else {
		for _, p := range perimees {
			sha := "inconnu"
			if p.VerifieCommit != nil && *p.VerifieCommit != "" {
				sha = *p.VerifieCommit
			}
			depuis := "commit vérifié introuvable dans le dépôt"
			if p.CommitsDepuis >= 0 {
				depuis = pluriel(p.CommitsDepuis, "commit") + " depuis"
			}
			fmt.Fprintf(&b, "- [[%s]] — vérifiée à %s, %s\n", lien(p.Nom), sha, depuis)
		}
	}
	b.WriteString("\n## IA de la semaine\n\n")
	if len(ias) == 0 {
		b.WriteString("Aucune.\n")
	} else {
		for _, ia := range ias {
			nom := "inconnue"
			if ia.Ia != nil {
				nom = *ia.Ia
			}
			if nom == "Claude Code" {
				nom = "Claude Code|Claude"
			}
			fmt.Fprintf(&b, "- [[%s]] — %s\n", lien(nom), pluriel(ia.Sessions, "session"))
		}
	}
	b.WriteString("\n## Jours les plus actifs\n\n")
	type jourActif struct {
		date    string
		commits int64
	}
	var jours []jourActif
	for _, j := range calendrier {
		if j.Jour >= lundi.Format(jour) && j.Jour <= aujourdhui && j.Commits > 0 {
			jours = append(jours, jourActif{date: j.Jour, commits: j.Commits})
		}
	}
	sort.SliceStable(jours, func(i, j int) bool {
		if jours[i].commits == jours[j].commits {
			return jours[i].date < jours[j].date
		}
		return jours[i].commits > jours[j].commits
	})
	if len(jours) > 3 {
		jours = jours[:3]
	}
	if len(jours) == 0 {
		b.WriteString("Aucun.\n")
	} else {
		for _, j := range jours {
			fmt.Fprintf(&b, "- %s — %s\n", j.date, pluriel(j.commits, "commit"))
		}
	}
	return []byte(b.String()), nil
}

// Ecrire atomically replaces an existing generated report, refusing unrelated files.
func Ecrire(coffre string, contenu []byte) error {
	if err := os.MkdirAll(coffre, 0755); err != nil {
		return err
	}
	target := filepath.Join(coffre, "Statistiques.md")
	if old, err := os.ReadFile(target); err == nil {
		if !strings.Contains(string(old), "genere_par: observatoire") {
			return fmt.Errorf("refus d'ecraser %s: front matter genere_par: observatoire absent", target)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(coffre, ".Statistiques-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(contenu); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, target)
}

func nombre(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return s
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "\u202f" + s[i:]
	}
	return s
}

func valeur(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	s := fmt.Sprint(v)
	if s == "" || s == "<nil>" {
		return fallback
	}
	return s
}

func lien(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "]", "") }

// premiers garde les n premiers caractères (la date d'un instant ISO).
func premiers(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// pluriel : « 1 commit », « 3 commits », « 0 commit » (règle française).
func pluriel(n int64, mot string) string {
	if n > 1 {
		mot += "s"
	}
	return nombre(n) + " " + mot
}
