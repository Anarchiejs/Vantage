-- name: Search :many
-- MATCH uses the FTS index; relational joins supply optional project and date metadata.
-- Prefix comparison treats project names literally, including LIKE metacharacters.
SELECT CAST(search_index.source AS TEXT) AS source,CAST(search_index.ref AS TEXT) AS ref,
 CAST(highlight(search_index,0,char(57344),char(57345)) AS TEXT) AS title,
 CAST(snippet(search_index,1,char(57344),char(57345),char(8230),12) AS TEXT) AS excerpt,
 p.name AS project, CASE WHEN c.sha IS NOT NULL THEN c.date_utc ELSE n.updated END AS date,n.id AS note_id
FROM search_index(sqlc.arg(terms)) AS search_index
LEFT JOIN commit_git c ON search_index.source='commit' AND printf('%d:%s', c.repository_id, c.sha)=search_index.ref
LEFT JOIN repository d ON d.id=c.repository_id
LEFT JOIN note n ON search_index.source='note' AND n.path=search_index.ref
LEFT JOIN session s ON s.note_id=n.id
LEFT JOIN project p ON p.id=coalesce(d.project_id,s.project_id) OR
 (d.project_id IS NULL AND s.project_id IS NULL AND substr(n.path,1,length('Projects/' || p.name || '/'))='Projects/' || p.name || '/')
WHERE (sqlc.narg(source) IS NULL OR search_index.source=sqlc.narg(source))
 AND (sqlc.narg(project) IS NULL OR p.name=sqlc.narg(project))
 AND (sqlc.narg(folder) IS NULL OR n.folder=sqlc.narg(folder))
 AND (sqlc.narg(since) IS NULL OR coalesce(c.date_utc,n.updated)>=sqlc.narg(since))
 AND (sqlc.narg(until) IS NULL OR coalesce(c.date_utc,n.updated)<date(sqlc.narg(until),'+1 day'))
ORDER BY bm25(search_index),search_index.source,search_index.ref LIMIT sqlc.arg(row_limit);
