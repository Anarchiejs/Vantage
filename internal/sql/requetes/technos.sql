-- name: SankeyLiens :many
-- Every project -> technology link weighs 1: weighting by commits let one huge repository
-- (thousands of upstream commits) flatten every other project into an unreadable sliver.
SELECT p.nom AS source, t.nom AS cible, CAST(1 AS INTEGER) AS valeur
FROM projet_techno pt JOIN projet p ON p.id=pt.projet_id JOIN techno t ON t.id=pt.techno_id ORDER BY p.nom, t.nom;

-- name: TendanceTechnos :many
-- Calendar cross join technologies retains empty months and unused technologies.
WITH RECURSIVE mois(mois,n) AS (
 SELECT date(sqlc.arg(aujourdhui),'start of month','-11 months') AS mois,1 AS n
 UNION ALL SELECT date(mois,'+1 month'),n+1 FROM mois WHERE n<12
)
SELECT CAST(mois.mois AS TEXT) AS mois, t.nom AS techno, CAST(coalesce((
 SELECT sum(a.nb_commits) FROM projet_techno pt JOIN activite_jour a ON a.projet_id=pt.projet_id
 WHERE pt.techno_id=t.id AND a.jour>=mois.mois AND a.jour<date(mois.mois,'+1 month') AND a.jour<=sqlc.arg(aujourdhui)
),0) AS INTEGER) AS commits FROM mois CROSS JOIN techno t ORDER BY mois.mois,t.nom;

-- name: TechnosSansProjet :many
-- Anti-join through NOT EXISTS uses the reverse junction index.
SELECT t.nom FROM techno t WHERE NOT EXISTS (SELECT 1 FROM projet_techno pt WHERE pt.techno_id=t.id) ORDER BY t.nom;

-- name: ActiviteIA :many
-- One row per identified assistant.
SELECT ia, count(*) AS sessions, min(date) AS premiere, max(date) AS derniere
FROM session WHERE ia IS NOT NULL GROUP BY ia ORDER BY ia;

-- name: ProjetsParIA :many
-- DISTINCT removes repeated sessions on the same project.
SELECT DISTINCT s.ia, p.id AS projet_id, p.nom FROM session s JOIN projet p ON p.id=s.projet_id
WHERE s.ia IS NOT NULL ORDER BY s.ia,p.nom;

-- name: SessionsIAParMois :many
-- Twelve inclusive months for each identified IA, including empty months.
WITH RECURSIVE mois(mois,n) AS (
 SELECT date(sqlc.arg(aujourdhui),'start of month','-11 months') AS mois,1 AS n
 UNION ALL SELECT date(mois,'+1 month'),n+1 FROM mois WHERE n<12
), ias AS (SELECT DISTINCT ia FROM session WHERE ia IS NOT NULL)
SELECT ias.ia, CAST(mois.mois AS TEXT) AS mois, count(s.note_id) AS sessions FROM mois CROSS JOIN ias
LEFT JOIN session s ON s.ia=ias.ia AND s.date>=mois.mois AND s.date<date(mois.mois,'+1 month') AND s.date<date(sqlc.arg(aujourdhui),'+1 day')
GROUP BY ias.ia,mois.mois ORDER BY ias.ia,mois.mois;

-- name: SessionsIAPeriode :many
-- Sessions par IA entre deux dates incluses (rapport hebdomadaire). Utilise session_date_projet.
SELECT ia, CAST(count(*) AS INTEGER) AS sessions
FROM session
WHERE ia IS NOT NULL AND date >= CAST(sqlc.arg(debut) AS TEXT) AND date <= CAST(sqlc.arg(fin) AS TEXT)
GROUP BY ia
ORDER BY sessions DESC, ia;
