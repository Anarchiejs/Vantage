-- name: GraphNodes :many
-- Count unique resolved directed edges, excluding self-loops, same as GraphLinks.
SELECT n.id,n.title,n.folder,
 (SELECT count(DISTINCT target_id) FROM link WHERE source_id=n.id AND target_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM link WHERE target_id=n.id AND source_id<>n.id) AS degree
FROM note n ORDER BY n.id;

-- name: GraphLinks :many
-- DISTINCT merges aliases that resolve to the same directed edge.
SELECT DISTINCT source_id AS source,target_id AS target FROM link
WHERE target_id IS NOT NULL AND source_id<>target_id ORDER BY source_id,target_id;

-- name: BrokenLinks :many
-- The partial index selects unresolved links; the source uses its primary key.
SELECT l.source_id AS source,n.title AS source_title,l.target_title
FROM link l JOIN note n ON n.id=l.source_id WHERE l.target_id IS NULL ORDER BY l.source_id,l.target_title;

-- name: NoteByID :one
-- FTS stores note content keyed by its relative path.
SELECT n.id,n.title,n.path,n.folder,n.updated,
 CAST(coalesce((SELECT r.content FROM search_index r WHERE r.source='note' AND r.ref=n.path LIMIT 1),'') AS TEXT) AS content
FROM note n WHERE n.id=sqlc.arg(id);

-- name: Neighbors :many
-- UNION deduplicates neighbors reached in either direction.
SELECT n.id,n.title,n.folder,
 (SELECT count(DISTINCT target_id) FROM link WHERE source_id=n.id AND target_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM link WHERE target_id=n.id AND source_id<>n.id) AS degree
FROM note n WHERE n.id<>sqlc.arg(id) AND n.id IN (
 SELECT target_id FROM link WHERE source_id=sqlc.arg(id) AND target_id IS NOT NULL
 UNION SELECT source_id FROM link WHERE target_id=sqlc.arg(id)
) ORDER BY n.id;

-- name: ShortestPath :many
-- Breadth-first recursion (queue ordered by depth), with a slash-delimited visited set.
-- Split only the shortest selected path, preserving traversal order.
WITH RECURSIVE walk(id,trail,depth) AS (
 SELECT note.id,'/' || note.id || '/' AS trail,0 AS depth FROM note WHERE note.id=sqlc.arg(start_id)
 UNION
 SELECT n.id,w.trail || n.id || '/',w.depth+1
 FROM walk w JOIN link l ON (l.source_id=w.id OR l.target_id=w.id) AND l.target_id IS NOT NULL
 JOIN note n ON n.id=CASE WHEN l.source_id=w.id THEN l.target_id ELSE l.source_id END
 WHERE w.depth<6 AND w.id<>(SELECT CAST(sqlc.arg(end_id) AS INTEGER)) AND instr(w.trail,'/' || n.id || '/')=0
 ORDER BY 3
), best AS (
 SELECT trail FROM walk WHERE id=sqlc.arg(end_id) ORDER BY depth,trail LIMIT 1
), parts(rest,id,position) AS (
 SELECT substr(trail,2) AS rest,CAST(NULL AS INTEGER) AS id,0 AS position FROM best
 UNION ALL
 SELECT substr(rest,instr(rest,'/')+1),CAST(substr(rest,1,instr(rest,'/')-1) AS INTEGER),position+1
 FROM parts WHERE rest<>''
)
SELECT n.id,n.title,n.folder,
 (SELECT count(DISTINCT target_id) FROM link WHERE source_id=n.id AND target_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM link WHERE target_id=n.id AND source_id<>n.id) AS degree
FROM parts p JOIN note n ON n.id=p.id ORDER BY p.position;
