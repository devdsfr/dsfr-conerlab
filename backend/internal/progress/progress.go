// Package progress acompanha o andamento de tarefas longas disparadas pela API.
//
// Existe porque duas ações do painel passaram a levar minutos em vez de segundos:
// "Sincronizar agora" (o cliente da API-Football espaça as chamadas para não tomar
// 429) e "Procurar agora" (a varredura roda milhares de backtests). Segurar uma
// requisição HTTP aberta todo esse tempo é frágil — proxy e navegador cortam — e
// deixa o usuário sem noção de quanto falta.
//
// O padrão adotado nos dois casos é o mesmo: o POST inicia o trabalho em segundo
// plano e responde na hora; um GET separado devolve este Snapshot, que o frontend
// consulta a cada poucos segundos para desenhar a barra.
//
// O andamento reportado é REAL, nunca estimado: só chamamos Phase() quando já
// sabemos o total de passos daquela fase.
package progress

import (
	"sync"
	"time"
)

// Fases genéricas de qualquer tarefa acompanhada.
const (
	PhaseIdle  = "idle"
	PhaseDone  = "concluido"
	PhaseError = "erro"
)

// Snapshot é a leitura imutável do estado, serializada para o frontend.
type Snapshot struct {
	Running    bool       `json:"running"`
	Phase      string     `json:"phase"`
	PhaseLabel string     `json:"phase_label"`
	Current    int        `json:"current"`
	Total      int        `json:"total"`
	Percent    float64    `json:"percent"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	DurationMs int64      `json:"duration_ms"`
	Error      string     `json:"error"`

	// Result carrega o resultado final da tarefa (formato livre, definido por
	// quem a executou). Só é preenchido quando Phase == PhaseDone.
	Result any `json:"result,omitempty"`
}

// Tracker guarda o estado de UMA tarefa em memória. Estado em memória basta porque
// a barra só interessa enquanto alguém está olhando — o registro durável do que
// aconteceu continua sendo o banco (sync_runs, worker_runs).
type Tracker struct {
	mu   sync.Mutex
	snap Snapshot
}

func NewTracker() *Tracker {
	return &Tracker{snap: Snapshot{Phase: PhaseIdle}}
}

// Start marca a tarefa como em andamento. Devolve false se já houver uma rodando —
// é o que impede dois cliques seguidos de dobrarem o trabalho (e, no caso da
// sincronização, o consumo da cota da API externa).
func (t *Tracker) Start(label string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.snap.Running {
		return false
	}
	agora := time.Now()
	t.snap = Snapshot{Running: true, Phase: PhaseIdle, PhaseLabel: label, StartedAt: &agora}
	return true
}

// Phase inicia uma fase com total conhecido de passos, zerando o contador.
func (t *Tracker) Phase(phase, label string, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snap.Phase = phase
	t.snap.PhaseLabel = label
	t.snap.Current = 0
	t.snap.Total = total
	// Fase sem trabalho já nasce completa, senão a barra trava em 0%.
	if total == 0 {
		t.snap.Percent = 100
	} else {
		t.snap.Percent = 0
	}
}

// Step avança um passo da fase atual.
func (t *Tracker) Step(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snap.Current++
	t.snap.Message = message
	if t.snap.Total > 0 {
		t.snap.Percent = float64(t.snap.Current) / float64(t.snap.Total) * 100
		if t.snap.Percent > 100 {
			t.snap.Percent = 100
		}
	}
}

// Detail atualiza só a mensagem, sem mover a barra. Serve para tarefas cujo passo
// é grosso (ex: uma liga inteira) mas que têm subtarefas rápidas por dentro: a
// barra anda de liga em liga, e o texto mostra que algo está acontecendo no meio.
func (t *Tracker) Detail(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snap.Message = message
}

// Finish encerra a tarefa, com erro ou com o resultado.
func (t *Tracker) Finish(err error, result any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	agora := time.Now()
	t.snap.Running = false
	t.snap.FinishedAt = &agora
	if t.snap.StartedAt != nil {
		t.snap.DurationMs = agora.Sub(*t.snap.StartedAt).Milliseconds()
	}
	if err != nil {
		t.snap.Phase = PhaseError
		t.snap.PhaseLabel = "Falhou"
		t.snap.Error = err.Error()
		return
	}
	t.snap.Phase = PhaseDone
	t.snap.PhaseLabel = "Concluído"
	t.snap.Percent = 100
	t.snap.Result = result
}

// Snapshot devolve uma cópia do estado atual.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snap
}

// Reporter é o contrato mínimo que os usecases usam para reportar andamento. Fica
// como interface para que eles não dependam do Tracker concreto e continuem
// testáveis sem ele.
type Reporter interface {
	Phase(phase, label string, total int)
	Step(message string)
	Detail(message string)
}

// Noop é o reporter usado quando ninguém está acompanhando (ex: ciclo do cron).
type Noop struct{}

func (Noop) Phase(string, string, int) {}
func (Noop) Step(string)               {}
func (Noop) Detail(string)             {}
