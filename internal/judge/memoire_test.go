package judge

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMemoireCycles : mesure du RSS d'une instance Oxigraph EN MEMOIRE (comme en service, sans --location) sur N cycles complets du
// pipeline (faits d'un instantane reel dont les horodatages avancent). Variables : MEM_CYCLES (defaut : test saute), SYLVATWIN_POC,
// SYLVATWIN_SAIN_DIR, OXIGRAPH_BIN, Instance dediee, port aleatoire : jamais le service.
func TestMemoireCycles(t *testing.T) {
	ns, poc, dir, bin := os.Getenv("MEM_CYCLES"), os.Getenv("SYLVATWIN_POC"), os.Getenv("SYLVATWIN_SAIN_DIR"), os.Getenv("OXIGRAPH_BIN")
	if ns == "" || poc == "" || dir == "" || bin == "" {
		t.Skip("MEM_CYCLES / SYLVATWIN_POC / SYLVATWIN_SAIN_DIR / OXIGRAPH_BIN non definis")
	}
	n, _ := strconv.Atoi(ns)
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cmd := exec.Command(bin, "serve", "--bind", fmt.Sprintf("127.0.0.1:%d", port)) // memoire, comme le service
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	e, err := NewEndpoint(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := e.Select(context.Background(), "ASK {}"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	rss := func() int {
		b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", cmd.Process.Pid))
		m := regexp.MustCompile(`VmRSS:\s+(\d+)`).FindSubmatch(b)
		v, _ := strconv.Atoi(string(m[1]))
		return v / 1024
	}
	facts := mustReadFile(t, filepath.Join(dir, "facts.nt"))
	ts := regexp.MustCompile(`(core#(?:observedAt|lastSyncTime|lastWitnessRenewTime)> ")(\d{4}-\d\d-\d\dT\d\d:\d\d:)(\d\d)(Z")`)
	hist := filepath.Join(t.TempDir(), "history.nt")
	if err := os.WriteFile(hist, mustReadFile(t, filepath.Join(dir, "history.nt")), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600, HistoryFile: hist, TBoxFile: filepath.Join(poc, "sylvatwin-core-v1.ttl")}
	var courbe []string
	t0 := time.Now()
	for i := 0; i < n; i++ {
		f := ts.ReplaceAll(facts, []byte("${1}${2}"+fmt.Sprintf("%02d", i%60)+"${4}"))
		now := time.Date(2026, 11, 14, 20, 30, 40, 0, time.UTC).Add(time.Duration(i) * time.Second)
		if _, err := p.Run(context.Background(), f, now); err != nil {
			t.Fatal(err)
		}
		if i%25 == 0 || i == n-1 {
			courbe = append(courbe, fmt.Sprintf("%d:%d", i, rss()))
		}
	}
	t.Logf("%d cycles en %s ; RSS MB (cycle:MB) %s", n, time.Since(t0).Round(time.Second), strings.Join(courbe, " "))
}
