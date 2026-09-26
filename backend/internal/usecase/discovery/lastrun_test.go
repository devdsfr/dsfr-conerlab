package discovery

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P4 (item 27) — último ciclo persistido.

func ts(h int) *time.Time {
	t := time.Date(2026, 9, 26, h, 0, 0, 0, time.UTC)
	return &t
}

func detalhes(t *testing.T, r Result, trigger string) string {
	t.Helper()
	b, err := json.Marshal(r.RunDetails(trigger))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLatestFinished_SemRegistro(t *testing.T) {
	if LatestFinished(nil) != nil {
		t.Error("sem worker_run deveria devolver nil (estado vazio)")
	}
	// Só um ciclo ainda rodando / morto no meio: não há resultado a mostrar.
	if LatestFinished([]domain.WorkerRun{{ID: 9, Worker: WorkerName, Status: "running"}}) != nil {
		t.Error("ciclo sem finished_at não pode ser o 'último ciclo'")
	}
}

func TestLatestFinished_MaisNovoVence(t *testing.T) {
	runs := []domain.WorkerRun{
		{ID: 40, Worker: WorkerName, Status: "running"},                // mais novo, mas não terminou
		{ID: 38, Worker: WorkerName, Status: "ok", FinishedAt: ts(11)}, // cron
		{ID: 39, Worker: WorkerName, Status: "ok", FinishedAt: ts(14)}, // manual, terminou depois
		{ID: 37, Worker: "strategy", Status: "ok", FinishedAt: ts(23)}, // outro worker
	}
	got := LatestFinished(runs)
	if got == nil || got.ID != 39 {
		t.Fatalf("esperado id 39 (maior finished_at entre os concluídos), veio %+v", got)
	}
}

func TestLatestFinished_EmpateDesfeitoPeloID(t *testing.T) {
	runs := []domain.WorkerRun{
		{ID: 50, Worker: WorkerName, Status: "ok", FinishedAt: ts(11)},
		{ID: 51, Worker: WorkerName, Status: "ok", FinishedAt: ts(11)},
	}
	if got := LatestFinished(runs); got.ID != 51 {
		t.Errorf("empate em finished_at: esperado o maior id (51), veio %d", got.ID)
	}
}

func resultadoProducao() Result {
	// Números da execução manual real de 26/09/2026 01:46 UTC.
	return Result{
		Leagues: 12, Combinations: 1944,
		Rejections: map[string]int{"sem_odd_real": 1671, "amostra_insuficiente": 273},
		Funnel:     Funnel{Generated: 1944, NoRealOdds: 1671, InsufficientSample: 273},
	}
}

func TestToLastRun_CronEManual_FunilPreservado(t *testing.T) {
	for _, trigger := range []string{TriggerCron, TriggerManual} {
		w := domain.WorkerRun{
			ID: 70, Worker: WorkerName, Status: "ok",
			StartedAt: *ts(11), FinishedAt: ts(12), Errors: 0,
			Details: detalhes(t, resultadoProducao(), trigger),
		}
		lr := ToLastRun(w)
		if lr.Trigger == nil || *lr.Trigger != trigger {
			t.Errorf("trigger = %v, esperado %q", lr.Trigger, trigger)
		}
		if lr.Funnel == nil {
			t.Fatalf("%s: funil perdido", trigger)
		}
		if lr.Funnel.Generated != 1944 || lr.Funnel.NoRealOdds != 1671 || lr.Funnel.InsufficientSample != 273 {
			t.Errorf("%s: funil alterado na ida e volta: %+v", trigger, lr.Funnel)
		}
		if msg := lr.Funnel.Check(); msg != "" {
			t.Errorf("%s: funil persistido não fecha: %s", trigger, msg)
		}
		if lr.Leagues == nil || *lr.Leagues != 12 || lr.Combinations == nil || *lr.Combinations != 1944 {
			t.Errorf("%s: ligas/combinações perdidas", trigger)
		}
		if lr.Rejections["sem_odd_real"] != 1671 {
			t.Errorf("%s: motivos perdidos: %v", trigger, lr.Rejections)
		}
	}
}

func TestToLastRun_CicloAntigoSemFunilNaoInventaZero(t *testing.T) {
	// Formato gravado pelo cron ANTES do REV-P4 (worker_runs 30/33/36).
	w := domain.WorkerRun{
		ID: 36, Worker: WorkerName, Status: "ok", StartedAt: *ts(11), FinishedAt: ts(11),
		Details: `{"leagues": 12, "published": 0, "deactivated": 0, "combinations": 24804}`,
	}
	lr := ToLastRun(w)
	if lr.Funnel != nil {
		t.Errorf("ciclo sem funil ganhou funil %+v — seria zero inventado", lr.Funnel)
	}
	if lr.Trigger != nil {
		t.Errorf("ciclo sem origem registrada ganhou origem %q", *lr.Trigger)
	}
	if lr.Combinations == nil || *lr.Combinations != 24804 {
		t.Error("campos que existiam precisam continuar lá")
	}
}

func TestRunStatus_ErroNaoViraSucesso(t *testing.T) {
	ok := resultadoProducao()
	if RunStatus(ok, nil) != "ok" {
		t.Error("ciclo completo sem erro deveria ser ok")
	}
	if RunStatus(ok, errors.New("listar ligas")) != "error" {
		t.Error("erro de execução virou 'ok'")
	}
	interrompido := ok
	interrompido.Funnel.Interrupted = true
	if RunStatus(interrompido, nil) != "error" {
		t.Error("ciclo interrompido virou 'ok' — apareceria como varredura bem-sucedida")
	}
	w := domain.WorkerRun{ID: 1, Worker: WorkerName, Status: RunStatus(ok, errors.New("x")), FinishedAt: ts(1),
		Details: detalhes(t, ok, TriggerManual)}
	if ToLastRun(w).Status != "error" {
		t.Error("status de erro perdido na leitura")
	}
}

func TestRunDetails_MesmoFormatoParaCronEManual(t *testing.T) {
	r := resultadoProducao()
	c, m := r.RunDetails(TriggerCron), r.RunDetails(TriggerManual)
	if len(c) != len(m) {
		t.Fatalf("formatos diferentes: cron %d chaves, manual %d", len(c), len(m))
	}
	for k := range c {
		if _, ok := m[k]; !ok {
			t.Errorf("chave %q só existe no cron", k)
		}
	}
}
