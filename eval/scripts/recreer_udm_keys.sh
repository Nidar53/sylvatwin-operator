#!/bin/bash
# Procedure §3.11 du guide Open5GS (cles SUCI d'EXEMPLE, publiques), par un pipe.
source "$(dirname "$0")/env.sh"
docker run --rm --network none --entrypoint sh gradiant/open5gs:2.7.2 \
  -c 'cd /opt/open5gs/etc/open5gs/hnet && tar cf - curve25519-1.key secp256r1-2.key' \
| python3 -c '
import sys, tarfile, io, json, base64
t = tarfile.open(fileobj=io.BytesIO(sys.stdin.buffer.read()))
data = {m.name: base64.b64encode(t.extractfile(m).read()).decode() for m in t.getmembers()}
print(json.dumps({"apiVersion":"v1","kind":"Secret","type":"Opaque",
  "metadata":{"name":"udm-keys","namespace":"sylvatwin-5gc",
    "labels":{"app.kubernetes.io/part-of":"sylvatwin-5gc","sylvatwin.orange.com/hors-git":"true"}},
  "data": data}))' \
| kubectl --kubeconfig $WL_KC apply -f -
