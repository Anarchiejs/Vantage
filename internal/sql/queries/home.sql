-- name: Calendar :many
-- Recursive calendar. Scalar aggregates avoid multiplying sessions by projects.
WITH RECURSIVE days(day) AS (
 SELECT date(sqlc.arg(today), '-364 days') AS day
 UNION ALL SELECT date(day, '+1 day') FROM days WHERE day < date(sqlc.arg(today))
)
SELECT CAST(day AS TEXT) AS day,
 CAST(coalesce((SELECT sum(commit_count) FROM daily_activity a WHERE a.day = days.day),0) AS INTEGER) AS commits,
 CAST(coalesce((SELECT sum(additions) FROM daily_activity a WHERE a.day = days.day),0) AS INTEGER) AS additions,
 CAST(coalesce((SELECT sum(deletions) FROM daily_activity a WHERE a.day = days.day),0) AS INTEGER) AS deletions,
 (SELECT count(*) FROM session s WHERE s.date >= days.day AND s.date < date(days.day, '+1 day')) AS sessions,
 CAST(coalesce((SELECT group_concat(name, char(31)) FROM (
   SELECT DISTINCT p.name FROM project p WHERE p.id IN (
     SELECT a.project_id FROM daily_activity a WHERE a.day = days.day AND a.commit_count > 0
     UNION SELECT s.project_id FROM session s WHERE s.date >= days.day AND s.date < date(days.day, '+1 day')
   ) ORDER BY p.name
 )), '') AS TEXT) AS projects
FROM days ORDER BY day;

-- name: PeriodStats :one
-- An independent scalar session aggregate keeps the daily totals intact.
SELECT CAST(coalesce(sum(commit_count),0) AS INTEGER) AS commits,
 CAST(coalesce(sum(additions),0) AS INTEGER) AS additions,
 CAST(coalesce(sum(deletions),0) AS INTEGER) AS deletions,
 (SELECT count(*) FROM session WHERE date >= sqlc.arg(start_date) AND date < date(sqlc.arg(end_date), '+1 day')) AS sessions,
 count(DISTINCT CASE WHEN commit_count > 0 THEN project_id END) AS active_projects
FROM daily_activity WHERE day BETWEEN sqlc.arg(start_date) AND sqlc.arg(end_date);

-- name: ActiveProjects :many
-- Thirty inclusive local days, with the latest timestamp from the whole history.
SELECT p.id, p.name, (SELECT max(c.date_utc) FROM repository d JOIN commit_git c ON c.repository_id=d.id WHERE d.project_id=p.id) AS last_commit,
 CAST(sum(a.commit_count) AS INTEGER) AS commits_30d
FROM project p JOIN daily_activity a ON a.project_id=p.id
WHERE a.day BETWEEN date(sqlc.arg(today), '-29 days') AND sqlc.arg(today)
GROUP BY p.id HAVING sum(a.commit_count)>0 ORDER BY commits_30d DESC, p.name;

-- name: DormantProjects :many
-- LEFT JOIN keeps projects that have never received a commit; -1 means never.
SELECT p.id, p.name, max(c.date_utc) AS last_commit,
 CAST(coalesce(julianday(sqlc.arg(today))-julianday(max(c.local_day)), -1) AS INTEGER) AS days
FROM project p LEFT JOIN repository d ON d.project_id=p.id
LEFT JOIN commit_git c ON c.repository_id=d.id AND c.local_day <= sqlc.arg(today)
WHERE lower(trim(p.status)) IN ('in progress', 'active', 'ongoing') GROUP BY p.id
HAVING max(c.local_day) IS NULL OR max(c.local_day)<date(sqlc.arg(today), '-30 days')
ORDER BY p.name;

-- name: QuietOutsideStatus :one
-- Quiet cards the dormant list hides because their status is outside the watched set.
SELECT count(*) FROM (
  SELECT p.id
  FROM project p
  LEFT JOIN repository d ON d.project_id = p.id
  LEFT JOIN commit_git c ON c.repository_id = d.id AND c.local_day <= sqlc.arg(today)
  WHERE lower(trim(coalesce(p.status, ''))) NOT IN ('in progress', 'active', 'ongoing')
  GROUP BY p.id
  HAVING max(c.local_day) IS NULL OR max(c.local_day) < date(sqlc.arg(today), '-30 days')
);

-- name: StaleCards :many
-- Prefix matching supports abbreviated SHAs; an unknown verification returns -1.
SELECT p.id, p.name, p.verified_commit, d.last_imported_sha AS last_sha,
 CAST(CASE WHEN v.sha IS NULL THEN -1 ELSE
 (SELECT count(*) FROM commit_git c WHERE c.repository_id=d.id AND c.date_utc>v.date_utc) END AS INTEGER) AS commits_since
FROM project p JOIN repository d ON d.project_id=p.id
LEFT JOIN commit_git v ON v.sha=(SELECT c.sha FROM commit_git c WHERE c.repository_id=d.id
 AND length(p.verified_commit)>0 AND substr(c.sha,1,length(p.verified_commit))=p.verified_commit ORDER BY c.sha LIMIT 1)
WHERE d.last_imported_sha IS NOT NULL AND
 (p.verified_commit IS NULL OR p.verified_commit='' OR substr(d.last_imported_sha,1,length(p.verified_commit))<>p.verified_commit)
ORDER BY p.name, d.id;
