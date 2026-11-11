-- Indexes proposed after reading the plans (internal/sql/queries/INDEX_PROPOSED.md).

-- +goose Up
-- Calendar and PeriodStats filter sessions by date range.
CREATE INDEX session_date_project ON session(date, project_id);
-- AIActivity, ProjectsByAI, and AISessionsByMonth group by AI then date (covering index: the table is not read).
CREATE INDEX session_ai_date_project ON session(ai, date, project_id);

-- +goose Down
DROP INDEX session_ai_date_project;
DROP INDEX session_date_project;
