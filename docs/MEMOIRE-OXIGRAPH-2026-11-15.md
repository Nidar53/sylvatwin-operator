# Mémoire d'Oxigraph : mesure de trois variantes de chargement (15/11/2026)

Seconde instance d'Oxigraph (port 7979 ou aléatoire, en mémoire comme le service, hors service), 300 cycles de faits d'un instantané réel
(`eval/v2/bench_memoire_oxigraph.py`, `internal/judge/memoire_test.go`). RSS en Mo, cycle 0 → cycle 299.

| variante | faits stables (horodatages seuls qui bougent) | faits avec renouvellement des identifiants (uid, pire cas) |
|---|---|---|
| (a) PUT du graphe (code actuel) | 22 → 50 (plateau vers 50) ; pipeline complet : 24 → 53 | 22 → 205, croissance linéaire |
| (b) DROP GRAPH puis chargement | 22 → **128**, croissance linéaire (pire) | non mesuré (déjà pire) |
| (c) envoi du seul delta (DELETE/INSERT DATA) | 22 → 48 (équivalent à (a)) | 22 → 208, croissance linéaire |

**Constat (VÉRIFIÉ)** : aucune variante ne stabilise la mémoire quand les faits renouvellent des chaînes (pods, uid) : la croissance suit le
nombre de chaînes nouvelles, ce que le service voit à chaque répétition. (b) est pire, (c) n'apporte rien sur (a). **SUPPOSÉ** : le dictionnaire
de chaînes de l'instance en mémoire n'est pas libéré.

**Décision** : code du juge inchangé. La campagne redémarre Oxigraph (et le contrôleur) entre deux manches (`eval/v2/redemarrer_oxigraph.sh`,
appelé par `campagne.py` ; history relue depuis `out/history.nt`, TBox rechargée au premier cycle, état sain strict exigé). Mesuré : service à
343 Mo avant, 14 Mo après, retour à l'état sain strict en 11 s.
