package usecase

import (
	"context"
	"fmt"
	"strconv"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata"
)

// SyncRepository é o contrato mínimo de persistência exigido pelo SyncUsecase — ver
// implementação concreta em internal/repository/postgres.SyncRepo.
type SyncRepository interface {
	UpsertLeague(ctx context.Context, externalID, name, country, tier string) (int64, error)
	UpsertSeason(ctx context.Context, leagueID int64, year int, label string) (int64, error)
	UpsertTeam(ctx context.Context, externalID, name, shortName, country string) (int64, error)
	LinkTeamToLeague(ctx context.Context, leagueID, teamID int64) error
	UpsertMatch(ctx context.Context, externalID string, leagueID, seasonID int64, round int, matchDate any,
		homeTeamID, awayTeamID int64, homeCorners, awayCorners, homeGoals, awayGoals int, cornerOdds map[string]float64) error
}

type SyncUsecase struct {
	provider sportsdata.Provider
	repo     SyncRepository
}

func NewSyncUsecase(provider sportsdata.Provider, repo SyncRepository) *SyncUsecase {
	return &SyncUsecase{provider: provider, repo: repo}
}

type SyncResult struct {
	Season         int
	FixturesFound  int
	MatchesSynced  int
	CornersMissing int
}

// SyncSeason busca todas as partidas de um campeonato/temporada no provedor
// configurado e grava (ou atualiza) leagues/seasons/teams/matches no PostgreSQL. Para
// partidas sem escanteios na listagem, tenta uma chamada adicional via
// provider.FetchCorners. Quando não há odds reais disponíveis, gera odds sintéticas
// (ver usecase.SyntheticCornerOdds) apenas para viabilizar o Simulador de Filtros —
// isso é sempre deixado explícito no disclaimer dos resultados de backtest.
func (u *SyncUsecase) SyncSeason(ctx context.Context, leagueName, country string, year int) (*SyncResult, error) {
	fixtures, err := u.provider.FetchFixtures(ctx, leagueName, country, year)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar partidas em %s: %w", u.provider.Name(), err)
	}
	result := &SyncResult{Season: year, FixturesFound: len(fixtures)}
	if len(fixtures) == 0 {
		return result, nil
	}

	// Backfill de escanteios ausentes (necessário para provedores como a API-Football,
	// que exigem uma chamada por partida).
	for i := range fixtures {
		if fixtures[i].HomeCorners != nil && fixtures[i].AwayCorners != nil {
			continue
		}
		home, away, ok, err := u.provider.FetchCorners(ctx, fixtures[i].ExternalID)
		if err != nil || !ok {
			result.CornersMissing++
			continue
		}
		fixtures[i].HomeCorners = &home
		fixtures[i].AwayCorners = &away
	}

	// Média de escanteios totais do lote, usada como parâmetro para as odds
	// sintéticas (ver comentário em SyntheticCornerOdds).
	sum, count := 0, 0
	for _, f := range fixtures {
		if f.HomeCorners != nil && f.AwayCorners != nil {
			sum += *f.HomeCorners + *f.AwayCorners
			count++
		}
	}
	batchMu := 9.5 // fallback razoável quando nenhuma partida do lote tem escanteios
	if count > 0 {
		batchMu = float64(sum) / float64(count)
	}
	odds := SyntheticCornerOdds(batchMu)

	leagueID, err := u.repo.UpsertLeague(ctx, fixtures[0].LeagueExternalID, leagueName, country, "G6")
	if err != nil {
		return nil, err
	}
	seasonID, err := u.repo.UpsertSeason(ctx, leagueID, year, strconv.Itoa(year))
	if err != nil {
		return nil, err
	}

	teamIDCache := map[string]int64{}
	resolveTeam := func(externalID, name string) (int64, error) {
		if id, ok := teamIDCache[externalID]; ok {
			return id, nil
		}
		id, err := u.repo.UpsertTeam(ctx, externalID, name, shortName(name), country)
		if err != nil {
			return 0, err
		}
		if err := u.repo.LinkTeamToLeague(ctx, leagueID, id); err != nil {
			return 0, err
		}
		teamIDCache[externalID] = id
		return id, nil
	}

	for _, f := range fixtures {
		homeID, err := resolveTeam(f.HomeTeamExternalID, f.HomeTeamName)
		if err != nil {
			return nil, err
		}
		awayID, err := resolveTeam(f.AwayTeamExternalID, f.AwayTeamName)
		if err != nil {
			return nil, err
		}

		homeCorners, awayCorners := 0, 0
		if f.HomeCorners != nil {
			homeCorners = *f.HomeCorners
		}
		if f.AwayCorners != nil {
			awayCorners = *f.AwayCorners
		}

		if err := u.repo.UpsertMatch(ctx, f.ExternalID, leagueID, seasonID, f.Round, f.MatchDate,
			homeID, awayID, homeCorners, awayCorners, f.HomeGoals, f.AwayGoals, odds); err != nil {
			return nil, err
		}
		result.MatchesSynced++
	}

	return result, nil
}

func shortName(name string) string {
	if len(name) <= 20 {
		return name
	}
	return name[:20]
}
