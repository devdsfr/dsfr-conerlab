package domain

import "time"

// SyncRun registra uma execução do ciclo de sincronização (descoberta + atualização)
// — disparada manualmente pelo botão "Sincronizar agora" ou pelo Render Cron Job.
// Serve para o painel Integrações mostrar "Última sincronização: ..." sem depender de
// estado local do navegador (que se perde ao recarregar a página).
type SyncRun struct {
	ID                int64     `json:"id" db:"id"`
	TriggeredBy       string    `json:"triggered_by" db:"triggered_by"` // "manual" | "cron"
	Targets           int       `json:"targets" db:"targets"`
	FixturesFound     int       `json:"fixtures_found" db:"fixtures_found"`
	FixturesUpserted  int       `json:"fixtures_upserted" db:"fixtures_upserted"`
	MatchesChecked    int       `json:"matches_checked" db:"matches_checked"`
	MatchesFinalized  int       `json:"matches_finalized" db:"matches_finalized"`
	Errors            int       `json:"errors" db:"errors"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}
