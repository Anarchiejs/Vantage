-- name: ListProjects :many
-- Rank aggregated languages at each repository's latest measurement.
WITH code_langs AS (
 SELECT d.project_id, m.language AS lang_name, sum(m.code_lines) AS line_count
 FROM repository d JOIN language_stat m ON m.repository_id=d.id
 WHERE m.measured_on=(SELECT max(m2.measured_on) FROM language_stat m2 WHERE m2.repository_id=d.id)
 GROUP BY d.project_id, m.language
), ranked_langs AS (
 -- Data and documentation formats (JSON, YAML, Markdown...) only win when the project has no code at all.
 SELECT project_id, lang_name, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY
   lang_name IN ('JSON','JSONL','YAML','XML','CSV','TSV','SVG','Markdown','Plain Text','Text','License','TOML','INI',
               'gitignore','Docker ignore','ReStructuredText','AsciiDoc','Properties File','Jupyter') ,
   line_count DESC, lang_name) AS place FROM code_langs
)
SELECT p.id, p.name, p.status, count(c.sha) AS commits, max(c.date_utc) AS last_commit,
 ranked_langs.lang_name AS primary_language
FROM project p LEFT JOIN repository d ON d.project_id=p.id LEFT JOIN commit_git c ON c.repository_id=d.id
LEFT JOIN ranked_langs ON ranked_langs.project_id=p.id AND ranked_langs.place=1
GROUP BY p.id ORDER BY p.name;

-- name: ProjectByID :one
-- Primary-key lookup leaves nullable metadata unchanged.
SELECT * FROM project WHERE id=sqlc.arg(id);

-- name: ProjectTechnologies :many
-- The junction primary key selects only this project's technologies.
SELECT t.name FROM project_technology pt JOIN technology t ON t.id=pt.technology_id
WHERE pt.project_id=sqlc.arg(project_id) ORDER BY t.name;

-- name: Timeline :many
-- UNION ALL keeps distinct events that happen at the same instant.
-- A commit title is its subject line: everything before the first line feed.
SELECT c.date_utc AS date, 'commit' AS type,
 CASE WHEN instr(c.message, char(10)) > 0 THEN substr(c.message, 1, instr(c.message, char(10)) - 1) ELSE c.message END AS title,
 c.sha AS ref, CAST(NULL AS TEXT) AS ai
FROM repository d JOIN commit_git c ON c.repository_id=d.id WHERE d.project_id=sqlc.arg(project_id)
UNION ALL
SELECT s.date, 'session', n.title, n.path, s.ai FROM session s JOIN note n ON n.id=s.note_id WHERE s.project_id=sqlc.arg(project_id)
UNION ALL
SELECT j.date, 'journal', j.body, CAST(j.id AS TEXT), j.ai FROM journal j JOIN project p ON p.id=sqlc.arg(project_id)
WHERE j.body LIKE '%[[' || replace(replace(replace(p.name, '\', '\\'), '%', '\%'), '_', '\_') || ']]%' ESCAPE '\'
ORDER BY date DESC, type, ref LIMIT 500;

-- name: WeeklyVolume :many
-- Weekday arithmetic anchors the recursive series on Monday, even on Sundays.
WITH RECURSIVE weeks(week, n) AS (
 SELECT date(sqlc.arg(today), '-' || ((CAST(strftime('%w', sqlc.arg(today)) AS INTEGER)+6)%7) || ' days', '-357 days') AS week, 1 AS n
 UNION ALL SELECT date(week, '+7 days'), n+1 FROM weeks WHERE n<52
)
SELECT CAST(week AS TEXT) AS week, CAST(coalesce(sum(a.commit_count),0) AS INTEGER) AS commits,
 CAST(coalesce(sum(a.additions),0) AS INTEGER) AS additions, CAST(coalesce(sum(a.deletions),0) AS INTEGER) AS deletions
FROM weeks LEFT JOIN daily_activity a ON a.project_id=sqlc.arg(project_id) AND a.day>=week
 AND a.day<date(week,'+7 days') AND a.day<=sqlc.arg(today)
GROUP BY week ORDER BY week;

-- name: LatestLanguages :many
-- Aggregate repositories before dividing by the window total; zero code has zero share.
WITH latest AS (
 SELECT m.language, sum(m.code_lines) AS code_lines FROM repository d JOIN language_stat m ON m.repository_id=d.id
 WHERE d.project_id=sqlc.arg(project_id) AND m.measured_on=(SELECT max(m2.measured_on) FROM language_stat m2 WHERE m2.repository_id=d.id)
 GROUP BY m.language
)
SELECT language, CAST(code_lines AS INTEGER) AS code_lines,
 CAST(coalesce(1.0*code_lines/nullif(SUM(code_lines) OVER (),0),0.0) AS REAL) AS share
FROM latest ORDER BY code_lines DESC, language;

-- name: LanguageEvolution :many
-- Sum repositories that share a project, date, and language.
SELECT m.measured_on, m.language, CAST(sum(m.code_lines) AS INTEGER) AS code_lines
FROM repository d JOIN language_stat m ON m.repository_id=d.id WHERE d.project_id=sqlc.arg(project_id)
GROUP BY m.measured_on, m.language ORDER BY m.measured_on, m.language;

-- name: Hotspots :many
-- Start from the project's repositories, then indexed commits and changed files.
SELECT f.path, count(*) AS modifications, CAST(coalesce(sum(f.additions),0) AS INTEGER) AS additions,
 CAST(coalesce(sum(f.deletions),0) AS INTEGER) AS deletions
FROM repository d JOIN commit_git c ON c.repository_id=d.id JOIN changed_file f ON f.repository_id=c.repository_id AND f.commit_sha=c.sha
WHERE d.project_id=sqlc.arg(project_id) GROUP BY f.path ORDER BY modifications DESC, f.path LIMIT 20;

-- name: ProjectAI :many
-- Ignore unassigned AI values; their sessions still count in other totals.
SELECT ai, count(*) AS sessions, min(date) AS first_on, max(date) AS last_on FROM session
WHERE project_id=sqlc.arg(project_id) AND ai IS NOT NULL GROUP BY ai ORDER BY ai;

-- name: ProjectNotes :many
-- Match both the title and the project folder so same-titled notes elsewhere are excluded.
SELECT n.title FROM project p JOIN note n ON n.path IN
 ('Projects/' || p.name || '/' || p.name || ' - Summary.md', 'Projects/' || p.name || '/' || p.name || ' - Detail.md')
WHERE p.id=sqlc.arg(project_id) ORDER BY n.title;

-- name: ProjectRepositories :many
-- Last imported commit of each repository for this project (card health).
SELECT id, path, branch, last_imported_sha FROM repository WHERE project_id = ? ORDER BY path;

-- name: CommitsSinceSession :many
-- Commits strictly after the latest session date. No session means no rows.
SELECT c.date_utc AS date, c.sha AS ref,
 CASE WHEN instr(c.message, char(10)) > 0 THEN substr(c.message, 1, instr(c.message, char(10)) - 1) ELSE c.message END AS title,
 (SELECT max(s.date) FROM session s WHERE s.project_id = sqlc.arg(project_id)) AS since
FROM repository d JOIN commit_git c ON c.repository_id = d.id
WHERE d.project_id = sqlc.arg(project_id)
 AND c.local_day > coalesce((SELECT max(s.date) FROM session s WHERE s.project_id = sqlc.arg(project_id)), '9999-12-31')
ORDER BY c.date_utc DESC
LIMIT 30;
