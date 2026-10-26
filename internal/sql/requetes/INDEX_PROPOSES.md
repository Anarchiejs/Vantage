# Index proposes (migration reservee a Claude)

Les plans dans PLANS.md sont ceux du schema actuel, sans ces index.
Aucun parcours complet evitable de commit_git, fichier_modifie ou lien n'est
observe : les acces selectifs utilisent leurs index existants. LiensGraphe lit
tous les liens resolus via lien_cible, puisque toutes ces lignes sont demandees.

```sql
-- Calendrier (deux sous-requetes par jour), ChiffresPeriode : plage globale de dates.
CREATE INDEX session_date_projet ON session(date, projet_id);

-- SessionsIAParMois : groupe IA puis plage de dates ; couvre aussi
-- ActiviteIA et ProjetsParIA et evite l'index automatique observe dans le plan.
CREATE INDEX session_ia_date_projet ON session(ia, date, projet_id);
```

NoteParID parcourt recherche pour source/ref : ce sont des colonnes FTS5
UNINDEXED, et SQLite interdit CREATE INDEX sur une table virtuelle. Ce parcours
ne peut donc pas etre corrige par un index ordinaire. Une future migration
pourrait ajouter une correspondance note_id -> rowid FTS ; ce changement de
schema est hors du lot S2. Rechercher utilise bien l'index MATCH (0:M4).

Le LIKE avec joker initial de Frise sur journal ne peut pas utiliser un index
ordinaire sur texte. Les lectures globales des petites tables projet, techno,
session et des mesures de langage sont attendues pour les vues globales.
