# Critères v2.1 — addendum à `criteres-v2.md` (14/11/2026)

`criteres-v2.md` et les `oracle.json` de la version 2 sont **conservés tels quels** (campagne v2 du 14/11, 45 répétitions). La version 2.1 est dans `scenarios/<id>/oracle-2.1.json`.

## Note datée — 14/11/2026 : erreur d'identifiant dans l'oracle de F1, F2, F3

L'oracle v2 donnait pour cible `Kustomization/sylvatwin-5gc`. L'identifiant réellement produit par `iri.Resource` est
`GitOpsSyncDrift/management-cluster/sylvatwin-flux/Kustomization.kustomize.toolkit.fluxcd.io/sylvatwin-5gc` (le groupe d'API fait partie du nom).
Le test de sous-chaîne échouait : la dérive était comptée « non détectée » **et** « fausse alerte ». C'est une erreur du banc, pas de SylvaTwin.
**Correction** : la cible devient `Kustomization.kustomize.toolkit.fluxcd.io/sylvatwin-5gc`. **Les attendus (classe, verdict, impacts, fenêtre) ne changent pas.**
Vérifié par `test_oracle_cibles.py` : en v2, 3 cibles orphelines (F1, F2, F3) ; en v2.1, 0.

## Autres changements de la v2.1 (banc seulement)

* `flux_objet` dans chaque oracle : l'objet que Flux est censé corriger/appliquer (lecture dans le journal du kustomize-controller).
* **K4 ajouté** (placement) avec son oracle, écrit avant tout essai : `scenarios/K4/`.
  Préparation hors mesure : commit `nodeSelector sylvatwin.orange.com/zone=eval` sur `bsf` (branche test, `./usecase/5gc`), attente de Flux, pose du label de test sur le nœud workload, retour à l'état stable.
  Injection (t0) : retrait du label. Attendu : `PlacementDrift` sur `Pod/bsf`, Confirmed tout de suite (UnattendedDrift), impact `bsf` seul (chemin direct) ; Flux SUPPOSÉ : ne signale ni ne corrige rien (il ne regarde pas les nœuds).
  Restauration : retrait du label + `git revert` du commit de préparation.
  Risque évalué pour l'unique nœud workload : aucun (label d'une clé de test, `bsf` est une NF feuille ; la mise à jour de `bsf` passe par un surge, l'ancien pod reste en service jusqu'à ce que le nouveau soit prêt). Si la préparation échoue, la restauration retire le label et annule le commit.
* Mesures Flux : source retenue = journal du kustomize-controller ; instant d'application du commit ; deux délais SylvaTwin pour les scénarios Git ; « Flux corrige l'objet » séparé de « la conséquence disparaît » ; rappel dans le modèle / hors modèle.
