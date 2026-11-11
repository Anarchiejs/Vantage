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

const dayLayout = "2006-01-02"

const reportName = "Statistics.md"

// Generate writes the Markdown report for the week ending on today.
func Generate(ctx context.Context, q *gen.Queries, today string) ([]byte, error) {
	end, err := time.Parse(dayLayout, today)
	if err != nil {
		return nil, fmt.Errorf("invalid today date: %w", err)
	}
	monday := end.AddDate(0, 0, -((int(end.Weekday()) + 6) % 7))
	previousEnd := monday.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -6)

	week, err := q.PeriodStats(ctx, gen.PeriodStatsParams{StartDate: monday.Format(dayLayout), EndDate: today})
	if err != nil {
		return nil, fmt.Errorf("week stats: %w", err)
	}
	previous, err := q.PeriodStats(ctx, gen.PeriodStatsParams{StartDate: previousStart.Format(dayLayout), EndDate: previousEnd.Format(dayLayout)})
	if err != nil {
		return nil, fmt.Errorf("previous week stats: %w", err)
	}
	active, err := q.ActiveProjects(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("active projects: %w", err)
	}
	dormant, err := q.DormantProjects(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("dormant projects: %w", err)
	}
	stale, err := q.StaleCards(ctx)
	if err != nil {
		return nil, fmt.Errorf("stale cards: %w", err)
	}
	assistants, err := q.AISessionsInPeriod(ctx, gen.AISessionsInPeriodParams{StartDate: monday.Format(dayLayout), EndDate: today})
	if err != nil {
		return nil, fmt.Errorf("AI sessions this week: %w", err)
	}
	calendar, err := q.Calendar(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntype: statistics\ngenerated_by: observatoire\ngenerated_on: %s\n---\n# Statistics — week of %s to %s\n\n", today, monday.Format(dayLayout), today)
	b.WriteString("Note generated automatically by [[Observatoire]]: do not edit it by hand; it is rewritten every week.\n\n")
	b.WriteString("## Activity\n\n| | This week | Previous week |\n|---|---:|---:|\n")
	metric := func(name string, current, earlier int64) {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", name, grouped(current), grouped(earlier))
	}
	metric("Commits", week.Commits, previous.Commits)
	metric("Lines added", week.Additions, previous.Additions)
	metric("Lines deleted", week.Deletions, previous.Deletions)
	metric("Sessions", week.Sessions, previous.Sessions)
	metric("Active projects", week.ActiveProjects, previous.ActiveProjects)
	b.WriteString("\n## Active projects\n\n")
	if len(active) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, project := range active {
			fmt.Fprintf(&b, "- [[%s]] — %s, last commit on %s\n", link(project.Name), plural(project.Commits30d, "commit"), first(value(project.LastCommit, "unknown"), 10))
		}
	}
	b.WriteString("\n## Dormant projects\n\n")
	if len(dormant) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, project := range dormant {
			state := "no imported commits"
			if project.Days >= 0 {
				state = "no commits for " + plural(project.Days, "day")
			}
			fmt.Fprintf(&b, "- [[%s]] — %s\n", link(project.Name), state)
		}
	}
	b.WriteString("\n## Stale cards\n\n")
	if len(stale) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, project := range stale {
			sha := "unknown"
			if project.VerifiedCommit != nil && *project.VerifiedCommit != "" {
				sha = *project.VerifiedCommit
			}
			since := "verified commit not found in the repository"
			if project.CommitsSince >= 0 {
				since = plural(project.CommitsSince, "commit") + " since"
			}
			fmt.Fprintf(&b, "- [[%s]] — verified at %s, %s\n", link(project.Name), sha, since)
		}
	}
	b.WriteString("\n## AI this week\n\n")
	if len(assistants) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, assistant := range assistants {
			name := "unknown"
			if assistant.Ai != nil {
				name = *assistant.Ai
			}
			fmt.Fprintf(&b, "- [[%s]] — %s\n", link(name), plural(assistant.Sessions, "session"))
		}
	}
	b.WriteString("\n## Most active days\n\n")
	type activeDay struct {
		date    string
		commits int64
	}
	var days []activeDay
	for _, day := range calendar {
		if day.Day >= monday.Format(dayLayout) && day.Day <= today && day.Commits > 0 {
			days = append(days, activeDay{date: day.Day, commits: day.Commits})
		}
	}
	sort.SliceStable(days, func(i, j int) bool {
		if days[i].commits == days[j].commits {
			return days[i].date < days[j].date
		}
		return days[i].commits > days[j].commits
	})
	if len(days) > 3 {
		days = days[:3]
	}
	if len(days) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, day := range days {
			fmt.Fprintf(&b, "- %s — %s\n", day.date, plural(day.commits, "commit"))
		}
	}
	return []byte(b.String()), nil
}

// Write atomically replaces an existing generated report and refuses unrelated files.
func Write(vault string, content []byte) error {
	if err := os.MkdirAll(vault, 0755); err != nil {
		return err
	}
	target := filepath.Join(vault, reportName)
	if old, err := os.ReadFile(target); err == nil {
		if !strings.Contains(string(old), "generated_by: observatoire") {
			return fmt.Errorf("refusing to overwrite %s: front matter generated_by: observatoire is missing", target)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(vault, ".Statistics-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(content); err != nil {
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

func grouped(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if negative {
		return "-" + s
	}
	return s
}

func value(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	s := fmt.Sprint(v)
	if s == "" || s == "<nil>" {
		return fallback
	}
	return s
}

func link(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "]", "") }

// first keeps the first n characters (the date of an ISO timestamp).
func first(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func plural(n int64, word string) string {
	if n != 1 {
		word += "s"
	}
	return grouped(n) + " " + word
}
