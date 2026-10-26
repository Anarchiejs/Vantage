-- name: NoeudsGraphe :many
-- Count unique resolved directed edges, excluding self-loops like LiensGraphe.
SELECT n.id,n.titre,n.dossier,
 (SELECT count(DISTINCT cible_id) FROM lien WHERE source_id=n.id AND cible_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM lien WHERE cible_id=n.id AND source_id<>n.id) AS degre
FROM note n ORDER BY n.id;

-- name: LiensGraphe :many
-- DISTINCT merges aliases that resolve to the same directed edge.
SELECT DISTINCT source_id AS source,cible_id AS cible FROM lien
WHERE cible_id IS NOT NULL AND source_id<>cible_id ORDER BY source_id,cible_id;

-- name: LiensCasses :many
-- Partial index selects unresolved links; the source uses its primary key.
SELECT l.source_id AS source,n.titre AS source_titre,l.cible_titre
FROM lien l JOIN note n ON n.id=l.source_id WHERE l.cible_id IS NULL ORDER BY l.source_id,l.cible_titre;

-- name: NoteParID :one
-- FTS stores note content keyed by its relative path.
SELECT n.id,n.titre,n.chemin,n.dossier,n.maj,
 CAST(coalesce((SELECT r.contenu FROM recherche r WHERE r.source='note' AND r.ref=n.chemin LIMIT 1),'') AS TEXT) AS contenu
FROM note n WHERE n.id=sqlc.arg(id);

-- name: Voisins :many
-- UNION deduplicates neighbors reached in either direction.
SELECT n.id,n.titre,n.dossier,
 (SELECT count(DISTINCT cible_id) FROM lien WHERE source_id=n.id AND cible_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM lien WHERE cible_id=n.id AND source_id<>n.id) AS degre
FROM note n WHERE n.id<>sqlc.arg(id) AND n.id IN (
 SELECT cible_id FROM lien WHERE source_id=sqlc.arg(id) AND cible_id IS NOT NULL
 UNION SELECT source_id FROM lien WHERE cible_id=sqlc.arg(id)
) ORDER BY n.id;

-- name: PlusCourtChemin :many
-- Breadth-first recursion (queue ordered by depth), with a slash-delimited visited set.
-- Split only the shortest selected path, preserving traversal order.
WITH RECURSIVE parcours(id,chemin,profondeur) AS (
 SELECT note.id,'/' || note.id || '/' AS chemin,0 AS profondeur FROM note WHERE note.id=sqlc.arg(depart)
 UNION
 SELECT n.id,p.chemin || n.id || '/',p.profondeur+1
 FROM parcours p JOIN lien l ON (l.source_id=p.id OR l.cible_id=p.id) AND l.cible_id IS NOT NULL
 JOIN note n ON n.id=CASE WHEN l.source_id=p.id THEN l.cible_id ELSE l.source_id END
 WHERE p.profondeur<6 AND p.id<>(SELECT CAST(sqlc.arg(arrivee) AS INTEGER)) AND instr(p.chemin,'/' || n.id || '/')=0
 ORDER BY 3
), meilleur AS (
 SELECT chemin FROM parcours WHERE id=sqlc.arg(arrivee) ORDER BY profondeur,chemin LIMIT 1
), morceaux(reste,id,position) AS (
 SELECT substr(chemin,2) AS reste,CAST(NULL AS INTEGER) AS id,0 AS position FROM meilleur
 UNION ALL
 SELECT substr(reste,instr(reste,'/')+1),CAST(substr(reste,1,instr(reste,'/')-1) AS INTEGER),position+1
 FROM morceaux WHERE reste<>''
)
SELECT n.id,n.titre,n.dossier,
 (SELECT count(DISTINCT cible_id) FROM lien WHERE source_id=n.id AND cible_id<>n.id) +
 (SELECT count(DISTINCT source_id) FROM lien WHERE cible_id=n.id AND source_id<>n.id) AS degre
FROM morceaux m JOIN note n ON n.id=m.id ORDER BY m.position;
