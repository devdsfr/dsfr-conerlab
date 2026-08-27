package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tentativas de conexão inicial. O Neon (Postgres serverless) SUSPENDE o compute
// depois de alguns minutos sem uso e só religa quando chega a primeira conexão —
// esse "cold start" costuma levar de 3 a 10 segundos, às vezes mais se a região
// estiver carregada. Com um único ping de 5s, o processo morria antes do banco
// terminar de acordar ("timeout: context deadline exceeded") e o deploy caía.
//
// Tentar de novo, com uma janela maior e espera progressiva, resolve o caso comum
// sem mascarar erro de verdade: credencial errada ou host inválido falham rápido e
// continuam falhando, então as tentativas se esgotam e o erro sobe igual.
const (
	connectAttempts   = 5
	connectPingWindow = 15 * time.Second
	connectRetryDelay = 3 * time.Second
)

// ErrDefaultDSN sinaliza que o processo caiu no DSN local padrão — quase sempre
// significa DATABASE_URL não configurada no ambiente (ver pkg/config).
var ErrDefaultDSN = errors.New("DATABASE_URL não configurada: o processo tentou conectar em localhost")

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, connectPingWindow)
		err := pool.Ping(pingCtx)
		cancel()

		if err == nil {
			if attempt > 1 {
				slog.Info("postgres conectado após espera", "tentativas", attempt)
			}
			return pool, nil
		}
		lastErr = err

		if attempt < connectAttempts {
			slog.Warn("postgres ainda não respondeu, tentando de novo",
				"tentativa", attempt, "de", connectAttempts, "erro", err)
			select {
			case <-time.After(connectRetryDelay):
			case <-ctx.Done():
				pool.Close()
				return nil, ctx.Err()
			}
		}
	}

	pool.Close()

	// Diagnóstico explícito para o erro mais comum em deploy: a variável de
	// ambiente não chegou no serviço e o processo tentou o localhost do container,
	// onde obviamente não há Postgres nenhum.
	if isLocalDSN(databaseURL) {
		return nil, fmt.Errorf("%w (último erro: %v)", ErrDefaultDSN, lastErr)
	}
	return nil, fmt.Errorf("não foi possível conectar no postgres após %d tentativas: %w", connectAttempts, lastErr)
}

func isLocalDSN(dsn string) bool {
	return strings.Contains(dsn, "@localhost") || strings.Contains(dsn, "@127.0.0.1")
}
