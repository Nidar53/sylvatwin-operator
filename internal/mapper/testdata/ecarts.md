# Barrière 1 : écarts entre l'opérateur et `sylvatwin-core-v1-example.ttl`

`compare.py` compare la sortie de l'opérateur, calculée sur la photo réelle (`snapshot-20261108/`), à l'exemple de référence. Celui-ci a été écrit **à la main** le 08/11 dans sylvatwin-poc.

## Neutralisé des deux côtés (faits volatils)

| Fait | Pourquoi |
|---|---|
| `observedAt`, `lastSyncTime` | horodatages de la photo |
| `desiredRevision`, `actualRevision` | l'exemple date du commit `9099eec`, la photo du commit `6f6fb11` (Flux suit la branche `test`) |

## Écrit par l'opérateur, absent de l'exemple : faits VRAIS

| Fait | Preuve dans la photo |
|---|---|
| Cluster `management-cluster` `managedBy` HelmRelease `sylva-system/cluster` | label `helm.toolkit.fluxcd.io/name=cluster`, namespace `sylva-system` |
| Cluster `management-cluster` `ready true` | condition `Available=True` |
| GitRepository `sylvatwin-poc` `reconcileTimeout 60.0` | `spec.timeout: 60s` |
| Secret `udm-keys` `hasLabel` `app.kubernetes.io/part-of=sylvatwin-5gc` | label présent sur le Secret (procédure §3.11 du guide Open5GS : deux labels). L'exemple n'en gardait qu'un |
| NF `smf` `exposesInterface` N3 | `smf.gtpu.server` présent dans `smf-config` (lié à 127.0.0.1) ; port par défaut 2152/UDP |

Ces cinq faits sont listés dans `compare.py` (`DOCUMENTED`). Tout autre écart fait échouer la barrière.

## Sujets écrits en plus, hors de l'exemple

L'exemple est un sous-ensemble choisi à la main. L'opérateur écrit en plus :
- les Pods et ReplicaSets d'AMF et de SMF ;
- le label du Secret ;
- l'interface N3.

`compare.py` en donne le décompte, sans les comparer.

## v1.3.0 (partie 3, 10/11/2026) : StaleConfigDrift (D4)

| Fait | Preuve dans la photo |
|---|---|
| `mountsConfigMap` (5 NF → leur ConfigMap) | `spec.template.spec.volumes[].configMap.name` |
| `progressDeadlineSeconds 600.0` (5 NF) | champ absent dans la photo → défaut Kubernetes 600 s |
| `startedAt` (3 pods) | `status.startTime`, non retenu dans l'exemple écrit à la main |

Ces faits alimentent la règle `StaleConfigDrift` (D4) et le nouvel horizon de `ConvergenceDrift` (`progressDeadlineSeconds`, au lieu de l'horizon Flux).
