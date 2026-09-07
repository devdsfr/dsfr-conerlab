package statsync

import (
	"sync"
	"time"
)

// O botão "Sincronizar agora" passou a levar minutos, não segundos: desde que o
// cliente da API-Football ganhou throttle (~9 req/min, para não tomar 429), um ciclo
// que verifica 50 partidas gasta perto de 6 minutos. Segurar uma requisição HTTP
// aberta por todo esse tempo é frágil (proxy e navegador cortam) e deixa o usuário
// sem noção nenhuma de quanto falta.
//
// A solução é separar disparo de acompanhamento: POST /sync/run inicia o ciclo em
// segundo plano e responde na hora; GET /sync/progress devolve o estado atual, que
// o frontend consulta a cada poucos segundos para desenhar a barra.
//
// O progresso é REAL, não estimado: as duas fases são laços de tamanho conhecido —
// a descoberta percorre os campeonatos alvo e a atualização percorre as partidas
// pendentes (no máximo maxPerCycle). Total e posição atual saem daí.

// Fases reportadas na barra de progresso.
const (
	PhaseIdle      = "idle"
	PhaseDiscovery = "descoberta"
	PhaseUpdate    = "atualizacao"
	PhaseDone      = "concluido"
	PhaseError     = "erro"
)

// Snapshot é a leitura imutável do estado do ciclo, serializada para o frontend.
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

	// Resultado final do ciclo, preenchido só quando Phase == PhaseDone.
	Discovery *DiscoveryResult `json:"discovery"`
	Update    *UpdateResult    `json:"update"`
}

// Tracker guarda o estado do ciclo em memória. Uma instância por processo: o ciclo
// é único e exclusivo (Start recusa um segundo disparo simultâneo), então não há o
// que particionar por usuário. Estado em memória é suficiente porque a barra só
// interessa enquanto a aba está aberta — o registro durável do que aconteceu
// continua sendo a tabela sync_runs.
type Tracker struct {
	mu   sync.Mutex
	snap Snapshot
}

func NewTracker() *Tracker {
	return &Tracker{snap: Snapshot{Phase: PhaseIdle}}
}

// Start marca o ciclo como em andamento. Devolve false se já houver um rodando —
// é o que impede dois cliques seguidos em "Sincronizar agora" de dobrarem o consumo
// da cota da API externa.
func (t *Tracker) Start() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.snap.Running {
		return false
	}
	agora := time.Now()
	t.snap = Snapshot{
		Running:    true,
		Phase:      PhaseDiscovery,
		PhaseLabel: "Preparando…",
		StartedAt:  &agora,
	}
	return true
}

// Phase inicia uma fase com um total conhecido de passos, zerando o contador.
func (t *Tracker) Phase(phase, label string, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snap.Phase = phase
	t.snap.PhaseLabel = label
	t.snap.Current = 0
	t.snap.Total = total
	t.snap.Percent = 0
	if total == 0 {
		// Fase sem trabalho: já nasce completa, senão a barra trava em 0%.
		t.snap.Percent = 100
	}
}

// Step avança um passo dentro da fase atual.
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

// Finish encerra o ciclo, com erro ou com o resultado das duas fases.
func (t *Tracker) Finish(err error, d *DiscoveryResult, u *UpdateResult) {
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
	t.snap.Discovery = d
	t.snap.Update = u
}

// Snapshot devolve uma cópia do estado atual.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snap
}

// reporter é o contrato mínimo que os usecases usam para reportar andamento. Fica
// como interface para que Discovery/Update não dependam do Tracker concreto e
// continuem testáveis sem ele.
type reporter interface {
	Phase(phase, label string, total int)
	Step(message string)
}

// noopReporter é o padrão quando ninguém está acompanhando (ex: ciclo do cron).
type noopReporter struct{}

func (noopReporter) Phase(string, string, int) {}
func (noopReporter) Step(string)               {}
