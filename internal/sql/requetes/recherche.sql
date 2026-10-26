-- name: Rechercher :many
-- MATCH uses the FTS index; relational joins supply optional project/date metadata.
-- Prefix comparison treats project names literally, including LIKE metacharacters.
SELECT CAST(recherche.source AS TEXT) AS source,CAST(recherche.ref AS TEXT) AS ref,
 CAST(highlight(recherche,0,'<mark>','</mark>') AS TEXT) AS titre,
 CAST(snippet(recherche,1,'<mark>','</mark>',char(8230),12) AS TEXT) AS extrait,
 p.nom AS projet, CASE WHEN c.sha IS NOT NULL THEN c.date_utc ELSE n.maj END AS date,n.id AS id_note
FROM recherche(sqlc.arg(termes)) AS recherche
LEFT JOIN commit_git c ON recherche.source='commit' AND c.sha=recherche.ref
LEFT JOIN depot d ON d.id=c.depot_id
LEFT JOIN note n ON recherche.source='note' AND n.chemin=recherche.ref
LEFT JOIN session s ON s.note_id=n.id
LEFT JOIN projet p ON p.id=coalesce(d.projet_id,s.projet_id) OR
 (d.projet_id IS NULL AND s.projet_id IS NULL AND substr(n.chemin,1,length('Projets/' || p.nom || '/'))='Projets/' || p.nom || '/')
WHERE (sqlc.narg(source) IS NULL OR recherche.source=sqlc.narg(source))
 AND (sqlc.narg(projet) IS NULL OR p.nom=sqlc.narg(projet))
 AND (sqlc.narg(dossier) IS NULL OR n.dossier=sqlc.narg(dossier))
 AND (sqlc.narg(depuis) IS NULL OR coalesce(c.date_utc,n.maj)>=sqlc.narg(depuis))
 AND (sqlc.narg(jusqua) IS NULL OR coalesce(c.date_utc,n.maj)<date(sqlc.narg(jusqua),'+1 day'))
ORDER BY bm25(recherche),recherche.source,recherche.ref LIMIT sqlc.arg(limite);
