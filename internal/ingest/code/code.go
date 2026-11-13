package code

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Anarchiejs/Vantage/internal/ingest/git"
	"github.com/Anarchiejs/Vantage/internal/model"
	"github.com/Anarchiejs/Vantage/internal/ospath"
	"github.com/boyter/scc/v3/processor"
)

// initSCC loads scc's language database once (global to the processor package).
var initSCC sync.Once

// Measure counts lines per language for files tracked by Git in repo.
// Environment files are always excluded, even when they are tracked.
func Measure(ctx context.Context, repo string) ([]model.LanguageStat, error) {
	initSCC.Do(processor.ProcessConstants)
	cmd := git.Command(ctx, repo, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %q: %w", repo, err)
	}

	stats := make(map[string]*model.LanguageStat)
	for _, rawPath := range bytes.Split(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		rel := ospath.ForCompare(filepath.FromSlash(string(rawPath)))
		base := filepath.Base(rel)
		if strings.HasPrefix(strings.ToLower(base), ".env") {
			continue
		}

		detected, _ := processor.DetectLanguage(base)
		if len(detected) == 0 || detected[0] == "" {
			continue
		}
		language := detected[0]
		path := filepath.Join(repo, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read tracked file %q: %w", string(rawPath), err)
		}
		job := &processor.FileJob{Language: language, Filename: base, Location: path, Content: content, Bytes: int64(len(content))}
		processor.LoadLanguageFeature(language)
		processor.CountStats(job)

		measurement := stats[language]
		if measurement == nil {
			measurement = &model.LanguageStat{Language: language}
			stats[language] = measurement
		}
		measurement.Files++
		measurement.CodeLines += job.Code
		measurement.CommentLines += job.Comment
	}

	result := make([]model.LanguageStat, 0, len(stats))
	for _, measurement := range stats {
		result = append(result, *measurement)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CodeLines == result[j].CodeLines {
			return result[i].Language < result[j].Language
		}
		return result[i].CodeLines > result[j].CodeLines
	})
	return result, nil
}
