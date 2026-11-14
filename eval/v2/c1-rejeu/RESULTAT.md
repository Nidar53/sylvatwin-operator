# C1 — Machine sans Node : REJEU, non mesuré en réel (aucune injection)

Faits construits : les faits de l'état sain du 14/11/2026 (`out/facts.nt`, 17 089 triplets) moins UNE ligne, le `:machineHasNode` de la Machine
`my-workload-kubeadm-capd-control-plane-6w58l`. Oxigraph jetable (port 17890, en mémoire), règles `derives/08` puis `10` / `11` rejouées à la main (SPARQL, sans code de détection en Go).

| | faits sains (témoin) | faits C1 |
|---|---|---|
| MachineNodeMismatchDrift (derives/08) | 0 | **2** : sur la Machine, et sur le Node (branche symétrique) |
| FunctionImpactDrift (derives/10, chemin `infrastructure`) | — | **10** (les 10 NF tournent sur ce Node) |
| derives/11 (dépendance) | — | 0 nouveau (les 10 NF sont déjà touchées) |

Verdict (SUPPOSÉ, règle `lifecycle/05` : MachineNodeMismatchDrift ⊑ UnattendedDrift, horizon infini) : `ConfirmedViolation` tout de suite. Le verdict et l'âge n'ont PAS été rejoués ici
(le cycle de vie suppose Oxigraph et l'historique). Source : issue sylva-core 1442. Fichiers : `facts-c1.nt`, `d08-c1.nt`, `d10-…`, `d11-…`.
