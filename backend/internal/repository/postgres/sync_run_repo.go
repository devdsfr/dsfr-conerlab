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

func (r *SyncRunRepo) AddRun(ctx context.Context, e *domain.SyncRun) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO sync_runs
			(triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
		e.TriggeredBy, e.Targets, e.FixturesFound, e.FixturesUpserted, e.MatchesChecked, e.MatchesFinalized, e.Errors, e.DurationMs).
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
	var e domain.SyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms, created_at
		FROM sync_runs
		WHERE errors = 0 AND (fixtures_upserted > 0 OR matches_finalized > 0)
		ORDER BY created_at DESC LIMIT 1`).
		Scan(&e.ID, &e.TriggeredBy, &e.Targets, &e.FixturesFound, &e.FixturesUpserted, &e.MatchesChecked, &e.MatchesFinalized, &e.Errors, &e.DurationMs, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
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

func (r *SyncRunRepo) LastRun(ctx context.Context) (*domain.SyncRun, error) {
	var e domain.SyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, triggered_by, targets, fixtures_found, fixtures_upserted, matches_checked, matches_finalized, errors, duration_ms, created_at
		FROM sync_runs ORDER BY created_at DESC LIMIT 1`).
		Scan(&e.ID, &e.TriggeredBy, &e.Targets, &e.FixturesFound, &e.FixturesUpserted, &e.MatchesChecked, &e.MatchesFinalized, &e.Errors, &e.DurationMs, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
