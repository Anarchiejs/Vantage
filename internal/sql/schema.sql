-- Final Vantage schema used by sqlc. Goose builds this through migrations/.
-- Every table is STRICT: SQLite rejects a value of the wrong type instead of converting it silently.
-- Dates are ISO 8601 text ("2026-10-26" or "2026-10-26T14:03:00Z"), so they compare and sort as text.
-- The commit table is named commit_git because COMMIT is a reserved SQL word.


-- Repositories and projects ---------------------------------------------------

CREATE TABLE project (
    id              INTEGER PRIMARY KEY,          -- rowid alias: autoincremented, free
    name            TEXT    NOT NULL UNIQUE,
    status          TEXT,                         -- card front matter (in progress, paused, ...)
    source_path     TEXT,                         -- primary repository copy
    card_updated    TEXT,
    verified_commit TEXT,
    verified_on     TEXT,
    branch          TEXT,                         -- branch to import; empty means HEAD
    expected_notes  TEXT                          -- JSON array of note titles the card expects
) STRICT;

CREATE TABLE repository (
    id                INTEGER PRIMARY KEY,
    project_id        INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    path              TEXT    NOT NULL UNIQUE,
    branch            TEXT,
    last_imported_sha TEXT                        -- resume point for incremental import
) STRICT;
CREATE INDEX repository_project ON repository(project_id);

-- Git -------------------------------------------------------------------------

CREATE TABLE commit_git (
    sha           TEXT    NOT NULL,
    repository_id INTEGER NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    author        TEXT    NOT NULL,
    date_utc      TEXT    NOT NULL,               -- "2026-10-26T14:03:00Z"
    offset_min    INTEGER NOT NULL DEFAULT 0,     -- author timezone at commit time, in minutes
    message       TEXT    NOT NULL,
    -- Generated columns: SQLite fills them on every write (STORED = persisted, so indexable).
    local_day     TEXT    GENERATED ALWAYS AS (date(date_utc, offset_min || ' minutes')) STORED,
    local_hour    INTEGER GENERATED ALWAYS AS (CAST(strftime('%H', date_utc, offset_min || ' minutes') AS INTEGER)) STORED,
    PRIMARY KEY (repository_id, sha)
) STRICT;
CREATE INDEX commit_repository_date ON commit_git(repository_id, date_utc);
CREATE INDEX commit_local_day ON commit_git(local_day);

