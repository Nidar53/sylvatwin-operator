#!/bin/bash
# sonde Flux K4 : 0 si le noeud workload porte le label de test (placement conforme), 1 sinon. Flux ne regarde pas les noeuds : reste a 1.
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
[ -n "$(k get nodes -o jsonpath="{.items[0].metadata.labels['sylvatwin\.orange\.com/zone']}")" ] && echo 0 || echo 1
