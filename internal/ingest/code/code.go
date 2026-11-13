package code

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Anarchiejs/Vantage/internal/ingest/git"
	"github.com/Anarchiejs/Vantage/internal/model"
	"github.com/Anarchiejs/Vantage/internal/ospath"
	"github.com/boyter/scc/v3/processor"
)

// initSCC loads scc's language database once (global to the processor package).
var initSCC sync.Once

// maxMeasuredBytes skips a blob instead of loading it into memory.
const maxMeasuredBytes = 1 << 20

type blob struct {
	sha, path, base, language string
}

// Measure counts lines per language for blobs in rev (empty means HEAD).
// It reads Git objects, so a dirty or incomplete worktree does not change the
// totals and does not block the import. Submodules, symlinks, environment
// files, and blobs over maxMeasuredBytes are skipped.
func Measure(ctx context.Context, repo, rev string) ([]model.LanguageStat, []string, error) {
	initSCC.Do(processor.ProcessConstants)
	if strings.TrimSpace(rev) == "" {
		rev = "HEAD"
	}
	cmd := git.Command(ctx, repo, "ls-tree", "-r", "-z", rev)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git ls-tree in %q: %w", repo, err)
	}

	var blobs []blob
	var warnings []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		tab := bytes.IndexByte(raw, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(string(raw[:tab]))
		if len(meta) < 3 || meta[1] != "blob" {
			continue
		}
		mode := meta[0]
		if mode == "120000" || mode == "160000" {
			continue
		}
		rel := ospath.ForCompare(filepath.FromSlash(string(raw[tab+1:])))
		base := filepath.Base(rel)
		if strings.HasPrefix(strings.ToLower(base), ".env") {
			continue
		}
		detected, _ := processor.DetectLanguage(base)
		if len(detected) == 0 || detected[0] == "" || detected[0] == "Binary" {
			continue
		}
		blobs = append(blobs, blob{sha: meta[2], path: rel, base: base, language: detected[0]})
	}

	contents, skipped, err := blobContents(ctx, repo, blobs)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, skipped...)

	stats := make(map[string]*model.LanguageStat)
	for _, item := range blobs {
		content, ok := contents[item.sha]
		if !ok {
			continue
		}
		job := &processor.FileJob{Language: item.language, Filename: item.base, Location: item.path, Content: content, Bytes: int64(len(content))}
		processor.LoadLanguageFeature(item.language)
		processor.CountStats(job)
		measurement := stats[item.language]
		if measurement == nil {
			measurement = &model.LanguageStat{Language: item.language}
			stats[item.language] = measurement
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
	return result, warnings, nil
}

// blobContents returns the bytes of each blob at most maxMeasuredBytes.
// A blob that cannot be read is named in the warning list and omitted.
func blobContents(ctx context.Context, repo string, blobs []blob) (map[string][]byte, []string, error) {
	if len(blobs) == 0 {
		return map[string][]byte{}, nil, nil
	}
	var input strings.Builder
	seen := map[string]bool{}
	for _, item := range blobs {
		if seen[item.sha] {
			continue
		}
		seen[item.sha] = true
		input.WriteString(item.sha)
		input.WriteByte('\n')
	}
	check := git.Command(ctx, repo, "cat-file", "--batch-check")
	check.Stdin = strings.NewReader(input.String())
	checkOut, err := check.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git cat-file --batch-check: %w", err)
	}
	sizes := map[string]int64{}
	var warnings []string
	for _, line := range strings.Split(strings.TrimRight(string(checkOut), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "blob" {
			warnings = append(warnings, fmt.Sprintf("skip %s", line))
			continue
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		if size > maxMeasuredBytes {
			warnings = append(warnings, fmt.Sprintf("skip %s (%d bytes)", fields[0], size))
			continue
		}
		sizes[fields[0]] = size
	}

	var batch strings.Builder
	for sha := range sizes {
		batch.WriteString(sha)
		batch.WriteByte('\n')
	}
	if batch.Len() == 0 {
		return map[string][]byte{}, warnings, nil
	}
	read := git.Command(ctx, repo, "cat-file", "--batch")
	read.Stdin = strings.NewReader(batch.String())
	stdout, err := read.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := read.Start(); err != nil {
		return nil, nil, err
	}
	contents := map[string][]byte{}
	in := bufio.NewReader(stdout)
	for {
		header, err := in.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = read.Process.Kill()
			return nil, nil, err
		}
		header = strings.TrimRight(header, "\r\n")
		fields := strings.Fields(header)
		if len(fields) < 3 || fields[1] != "blob" {
			continue
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(in, buf); err != nil {
			_ = read.Process.Kill()
			return nil, nil, err
		}
		if b, err := in.ReadByte(); err != nil || b != '\n' {
			_ = read.Process.Kill()
			return nil, nil, fmt.Errorf("git cat-file: missing trailing newline")
		}
		contents[fields[0]] = buf
	}
	if err := read.Wait(); err != nil {
		return nil, nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	return contents, warnings, nil
}
