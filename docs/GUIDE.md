# SylvaTwin v2 — opérateur Kubernetes (Kubebuilder)

Reprise du `fact_writer.py` sous forme d'opérateur. Même ontologie, mêmes
scénarios 0 et 1, même `detect.py`. Un seul cluster pour l'instant.

---

## 0. Prérequis

| Outil | Version | Vérifier |
|---|---|---|
| Go | ≥ 1.22 | `go version` |
| Docker | — | `docker ps` |
| kubectl | — | `kubectl version --client` |
| kind | — | `kind version` |
| make | — | `make --version` |

### Installer Go

```bash
cd /tmp
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz

# rendre go permanent
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### Installer Kubebuilder

```bash
curl -L -o kubebuilder \
  "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder
sudo mv kubebuilder /usr/local/bin/
kubebuilder version
```

---

## 1. Créer le projet

```bash
mkdir -p ~/sylvatwin-operator && cd ~/sylvatwin-operator

kubebuilder init \
  --domain orange.com \
  --repo github.com/orange/sylvatwin-operator \
  --project-name sylvatwin
```

Ce que ça génère :

| Chemin | Rôle |
|---|---|
| `cmd/main.go` | point d'entrée : démarre le manager |
| `Makefile` | toutes les commandes (`make run`, `make manifests`…) |
| `config/` | manifests Kustomize : CRD, RBAC, déploiement |
| `go.mod` | dépendances |

**Le `--repo` doit être exactement `github.com/orange/sylvatwin-operator`** — les
imports des fichiers fournis en dépendent.

---

## 2. Créer l'API (la CRD)

```bash
kubebuilder create api \
  --group sylvatwin \
  --version v1alpha1 \
  --kind TwinSync \
  --resource --controller
```

Répondre `y` aux deux questions. Ça crée :

- `api/v1alpha1/twinsync_types.go` — la structure de la CRD
- `internal/controller/twinsync_controller.go` — la boucle de réconciliation

---

## 3. Copier les fichiers fournis

```bash
cd ~/sylvatwin-operator

# écraser les squelettes générés
cp <fournis>/api/v1alpha1/twinsync_types.go              api/v1alpha1/
cp <fournis>/internal/controller/twinsync_controller.go  internal/controller/
cp <fournis>/internal/controller/mapper.go               internal/controller/

# nouveaux paquets
mkdir -p internal/rdf internal/desired
cp <fournis>/internal/rdf/graph.go       internal/rdf/
cp <fournis>/internal/desired/reader.go  internal/desired/

cp <fournis>/config/samples/sylvatwin_v1alpha1_twinsync.yaml config/samples/
```

Puis :

```bash
go mod tidy
make generate    # regénère zz_generated.deepcopy.go
make manifests   # regénère la CRD YAML + le rôle RBAC
```

`make generate` lit les marqueurs `+kubebuilder:object:root=true` et écrit les
méthodes `DeepCopy` obligatoires. `make manifests` lit les marqueurs
`+kubebuilder:rbac:` du contrôleur et produit `config/rbac/role.yaml`.

Vérifier que ça compile :

```bash
make build
```

---

## 4. Le cluster et l'état sain T0

Inchangé par rapport à la v1 :

```bash
kind create cluster --name sylvatwin --config kind-config.yaml
kubectl get nodes

kubectl label node sylvatwin-worker2 disktype=ssd
kubectl apply -f ~/sylvatwin-poc/git-desired/webapp-deployment.yaml
kubectl get pods -o wide
```

---

## 5. Installer la CRD et lancer l'opérateur

```bash
make install      # crée le type TwinSync dans le cluster
kubectl get crd twinsyncs.sylvatwin.orange.com
```

Adapter les chemins dans `config/samples/sylvatwin_v1alpha1_twinsync.yaml`
(remplacer `/chemin/vers/sylvatwin-poc` par le chemin de ton clone), puis :

```bash
kubectl apply -f config/samples/sylvatwin_v1alpha1_twinsync.yaml
```

Lancer l'opérateur **en local** (il tourne sur ton PC, parle au cluster) :

```bash
make run
```

C'est le mode de développement. Il utilise `~/.kube/config`, exactement comme
`fact_writer.py` v1, donc les chemins de fichiers du `spec` sont ceux de ton PC.

---

## 6. Vérifier

Dans un second terminal :

```bash
kubectl get twinsync
```

```
NAME              PHASE    TRIPLES   PODS   NODES   LASTSYNC
sylvatwin-local   Synced   147       8      3       5s
```

Le fichier est écrit :

```bash
ls -l ~/sylvatwin-poc/sylvatwin-graph.ttl
grep -c "a :Pod" ~/sylvatwin-poc/sylvatwin-graph.ttl
```

---

## 7. Jouer les scénarios

`detect.py` **ne change pas**. Les URI produites sont identiques à la v1.

### Scénario 0 — dérive de replicas

```bash
kubectl scale deployment webapp --replicas=5
# le git-desired dit toujours 3

cd ~/sylvatwin-poc && source .venv/bin/activate
python3 detect.py
```

### Scénario 1 — dérive de placement

```bash
kubectl label node sylvatwin-worker2 disktype-
kubectl get twinsync    # LASTSYNC repasse à quelques secondes

python3 detect.py
```

### Retour à l'état sain

```bash
kubectl label node sylvatwin-worker2 disktype=ssd
kubectl scale deployment webapp --replicas=3
```

---

## 8. Déployer dans le cluster (optionnel)

```bash
make docker-build IMG=sylvatwin-operator:v0.1.0
kind load docker-image sylvatwin-operator:v0.1.0 --name sylvatwin
make deploy IMG=sylvatwin-operator:v0.1.0

kubectl -n sylvatwin-system get pods
kubectl -n sylvatwin-system logs -f deploy/sylvatwin-controller-manager
```

**Attention** : dans le cluster, les chemins du `spec` pointent vers le
système de fichiers du pod, pas de ton PC. Il faut monter un volume
(`emptyDir` pour tester, PVC pour persister) et y placer la TBox et le
dossier `git-desired`. C'est justement ce que résoudra le passage à
Thing'in : le graphe ne sera plus un fichier.

Désinstaller :

```bash
make undeploy
make uninstall
```

---

## 9. Commandes utiles

| Commande | Effet |
|---|---|
| `make manifests generate` | après toute modif des types ou des marqueurs RBAC |
| `make build` | compile |
| `make run` | lance l'opérateur en local |
| `make install` / `uninstall` | pose / retire la CRD |
| `make deploy` / `undeploy` | déploie / retire l'opérateur dans le cluster |
| `kubectl describe twinsync sylvatwin-local` | statut détaillé |
| `kubectl edit twinsync sylvatwin-local` | changer `resyncPeriod` à chaud |

---

## 10. Erreurs fréquentes

**`no matches for kind "TwinSync"`** → `make install` oublié.

**Le fichier `.ttl` n'est pas écrit** → chemins du `spec` invalides. Vérifier
`kubectl get twinsync -o jsonpath='{.items[0].status.message}'`.

**`make run` : "unable to load kubeconfig"** → cluster kind non démarré.

**`undefined: SchemeBuilder`** → `make generate` oublié après avoir écrasé
`twinsync_types.go`.

**Le status ne se met pas à jour** → vérifier que le marqueur
`+kubebuilder:subresource:status` est bien présent, puis `make manifests install`.
