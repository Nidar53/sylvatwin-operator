#!/bin/bash
source "$(dirname "$0")/env.sh"
for g in facts drifts history; do
  curl -s --noproxy '*' -X POST -H 'Content-Type: application/sparql-update' \
    --data "DROP SILENT GRAPH <https://w3id.org/sylvatwin/graph/$g>" $SPARQL/update
done
