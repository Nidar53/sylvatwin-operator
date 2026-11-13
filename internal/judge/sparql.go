/*
Package judge : execute les regles de rules/ (depot sylvatwin-poc) sur un moteur
SPARQL 1.1 externe (Oxigraph), via le PROTOCOLE STANDARD W3C :

	SPARQL 1.1 Graph Store HTTP Protocol : charger les faits (PUT d'un graphe nomme)
	SPARQL 1.1 Protocol, query           : CONSTRUCT des regles derives/ et verdict/
	SPARQL 1.1 Protocol, update          : SPARQL Update des regles lifecycle/

INVARIANT GREFFIER / JUGE : ce paquet ne contient AUCUNE regle. Il lit des
fichiers .rq/.ru, les envoie dans l'ordre de leur nom, et renvoie les triplets.
La seule transformation est la substitution de parametres (__NOW__,
__VALIDITY__, __SECONDS__), identique a celle de tests/verif_v1.py.
La separation est "renforcee par l'architecture" (autre processus, interface
standard), pas "garantie" : une regle reste du code ecrit par nous.
*/
package judge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Graphes nommes (decision 6 : separation faits / derives / historique seulement).
const (
	GraphFacts   = "https://w3id.org/sylvatwin/graph/facts"
	GraphDrifts  = "https://w3id.org/sylvatwin/graph/drifts"
	GraphHistory = "https://w3id.org/sylvatwin/graph/history"
	// GraphTBox : la TBox (sylvatwin-core-v1.ttl), chargee au demarrage. Les regles
	// y lisent la hierarchie de classes de derive (rdfs:subClassOf*).
	GraphTBox = "https://w3id.org/sylvatwin/graph/tbox"
)

// Endpoint : un serveur SPARQL 1.1 (Oxigraph : /query, /update, /store).
type Endpoint struct {
	Base   string // ex. http://127.0.0.1:7878
	client *http.Client
}

// NewEndpoint : refuse toute adresse qui n'est pas en boucle locale (decision 5 :
// Oxigraph en ecoute sur 127.0.0.1 uniquement ; on ne parle a rien d'autre).
func NewEndpoint(base string) (*Endpoint, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("endpoint SPARQL %q refuse : boucle locale uniquement", base)
	}
	tr := &http.Transport{Proxy: nil} // jamais par le proxy Orange
	return &Endpoint{Base: strings.TrimRight(base, "/"), client: &http.Client{Transport: tr, Timeout: 5 * time.Minute}}, nil
}

func (e *Endpoint) do(ctx context.Context, method, path, ctype, accept string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, e.Base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s : HTTP %d : %s", method, path, resp.StatusCode, truncate(string(out), 400))
	}
	return out, nil
}

// PutGraph : remplace le contenu d'un graphe nomme (Graph Store Protocol).
func (e *Endpoint) PutGraph(ctx context.Context, graph string, ntriples []byte) error {
	_, err := e.do(ctx, http.MethodPut, "/store?graph="+url.QueryEscape(graph), "application/n-triples", "", ntriples)
	return err
}

// PutGraphTurtle : comme PutGraph, pour un document Turtle.
func (e *Endpoint) PutGraphTurtle(ctx context.Context, graph string, ttl []byte) error {
	_, err := e.do(ctx, http.MethodPut, "/store?graph="+url.QueryEscape(graph), "text/turtle", "", ttl)
	return err
}

// GetGraph : contenu d'un graphe nomme en N-Triples.
func (e *Endpoint) GetGraph(ctx context.Context, graph string) ([]byte, error) {
	return e.do(ctx, http.MethodGet, "/store?graph="+url.QueryEscape(graph), "", "application/n-triples", nil)
}

// DropGraph : vide un graphe nomme (sans erreur s'il n'existe pas).
func (e *Endpoint) DropGraph(ctx context.Context, graph string) error {
	return e.Update(ctx, "DROP SILENT GRAPH <"+graph+">")
}

// Construct : CONSTRUCT -> N-Triples, sur le graphe par defaut du serveur.
func (e *Endpoint) Construct(ctx context.Context, q string) ([]byte, error) {
	return e.ConstructIn(ctx, q, nil)
}

// ConstructIn : CONSTRUCT dont le graphe par defaut est l'UNION des graphes
// nommes donnes (parametre default-graph-uri du protocole SPARQL 1.1, section
// 2.1.4). Aucune copie : le serveur evalue directement sur ces graphes. Les
// regles de derives/ sont ecrites sans clause GRAPH ; c'est ce parametre qui
// leur dit ou lire. Liste vide : graphe par defaut du serveur.
func (e *Endpoint) ConstructIn(ctx context.Context, q string, graphs []string) ([]byte, error) {
	path := "/query"
	if len(graphs) > 0 {
		v := url.Values{}
		for _, g := range graphs {
			v.Add("default-graph-uri", g)
		}
		path += "?" + v.Encode()
	}
	return e.do(ctx, http.MethodPost, path, "application/sparql-query", "application/n-triples", []byte(q))
}

// Select : SELECT -> JSON (SPARQL 1.1 Query Results JSON).
func (e *Endpoint) Select(ctx context.Context, q string) ([]byte, error) {
	return e.do(ctx, http.MethodPost, "/query", "application/sparql-query", "application/sparql-results+json", []byte(q))
}

// Update : SPARQL 1.1 Update.
func (e *Endpoint) Update(ctx context.Context, u string) error {
	_, err := e.do(ctx, http.MethodPost, "/update", "application/sparql-update", "", []byte(u))
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
