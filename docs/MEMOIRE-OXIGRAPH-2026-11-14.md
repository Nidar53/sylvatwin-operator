# Mémoire d'Oxigraph — relevé du 14/11/2026 (service en marche depuis 13 h 10, sans redémarrage)

## Mesures (VÉRIFIÉ)

| Instant | RSS Oxigraph | Cycles dans journal.jsonl |
|---|---|---|
| début du run (13/11 21:11 UTC) | 30,3 Mo | ~0 (run de 2 h : 145 cycles) |
| +2 h (23:11 UTC) | 76,4 Mo | 145 |
| +13 h 10 (14/11 10:15 UTC) | 330,9 Mo (VmHWM = VmRSS) | 1892 |
| +13 h 30 (10:28 UTC) | 335,5 Mo | 1922 |

Triplets par graphe nommé (identiques au début et à la fin) : facts 17 089, tbox 1 247, drifts 75, history 48.
La mémoire monte donc alors que le contenu est stable.

Échantillonnage de 10 min (`rss-echantillons.txt`, un relevé toutes les 15 s) : +4,5 Mo pour 26 cycles, soit ~0,17 Mo par cycle,
par paliers d'environ 1,5 Mo tous les ~8 cycles. Sur 2 h : 23 Mo/h ; de 2 h à 13 h : 23 Mo/h. **Croissance linéaire, sans plateau
observé**, proportionnelle au nombre de cycles, pas au contenu.

## Cause probable (SUPPOSÉ, non prouvée)

Chaque cycle remplace le graphe `facts` en entier (PUT de ~17 000 triplets, `PutGraph`), ainsi que `drifts` et `history`.
Oxigraph en mémoire s'appuie sur RocksDB en mémoire : les versions remplacées et les marqueurs de suppression ne sont libérés
qu'à la compaction, ou jamais rendus à l'allocateur. ~0,17 Mo par cycle pour 17 k triplets remplacés est cohérent avec cela.

## Propositions (rien n'est appliqué)

1. **Plafond de mémoire systemd** (`MemoryMax=` sur `sylvatwin-oxigraph`, `Restart=always` déjà en place). Au plafond, Oxigraph
   redémarre ; `history` et la TBox sont rechargés (vérifié le 13/11 : 16 `firstDetectedAt` identiques). Simple, mais le jugement
   échoue pendant la reprise (la reprise à 5 s couvre ce cas).
2. **Envoyer seulement le delta des faits** (DELETE/INSERT DATA des triplets changés) au lieu d'un PUT complet. Réduit le travail
   et probablement la fuite ; à mesurer ; un cycle typique ne change presque rien. Plus de code côté greffier, aucune règle touchée.
3. **Vérifier sur une version plus récente d'Oxigraph** si le comportement persiste.

## Effet observé sur la durée des cycles (VÉRIFIÉ, journal.jsonl)

La durée du jugement augmente avec l'âge du processus Oxigraph (la lecture des clusters, `observe`, reste à ~1,3 s) :

| Fenêtre | Âge d'Oxigraph | TOTAL médiane | judge_lifecycle médiane |
|---|---|---|---|
| 13/11 21:13 → 23:11 | 0 → 2 h | 1911 ms | 127 ms |
| 13/11 23:13 → 14/11 10:28 | 2 → 13 h | 2202 ms | 330 ms |
| 14/11 10:05 → 10:28 | ~12,5 h | 2419 ms | 477 ms |

Cohérent avec la même cause (versions remplacées qui s'accumulent). Un redémarrage d'Oxigraph le confirmerait (non fait : demandé « sans rien redémarrer »).
