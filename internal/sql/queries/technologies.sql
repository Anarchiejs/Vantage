-- name: SankeyLinks :many
-- Every project -> technology link weighs 1. Weighting by commits let one huge repository
-- (thousands of upstream commits) flatten every other project into an unreadable sliver.
SELECT p.name AS source, t.name AS target, CAST(1 AS INTEGER) AS value
FROM project_technology pt JOIN project p ON p.id=pt.project_id JOIN technology t ON t.id=pt.technology_id ORDER BY p.name, t.name;

-- name: TechnologyTrend :many
-- A calendar cross join keeps empty months and unused technologies.
WITH RECURSIVE months(month_start,n) AS (
 SELECT date(sqlc.arg(today),'start of month','-11 months') AS month_start,1 AS n
 UNION ALL SELECT date(month_start,'+1 month'),n+1 FROM months WHERE n<12
)
SELECT CAST(months.month_start AS TEXT) AS month, t.name AS technology, CAST(coalesce((
 SELECT sum(a.commit_count) FROM project_technology pt JOIN daily_activity a ON a.project_id=pt.project_id
 WHERE pt.technology_id=t.id AND a.day>=months.month_start AND a.day<date(months.month_start,'+1 month') AND a.day<=sqlc.arg(today)
),0) AS INTEGER) AS commits FROM months CROSS JOIN technology t ORDER BY months.month_start,t.name;

-- name: UnassignedTechnologies :many
-- The anti-join through NOT EXISTS uses the reverse junction index.
SELECT t.name FROM technology t WHERE NOT EXISTS (SELECT 1 FROM project_technology pt WHERE pt.technology_id=t.id) ORDER BY t.name;

-- name: AIActivity :many
-- One row per identified assistant.
SELECT ai, count(*) AS sessions, min(date) AS first_on, max(date) AS last_on
FROM session WHERE ai IS NOT NULL GROUP BY ai ORDER BY ai;

-- name: ProjectsByAI :many
-- DISTINCT removes repeated sessions on the same project.
SELECT DISTINCT s.ai, p.id AS project_id, p.name FROM session s JOIN project p ON p.id=s.project_id
WHERE s.ai IS NOT NULL ORDER BY s.ai,p.name;

-- name: AISessionsByMonth :many
-- Twelve inclusive months for each identified AI, including empty months.
WITH RECURSIVE months(month_start,n) AS (
 SELECT date(sqlc.arg(today),'start of month','-11 months') AS month_start,1 AS n
 UNION ALL SELECT date(month_start,'+1 month'),n+1 FROM months WHERE n<12
), ais AS (SELECT DISTINCT ai FROM session WHERE ai IS NOT NULL)
SELECT ais.ai, CAST(months.month_start AS TEXT) AS month, count(s.note_id) AS sessions FROM months CROSS JOIN ais
LEFT JOIN session s ON s.ai=ais.ai AND s.date>=months.month_start AND s.date<date(months.month_start,'+1 month') AND s.date<date(sqlc.arg(today),'+1 day')
GROUP BY ais.ai,months.month_start ORDER BY ais.ai,months.month_start;

-- name: AISessionsInPeriod :many
-- Sessions per AI between two inclusive dates (weekly report). Uses session_date_project.
SELECT ai, CAST(count(*) AS INTEGER) AS sessions
FROM session
WHERE ai IS NOT NULL AND date >= CAST(sqlc.arg(start_date) AS TEXT) AND date <= CAST(sqlc.arg(end_date) AS TEXT)
GROUP BY ai
ORDER BY sessions DESC, ai;
