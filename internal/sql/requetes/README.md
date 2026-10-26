# Contrat d'integration S2

- 28 requetes nommees, generees exclusivement avec sqlc 1.31.1.
- Rechercher utilise `FROM recherche(sqlc.arg(termes))`, syntaxe FTS5 equivalente
  a `WHERE recherche MATCH ...`. sqlc rejette la colonne cachee portant le nom
  de la table dans la seconde syntaxe. Le plan confirme MATCH `0:M4`.
- `char(8230)` est le separateur snippet demande (U+2026), tout en gardant les
  sources SQL ASCII pour contourner le probleme de troncature de sqlc.
- Les champs calcules nullables peuvent etre des `interface{}` dans les structs
  sqlc (notamment Rechercher.Date) ; ils contiennent string ou nil. L'API doit
  les convertir vers son contrat. IAduProjet attend un `*int64` (colonne nullable).
- Calendrier.Projets est une chaine separee par char(31), vide si aucun projet.
  Les sessions touchent aussi un projet, meme sans commit ce jour-la.
- ProjetsDormants.Jours vaut -1 pour un projet sans aucun commit ; seuls les
  projets dont le statut est exactement `en cours` sont selectionnes.
- Les liens du graphe sont diriges, dedupliques et sans boucle ; le degre est
  la somme des degres entrants/sortants sur ces memes liens. La recherche de
  chemin les suit dans les deux sens, au plus six aretes, avec depart inclus.
- Les dernieres mesures sont choisies par depot avant agregation au projet.
  Les parts de langage sont des fractions entre 0 et 1, zero si le total est nul.
- Les vues sans borne temporelle (frise, IA, sante) lisent tout l'historique.
  Les series bornees n'incluent pas les jours futurs du mois/semaine en cours.
- FichesPerimees renvoie une ligne par depot perime. Un SHA abrege est accepte ;
  le nombre de commits posterieurs compare les dates UTC du meme depot.

# Verification des attendus

`go run ./scripts/fixtures/make-repos.go` puis
`go test ./internal/sql/requetes_test -count=1 -args -maj` regenere les JSON et
PLANS.md. Sans `-maj`, les tests ne modifient aucun fichier de reference.

Les 28 JSON ont ete relus. Comptages independants dans make-repos.go : chaque
depot a 20 changements plus le commit de fusion, soit 21 commits (42 au total).
Janvier en a 7, fevrier 8 (fusion comprise), mars 6. Le mois de mars donne donc
6 commits par techno (Go/TypeScript pour Atlas, Markdown pour Boussole), et
ProjetsActifs renvoie 6 commits pour chacun des deux projets au 2026-03-31.
La premiere semaine de janvier a 5 commits par depot ; la suivante 2, dont le
binaire sans ajouts/suppressions. Les fichiers texte ajoutent 59 lignes par
depot (118 au total) ; le renommage inchange ajoute zero ligne.

Les deux sessions sont datees de juillet/octobre 2026 : elles apparaissent
dans les vues IA/frise, mais pas dans le calendrier borne au 2026-03-31.
Les fixtures ont les statuts actif/archive, donc ProjetsDormants est vide ;
un test distinct couvre en cours avec historique ancien et sans aucun commit.
Les trois liens casses sont Note (ancre retiree), Note absente et Page introuvable.
Echantillonnage est orpheline et sa date est NULL ; la recherche sans accent la
retrouve. Atlas -> Cartographie est un chemin direct, aussi teste a l'envers.
