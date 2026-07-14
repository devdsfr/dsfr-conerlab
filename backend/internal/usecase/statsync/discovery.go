// Package statsync contém a lógica de negócio do Módulo de Sincronização de Dados
// (Statistics Provider): descoberta de novos jogos, atualização de jogos encerrados e
// verificação de saúde do provedor. cmd/worker é só um agendador fino (tickers) em
// cima destes usecases — toda a regra fica aqui, testável sem precisar de um processo
// rodando em background.
package statsync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
)

// DiscoveryUsecase é o Worker 1 do critério de aceite: encontra jogos novos (ainda não
// conhecidos pelo CornerLab) e grava com status AGENDADO, sem duplicar — a
// idempotência vem do UNIQUE(external_id) já existente em matches, com upsert.
type DiscoveryUsecase struct {
	provider  statsprovider.StatisticsProvider
	repo      *postgres.StatSyncRepo
	incidents *postgres.ProviderIncidentRepo
}

func NewDiscoveryUsecase(provider statsprovider.StatisticsProvider, repo *postgres.StatSyncRepo, incidents *postgres.ProviderIncidentRepo) *DiscoveryUsecase {
	return &DiscoveryUsecase{provider: provider, repo: repo, incidents: incidents}
}

type DiscoveryResult struct {
	Targets          int
	FixturesFound    int
	FixturesUpserted int
	Errors           int
}

func (u *DiscoveryUsecase) Run(ctx context.Context) (DiscoveryResult, error) {
	var result DiscoveryResult

	suspended, err := u.incidents.IsSuspended(ctx, u.provider.Name())
	if err != nil {
		return result, fmt.Errorf("erro ao checar suspensão do provedor: %w", err)
	}
	if suspended {
		slog.Warn("descoberta pulada: provedor com incidente de saúde aberto", "provider", u.provider.Name())
		return result, nil
	}

	targets, err := u.repo.ListSyncTargets(ctx)
	if err != nil {
		return result, fmt.Errorf("erro ao listar campeonatos observados: %w", err)
	}
	result.Targets = len(targets)

	teamIDCache := map[string]int64{}
	resolveTeam := func(externalID, name, country string) (int64, error) {
		key := externalID
		if id, ok := teamIDCache[key]; ok {
			return id, nil
		}
		id, err := u.repo.UpsertTeam(ctx, externalID, name, shortName(name), country)
		if err != nil {
			return 0, err
		}
		teamIDCache[key] = id
		return id, nil
	}

	for _, t := range targets {
		fixtures, err := u.provider.SyncFixtures(ctx, t.LeagueExternalID, t.SeasonYear)
		if err != nil {
			// Falha em um campeonato não pode travar os demais — registra e segue.
			result.Errors++
			slog.Error("erro ao buscar partidas no provedor", "provider", u.provider.Name(),
				"league", t.LeagueName, "season", t.SeasonYear, "error", err)
			continue
		}
		result.FixturesFound += len(fixtures)

		for _, f := range fixtures {
			homeID, err := resolveTeam(f.HomeTeamExternalID, f.HomeTeamName, t.Country)
			if err != nil {
				result.Errors++
				slog.Error("erro ao gravar equipe mandante", "team", f.HomeTeamName, "error", err)
				continue
			}
			awayID, err := resolveTeam(f.AwayTeamExternalID, f.AwayTeamName, t.Country)
			if err != nil {
				result.Errors++
				slog.Error("erro ao gravar equipe visitante", "team", f.AwayTeamName, "error", err)
				continue
			}
			if err := u.repo.LinkTeamToLeague(ctx, t.LeagueID, homeID); err != nil {
				result.Errors++
				continue
			}
			if err := u.repo.LinkTeamToLeague(ctx, t.LeagueID, awayID); err != nil {
				result.Errors++
				continue
			}

			if err := u.repo.UpsertScheduledFixture(ctx, f.ExternalID, t.LeagueID, t.SeasonID, f.Round, f.MatchDate, homeID, awayID); err != nil {
				result.Errors++
				slog.Error("erro ao gravar partida descoberta", "external_id", f.ExternalID, "error", err)
				continue
			}
			result.FixturesUpserted++
		}
	}

	return result, nil
}

func shortName(name string) string {
	if len(name) <= 20 {
		return name
	}
	return name[:20]
}
