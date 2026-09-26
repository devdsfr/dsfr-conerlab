package discovery

import (
	"encoding/json"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// REV-P4 (item 27) — o último ciclo do Discovery, lido de fonte PERSISTIDA.
//
// Antes, o funil só aparecia para quem acompanhava uma execução manual pela
// página: o resultado vivia na memória da API (progress.Tracker), sumia a cada
// deploy e o ciclo do cron — que roda em outro processo — nunca aparecia. A
// execução manual nem gravava em worker_runs.
//
// Agora cron e manual gravam em worker_runs com o MESMO formato de details
// (RunDetails), e a leitura escolhe o último ciclo CONCLUÍDO (LatestFinished).

// Trigger identifica quem disparou o ciclo.
const (
	TriggerCron   = "cron"
	TriggerManual = "manual"
)

// WorkerName é o valor de worker_runs.worker para os ciclos de descoberta.
const WorkerName = "discovery"

// RunDetails é o ÚNICO formato de worker_runs.details para ciclos de descoberta,
// usado pelo cron (cmd/worker) e pela execução manual (handler). Manter dois
// formatos faria a tela interpretar o mesmo ciclo de jeitos diferentes.
func (r Result) RunDetails(trigger string) map[string]any {
	return map[string]any{
		"trigger":      trigger,
		"leagues":      r.Leagues,
		"combinations": r.Combinations,
		"published":    r.Published,
		"deactivated":  r.Deactivated,
		"funnel":       r.Funnel,
		"rejections":   r.Rejections,
	}
}

// RunStatus decide o status gravado. Um ciclo com erro de execução OU
// interrompido NÃO é registrado como "ok" — ciclo que não terminou não pode
// aparecer na tela como uma varredura bem-sucedida que não achou nada.
func RunStatus(r Result, err error) string {
	if err != nil || r.Funnel.Interrupted {
		return "error"
	}
	return "ok"
}

// LatestFinished escolhe o último ciclo CONCLUÍDO entre os registros recebidos.
//
// Regra explícita (item 27, §10): maior finished_at; empate desfeito pelo maior
// id. Registros sem finished_at — ciclo ainda rodando, ou processo que morreu no
// meio e deixou a linha em "running" — são ignorados: não há resultado final a
// mostrar. Devolve nil se não houver nenhum ciclo concluído.
//
// Escolher em Go (e não só no ORDER BY) deixa a regra testável sem banco.
func LatestFinished(runs []domain.WorkerRun) *domain.WorkerRun {
	var best *domain.WorkerRun
	for i := range runs {
		r := &runs[i]
		if r.FinishedAt == nil || r.Worker != WorkerName {
			continue
		}
		if best == nil ||
			r.FinishedAt.After(*best.FinishedAt) ||
			(r.FinishedAt.Equal(*best.FinishedAt) && r.ID > best.ID) {
			best = r
		}
	}
	return best
}

// LastRun é o contrato de GET /discovery/last-run.
//
// Campos que um ciclo antigo não gravou ficam nulos — nunca zero inventado:
//   - Trigger nulo: ciclo gravado antes de a origem ser registrada;
//   - Funnel nulo: ciclo gravado antes do REV-P4 (sem funil).
type LastRun struct {
	ID           int64          `json:"id"`
	Status       string         `json:"status"`
	Trigger      *string        `json:"trigger"`
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at"`
	DurationMs   *int64         `json:"duration_ms"`
	Leagues      *int           `json:"leagues"`
	Combinations *int           `json:"combinations"`
	Published    *int           `json:"published"`
	Deactivated  *int           `json:"deactivated"`
	Errors       int            `json:"errors"`
	Funnel       *Funnel        `json:"funnel"`
	Rejections   map[string]int `json:"rejections"`
}

// ToLastRun traduz o registro persistido no contrato da API. Lê apenas as
// chaves conhecidas de details — nada do JSON bruto é repassado.
func ToLastRun(w domain.WorkerRun) LastRun {
	out := LastRun{
		ID:         w.ID,
		Status:     w.Status,
		StartedAt:  w.StartedAt,
		FinishedAt: w.FinishedAt,
		DurationMs: w.DurationMs,
		Errors:     w.Errors,
	}
	var d struct {
		Trigger      *string        `json:"trigger"`
		Leagues      *int           `json:"leagues"`
		Combinations *int           `json:"combinations"`
		Published    *int           `json:"published"`
		Deactivated  *int           `json:"deactivated"`
		Funnel       *Funnel        `json:"funnel"`
		Rejections   map[string]int `json:"rejections"`
	}
	if err := json.Unmarshal([]byte(w.Details), &d); err == nil {
		out.Trigger = d.Trigger
		out.Leagues = d.Leagues
		out.Combinations = d.Combinations
		out.Published = d.Published
		out.Deactivated = d.Deactivated
		out.Funnel = d.Funnel
		out.Rejections = d.Rejections
	}
	return out
}
