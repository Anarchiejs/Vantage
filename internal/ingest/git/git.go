// Package git lit les métadonnées Git sans lire les diffs ni écrire en base.
package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Anarchiejs/Observatoire/internal/model"
)

var ErrHistoriqueReecrit = errors.New("historique Git réécrit")

func commande(ctx context.Context, depot string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", append([]string{"-C", depot, "-c", "core.quotepath=false"}, args...)...)
}

func sortie(ctx context.Context, depot string, args ...string) (string, error) {
	cmd := commande(ctx, depot, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

// Tete renvoie le SHA de HEAD et sa branche (vide si HEAD est détachée).
func Tete(ctx context.Context, depot string) (sha string, branche string, err error) {
	sha, err = sortie(ctx, depot, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", "", err
	}
	branche, err = sortie(ctx, depot, "symbolic-ref", "--quiet", "--short", "HEAD")
	var e *exec.ExitError
	if errors.As(err, &e) && e.ExitCode() == 1 {
		err = nil
	}
	return sha, branche, err
}

// Lire renvoie les commits dans l'ordre topologique, parents avant enfants.
// Un point de reprise inconnu ou devenu inaccessible entraîne une relecture
// complète accompagnée d'ErrHistoriqueReecrit.
func Lire(ctx context.Context, depot string, depuisSHA string) ([]model.Commit, error) {
	revision := "HEAD"
	var avertissement error
	if depuisSHA != "" {
		sha, err := sortie(ctx, depot, "rev-parse", "--verify", "--end-of-options", depuisSHA+"^{commit}")
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var e *exec.ExitError
			if !errors.As(err, &e) {
				return nil, err
			}
			avertissement = fmt.Errorf("%w: %s", ErrHistoriqueReecrit, depuisSHA)
		} else {
			_, err = sortie(ctx, depot, "merge-base", "--is-ancestor", sha, "HEAD")
			if err == nil {
				revision = sha + "..HEAD"
			} else {
				var e *exec.ExitError
				if !errors.As(err, &e) || e.ExitCode() != 1 {
					return nil, err
				}
				avertissement = fmt.Errorf("%w: %s inaccessible depuis HEAD", ErrHistoriqueReecrit, depuisSHA)
			}
		}
	}
	cmd := commande(ctx, depot, "log", "--reverse", "--topo-order", "--numstat", "-z", "-M", "--no-ext-diff", "--no-textconv", "--diff-merges=off", "--format=%x00%H%x00%P%x00%an%x00%aI%x00%B%x00", revision, "--")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	commits, parseErr := lireFlux(stdout)
	if parseErr != nil {
		_ = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if parseErr != nil {
		return nil, parseErr
	}
	if err != nil {
		return nil, fmt.Errorf("git log: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return commits, avertissement
}

// Les NUL délimitent les champs et chemins, y compris les deux chemins d'un
// renommage. Aucun découpage par ligne : messages et noms peuvent en contenir.
func lireFlux(r io.Reader) ([]model.Commit, error) {
	b := bufio.NewReader(r)
	token := func() (string, error) {
		s, err := b.ReadString(0)
		if err != nil {
			if err == io.EOF && s != "" {
				err = io.ErrUnexpectedEOF
			}
			return "", err
		}
		return s[:len(s)-1], nil
	}
	commits := make([]model.Commit, 0)
	for {
		s, err := token()
		if err == io.EOF {
			return commits, nil
		}
		if err != nil {
			return nil, fmt.Errorf("lecture git: %w", err)
		}
		if s == "" || s == "\n" {
			continue
		}
		s = strings.TrimPrefix(s, "\n")
		if !strings.Contains(s, "\t") {
			fields := make([]string, 4)
			for i := range fields {
				fields[i], err = token()
				if err != nil {
					return nil, fmt.Errorf("entête git: %w", err)
				}
			}
			date, err := time.Parse(time.RFC3339, fields[2])
			if err != nil {
				return nil, fmt.Errorf("date du commit %s: %w", s, err)
			}
			commits = append(commits, model.Commit{SHA: s, Auteur: fields[1], Date: date, Message: strings.TrimRight(fields[3], " \t\r\n"), EstFusion: len(strings.Fields(fields[0])) > 1})
			continue
		}
		if len(commits) == 0 {
			return nil, errors.New("numstat sans commit")
		}
		parts := strings.SplitN(s, "\t", 3)
		if len(parts) != 3 {
			return nil, errors.New("numstat invalide")
		}
		chemin := parts[2]
		if chemin == "" {
			if _, err = token(); err != nil {
				return nil, fmt.Errorf("ancien chemin: %w", err)
			}
			chemin, err = token()
			if err != nil {
				return nil, fmt.Errorf("nouveau chemin: %w", err)
			}
		}
		f := model.FichierModifie{Chemin: chemin, Langage: langage(chemin)}
		if parts[0] != "-" || parts[1] != "-" {
			a, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || a < 0 {
				return nil, fmt.Errorf("ajouts invalides: %q", parts[0])
			}
			d, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || d < 0 {
				return nil, fmt.Errorf("suppressions invalides: %q", parts[1])
			}
			f.Ajouts, f.Suppressions = &a, &d
		}
		c := &commits[len(commits)-1]
		if !c.EstFusion {
			c.Fichiers = append(c.Fichiers, f)
		}
	}
}