CREATE TABLE changed_file (
    repository_id INTEGER NOT NULL,
    commit_sha    TEXT    NOT NULL,
    path          TEXT    NOT NULL,
    language      TEXT,
    additions     INTEGER,                        -- NULL for a binary file (git prints "-")
    deletions     INTEGER,
    PRIMARY KEY (repository_id, commit_sha, path),
    FOREIGN KEY (repository_id, commit_sha) REFERENCES commit_git(repository_id, sha) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE INDEX changed_file_path ON changed_file(path);

-- Vault -----------------------------------------------------------------------

CREATE TABLE note (
    id           INTEGER PRIMARY KEY,
    path         TEXT    NOT NULL UNIQUE,         -- relative to the vault root, separator "/"
    folder       TEXT    NOT NULL,                -- first level: Projects, Sessions, Concepts, ...
    title        TEXT    NOT NULL,                -- file name without ".md": this is what [[...]] targets
    type         TEXT,
    updated      TEXT,
    content_hash TEXT    NOT NULL                 -- skip an unchanged note on reimport
) STRICT;
CREATE INDEX note_folder ON note(folder);
CREATE INDEX note_title ON note(title);

CREATE TABLE link (
    source_id    INTEGER NOT NULL REFERENCES note(id) ON DELETE CASCADE,
    target_title  TEXT    NOT NULL,               -- cleaned wikilink, path kept, alias and heading removed
    target_id     INTEGER REFERENCES note(id) ON DELETE SET NULL,  -- NULL = broken link
    from_property INTEGER NOT NULL DEFAULT 0,     -- 1 when the link came from front matter
    PRIMARY KEY (source_id, target_title)
) STRICT, WITHOUT ROWID;
CREATE INDEX link_target ON link(target_id);
-- Partial index: only broken links, so it stays small and is the right place to find them.
CREATE INDEX link_broken ON link(target_title) WHERE target_id IS NULL;

CREATE TABLE session (
    note_id    INTEGER PRIMARY KEY REFERENCES note(id) ON DELETE CASCADE,
    date       TEXT    NOT NULL,
    project_id INTEGER REFERENCES project(id) ON DELETE SET NULL,
    ai         TEXT
) STRICT;
CREATE INDEX session_project_date ON session(project_id, date);

CREATE TABLE journal (
    id      INTEGER PRIMARY KEY,
    date    TEXT NOT NULL,
    ai      TEXT,
    body    TEXT NOT NULL,
    UNIQUE (date, ai, body)                       -- makes reimport idempotent
) STRICT;

CREATE TABLE technology (
    id       INTEGER PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    category TEXT
) STRICT;

CREATE TABLE project_technology (
    project_id    INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    technology_id INTEGER NOT NULL REFERENCES technology(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, technology_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX project_technology_technology ON project_technology(technology_id);

-- Code ------------------------------------------------------------------------

CREATE TABLE language_stat (
    repository_id INTEGER NOT NULL REFERENCES repository(id) ON DELETE CASCADE,
    measured_on   TEXT    NOT NULL,
    language      TEXT    NOT NULL,
    files         INTEGER NOT NULL,
    code_lines    INTEGER NOT NULL,
    comment_lines INTEGER NOT NULL,
    PRIMARY KEY (repository_id, measured_on, language)
) STRICT, WITHOUT ROWID;

-- Full-text search -------------------------------------------------------------
-- remove_diacritics 2: "naive" finds "Naive" with a diaeresis.
-- source is 'note' or 'commit'; ref is the note path or '<repository id>:<sha>'. UNINDEXED: stored, not searched.

CREATE VIRTUAL TABLE search_index USING fts5(
    title, content, source UNINDEXED, ref UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- Daily aggregate maintained by triggers --------------------------------------
-- Pages read this small table instead of grouping thousands of commits on every view.

CREATE TABLE daily_activity (
    day          TEXT    NOT NULL,
    project_id   INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    commit_count INTEGER NOT NULL DEFAULT 0,
    additions    INTEGER NOT NULL DEFAULT 0,
    deletions    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, project_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX daily_activity_project_day ON daily_activity(project_id, day);

CREATE TRIGGER commit_inserted AFTER INSERT ON commit_git
BEGIN
    INSERT INTO daily_activity (day, project_id, commit_count)
    VALUES (NEW.local_day, (SELECT project_id FROM repository WHERE id = NEW.repository_id), 1)
    ON CONFLICT (day, project_id) DO UPDATE SET commit_count = commit_count + 1;
END;

-- BEFORE, not AFTER: cascading file deletes happen after the commit is already gone,
-- so the file_deleted trigger can no longer find its day. Subtract the commit's lines
-- here while its files still exist; file_deleted then finds nothing and does nothing.
CREATE TRIGGER commit_deleted BEFORE DELETE ON commit_git
BEGIN
    UPDATE daily_activity
    SET commit_count = commit_count - 1,
        additions    = additions - (SELECT coalesce(sum(additions), 0) FROM changed_file WHERE repository_id = OLD.repository_id AND commit_sha = OLD.sha),
        deletions    = deletions - (SELECT coalesce(sum(deletions), 0) FROM changed_file WHERE repository_id = OLD.repository_id AND commit_sha = OLD.sha)
    WHERE day = OLD.local_day
      AND project_id = (SELECT project_id FROM repository WHERE id = OLD.repository_id);
END;

-- The commit is always inserted before its files: the daily_activity row already exists.
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

CREATE TABLE app_setting (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

-- Indexes from migration 0002.
CREATE INDEX session_date_project ON session(date, project_id);
CREATE INDEX session_ai_date_project ON session(ai, date, project_id);
