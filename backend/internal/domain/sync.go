package domain

import "time"

// SyncRun registra uma execução do ciclo de sincronização (descoberta + atualização)
// — disparada manualmente pelo botão "Sincronizar agora" ou pelo Render Cron Job.
// Serve para o painel Integrações mostrar "Última sincronização: ..." sem depender de
// estado local do navegador (que se perde ao recarregar a página).
type SyncRun struct {
	ID               int64     `json:"id" db:"id"`
	TriggeredBy      string    `json:"triggered_by" db:"triggered_by"` // "manual" | "cron"
	Targets          int       `json:"targets" db:"targets"`
	FixturesFound    int       `json:"fixtures_found" db:"fixtures_found"`
	FixturesUpserted int       `json:"fixtures_upserted" db:"fixtures_upserted"`
	MatchesChecked   int       `json:"matches_checked" db:"matches_checked"`
	MatchesFinalized int       `json:"matches_finalized" db:"matches_finalized"`
	Errors           int       `json:"errors" db:"errors"`
	DurationMs       int64     `json:"duration_ms" db:"duration_ms"` // quanto tempo o ciclo completo (descoberta + atualização) levou
	CreatedAt        time.Time `json:"created_at" db:"created_at"`

	// Status distingue "o ciclo terminou" de "o ciclo foi interrompido".
	//
	// Antes só existia a primeira possibilidade, porque o registro era gravado
	// depois das duas fases: um ciclo que morresse no meio não deixava linha
	// nenhuma e a tela seguia mostrando a sincronização ANTERIOR como a última.
	// Ou seja, falhar deixava o sistema com aparência melhor do que rodar e não
	// trazer dado.
	Status string `json:"status" db:"status"`

	// ErrorMessage é o erro que interrompeu o ciclo. Vazio quando Status é
	// SyncStatusSuccess.
	ErrorMessage string `json:"error_message,omitempty" db:"error_message"`
}

// Valores válidos de SyncRun.Status.
const (
	// SyncStatusSuccess = as duas fases (descoberta e atualização) terminaram.
	// NÃO significa que o ciclo trouxe dado: um ciclo pode completar com zero
	// partidas novas e zero finalizadas. Quem responde "trouxe dado?" é
	// SyncRunRepository.LastSuccessfulRun.
	SyncStatusSuccess = "success"
	// SyncStatusFailed = o ciclo foi interrompido por erro.
	SyncStatusFailed = "failed"
)

// Valores válidos de SyncRun.TriggeredBy. Ficam no domínio porque são gravados
// por cmd/worker e lidos pelo handler HTTP: com a string solta nos dois lados,
// bastaria um "Cron" com maiúscula para a tela passar a dizer que o ciclo
// automático nunca rodou — sem nenhum erro de compilação avisando.
const (
	// SyncTriggerCron é o ciclo agendado (Render Cron Job, cmd/worker).
	SyncTriggerCron = "cron"
	// SyncTriggerManual é o ciclo disparado pelo botão "Sincronizar agora".
	SyncTriggerManual = "manual"
)
