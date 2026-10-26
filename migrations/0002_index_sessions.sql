-- Index proposés par le lot S2 après lecture des plans (internal/sql/requetes/INDEX_PROPOSES.md).

-- +goose Up
-- Calendrier et ChiffresPeriode filtrent les sessions par plage de dates.
CREATE INDEX session_date_projet ON session(date, projet_id);
-- ActiviteIA, ProjetsParIA, SessionsIAParMois groupent par IA puis par date (index couvrant : la table n'est pas lue).
CREATE INDEX session_ia_date_projet ON session(ia, date, projet_id);

-- +goose Down
DROP INDEX session_ia_date_projet;
DROP INDEX session_date_projet;
