-- name: Calendrier :many
-- Recursive calendar, scalar aggregates avoid multiplying sessions by projects.
WITH RECURSIVE jours(jour) AS (
 SELECT date(sqlc.arg(aujourdhui), '-364 days') AS jour
 UNION ALL SELECT date(jour, '+1 day') FROM jours WHERE jour < date(sqlc.arg(aujourdhui))
)
SELECT CAST(jour AS TEXT) AS jour,
 CAST(coalesce((SELECT sum(nb_commits) FROM activite_jour a WHERE a.jour = jours.jour),0) AS INTEGER) AS commits,
 CAST(coalesce((SELECT sum(ajouts) FROM activite_jour a WHERE a.jour = jours.jour),0) AS INTEGER) AS ajouts,
 CAST(coalesce((SELECT sum(suppressions) FROM activite_jour a WHERE a.jour = jours.jour),0) AS INTEGER) AS suppressions,
 (SELECT count(*) FROM session s WHERE s.date >= jours.jour AND s.date < date(jours.jour, '+1 day')) AS sessions,
 CAST(coalesce((SELECT group_concat(nom, char(31)) FROM (
   SELECT DISTINCT p.nom FROM projet p WHERE p.id IN (
     SELECT a.projet_id FROM activite_jour a WHERE a.jour = jours.jour AND a.nb_commits > 0
     UNION SELECT s.projet_id FROM session s WHERE s.date >= jours.jour AND s.date < date(jours.jour, '+1 day')
   ) ORDER BY p.nom
 )), '') AS TEXT) AS projets
FROM jours ORDER BY jour;

-- name: ChiffresPeriode :one
-- Independent scalar session aggregate keeps the daily totals intact.
SELECT CAST(coalesce(sum(nb_commits),0) AS INTEGER) AS commits,
 CAST(coalesce(sum(ajouts),0) AS INTEGER) AS ajouts,
 CAST(coalesce(sum(suppressions),0) AS INTEGER) AS suppressions,
 (SELECT count(*) FROM session WHERE date >= sqlc.arg(debut) AND date < date(sqlc.arg(fin), '+1 day')) AS sessions,
 count(DISTINCT CASE WHEN nb_commits > 0 THEN projet_id END) AS projets_actifs
FROM activite_jour WHERE jour BETWEEN sqlc.arg(debut) AND sqlc.arg(fin);

-- name: ProjetsActifs :many
-- Thirty inclusive local days, with the latest timestamp from the whole history.
SELECT p.id, p.nom, (SELECT max(c.date_utc) FROM depot d JOIN commit_git c ON c.depot_id=d.id WHERE d.projet_id=p.id) AS dernier_commit,
 CAST(sum(a.nb_commits) AS INTEGER) AS commits_30j
FROM projet p JOIN activite_jour a ON a.projet_id=p.id
WHERE a.jour BETWEEN date(sqlc.arg(aujourdhui), '-29 days') AND sqlc.arg(aujourdhui)
GROUP BY p.id HAVING sum(a.nb_commits)>0 ORDER BY commits_30j DESC, p.nom;

-- name: ProjetsDormants :many
-- LEFT JOIN retains projects that have never received a commit; -1 means never.
SELECT p.id, p.nom, max(c.date_utc) AS dernier_commit,
 CAST(coalesce(julianday(sqlc.arg(aujourdhui))-julianday(max(c.jour_local)), -1) AS INTEGER) AS jours
FROM projet p LEFT JOIN depot d ON d.projet_id=p.id
LEFT JOIN commit_git c ON c.depot_id=d.id AND c.jour_local <= sqlc.arg(aujourdhui)
WHERE p.statut='en cours' GROUP BY p.id
HAVING max(c.jour_local) IS NULL OR max(c.jour_local)<date(sqlc.arg(aujourdhui), '-30 days')
ORDER BY p.nom;

-- name: FichesPerimees :many
-- Prefix matching supports abbreviated SHAs; unknown verification returns -1.
SELECT p.id, p.nom, p.verifie_commit, d.dernier_sha_importe AS dernier_sha,
 CAST(CASE WHEN v.sha IS NULL THEN -1 ELSE
 (SELECT count(*) FROM commit_git c WHERE c.depot_id=d.id AND c.date_utc>v.date_utc) END AS INTEGER) AS commits_depuis
FROM projet p JOIN depot d ON d.projet_id=p.id
LEFT JOIN commit_git v ON v.sha=(SELECT c.sha FROM commit_git c WHERE c.depot_id=d.id
 AND length(p.verifie_commit)>0 AND substr(c.sha,1,length(p.verifie_commit))=p.verifie_commit ORDER BY c.sha LIMIT 1)
WHERE d.dernier_sha_importe IS NOT NULL AND
 (p.verifie_commit IS NULL OR p.verifie_commit='' OR substr(d.dernier_sha_importe,1,length(p.verifie_commit))<>p.verifie_commit)
ORDER BY p.nom, d.id;
