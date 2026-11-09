# Critères d'évaluation, fixés AVANT les scénarios

**Écrit le 09/11/2026, avant la première injection. Ces critères ne seront plus modifiés.** Si l'un d'eux se révèle mal choisi, ce sera dit à part, dans la discussion du rapport, sans changer le verdict ci-dessous.

Versions de référence :
- opérateur : `sylvatwin-operator@test`, commit de ce fichier ;
- règles et ontologie : `sylvatwin-poc@test` `ce6ecc7`.

Banc : Sylva 1.7 en mode CAPD, management + 1 workload (1 nœud), cœur Open5GS partiel dans `sylvatwin-5gc`.

## Protocole commun

- **Mesure SylvaTwin.** `sylvatwin-once` tourne en boucle (tmux), avec **une période de 15 s entre la fin d'un cycle et le début du suivant**. L'historique Oxigraph (graphe nommé `history`) est conservé entre les cycles d'un même scénario, puis vidé entre deux scénarios.
- **Délai de détection.** C'est l'écart entre l'heure de l'injection (horodatage UTC du script, juste après le retour de la commande) et le `start` du premier cycle où la dérive attendue apparaît. Il **dépend de la période de la boucle** : sa borne attendue vaut période + durée d'un cycle.
- **Répétitions.** 3 par scénario et par événement bénin. Les heures d'injection sont journalisées dans `eval/runs/<scenario>/<rep>/`.
- **État sain.** Avant et après chaque répétition, on vérifie :
  - les 11 pods de `sylvatwin-5gc` sont `Running` et `Ready`, sans redémarrage imprévu ;
  - la Kustomization `sylvatwin-5gc` est `Ready=True` ;
  - sa révision appliquée est égale à celle de l'artefact ;
  - `udm-keys` est présent ;
  - le label `sylvatwin.orange.com/zone` est absent du nœud (sauf pendant S1 et S3-c) ;
  - un cycle SylvaTwin ne donne **aucune dérive**.
- **Flux.** On relève le statut et les events de la Kustomization, et les logs de `kustomize-controller` filtrés sur `sylvatwin-5gc`. « Flux voit » veut dire : Flux produit un statut, un event ou un log qui désigne l'objet ou l'écart. « Flux corrige » veut dire : la dérive disparaît sans action humaine. Délai de correction = résolution observée − injection.
- **kubectl.** On regarde ce qu'un opérateur humain verrait avec `kubectl get`, `kubectl describe` et `kubectl diff -k` (comparé au dépôt à la révision appliquée), sur l'objet concerné et le namespace.

**Limite connue avant les tests, issue de la barrière 3.** Le SCP n'a pas de classe dans l'ontologie v1.2.0 : il n'a donc pas de `NetworkFunction` et n'apparaîtra jamais dans les NF touchées. Ce n'est pas une erreur de l'oracle.

## Critères

