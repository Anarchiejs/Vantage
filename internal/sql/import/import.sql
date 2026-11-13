-- Import writes. All of them are idempotent: running the import again creates no duplicates.

-- name: UpsertProject :one
-- Create the project or update its card fields; return its id in both cases.
INSERT INTO project (name, status, source_path, card_updated, verified_commit, verified_on, branch, expected_notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE SET
    status = excluded.status,
    source_path = excluded.source_path,
    card_updated = excluded.card_updated,
    verified_commit = excluded.verified_commit,
    verified_on = excluded.verified_on,
    branch = excluded.branch,
    expected_notes = excluded.expected_notes
RETURNING id;

-- name: UpsertRepository :one
INSERT INTO repository (project_id, path, branch)
VALUES (?, ?, ?)
ON CONFLICT (path) DO UPDATE SET project_id = excluded.project_id, branch = excluded.branch
RETURNING id, last_imported_sha;

-- name: MarkRepositoryImported :exec
UPDATE repository SET last_imported_sha = ? WHERE id = ?;

-- name: InsertCommit :execrows
-- 0 rows touched means this repository already stored the commit.
INSERT INTO commit_git (sha, repository_id, author, date_utc, offset_min, message)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (repository_id, sha) DO NOTHING;

-- name: InsertChangedFile :exec
INSERT INTO changed_file (repository_id, commit_sha, path, language, additions, deletions)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (repository_id, commit_sha, path) DO NOTHING;

-- name: NoteHash :one
SELECT id, content_hash FROM note WHERE path = ?;

-- name: UpsertNote :one
INSERT INTO note (path, folder, title, type, updated, content_hash)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (path) DO UPDATE SET
    folder = excluded.folder,
    title = excluded.title,
    type = excluded.type,
    updated = excluded.updated,
    content_hash = excluded.content_hash
RETURNING id;

-- name: DeleteNoteLinks :exec
DELETE FROM link WHERE source_id = ?;

-- name: InsertLink :exec
INSERT INTO link (source_id, target_title, from_property)
VALUES (?, ?, ?)
ON CONFLICT (source_id, target_title) DO UPDATE SET from_property = min(link.from_property, excluded.from_property);

-- name: SetLinkTarget :exec
UPDATE link SET target_id = ? WHERE source_id = ? AND target_title = ?;

-- name: ListLinks :many
SELECT l.source_id, n.path AS source_path, l.target_title
FROM link l JOIN note n ON n.id = l.source_id
ORDER BY l.source_id, l.target_title;

-- name: UpsertSession :exec
INSERT INTO session (note_id, date, project_id, ai)
VALUES (?, ?, ?, ?)
ON CONFLICT (note_id) DO UPDATE SET date = excluded.date, project_id = excluded.project_id, ai = excluded.ai;

-- name: InsertJournal :exec
INSERT INTO journal (date, ai, body)
VALUES (?, ?, ?)
ON CONFLICT (date, ai, body) DO NOTHING;

-- name: UpsertTechnology :one
INSERT INTO technology (name, category)
VALUES (?, ?)
ON CONFLICT (name) DO UPDATE SET category = coalesce(excluded.category, technology.category)
RETURNING id;

-- name: LinkProjectTechnology :exec
INSERT INTO project_technology (project_id, technology_id)
VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: InsertLanguageStat :exec
INSERT INTO language_stat (repository_id, measured_on, language, files, code_lines, comment_lines)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (repository_id, measured_on, language) DO UPDATE SET
    files = excluded.files,
    code_lines = excluded.code_lines,
    comment_lines = excluded.comment_lines;

-- name: DeleteSearch :exec
DELETE FROM search_index WHERE source = ? AND ref = ?;

-- name: InsertSearch :exec
INSERT INTO search_index (title, content, source, ref) VALUES (?, ?, ?, ?);

-- name: ListNotes :many
SELECT id, path FROM note;

-- name: DeleteNote :exec
-- Links, the session, and incoming links (target_id -> NULL) follow the foreign keys.
DELETE FROM note WHERE id = ?;

-- name: DeleteRepositoryCommits :exec
-- Rewritten history (rebase, force-push): start over for this repository. Triggers fix daily_activity.
DELETE FROM commit_git WHERE repository_id = ?;

-- name: DeleteRepositoryCommitSearch :exec
DELETE FROM search_index WHERE source = 'commit' AND ref IN (
    SELECT printf('%d:%s', repository_id, sha) FROM commit_git WHERE repository_id = ?);

-- name: LastRepositorySHA :one
SELECT last_imported_sha FROM repository WHERE path = ?;

-- name: ListProjectIDs :many
SELECT id, name FROM project ORDER BY id;

-- name: DeleteCommitSearchForRepository :exec
DELETE FROM search_index WHERE source = 'commit' AND ref IN (
    SELECT printf('%d:%s', repository_id, sha) FROM commit_git WHERE repository_id = ?);

-- name: DeleteRepository :exec
DELETE FROM repository WHERE id = ?;

-- name: DeleteProject :exec
DELETE FROM project WHERE id = ?;

-- name: DeleteJournal :exec
DELETE FROM journal;

-- name: DeleteSessions :exec
DELETE FROM session;

-- name: ClearProjectTechnologies :exec
DELETE FROM project_technology WHERE project_id = ?;

-- name: DeleteUnusedTechnologies :exec
DELETE FROM technology WHERE id NOT IN (SELECT technology_id FROM project_technology);

-- name: UpsertSetting :exec
INSERT INTO app_setting (key, value) VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value;

-- name: Setting :one
SELECT value FROM app_setting WHERE key = ?;
