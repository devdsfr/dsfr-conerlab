package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/delivery/http/handlers"
	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/pkg/adminaccess"
	"github.com/devdsfr/cornerlab/pkg/jwtutil"
	"github.com/gin-gonic/gin"
)

// REV-P4 (item 27) — GET /discovery/last-run no roteador REAL.

type runsFalsos struct{ runs []domain.WorkerRun }

func (f runsFalsos) StartWorkerRun(context.Context, string) (int64, error) { return 1, nil }
func (f runsFalsos) FinishWorkerRun(context.Context, int64, string, int, int, time.Time, map[string]any) error {
	return nil
}
func (f runsFalsos) RecentWorkerRuns(context.Context, string, int) ([]domain.WorkerRun, error) {
	return f.runs, nil
}

func getLastRun(t *testing.T, runs []domain.WorkerRun, comToken bool) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	adminaccess.Configure([]string{"admin@cornerlab.test"})
	t.Cleanup(func() { adminaccess.Configure(nil) })

	const segredo = "segredo-de-teste"
	r := NewRouter(Handlers{Discovery: handlers.NewDiscoveryHandler(nil, nil, runsFalsos{runs: runs})},
		segredo, usersRoteador{})
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/discovery/last-run", nil)
	if comToken {
		tok, err := jwtutil.GenerateToken(segredo, time.Hour, 2, "comum@cornerlab.test")
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestLastRun_UsuarioComumLe(t *testing.T) {
	fim := time.Date(2026, 9, 26, 1, 47, 6, 0, time.UTC)
	runs := []domain.WorkerRun{{
		ID: 37, Worker: "discovery", Status: "ok", StartedAt: fim.Add(-27 * time.Second), FinishedAt: &fim,
		Details: `{"trigger":"manual","leagues":12,"combinations":1944,"published":0,"deactivated":0,` +
			`"funnel":{"generated":1944,"rejected_no_real_odds":1671,"rejected_insufficient_sample":273},` +
			`"rejections":{"sem_odd_real":1671,"amostra_insuficiente":273}}`,
	}}
	w, body := getLastRun(t, runs, true)
	if w.Code != nethttp.StatusOK {
		t.Fatalf("usuário comum: status %d, esperado 200 — ler o funil não exige admin", w.Code)
	}
	if body["available"] != true {
		t.Fatalf("available = %v", body["available"])
	}
	run := body["run"].(map[string]any)
	if run["trigger"] != "manual" || run["status"] != "ok" {
		t.Errorf("trigger/status: %v / %v", run["trigger"], run["status"])
	}
	f := run["funnel"].(map[string]any)
	if f["generated"].(float64) != 1944 || f["rejected_no_real_odds"].(float64) != 1671 {
		t.Errorf("funil: %v", f)
	}
	// Nada sensível no corpo.
	raw := w.Body.String()
	for _, proibido := range []string{"admin@", "Bearer", "SELECT", "token"} {
		if contains(raw, proibido) {
			t.Errorf("resposta expõe %q: %s", proibido, raw)
		}
	}
}

func TestLastRun_SemCicloEstadoVazio(t *testing.T) {
	w, body := getLastRun(t, nil, true)
	if w.Code != nethttp.StatusOK || body["available"] != false {
		t.Errorf("sem ciclo: status %d, body %v — esperado 200 {available:false}", w.Code, body)
	}
	if _, tem := body["run"]; tem {
		t.Error("estado vazio não pode trazer um 'run' com zeros")
	}
}

func TestLastRun_ExigeLogin(t *testing.T) {
	w, _ := getLastRun(t, nil, false)
	if w.Code != nethttp.StatusUnauthorized {
		t.Errorf("sem token: %d, esperado 401", w.Code)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
