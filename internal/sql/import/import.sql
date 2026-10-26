-- Ecritures de l'import. Toutes sont idempotentes : relancer l'import ne cree aucun doublon.

-- name: UpsertProjet :one
-- Cree le projet ou met a jour les champs de sa fiche ; renvoie son id dans les deux cas.
INSERT INTO projet (nom, statut, chemin_source, fiche_maj, verifie_commit, verifie_le)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (nom) DO UPDATE SET
    statut = excluded.statut,
    chemin_source = excluded.chemin_source,
    fiche_maj = excluded.fiche_maj,
    verifie_commit = excluded.verifie_commit,
    verifie_le = excluded.verifie_le
RETURNING id;

-- name: UpsertDepot :one
INSERT INTO depot (projet_id, chemin, branche)
VALUES (?, ?, ?)
ON CONFLICT (chemin) DO UPDATE SET projet_id = excluded.projet_id, branche = excluded.branche
RETURNING id, dernier_sha_importe;

-- name: MarquerDepotImporte :exec
UPDATE depot SET dernier_sha_importe = ? WHERE id = ?;

-- name: InsererCommit :execrows
-- 0 ligne touchee = commit deja connu ; l'appelant saute alors ses fichiers.
INSERT INTO commit_git (sha, depot_id, auteur, date_utc, decalage_min, message)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (sha) DO NOTHING;

-- name: InsererFichierModifie :exec
INSERT INTO fichier_modifie (commit_sha, chemin, langage, ajouts, suppressions)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commit_sha, chemin) DO NOTHING;

-- name: HashNote :one
SELECT id, contenu_hash FROM note WHERE chemin = ?;

-- name: UpsertNote :one
INSERT INTO note (chemin, dossier, titre, type, maj, contenu_hash)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (chemin) DO UPDATE SET
    dossier = excluded.dossier,
    titre = excluded.titre,
    type = excluded.type,
    maj = excluded.maj,
    contenu_hash = excluded.contenu_hash
RETURNING id;

-- name: SupprimerLiensDeNote :exec
DELETE FROM lien WHERE source_id = ?;

-- name: InsererLien :exec
INSERT INTO lien (source_id, cible_titre)
VALUES (?, ?)
ON CONFLICT (source_id, cible_titre) DO NOTHING;

-- name: ResoudreLiens :exec
-- Apres l'import de toutes les notes : relie chaque lien au titre qu'il vise (NULL si casse).
UPDATE lien SET cible_id = (SELECT n.id FROM note n WHERE n.titre = lien.cible_titre LIMIT 1);

-- name: UpsertSession :exec
INSERT INTO session (note_id, date, projet_id, ia)
VALUES (?, ?, ?, ?)
ON CONFLICT (note_id) DO UPDATE SET date = excluded.date, projet_id = excluded.projet_id, ia = excluded.ia;

-- name: InsererJournal :exec
INSERT INTO journal (date, ia, texte)
VALUES (?, ?, ?)
ON CONFLICT (date, ia, texte) DO NOTHING;

-- name: UpsertTechno :one
INSERT INTO techno (nom, categorie)
VALUES (?, ?)
ON CONFLICT (nom) DO UPDATE SET categorie = coalesce(excluded.categorie, techno.categorie)
RETURNING id;

-- name: LierProjetTechno :exec
INSERT INTO projet_techno (projet_id, techno_id)
VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: InsererMesureLangage :exec
INSERT INTO mesure_langage (depot_id, date_mesure, langage, fichiers, lignes_code, lignes_commentaires)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (depot_id, date_mesure, langage) DO UPDATE SET
    fichiers = excluded.fichiers,
    lignes_code = excluded.lignes_code,
    lignes_commentaires = excluded.lignes_commentaires;

-- name: SupprimerRecherche :exec
DELETE FROM recherche WHERE source = ? AND ref = ?;

-- name: InsererRecherche :exec
INSERT INTO recherche (titre, contenu, source, ref) VALUES (?, ?, ?, ?);

-- name: ListerNotes :many
SELECT id, chemin FROM note;

-- name: SupprimerNote :exec
-- Les liens, la session et les liens entrants (cible_id -> NULL) suivent par les cles etrangeres.
DELETE FROM note WHERE id = ?;

-- name: SupprimerCommitsDepot :exec
-- Historique reecrit (rebase, force-push) : on repart de zero pour ce depot. Les triggers corrigent activite_jour.
DELETE FROM commit_git WHERE depot_id = ?;

-- name: SupprimerRechercheCommitsDepot :exec
DELETE FROM recherche WHERE source = 'commit' AND ref IN (SELECT sha FROM commit_git WHERE depot_id = ?);

-- name: DernierShaDepot :one
SELECT dernier_sha_importe FROM depot WHERE chemin = ?;
