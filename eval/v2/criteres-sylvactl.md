# Critères sylvactl — écrits le 20/11/2026, AVANT tout essai (ne plus modifier)

## L'outil (VÉRIFIÉ le 20/11/2026)

* Binaire : `/opt/nidar/sylva-core/bin/sylvactl`, version **v0.14.10**. Lit le **cluster de management** (kubeconfig du management).
* Commandes : `status` (résumé puis sortie), `watch` (état des ressources Flux), `inventory` (inventaire d'une Kustomization/HelmRelease) :
  lecture seule d'après `--help`. **Jamais utilisés** : `bootstrap`, `cleanup`, `reset-bmc`, `inspect-bmh` ; et, pour `watch`, les options
  qui modifient le cluster : `--reconcile`, `--unit-timeout` (active `--reconcile`), `--resume-suspended`.
* Ce qu'il suit : les **ressources Flux** du management et leurs conditions (`watch -A --once` : 23 GitRepository, 33 HelmRepository,
  57 HelmChart, 57 HelmRelease, 153 Kustomization, 1 ArtifactGenerator, 1 ExternalArtifact ; namespaces `sylva-system`,
  `my-workload-kubeadm-capd` et **`sylvatwin-flux`**). Il **suit donc notre Kustomization `sylvatwin-flux/sylvatwin-5gc`** et notre
  GitRepository `sylvatwin-poc` (ligne `ReconciliationSucceeded - Applied revision: test@sha1:…`).
* Ce qu'il ne lit pas : les objets du **cluster workload** (pods, ConfigMaps, Secrets, nœuds). Il ne voit le cœur 5G qu'à travers la condition
  `Ready` de la Kustomization qui le déploie.
* État sain : chaque ressource sur une ligne (`Succeeded`, `ReconciliationSucceeded`, `InstallSucceeded`…), puis
  `All Done: all resources are ready!` ; `status -A` n'affiche que `All Done: all resources are ready!`.
* `watch` **sort dès que tout est prêt** (avec ou sans `--log`, avec ou sans cible) : il n'y a pas de mode continu sur un état sain. Le
  collecteur l'appellera donc périodiquement (`watch --once`, toutes les 10 s), comme la sonde Flux.

## Mesure

* Commande du collecteur (toutes les 10 s, horloge du serveur) : `sylvactl watch --once Kustomization/sylvatwin-flux/sylvatwin-5gc` et
  `sylvactl status -A`. Sortie brute gardée.
* **sylvactl signale** (dans la fenêtre t0 → avant `restore.sh`) : la sortie pour notre Kustomization n'est pas « ready » (pas de ligne
  `All Done: resource Kustomization/sylvatwin-flux/sylvatwin-5gc is ready!`), ou une ligne de notre Kustomization / GitRepository porte un état autre
  que `Succeeded` / `ReconciliationSucceeded`. Mesures : oui / non, instant du premier signal depuis t0, message exact, instant du retour à « prêt ».
* `status -A` est relevé à côté (il couvre toutes les unités de Sylva) mais ne compte pas pour notre scénario s'il signale une autre ressource.
* Avec 2 répétitions : résultats en k/2, sans intervalle de confiance.

## Attendus par scénario (tous SUPPOSÉ)

« Non » = sylvactl n'est **pas conçu** pour voir ce type d'écart (il ne lit que l'état des ressources Flux), pas « il rate la dérive ».

| id | sylvactl signale ? | ce qu'il est censé afficher (SUPPOSÉ) |
|---|---|---|
| K1 | non | rien : la Kustomization reste Ready (Flux ré-applique `replicas` en silence) |
| K2 | non | rien : Secret hors Git, Kustomization Ready |
| K3 | non | rien : idem K2, le pod udm en échec n'est pas une ressource Flux |
| K4 | non | rien : les nœuds et le placement ne sont pas suivis |
| K5 | non | rien : Flux remet la ConfigMap, la Kustomization reste Ready ; le pod non redémarré est invisible |
| F1 | **oui** | la Kustomization suspendue n'est plus « ready » (état suspendu affiché ; `--ignore-suspended` n'est pas utilisé), dès le premier relevé après t0 |
| F2 | **oui** | comme F1 (la suspension) ; le scale de udr lui-même est invisible |
| F3 | **oui** | `Ready=False`, raison `ReconciliationFailed`, message du dry-run (`imagePullPolicy` « Bogus »), après la première tentative de Flux (jusqu'à ~2 min après le push) ; ne revient à « prêt » qu'après le revert |
| N1 | non | rien : Flux remet `nrf-config`, la Kustomization reste Ready |
| N2 | non | rien : le commit s'applique sans erreur (au plus un état « en cours » fugace, non attendu à 10 s) |
| H1 | non | rien |
| H2 | non | rien |
| B1 | non | rien |
| B2 | non | rien |
| B3 | non | rien (au plus un état « en cours » fugace pendant l'application du commit) |
| B4 | non | rien (au plus un état « en cours » fugace pendant la réconciliation forcée) |

Un signal fugace (un seul relevé) sur B3, B4 ou N2 sera noté comme un résultat, sans modifier ce fichier.
