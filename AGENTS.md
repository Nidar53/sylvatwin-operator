# AGENTS.md — consignes pour un agent qui reprend ce dépôt

Lire d'abord `README.md` puis `HANDOFF.md`. Règles non négociables :

* Synchronisation **temps réel pilotée par Watch** ; aucune boucle périodique qui relit le cluster.
* **Aucune logique de détection en Go** : le juge SPARQL (`sylvatwin-poc/rules`) décide. Ne pas changer le sens d'une règle sans l'accord de l'auteur.
* Le greffier (`internal/mapper`) reste une fonction pure d'instantanés.
* TBox : modifications **additives** (OWL 2 DL et RL, à vérifier avec ROBOT).
* **Lecture seule** sur les clusters ; jamais de jeton, de kubeconfig ni de contenu de Secret affiché.
* Ne jamais supprimer d'archive de mesures ; ne pousser que la branche de travail convenue.
* Pas de test long (plus de 2 minutes) sans prévenir ; ne pas lancer `TestDisqueA4` / `TestMesureA4`.
