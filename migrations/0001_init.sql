-- Schéma initial d'Observatoire.
-- Toutes les tables sont STRICT : SQLite refuse une valeur du mauvais type au lieu de la convertir en silence.
-- Dates : texte ISO 8601 (« 2026-10-26 » ou « 2026-10-26T14:03:00Z »), comparables et triables comme du texte.
-- La table des commits s'appelle commit_git car COMMIT est un mot réservé du SQL.

-- +goose Up

-- Dépôts et projets ----------------------------------------------------------

CREATE TABLE projet (
    id             INTEGER PRIMARY KEY,           -- alias de rowid : auto-incrémenté, gratuit
    nom            TEXT    NOT NULL UNIQUE,
    statut         TEXT,                          -- valeur du front matter de la fiche (en cours, en pause…)
    chemin_source  TEXT,                          -- copie principale du dépôt (seule importée)
    fiche_maj      TEXT,
    verifie_commit TEXT,
    verifie_le     TEXT
) STRICT;

CREATE TABLE depot (
    id                  INTEGER PRIMARY KEY,
    projet_id           INTEGER NOT NULL REFERENCES projet(id) ON DELETE CASCADE,
    chemin              TEXT    NOT NULL UNIQUE,
    branche             TEXT,
    dernier_sha_importe TEXT                      -- point de reprise de l'import incrémental
) STRICT;
CREATE INDEX depot_projet ON depot(projet_id);

-- Git ------------------------------------------------------------------------

CREATE TABLE commit_git (
    sha          TEXT    PRIMARY KEY,
    depot_id     INTEGER NOT NULL REFERENCES depot(id) ON DELETE CASCADE,
    auteur       TEXT    NOT NULL,
    date_utc     TEXT    NOT NULL,                -- « 2026-10-26T14:03:00Z »
    decalage_min INTEGER NOT NULL DEFAULT 0,      -- fuseau de l'auteur au moment du commit, en minutes
    message      TEXT    NOT NULL,
    -- Colonnes calculées : SQLite les remplit seul à chaque écriture (STORED = enregistrées, donc indexables).
    jour_local   TEXT    GENERATED ALWAYS AS (date(date_utc, decalage_min || ' minutes')) STORED,
    heure_locale INTEGER GENERATED ALWAYS AS (CAST(strftime('%H', date_utc, decalage_min || ' minutes') AS INTEGER)) STORED
) STRICT;
CREATE INDEX commit_depot_date ON commit_git(depot_id, date_utc);
CREATE INDEX commit_jour ON commit_git(jour_local);

CREATE TABLE fichier_modifie (
    commit_sha   TEXT    NOT NULL REFERENCES commit_git(sha) ON DELETE CASCADE,
    chemin       TEXT    NOT NULL,
    langage      TEXT,
    ajouts       INTEGER,                         -- NULL pour un fichier binaire (git affiche « - »)
    suppressions INTEGER,
    PRIMARY KEY (commit_sha, chemin)
) STRICT, WITHOUT ROWID;                          -- la clé primaire EST la table : pas de rowid caché
CREATE INDEX fichier_chemin ON fichier_modifie(chemin);

-- Coffre ---------------------------------------------------------------------

CREATE TABLE note (
    id           INTEGER PRIMARY KEY,
    chemin       TEXT    NOT NULL UNIQUE,         -- relatif à la racine du coffre, séparateur « / »
    dossier      TEXT    NOT NULL,                -- premier niveau : Projets, Sessions, Concepts…
    titre        TEXT    NOT NULL,                -- nom du fichier sans « .md » : c'est ce que vise [[...]]
    type         TEXT,
    maj          TEXT,
    contenu_hash TEXT    NOT NULL                 -- permet de sauter une note inchangée à la réimportation
) STRICT;
CREATE INDEX note_dossier ON note(dossier);
CREATE INDEX note_titre ON note(titre);

CREATE TABLE lien (
    source_id   INTEGER NOT NULL REFERENCES note(id) ON DELETE CASCADE,
    cible_titre TEXT    NOT NULL,                 -- texte du lien avant l'éventuel « | alias »
    cible_id    INTEGER REFERENCES note(id) ON DELETE SET NULL,  -- NULL = lien cassé
    PRIMARY KEY (source_id, cible_titre)
) STRICT, WITHOUT ROWID;
CREATE INDEX lien_cible ON lien(cible_id);
-- Index partiel : ne contient que les liens cassés, donc minuscule et parfait pour les retrouver.
CREATE INDEX lien_casse ON lien(cible_titre) WHERE cible_id IS NULL;

CREATE TABLE session (
    note_id   INTEGER PRIMARY KEY REFERENCES note(id) ON DELETE CASCADE,
    date      TEXT    NOT NULL,
    projet_id INTEGER REFERENCES projet(id) ON DELETE SET NULL,
    ia        TEXT
) STRICT;
CREATE INDEX session_projet_date ON session(projet_id, date);

CREATE TABLE journal (
    id    INTEGER PRIMARY KEY,
    date  TEXT NOT NULL,
    ia    TEXT,
    texte TEXT NOT NULL,
    UNIQUE (date, ia, texte)                      -- rend la réimportation idempotente
) STRICT;

