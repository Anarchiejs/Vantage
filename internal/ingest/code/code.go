package code

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Anarchiejs/Observatoire/internal/model"
	"github.com/boyter/scc/v3/processor"
)

// initScc charge une seule fois la base des langages de scc (globale au package processor).
var initScc sync.Once

// Mesurer compte les lignes par langage des fichiers suivis par Git dans depot.
// Les fichiers d'environnement sont toujours exclus, même s'ils sont suivis.
func Mesurer(ctx context.Context, depot string) ([]model.MesureLangage, error) {
	initScc.Do(processor.ProcessConstants)
	cmd := exec.CommandContext(ctx, "git", "-C", depot, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files dans %q: %w", depot, err)
	}

	stats := make(map[string]*model.MesureLangage)
	for _, rawPath := range bytes.Split(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		rel := filepath.FromSlash(string(rawPath))
		base := filepath.Base(rel)
		if strings.HasPrefix(strings.ToLower(base), ".env") {
			continue
		}

		languages, _ := processor.DetectLanguage(base)
		if len(languages) == 0 || languages[0] == "" {
			continue
		}
		language := languages[0]
		path := filepath.Join(depot, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("lire le fichier suivi %q: %w", string(rawPath), err)
		}
		job := &processor.FileJob{Language: language, Filename: base, Location: path, Content: content, Bytes: int64(len(content))}
		processor.LoadLanguageFeature(language)
		processor.CountStats(job)

		measurement := stats[language]
		if measurement == nil {
			measurement = &model.MesureLangage{Langage: language}
			stats[language] = measurement
		}
		measurement.Fichiers++
		measurement.LignesCode += job.Code
		measurement.LignesCommentaires += job.Comment
	}

	result := make([]model.MesureLangage, 0, len(stats))
	for _, measurement := range stats {
		result = append(result, *measurement)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LignesCode == result[j].LignesCode {
			return result[i].Langage < result[j].Langage
		}
		return result[i].LignesCode > result[j].LignesCode
	})
	return result, nil
}
