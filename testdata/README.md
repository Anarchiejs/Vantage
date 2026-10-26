# Fixtures de test

Ces données sont entièrement fictives et peuvent être recréées localement.

## `coffre/`

Mini-coffre Obsidian entièrement fictif : deux fiches `type: projet`, leurs
résumés/détails, deux sessions au format `type: session`, un journal au format
`type: journal`, des concepts et des notes d’IA. Cas couverts : accent dans un
titre, fiche `chemin_source: aucun (pas encore créé)`, `verifie_commit` périmé,
note orpheline, YAML invalide, deux liens cassés, alias, lien avec ancre,
inclusion d’image, lien ignoré dans un bloc de code, sous-note `.obsidian`
ignorée et fichier CRLF.

## `depots/`

Deux dépôts Git générés par `scripts/fixtures/make-repos.go`. Le générateur
produit des historiques déterministes avec plusieurs langages, des dates ISO
avec fuseaux +02:00, -05:00 et +05:30 (dont 23:30 UTC basculant au jour local
suivant), plusieurs auteurs, un renommage détectable par `git log -M` d’un
fichier inchangé de plus de 10 lignes, un chemin avec espace et accent, un
message multi-ligne, un fichier binaire et une fusion. Les répertoires Git
générés sont ignorés par Git et supprimés puis recréés à chaque exécution.
