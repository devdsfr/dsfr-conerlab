package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncRunRepo implementa repository.SyncRunRepository.
type SyncRunRepo struct {
	db *pgxpool.Pool
}

func NewSyncRunRepo(db *pgxpool.Pool) *SyncRunRepo {
	return &SyncRunRepo{db: db}
}

// colunasSyncRun mantém a lista de colunas em UM lugar só. As três consultas de
// leitura abaixo precisam devolver exatamente os mesmos campos, na mesma ordem
// do Scan; quando isso era repetido à mão, acrescentar uma coluna significava
// lembrar de três lugares.
const colunasSyncRun = `id, triggered_by, targets, fixtures_found, fixtures_upserted,
	matches_checked, matches_finalized, errors, duration_ms, created_at,
	status, coalesce(error_message, '')`

func (r *SyncRunRepo) scanRun(row interface{ Scan(...any) error }) (*domain.SyncRun, error) {
	var e domain.SyncRun
	err := row.Scan(&e.ID, &e.TriggeredBy, &e.Targets, &e.FixturesFound, &e.FixturesUpserted,
		&e.MatchesChecked, &e.MatchesFinalized, &e.Errors, &e.DurationMs, &e.CreatedAt,
		&e.Status, &e.ErrorMessage)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SyncRunRepo) AddRun(ctx context.Context, e *domain.SyncRun) error {
	// Status vazio vira "success" em vez de estourar a constraint. Chamador antigo
	// que não conheça o campo continua gravando o que sempre gravou — um ciclo
	// completo — em vez de quebrar.
	status := e.Status
	if status == "" {
		status = domain.SyncStatusSuccess
	}

	// error_message só faz sentido quando houve erro. Gravar string vazia num
	// ciclo bem-sucedido deixaria a coluna ambígua ("falhou com mensagem vazia"?).
	var errMsg *string
	if e.ErrorMessage != "" {
		msg := e.ErrorMessage
		errMsg = &msg
	}

	return r.db.QueryRow(ctx, `
		INSERT INTO sync_runs
			(triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`,
		e.TriggeredBy, e.Targets, e.FixturesFound, e.FixturesUpserted, e.MatchesChecked, e.MatchesFinalized, e.Errors, e.DurationMs, status, errMsg).
		Scan(&e.ID, &e.CreatedAt)
}

// LastSuccessfulRun devolve o último ciclo que REALMENTE trouxe dado — não a
// última tentativa.
//
// A diferença é o motivo de este método existir. Entre 02/08 e 12/09 o cron
// rodou todos os dias, então "última sincronização" respondia com a data de
// hoje enquanto o banco estava seis semanas parado: a API recusava as chamadas
// devolvendo HTTP 200, o ciclo registrava a execução e ninguém percebia.
//
// "Trouxe dado" = gravou partida nova OU finalizou partida existente, sem erro.
// A exigência de errors = 0 é deliberada: um ciclo que grava 3 partidas e falha
// em 59 não é uma sincronização saudável, e tratá-lo como sucesso esconderia
// exatamente o tipo de degradação que se quer enxergar.
func (r *SyncRunRepo) LastSuccessfulRun(ctx context.Context) (*domain.SyncRun, error) {
	return r.scanRun(r.db.QueryRow(ctx, `
		SELECT `+colunasSyncRun+`
		FROM sync_runs
		WHERE status = '`+domain.SyncStatusSuccess+`'
		  AND errors = 0 AND (fixtures_upserted > 0 OR matches_finalized > 0)
		ORDER BY created_at DESC LIMIT 1`))
}

// LastProviderError devolve a mensagem do erro mais recente do provedor, para a
// tela poder dizer POR QUE a sincronização está parada em vez de só avisar que
// está. É a diferença entre "sem atualizar há 2 dias" e "sem atualizar há 2
// dias: sua conta na API-Football está suspensa".
func (r *SyncRunRepo) LastProviderError(ctx context.Context) (string, time.Time, error) {
	var msg string
	var at time.Time
	err := r.db.QueryRow(ctx, `
		SELECT coalesce(error_message, ''), created_at
		FROM api_usage_log
		WHERE success = false AND coalesce(error_message, '') <> ''
		ORDER BY created_at DESC LIMIT 1`).Scan(&msg, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, err
	}
	return msg, at, nil
}

// LastRunBySource devolve o último ciclo de uma origem específica.
//
// triggeredBy é o mesmo valor gravado em AddRun: "cron" (Render Cron Job, ver
// recordRun em cmd/worker/main.go) ou "manual" (botão "Sincronizar agora", ver
// SyncHandler.Run). Qualquer outro valor simplesmente não encontra linha.
//
// Existe para a tela conseguir responder "o worker rodou?" — pergunta que
// LastRun não responde, porque ele devolve o ciclo mais recente de qualquer
// origem e um clique manual esconde a ausência do automático.
func (r *SyncRunRepo) LastRunBySource(ctx context.Context, triggeredBy string) (*domain.SyncRun, error) {
	return r.scanRun(r.db.QueryRow(ctx, `
		SELECT `+colunasSyncRun+`
		FROM sync_runs
		WHERE triggered_by = $1
		ORDER BY created_at DESC LIMIT 1`, triggeredBy))
}

func (r *SyncRunRepo) LastRun(ctx context.Context) (*domain.SyncRun, error) {
	return r.scanRun(r.db.QueryRow(ctx, `
		SELECT `+colunasSyncRun+`
		FROM sync_runs ORDER BY created_at DESC LIMIT 1`))
}
