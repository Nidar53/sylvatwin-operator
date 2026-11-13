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

## fix/canonical-cluster-name (13/11/2026) : nom CANONIQUE du cluster workload (D2)

Avant ce correctif, le Snapshot du workload portait son nom NU
(`my-workload-kubeadm-capd`) comme `Snapshot.Cluster`. C'était un bug (D2,
confirmé) : le provider cluster-api de multicluster-runtime v0.24.1 engage
toujours un cluster workload sous le nom `types.NamespacedName.String()`
("namespace/nom"), jamais le nom nu. Sur CE banc, namespace et nom du Cluster
CAPI valent tous deux `my-workload-kubeadm-capd`, ce qui faisait
COÏNCIDER les deux formes en texte clair et masquait le bug dans les tests
figés (la fixture `photo()` de `mapper_test.go` reproduisait l'erreur sans
le savoir). `machineHasNode` (seul endroit qui construisait l'IRI du Node
visé SANS passer par le nom du `Snapshot.Cluster` lui-même) utilisait encore
le nom nu : incohérence entre l'IRI qu'il produisait et l'IRI réelle du Node
observé, d'où les `MachineNodeMismatchDrift` artefacts documentés dans
`HANDOFF.md`.

La fixture `photo()` a été corrigée pour refléter la réalité du watcher
(`Snapshot.Cluster = "my-workload-kubeadm-capd/my-workload-kubeadm-capd"`,
`internal/iri.CanonicalCluster`). `sylvatwin-core-v1-example.ttl` (dépôt
externe `sylvatwin-poc`, non modifié) reste écrit avec le nom nu. `compare.py`
réécrit donc, **côté opérateur uniquement**, le préfixe canonique vers le
préfixe nu avant comparaison (fonction `uncanon`), en plus des deux faits
désormais produits en plus (non documentés précédemment car invisibles tant
que le bug masquait tout) :

| Fait | Pourquoi |
|---|---|
| `:KubernetesCluster` en plus dans "sujets écrits en plus" | le Cluster CAPI nommé `my-workload-kubeadm-capd` produit maintenant SON PROPRE individu `:KubernetesCluster` distinct de celui du Cluster de management (comportement correct et attendu, pas un artefact) |
| préfixe d'IRI réécrit dans `compare.py` | voir ci-dessus ; ne cache aucune vraie erreur, seulement une différence de forme d'écriture du même nom sur ce banc précis (namespace == nom) |
