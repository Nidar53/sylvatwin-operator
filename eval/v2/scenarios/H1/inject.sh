#!/bin/bash
# Banc v2 : scenario H1. image changee a la main (scp)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] kubectl set image deploy/scp (meme image epinglee par digest)"; exit 0; fi
mark t0
dig=$(k -n $NS get pod -l app.kubernetes.io/name=scp -o jsonpath='{.items[0].status.containerStatuses[?(@.name=="scp")].imageID}' | sed 's/.*@//')
case "$dig" in sha256:*) ;; *) die "digest de l'image scp introuvable";; esac
k -n $NS set image deploy/scp scp=gradiant/open5gs:2.7.2@$dig >/dev/null || die "set image"
mark inject_fait