CREATE TABLE techno (
    id        INTEGER PRIMARY KEY,
    nom       TEXT NOT NULL UNIQUE,
    categorie TEXT
) STRICT;

CREATE TABLE projet_techno (
    projet_id INTEGER NOT NULL REFERENCES projet(id) ON DELETE CASCADE,
    techno_id INTEGER NOT NULL REFERENCES techno(id) ON DELETE CASCADE,
    PRIMARY KEY (projet_id, techno_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX projet_techno_techno ON projet_techno(techno_id);

-- Code -----------------------------------------------------------------------

CREATE TABLE mesure_langage (
    depot_id            INTEGER NOT NULL REFERENCES depot(id) ON DELETE CASCADE,
    date_mesure         TEXT    NOT NULL,
    langage             TEXT    NOT NULL,
    fichiers            INTEGER NOT NULL,
    lignes_code         INTEGER NOT NULL,
    lignes_commentaires INTEGER NOT NULL,
    PRIMARY KEY (depot_id, date_mesure, langage)
) STRICT, WITHOUT ROWID;

-- Recherche plein texte ----------------------------------------------------
-- remove_diacritics 2 : « résumé » se trouve en tapant « resume ».
-- source vaut 'note' ou 'commit' ; ref vaut le chemin de la note ou le sha. UNINDEXED : stockées, pas cherchées.

CREATE VIRTUAL TABLE recherche USING fts5(
    titre, contenu, source UNINDEXED, ref UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- Agrégat quotidien tenu à jour par triggers ---------------------------------
-- Les pages lisent cette petite table au lieu de regrouper des milliers de commits à chaque affichage.

CREATE TABLE activite_jour (
    jour         TEXT    NOT NULL,
    projet_id    INTEGER NOT NULL REFERENCES projet(id) ON DELETE CASCADE,
    nb_commits   INTEGER NOT NULL DEFAULT 0,
    ajouts       INTEGER NOT NULL DEFAULT 0,
    suppressions INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (jour, projet_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX activite_projet_jour ON activite_jour(projet_id, jour);

-- +goose StatementBegin
CREATE TRIGGER commit_ajoute AFTER INSERT ON commit_git
BEGIN
    INSERT INTO activite_jour (jour, projet_id, nb_commits)
    VALUES (NEW.jour_local, (SELECT projet_id FROM depot WHERE id = NEW.depot_id), 1)
    ON CONFLICT (jour, projet_id) DO UPDATE SET nb_commits = nb_commits + 1;
END;
-- +goose StatementEnd

-- BEFORE et non AFTER : la suppression en cascade des fichiers a lieu quand le commit a déjà disparu,
-- le trigger fichier_supprime ne peut alors plus retrouver son jour. On retire donc ici les lignes du
-- commit pendant que ses fichiers existent encore ; fichier_supprime ne trouvera rien et ne fera rien.
-- +goose StatementBegin
CREATE TRIGGER commit_supprime BEFORE DELETE ON commit_git
BEGIN
    UPDATE activite_jour
    SET nb_commits   = nb_commits - 1,
        ajouts       = ajouts - (SELECT coalesce(sum(ajouts), 0) FROM fichier_modifie WHERE commit_sha = OLD.sha),
        suppressions = suppressions - (SELECT coalesce(sum(suppressions), 0) FROM fichier_modifie WHERE commit_sha = OLD.sha)
    WHERE jour = OLD.jour_local
      AND projet_id = (SELECT projet_id FROM depot WHERE id = OLD.depot_id);
END;
-- +goose StatementEnd

-- Le commit est toujours inséré avant ses fichiers : la ligne d'activite_jour existe déjà.
-- +goose StatementBegin
CREATE TRIGGER fichier_ajoute AFTER INSERT ON fichier_modifie
BEGIN
    UPDATE activite_jour
    SET ajouts       = ajouts + coalesce(NEW.ajouts, 0),
        suppressions = suppressions + coalesce(NEW.suppressions, 0)
    WHERE (jour, projet_id) = (
        SELECT c.jour_local, d.projet_id
        FROM commit_git c JOIN depot d ON d.id = c.depot_id
        WHERE c.sha = NEW.commit_sha
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER fichier_supprime AFTER DELETE ON fichier_modifie
BEGIN
    UPDATE activite_jour
    SET ajouts       = ajouts - coalesce(OLD.ajouts, 0),
        suppressions = suppressions - coalesce(OLD.suppressions, 0)
    WHERE (jour, projet_id) = (
        SELECT c.jour_local, d.projet_id
        FROM commit_git c JOIN depot d ON d.id = c.depot_id
        WHERE c.sha = OLD.commit_sha
    );
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE activite_jour;
DROP TABLE recherche;
DROP TABLE mesure_langage;
DROP TABLE projet_techno;
DROP TABLE techno;
DROP TABLE journal;
DROP TABLE session;
DROP TABLE lien;
DROP TABLE note;
DROP TABLE fichier_modifie;
DROP TABLE commit_git;
DROP TABLE depot;
DROP TABLE projet;
