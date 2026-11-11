# Proposed indexes

The plans in PLANS.md are those of the current schema, without these indexes.
They are already created by migration 0002.

```sql
-- Calendar (two subqueries per day) and PeriodStats: a global date range.
CREATE INDEX session_date_project ON session(date, project_id);

-- AISessionsByMonth: group by AI then a date range; also covers
-- AIActivity and ProjectsByAI and avoids the automatic index seen in the plan.
CREATE INDEX session_ai_date_project ON session(ai, date, project_id);
```

NoteByID scans search_index for source/ref: those are UNINDEXED FTS5 columns, and SQLite forbids CREATE INDEX on a virtual table. An ordinary index cannot fix that scan. Search does use the MATCH index (0:M4).
