-- name: ListerProjets :many
-- Rank aggregated languages at each repository's latest measurement.
WITH langages AS (
 SELECT d.projet_id, m.langage, sum(m.lignes_code) AS lignes
 FROM depot d JOIN mesure_langage m ON m.depot_id=d.id
 WHERE m.date_mesure=(SELECT max(m2.date_mesure) FROM mesure_langage m2 WHERE m2.depot_id=d.id)
 GROUP BY d.projet_id, m.langage
), classes AS (
 -- Data and documentation formats (JSON, YAML, Markdown...) only win when the project has no code at all.
 SELECT projet_id, langage, ROW_NUMBER() OVER (PARTITION BY projet_id ORDER BY
   langage IN ('JSON','JSONL','YAML','XML','CSV','TSV','SVG','Markdown','Plain Text','Text','License','TOML','INI',
               'gitignore','Docker ignore','ReStructuredText','AsciiDoc','Properties File','Jupyter') ,
   lignes DESC, langage) AS rang FROM langages
)
SELECT p.id, p.nom, p.statut, count(c.sha) AS commits, max(c.date_utc) AS dernier_commit,
 cl.langage AS langage_principal
FROM projet p LEFT JOIN depot d ON d.projet_id=p.id LEFT JOIN commit_git c ON c.depot_id=d.id
LEFT JOIN classes cl ON cl.projet_id=p.id AND cl.rang=1
GROUP BY p.id ORDER BY p.nom;

-- name: ProjetParID :one
-- Primary-key lookup leaves nullable metadata unchanged.
SELECT * FROM projet WHERE id=sqlc.arg(id);

-- name: TechnosDuProjet :many
-- Junction primary key selects only this project's technologies.
SELECT t.nom FROM projet_techno pt JOIN techno t ON t.id=pt.techno_id
WHERE pt.projet_id=sqlc.arg(projet_id) ORDER BY t.nom;

-- name: Frise :many
-- UNION ALL preserves distinct events that happen at the same instant.
-- A commit's title is its subject line: everything before the first line feed.
SELECT c.date_utc AS date, 'commit' AS type,
 CASE WHEN instr(c.message, char(10)) > 0 THEN substr(c.message, 1, instr(c.message, char(10)) - 1) ELSE c.message END AS titre,
 c.sha AS ref, CAST(NULL AS TEXT) AS ia
FROM depot d JOIN commit_git c ON c.depot_id=d.id WHERE d.projet_id=sqlc.arg(projet_id)
UNION ALL
SELECT s.date, 'session', n.titre, n.chemin, s.ia FROM session s JOIN note n ON n.id=s.note_id WHERE s.projet_id=sqlc.arg(projet_id)
UNION ALL
SELECT j.date, 'journal', j.texte, CAST(j.id AS TEXT), j.ia FROM journal j JOIN projet p ON p.id=sqlc.arg(projet_id)
WHERE j.texte LIKE '%[[' || replace(replace(replace(p.nom, '\', '\\'), '%', '\%'), '_', '\_') || ']]%' ESCAPE '\'
ORDER BY date DESC, type, ref LIMIT 500;

-- name: VolumeHebdo :many
-- Weekday arithmetic anchors the recursive series on Monday, even on Sundays.
WITH RECURSIVE semaines(semaine, n) AS (
 SELECT date(sqlc.arg(aujourdhui), '-' || ((CAST(strftime('%w', sqlc.arg(aujourdhui)) AS INTEGER)+6)%7) || ' days', '-357 days') AS semaine, 1 AS n
 UNION ALL SELECT date(semaine, '+7 days'), n+1 FROM semaines WHERE n<52
)
SELECT CAST(semaine AS TEXT) AS semaine, CAST(coalesce(sum(a.nb_commits),0) AS INTEGER) AS commits,
 CAST(coalesce(sum(a.ajouts),0) AS INTEGER) AS ajouts, CAST(coalesce(sum(a.suppressions),0) AS INTEGER) AS suppressions
FROM semaines LEFT JOIN activite_jour a ON a.projet_id=sqlc.arg(projet_id) AND a.jour>=semaine
 AND a.jour<date(semaine,'+7 days') AND a.jour<=sqlc.arg(aujourdhui)
GROUP BY semaine ORDER BY semaine;

-- name: LangagesDerniereMesure :many
-- Aggregate repositories before dividing by the window total; zero code has zero share.
WITH derniers AS (
 SELECT m.langage, sum(m.lignes_code) AS lignes_code FROM depot d JOIN mesure_langage m ON m.depot_id=d.id
 WHERE d.projet_id=sqlc.arg(projet_id) AND m.date_mesure=(SELECT max(m2.date_mesure) FROM mesure_langage m2 WHERE m2.depot_id=d.id)
 GROUP BY m.langage
)
SELECT langage, CAST(lignes_code AS INTEGER) AS lignes_code,
 CAST(coalesce(1.0*lignes_code/nullif(SUM(lignes_code) OVER (),0),0.0) AS REAL) AS part
FROM derniers ORDER BY lignes_code DESC, langage;

-- name: LangagesEvolution :many
-- Sum repositories sharing a project, date and language.
SELECT m.date_mesure, m.langage, CAST(sum(m.lignes_code) AS INTEGER) AS lignes_code
FROM depot d JOIN mesure_langage m ON m.depot_id=d.id WHERE d.projet_id=sqlc.arg(projet_id)
GROUP BY m.date_mesure, m.langage ORDER BY m.date_mesure, m.langage;

-- name: PointsChauds :many
-- Start from project repositories, then indexed commits and modified files.
SELECT f.chemin, count(*) AS modifications, CAST(coalesce(sum(f.ajouts),0) AS INTEGER) AS ajouts,
 CAST(coalesce(sum(f.suppressions),0) AS INTEGER) AS suppressions
FROM depot d JOIN commit_git c ON c.depot_id=d.id JOIN fichier_modifie f ON f.commit_sha=c.sha
WHERE d.projet_id=sqlc.arg(projet_id) GROUP BY f.chemin ORDER BY modifications DESC, f.chemin LIMIT 20;

-- name: IAduProjet :many
-- Ignore unassigned IA values; keep their sessions in other totals.
SELECT ia, count(*) AS sessions, min(date) AS premiere, max(date) AS derniere FROM session
WHERE projet_id=sqlc.arg(projet_id) AND ia IS NOT NULL GROUP BY ia ORDER BY ia;

-- name: NotesDuProjet :many
-- Match both title and project folder to exclude same-title notes elsewhere.
SELECT n.titre FROM projet p JOIN note n ON n.chemin IN
 ('Projets/' || p.nom || '/' || p.nom || ' - Resume.md', 'Projets/' || p.nom || '/' || p.nom || ' - Detaille.md')
WHERE p.id=sqlc.arg(projet_id) ORDER BY n.titre;

-- name: DepotsDuProjet :many
-- Dernier commit importe de chaque depot du projet (sante de la fiche).
SELECT id, chemin, dernier_sha_importe FROM depot WHERE projet_id = ? ORDER BY chemin;
