// Contrat de l'API JSON. Miroir exact de internal/api/contrat.go : toute modification se fait des deux côtés.
// Dates : « AAAA-MM-JJ » ; instants : ISO 8601 UTC « 2026-10-26T14:03:00Z » ; mois : « AAAA-MM » ; semaine : lundi « AAAA-MM-JJ ».

export interface Erreur {
  erreur: string
}

// ---- GET /api/accueil -------------------------------------------------------

export interface JourActivite {
  jour: string
  commits: number
  ajouts: number
  suppressions: number
  sessions: number
  projets: string[] // noms des projets touchés ce jour
}

export interface Chiffres {
  commits: number
  ajouts: number
  suppressions: number
  sessions: number
  projets_actifs: number
}

export interface ProjetActif {
  id: number
  nom: string
  dernier_commit: string | null // instant
  commits_30j: number
}

export interface ProjetDormant {
  id: number
  nom: string
  dernier_commit: string | null
  jours: number // depuis le dernier commit
}

export interface FichePerimee {
  id: number
  nom: string
  verifie_commit: string | null
  dernier_sha: string | null
  commits_depuis: number // commits arrivés après verifie_commit (-1 si verifie_commit inconnu du dépôt)
}

export interface Accueil {
  calendrier: JourActivite[] // 365 jours finissant aujourd'hui, jours vides compris, ordre chronologique
  semaine: Chiffres // 7 derniers jours
  semaine_precedente: Chiffres
  mois: Chiffres // 30 derniers jours
  mois_precedent: Chiffres
  actifs: ProjetActif[]
  dormants: ProjetDormant[] // statut « en cours » sans commit depuis plus de 30 jours
  fiches_perimees: FichePerimee[]
}

// ---- GET /api/projets et /api/projets/{id} --------------------------------

export interface ProjetResume {
  id: number
  nom: string
  statut: string | null
  commits: number
  dernier_commit: string | null
  langage_principal: string | null
}

export type TypeEvenement = 'commit' | 'session' | 'journal'

export interface EvenementFrise {
  date: string // instant pour un commit, date sinon
  type: TypeEvenement
  titre: string // sujet du commit, titre de la session, texte du journal
  ref: string // sha, chemin de note, ou id de journal
  ia: string | null
}

export interface VolumeSemaine {
  semaine: string
  commits: number
  ajouts: number
  suppressions: number
}

export interface PartLangage {
  langage: string
  lignes_code: number
  part: number // 0..1
}

export interface PointLangage {
  date_mesure: string
  langage: string
  lignes_code: number
}

export interface PointChaud {
  chemin: string
  modifications: number
  ajouts: number
  suppressions: number
}

export interface Sante {
  verifie_commit: string | null
  verifie_le: string | null
  dernier_sha: string | null
  a_jour: boolean
  commits_depuis: number
  notes_manquantes: string[] // parmi « <Nom> - Resume », « <Nom> - Detaille »
}

export interface ActiviteIA {
  ia: string
  sessions: number
  premiere: string
  derniere: string
}

export interface ProjetDetail {
  projet: ProjetResume & { chemin_source: string | null; technos: string[] }
  frise: EvenementFrise[] // plus récent d'abord, 500 au plus
  volume: VolumeSemaine[] // 52 dernières semaines, semaines vides comprises
  langages: PartLangage[] // dernière mesure
  langages_evolution: PointLangage[]
  points_chauds: PointChaud[] // 20 fichiers les plus modifiés
  sante: Sante
  ia: ActiviteIA[]
}

// ---- GET /api/technos et /api/ia -------------------------------------------

export interface NoeudSankey {
  nom: string
  type: 'projet' | 'techno'
}

export interface LienSankey {
  source: string
  cible: string
  valeur: number // 1 par lien projet → techno
}

export interface TendanceTechno {
  mois: string
  techno: string
  commits: number // commits des projets qui utilisent la techno, ce mois
}

export interface Technos {
  sankey: { noeuds: NoeudSankey[]; liens: LienSankey[] }
  tendance: TendanceTechno[] // 12 derniers mois
  sans_projet: string[]
}

export interface IA {
  ia: string
  sessions: number
  projets: string[]
  par_mois: { mois: string; sessions: number }[]
}

// ---- GET /api/graphe, /api/graphe/chemin, /api/notes/{id} -----------------

export interface NoeudGraphe {
  id: number
  titre: string
  dossier: string
  degre: number // liens entrants + sortants résolus
}

export interface Graphe {
  noeuds: NoeudGraphe[]
  liens: { source: number; cible: number }[]
  orphelines: NoeudGraphe[] // degre = 0
  centrales: NoeudGraphe[] // 10 plus grands degrés
  casses: { source: number; source_titre: string; cible_titre: string }[]
}

export interface Chemin {
  chemin: NoeudGraphe[] // vide si aucun chemin ; liens suivis dans les deux sens
}

export interface NoteDetail {
  id: number
  titre: string
  chemin: string
  dossier: string
  maj: string | null
  contenu: string
  voisins: NoeudGraphe[]
}

// ---- GET /api/recherche ------------------------------------------------------

export interface ResultatRecherche {
  source: 'note' | 'commit'
  ref: string // chemin de note ou sha
  titre: string // avec <mark>…</mark> autour des termes trouvés
  extrait: string // idem, avec « … » aux coupures
  projet: string | null
  date: string | null
  id_note: number | null
}

// Paramètres : q (obligatoire), source, projet, dossier, depuis, jusqua, limite (défaut 50)

// ---- Console SQL -------------------------------------------------------------

export interface DemandeSQL {
  sql: string
}

export interface ResultatSQL {
  colonnes: string[]
  lignes: (string | number | null)[][]
  duree_ms: number
  tronque: boolean // plus de 1000 lignes : seules les 1000 premières sont renvoyées
}

export interface TableSchema {
  table: string
  colonnes: { nom: string; type: string }[]
}

export interface RequeteEnregistree {
  nom: string // nom de fichier sans « .sql », [a-z0-9-] uniquement
  sql: string
}

// ---- GET /api/evenements (Server-Sent Events) -----------------------------
// événement « maj » : { "tables": ["commit_git", "note", ...] } après chaque import.

export interface EvenementMaj {
  tables: string[]
}
