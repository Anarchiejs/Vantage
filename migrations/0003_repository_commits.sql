-- Each repository keeps its own copy of a commit, even when two clones share a SHA.
-- Links may come from front matter. Project cards can name a branch and expected notes.
-- app_setting remembers the vault path used to build Obsidian links.

-- +goose Up

PRAGMA foreign_keys=OFF;

DROP TRIGGER IF EXISTS commit_inserted;
DROP TRIGGER IF EXISTS commit_deleted;
DROP TRIGGER IF EXISTS file_inserted;
DROP TRIGGER IF EXISTS file_deleted;

ALTER TABLE commit_git RENAME TO commit_git_old;
ALTER TABLE changed_file RENAME TO changed_file_old;

CREATE TABLE commit_git (
    sha           TEXT    NOT NULL,
    repository_id INTEGER NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    author        TEXT    NOT NULL,
    date_utc      TEXT    NOT NULL,
    offset_min    INTEGER NOT NULL DEFAULT 0,
    message       TEXT    NOT NULL,
    local_day     TEXT    GENERATED ALWAYS AS (date(date_utc, offset_min || ' minutes')) STORED,
    local_hour    INTEGER GENERATED ALWAYS AS (CAST(strftime('%H', date_utc, offset_min || ' minutes') AS INTEGER)) STORED,
    PRIMARY KEY (repository_id, sha)
) STRICT;

INSERT INTO commit_git (sha, repository_id, author, date_utc, offset_min, message)
SELECT sha, repository_id, author, date_utc, offset_min, message FROM commit_git_old;

CREATE TABLE changed_file (
    repository_id INTEGER NOT NULL,
    commit_sha    TEXT    NOT NULL,
    path          TEXT    NOT NULL,
    language      TEXT,
    additions     INTEGER,
    deletions     INTEGER,
    PRIMARY KEY (repository_id, commit_sha, path),
    FOREIGN KEY (repository_id, commit_sha) REFERENCES commit_git(repository_id, sha) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;

INSERT INTO changed_file (repository_id, commit_sha, path, language, additions, deletions)
SELECT c.repository_id, f.commit_sha, f.path, f.language, f.additions, f.deletions
FROM changed_file_old f
JOIN commit_git c ON c.sha = f.commit_sha;

DROP TABLE changed_file_old;
DROP TABLE commit_git_old;

CREATE INDEX commit_repository_date ON commit_git(repository_id, date_utc);
CREATE INDEX commit_local_day ON commit_git(local_day);
CREATE INDEX changed_file_path ON changed_file(path);

-- +goose StatementBegin
CREATE TRIGGER commit_inserted AFTER INSERT ON commit_git
BEGIN
    INSERT INTO daily_activity (day, project_id, commit_count)
    VALUES (NEW.local_day, (SELECT project_id FROM repository WHERE id = NEW.repository_id), 1)
    ON CONFLICT (day, project_id) DO UPDATE SET commit_count = commit_count + 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER commit_deleted BEFORE DELETE ON commit_git
BEGIN
    UPDATE daily_activity
    SET commit_count = commit_count - 1,
        additions    = additions - (SELECT coalesce(sum(additions), 0) FROM changed_file WHERE repository_id = OLD.repository_id AND commit_sha = OLD.sha),
        deletions    = deletions - (SELECT coalesce(sum(deletions), 0) FROM changed_file WHERE repository_id = OLD.repository_id AND commit_sha = OLD.sha)
    WHERE day = OLD.local_day
      AND project_id = (SELECT project_id FROM repository WHERE id = OLD.repository_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER file_inserted AFTER INSERT ON changed_file
BEGIN
    UPDATE daily_activity
    SET additions = additions + coalesce(NEW.additions, 0),
        deletions = deletions + coalesce(NEW.deletions, 0)
    WHERE (day, project_id) = (
        SELECT c.local_day, d.project_id
        FROM commit_git c JOIN repository d ON d.id = c.repository_id
        WHERE c.repository_id = NEW.repository_id AND c.sha = NEW.commit_sha
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER file_deleted AFTER DELETE ON changed_file
BEGIN
    UPDATE daily_activity
    SET additions = additions - coalesce(OLD.additions, 0),
        deletions = deletions - coalesce(OLD.deletions, 0)
    WHERE (day, project_id) = (
        SELECT c.local_day, d.project_id
        FROM commit_git c JOIN repository d ON d.id = c.repository_id
        WHERE c.repository_id = OLD.repository_id AND c.sha = OLD.commit_sha
    );
END;
-- +goose StatementEnd

ALTER TABLE link ADD COLUMN from_property INTEGER NOT NULL DEFAULT 0;
ALTER TABLE project ADD COLUMN branch TEXT;
ALTER TABLE project ADD COLUMN expected_notes TEXT;

CREATE TABLE app_setting (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

UPDATE search_index
SET ref = (
    SELECT printf('%d:%s', c.repository_id, c.sha)
    FROM commit_git c
    WHERE c.sha = search_index.ref
    ORDER BY c.repository_id
    LIMIT 1
)
WHERE source = 'commit' AND instr(ref, ':') = 0;

PRAGMA foreign_keys=ON;

-- +goose Down

-- The composite primary key cannot be folded back into a single sha key
-- without dropping commits that exist in more than one repository.
SELECT 1;
