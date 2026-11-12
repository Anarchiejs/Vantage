// Package git reads Git metadata without reading diffs or writing to the database.
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
	"github.com/Anarchiejs/Observatoire/internal/ospath"
)

var ErrHistoryRewritten = errors.New("git history was rewritten")

// Command runs git in repo. core.quotepath keeps non-ASCII paths literal.
// core.precomposeunicode is ignored on Linux and Windows; on macOS it makes
// Git return the same precomposed paths those systems already store.
func Command(ctx context.Context, repo string, args ...string) *exec.Cmd {
	prefix := []string{"-C", repo, "-c", "core.quotepath=false", "-c", "core.precomposeunicode=true"}
	return exec.CommandContext(ctx, "git", append(prefix, args...)...)
}

func output(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := Command(ctx, repo, args...)
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

// Head returns the HEAD SHA and its branch (empty when HEAD is detached).
func Head(ctx context.Context, repo string) (sha string, branch string, err error) {
	sha, err = output(ctx, repo, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", "", err
	}
	branch, err = output(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		err = nil
	}
	return sha, branch, err
}

// Read returns commits in topological order, parents before children.
// An unknown or unreachable resume point triggers a full reread and ErrHistoryRewritten.
func Read(ctx context.Context, repo string, sinceSHA string) ([]model.Commit, error) {
	revision := "HEAD"
	var warning error
	if sinceSHA != "" {
		sha, err := output(ctx, repo, "rev-parse", "--verify", "--end-of-options", sinceSHA+"^{commit}")
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return nil, err
			}
			warning = fmt.Errorf("%w: %s", ErrHistoryRewritten, sinceSHA)
		} else {
			_, err = output(ctx, repo, "merge-base", "--is-ancestor", sha, "HEAD")
			if err == nil {
				revision = sha + "..HEAD"
			} else {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					return nil, err
				}
				warning = fmt.Errorf("%w: %s unreachable from HEAD", ErrHistoryRewritten, sinceSHA)
			}
		}
	}
	cmd := Command(ctx, repo, "log", "--reverse", "--topo-order", "--numstat", "-z", "-M", "--no-ext-diff", "--no-textconv", "--diff-merges=off", "--format=%x00%H%x00%P%x00%an%x00%aI%x00%B%x00", revision, "--")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	commits, parseErr := readStream(stdout)
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
	return commits, warning
}

// NULs separate fields and paths, including both paths of a rename.
// Nothing is split on lines: messages and names may contain them.
func readStream(r io.Reader) ([]model.Commit, error) {
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
			return nil, fmt.Errorf("git read: %w", err)
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
					return nil, fmt.Errorf("git header: %w", err)
				}
			}
			date, err := time.Parse(time.RFC3339, fields[2])
			if err != nil {
				return nil, fmt.Errorf("commit date %s: %w", s, err)
			}
			commits = append(commits, model.Commit{SHA: s, Author: fields[1], Date: date, Message: strings.TrimRight(fields[3], " \t\r\n"), IsMerge: len(strings.Fields(fields[0])) > 1})
			continue
		}
		if len(commits) == 0 {
			return nil, errors.New("numstat without a commit")
		}
		parts := strings.SplitN(s, "\t", 3)
		if len(parts) != 3 {
			return nil, errors.New("invalid numstat")
		}
		filePath := parts[2]
		if filePath == "" {
			if _, err = token(); err != nil {
				return nil, fmt.Errorf("old path: %w", err)
			}
			filePath, err = token()
			if err != nil {
				return nil, fmt.Errorf("new path: %w", err)
			}
		}
		filePath = ospath.ForCompare(filePath)
		f := model.ChangedFile{Path: filePath, Language: language(filePath)}
		if parts[0] != "-" || parts[1] != "-" {
			additions, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || additions < 0 {
				return nil, fmt.Errorf("invalid additions: %q", parts[0])
			}
			deletions, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || deletions < 0 {
				return nil, fmt.Errorf("invalid deletions: %q", parts[1])
			}
			f.Additions, f.Deletions = &additions, &deletions
		}
		c := &commits[len(commits)-1]
		if !c.IsMerge {
			c.Files = append(c.Files, f)
		}
	}
}
