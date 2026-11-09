# Definition des scenarios (source par rep.sh). Cadre STRICT :
#   - namespace sylvatwin-5gc du workload, label sylvatwin.orange.com/zone du noeud ;
#   - Flux : suspend/resume de NOTRE Kustomization sylvatwin-5gc uniquement ;
#   - Git : commits de test sur sylvatwin-poc@test, annules par revert (S1, S4-b).
# Pour chaque scenario X : inject_X, observe_X, kubectl_view_X, undo_X.

# ---------- outils ----------
diff_k() {  # kubectl diff de nos manifestes, a la revision APPLIQUEE (clone du cache)
  local rev sha d
  rev=$(km -n $KS_NS get ks $KS -o jsonpath='{.status.lastAppliedRevision}'); sha=${rev##*:}
  d=/opt/nidar/cache-desired/$sha
  if [ ! -d "$d" ]; then
    git clone -q --filter=blob:none --no-checkout -b test https://github.com/Nidar53/sylvatwin-poc "$d.tmp" \
      && git -C "$d.tmp" checkout -q --detach "$sha" && mv "$d.tmp" "$d"
  fi
  # diff : code 0 = aucune difference, 1 = differences
  k diff -k "$d/usecase/5gc" 2>&1 | grep -vE '^(diff|---|\+\+\+) ' | grep -E '^[-+]' | grep -vE 'generation|resourceVersion|managedFields|uid:|creationTimestamp|^[-+]\s+(time|manager|operation|apiVersion|fieldsType|fieldsV1|f:)' | head -30
  echo "(fin du diff)"
}
flux_reconcile() { flux --kubeconfig $MGMT_KC -n $KS_NS reconcile source git sylvatwin-poc >/dev/null 2>&1; flux --kubeconfig $MGMT_KC -n $KS_NS reconcile kustomization $KS --timeout=5m 2>&1 | tail -1; }
git_push_test() {  # $1 = message ; commit des fichiers deja modifies dans POC
  git -C $POC add -A usecase/5gc && git -C $POC commit -q -m "$1" && git -C $POC push -q origin test && git -C $POC rev-parse HEAD
}
git_revert_last() {  # annule le dernier commit de test (revert, jamais de reecriture)
  git -C $POC revert --no-edit HEAD >/dev/null && git -C $POC push -q origin test && git -C $POC rev-parse HEAD
}

# ---------- S0-A / S3-b : replicas modifies a la main (nrf a 0) ----------
inject_S0-A() { k -n $NS scale deploy/nrf --replicas=0; }
observe_S0-A() { k -n $NS get deploy nrf -o jsonpath='nrf spec={.spec.replicas} ready={.status.readyReplicas}{"\n"}'; }
kubectl_view_S0-A() { k -n $NS get deploy nrf; k -n $NS describe deploy nrf | sed -n '/Replicas:/p;/Events:/,$p'; echo "--- diff"; diff_k; }
undo_S0-A() { flux_reconcile; }     # Flux l'a normalement deja corrige
inject_S3-b() { inject_S0-A; }; observe_S3-b() { observe_S0-A; }; kubectl_view_S3-b() { kubectl_view_S0-A; }; undo_S3-b() { undo_S0-A; }

# ---------- S0-B : Kubernetes ne converge pas (udm sans son Secret) ----------
inject_S0-B() { k -n $NS delete secret udm-keys; k -n $NS delete pod -l app.kubernetes.io/name=udm --wait=false; }
observe_S0-B() { k -n $NS get pods -l app.kubernetes.io/name=udm --no-headers; }
kubectl_view_S0-B() { k -n $NS get deploy udm; k -n $NS get pods -l app.kubernetes.io/name=udm; k -n $NS describe pod -l app.kubernetes.io/name=udm | sed -n '/Events:/,$p' | tail -8; echo "--- diff"; diff_k; }
undo_S0-B() { "$(dirname "${BASH_SOURCE[0]}")/recreer_udm_keys.sh"; }

# ---------- S2 / S3-a : Secret hors Git supprime ----------
inject_S2() { k -n $NS delete secret udm-keys; }
observe_S2() { k -n $NS get secret udm-keys --no-headers 2>&1 | head -1; }
kubectl_view_S2() { k -n $NS get deploy udm; k -n $NS get pods -l app.kubernetes.io/name=udm; k -n $NS get secret udm-keys 2>&1; echo "--- diff"; diff_k; }
undo_S2() { "$(dirname "${BASH_SOURCE[0]}")/recreer_udm_keys.sh"; }
inject_S3-a() { inject_S2; }; observe_S3-a() { observe_S2; }; kubectl_view_S3-a() { kubectl_view_S2; }; undo_S3-a() { undo_S2; }

# ---------- S1 / S3-c : noeud re-etiquete, pod bsf mal place ----------
# Pre-requis (prepare_S1, une fois) : label zone=a sur le noeud PUIS nodeSelector
# zone=a dans bsf.yaml (commit Git, applique par Flux).
inject_S1() { k label node $NODE $ZONE-; }
observe_S1() { k get node $NODE -o jsonpath="zone={.metadata.labels.sylvatwin\.orange\.com/zone}{\"\n\"}"; k -n $NS get pods -l app.kubernetes.io/name=bsf -o wide --no-headers; }
kubectl_view_S1() { k get node $NODE --show-labels | tr ',' '\n' | grep -c zone; k -n $NS get pods -l app.kubernetes.io/name=bsf -o wide; k -n $NS get deploy bsf -o jsonpath='{.spec.template.spec.nodeSelector}{"\n"}'; echo "--- diff"; diff_k; }
undo_S1() { k label node $NODE $ZONE=a --overwrite; }
inject_S3-c() { inject_S1; }; observe_S3-c() { observe_S1; }; kubectl_view_S3-c() { kubectl_view_S1; }; undo_S3-c() { undo_S1; }

# ---------- S4-a : PLMN du NRF modifie en direct (ConfigMap) ----------
inject_S4-a() { k -n $NS get cm nrf-config -o json | sed 's/mnc: 70/mnc: 71/' | k apply -f - 2>&1 | grep -v 'Warning: resource configmaps/nrf-config is missing'; }
observe_S4-a() { k -n $NS get cm nrf-config -o jsonpath='{.data.nrf\.yaml}' | grep plmn_id; }
kubectl_view_S4-a() { k -n $NS get cm nrf-config -o jsonpath='{.data.nrf\.yaml}' | grep plmn; k -n $NS get cm amf-config -o jsonpath='{.data.amf\.yaml}' | grep -m1 plmn; echo "--- diff"; diff_k; }
undo_S4-a() { flux_reconcile; }

# ---------- S4-b : tranche ajoutee au SMF dans Git, oubliee dans l'AMF ----------
inject_S4-b() {
  python3 - "$POC/usecase/5gc/config/smf.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old="    - s_nssai:\n        - sst: 1\n          dnn:\n            - internet\n"
assert old in s, "structure smf.yaml inattendue"
s=s.replace(old, old+"        - sst: 2\n          dnn:\n            - internet\n",1); open(p,"w").write(s)
PY
  git_push_test "TEST S4-b (a annuler) : tranche sst 2 ajoutee au SMF, pas a l'AMF"
  flux_reconcile
}
observe_S4-b() { k -n $NS get cm smf-config -o jsonpath='{.data.smf\.yaml}' | grep -c 'sst:'; }
kubectl_view_S4-b() { k -n $NS get cm smf-config -o jsonpath='{.data.smf\.yaml}' | grep -A1 sst; k -n $NS get cm amf-config -o jsonpath='{.data.amf\.yaml}' | grep -A1 sst; echo "--- diff (attendu : vide, conforme a Git)"; diff_k; }
undo_S4-b() { git_revert_last; flux_reconcile; }

# ---------- S5 : Kustomization suspendue ----------
inject_S5() { flux --kubeconfig $MGMT_KC -n $KS_NS suspend kustomization $KS; }
observe_S5() { km -n $KS_NS get ks $KS -o jsonpath='suspend={.spec.suspend}{"\n"}'; }
kubectl_view_S5() { km -n $KS_NS get ks $KS; flux --kubeconfig $MGMT_KC -n $KS_NS get kustomization $KS 2>&1; }
undo_S5() { flux --kubeconfig $MGMT_KC -n $KS_NS resume kustomization $KS 2>&1 | tail -1; }

# ---------- Evenements BENINS ----------
inject_B1() { k -n $NS delete pod -l app.kubernetes.io/name=ausf --wait=false; }
observe_B1() { k -n $NS get pods -l app.kubernetes.io/name=ausf --no-headers; }
kubectl_view_B1() { k -n $NS get pods -l app.kubernetes.io/name=ausf; }
undo_B1() { :; }
inject_B2() { k -n $NS rollout restart deploy/pcf; }
observe_B2() { k -n $NS get pods -l app.kubernetes.io/name=pcf --no-headers; }
kubectl_view_B2() { k -n $NS rollout status deploy/pcf --timeout=10s 2>&1; }
undo_B2() { k -n $NS rollout status deploy/pcf --timeout=180s; }
