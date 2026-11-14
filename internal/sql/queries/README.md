# Query contract

- 35 named queries, generated only with sqlc 1.31.1.
- Search uses `FROM search_index(sqlc.arg(terms))`, FTS5 syntax equivalent to `WHERE search_index MATCH ...`. sqlc rejects the hidden column that has the table name in the second syntax. The plan confirms MATCH `0:M4`.
- `char(8230)` is the requested snippet separator (U+2026), while SQL sources stay ASCII to avoid sqlc's truncation problem.
- Nullable computed fields may be `interface{}` in sqlc structs (notably Search.Date); they contain a string or nil. The API converts them to its contract. ProjectAI expects a `*int64` (nullable column).
- Calendar.Projects is a string separated by char(31), empty when there is no project. Sessions also touch a project, even without a commit that day.
- DormantProjects.Days is -1 for a project with no commit; only projects whose status is exactly `in progress` are selected.
- Graph links are directed, deduplicated, and without loops; degree is the sum of incoming and outgoing degrees on those same links. Path search follows them in both directions, at most six edges, including the start.
- The latest measurements are chosen per repository before aggregation to the project. Language shares are fractions between 0 and 1, zero when the total is zero.
- Views without a time bound (timeline, AI, health) read the whole history. Bounded series do not include future days of the current month or week.
- StaleCards returns one row per stale repository. An abbreviated SHA is accepted; the number of later commits compares UTC dates in the same repository.

# Checking expectations

`go run ./scripts/fixtures/make-repos.go` then
`go test ./internal/sql/queries_test -count=1 -args -update` regenerates the JSON files and
PLANS.md. Without `-update`, the tests do not modify any reference file.
