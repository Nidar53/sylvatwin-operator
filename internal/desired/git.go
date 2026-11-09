/*
Package desired : le cote DESIRE, lu depuis la VRAIE source Flux.

Methode (decision 4) :
 1. lire le GitRepository Flux : url, branche, status.artifact.revision
    ("<branche>@sha1:<sha>") ;
 2. cloner l'URL et faire un checkout du sha EXACT ;
 3. verifier que le sha obtenu est celui de l'artefact ; sinon le desire est
    declare "inconnu" (Known=false) : aucun fait desired* ne sera ecrit ;
 4. kustomize build du chemin de la Kustomization, avec ses options
    (targetNamespace) -> objets rendus (ConfigMaps generees comprises).

On n'utilise PAS l'artefact du source-controller : les NetworkPolicies de Sylva
en bloquent l'acces depuis un namespace tiers (observe le 09/11/2026), et on ne
les modifie pas.
*/
package desired

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Source : la description Flux d'un cote desire.
type Source struct {
	URL      string // GitRepository.spec.url
	Branch   string // GitRepository.spec.ref.branch
	Revision string // GitRepository.status.artifact.revision ("test@sha1:<sha>")
	Path     string // Kustomization.spec.path ("./usecase/5gc")
	// TargetNamespace : Kustomization.spec.targetNamespace (peut etre vide).
	TargetNamespace string
}

var revRe = regexp.MustCompile(`@sha1:([0-9a-f]{40})$`)

// SHA extrait le sha d'une revision Flux ("" si format inconnu).
func SHA(revision string) string {
	m := revRe.FindStringSubmatch(revision)
	if m == nil {
		return ""
	}
	return m[1]
}

// Checkout clone src.URL dans un dossier de cache (cacheDir/<sha>) et fait un
// checkout du sha de src.Revision. Reutilise le dossier s'il existe deja au bon
// sha. Renvoie le dossier et le sha REELLEMENT obtenu.
func Checkout(ctx context.Context, cacheDir string, src Source) (dir, gotSHA string, err error) {
	want := SHA(src.Revision)
	if want == "" {
		return "", "", fmt.Errorf("revision Flux illisible : %q", src.Revision)
	}
	dir = filepath.Join(cacheDir, want)
	if head, err := gitOut(ctx, dir, "rev-parse", "HEAD"); err == nil && head == want {
		return dir, head, nil
	}
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	tmp, err := os.MkdirTemp(cacheDir, ".clone-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tmp)
	// Clone partiel de la branche, puis checkout du sha exact. Le sha peut etre
	// plus ancien que la tete de branche : --filter=blob:none garde l'historique.
	args := []string{"clone", "--quiet", "--filter=blob:none", "--no-checkout"}
	if src.Branch != "" {
		args = append(args, "--branch", src.Branch)
	}
	args = append(args, src.URL, tmp)
	if _, err := gitOut(ctx, "", args...); err != nil {
		return "", "", fmt.Errorf("git clone %s : %w", src.URL, err)
	}
	if _, err := gitOut(ctx, tmp, "checkout", "--quiet", "--detach", want); err != nil {
		return "", "", fmt.Errorf("git checkout %s : %w", want, err)
	}
	got, err := gitOut(ctx, tmp, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return "", "", err
	}
	return dir, got, nil
}

func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Jamais d'invite interactive (identifiants) ; le proxy vient de l'environnement.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v : %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