| # | Critère | Seuil (décidé avant) |
|---|---|---|
| C1 | **Détection de bout en bout.** Pour chaque scénario injecté, SylvaTwin produit la dérive attendue (classe et ressource), sur le vrai cluster, dans au moins 3 répétitions sur 3. | 10 scénarios sur 10 |
| C2 | **Délai.** Le délai de détection médian est inférieur ou égal à **période + 2 × durée d'un cycle**, soit environ 20 s. | au moins 9 scénarios sur 10 |
| C3 | **Apport par rapport aux outils existants.** Nombre de scénarios où Flux **et** kubectl ne voient pas l'écart ou ne l'expliquent pas (pas d'impact sur la NF, pas de cause inter-couches, ou rien du tout), alors que SylvaTwin le détecte. | **N ≥ 4** |
| C4 | **Impacts inter-couches corrects.** Pour S3-a, S3-b et S3-c, les NF touchées et le chemin (direct, dépendance, flux) sont ceux de l'oracle ci-dessous, sans NF en trop ni en moins. | 3 sur 3, à chaque répétition |
| C5 | **Zéro faux positif.** Aucun verdict `ConfirmedViolation` sur les événements bénins, à aucune répétition. Une `SuspectedViolation` passagère est tolérée, mais comptée et rapportée. Aucune dérive non plus sur l'état sain avant chaque scénario. | 0 confirmée |
| C6 | **Verdict.** Les verdicts suspectée / confirmée sont ceux de l'oracle : une dérive que Flux corrige est suspectée puis résolue ; une dérive que rien ne corrige est confirmée dès sa détection. | 10 scénarios sur 10 |
| C7 | **Reproductibilité.** Depuis GitHub (`test`), `go test ./...` passe (barrières 1 et 2 incluses). Les scripts de scénarios et les journaux bruts sont dans `eval/`. | oui |
| C8 | **Retour à l'état sain** après chaque répétition, sans intervention hors du cadre. | 100 % |

**Pourquoi N ≥ 4 pour C3.** Flux corrige déjà S0-A, S3-b, S4-a (la ConfigMap) et les champs gérés. Prétendre que SylvaTwin « voit ce que Flux ne voit pas » sur ces cas serait faux. Les scénarios où l'on attend un vrai apport sont S0-B, S1, S2, S3-a, S3-c, S4-b et S5 (impact sur les NF). Avec N ≥ 4 sur ces 7, l'apport est majoritaire sans exiger qu'il soit total. Avec N < 4, la contribution se réduirait à « Flux en plus lent, avec des explications ».

## Oracle (attendus, écrits avant)

| Scénario | Injection | Dérive attendue | NF touchées (chemin) | Verdict attendu | Flux corrige ? (attendu) |
|---|---|---|---|---|---|
| S0-A | `kubectl scale deploy/nrf --replicas=0` | SpecDrift(nrf) | nrf (direct) ; amf, ausf, bsf, nssf, pcf, smf, udm, udr (dépendance) : les 8 NF modélisées qui déclarent le NRF (relevé de la barrière 3) | Suspectée, puis résolue | oui, ≤ 2 min + jitter |
| S0-B | Supprimer `udm-keys`, puis supprimer le pod udm : le nouveau pod reste en `ContainerCreating` | ConvergenceDrift(udm) + BrokenReferenceDrift | udm (direct) | ConvergenceDrift suspectée (horizon Flux, voir la limite) ; BrokenReference confirmée | non |
| S1 | Label `zone=a` posé, `nodeSelector` ajouté à `bsf.yaml` (commit Git), puis label retiré | PlacementDrift(pod bsf) | bsf (direct) | Confirmée | non |
| S2 | `kubectl delete secret udm-keys` | BrokenReferenceDrift(udm → udm-keys) | udm (direct) | Confirmée | non (Secret hors Git) |
| S3-a | = S2, sous l'angle des NF | FunctionImpactDrift(udm, direct) | udm seulement | Confirmée | non |
| S3-b | = S0-A, sous l'angle des NF | FunctionImpactDrift nrf (direct) + dépendantes (dépendance) | voir S0-A | Suspectée, puis résolue | oui |
| S3-c | = S1, sous l'angle des NF | FunctionImpactDrift(bsf, direct) | bsf seulement (aucune autre NF du nœud) | Confirmée | non |
| S4-a | `nrf-config` modifiée en direct (mnc 70 → 71) | InterNFConsistencyDrift (nrf/amf, nrf/smf) | nrf, amf, smf | Confirmée, puis résolue quand Flux restaure la ConfigMap | oui, sur la ConfigMap seulement |
| S4-b | `smf.yaml` dans Git : `sst: 2` ajouté (commit), appliqué par Flux | InterNFConsistencyDrift(smf) | smf | Confirmée | non (conforme à Git) |
| S5 | `flux suspend kustomization sylvatwin-5gc` | GitOpsSyncDrift | toutes les NF gérées (chemin « flux ») | Confirmée | non (tant que la Kustomization est suspendue) |
| B1 (bénin) | `kubectl delete pod` d'une NF (se recrée seul) | rien, ou ConvergenceDrift passagère | — | jamais Confirmée | — |
| B2 (bénin) | `kubectl rollout restart deploy/<nf>` | rien, ou passagère | — | jamais Confirmée | — |
