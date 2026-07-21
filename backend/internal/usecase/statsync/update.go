package statsync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
)

// UpdateUsecase é o Worker 2 do critério de aceite: revisita partidas com status
// AGENDADO cuja data já passou, busca o resultado completo no provedor e grava como
// FINALIZADO. Se o provedor ainda não tem o resultado pronto (jogo atrasado, dados
// ainda não publicados), a partida simplesmente continua AGENDADO e é tentada de novo
// no próximo ciclo — nunca é um erro fatal.
type UpdateUsecase struct {
	provider  statsprovider.StatisticsProvider
	repo      *postgres.StatSyncRepo
	incidents *postgres.ProviderIncidentRepo
}

func NewUpdateUsecase(provider statsprovider.StatisticsProvider, repo *postgres.StatSyncRepo, incidents *postgres.ProviderIncidentRepo) *UpdateUsecase {
	return &UpdateUsecase{provider: provider, repo: repo, incidents: incidents}
}

// dueBuffer evita buscar o resultado de uma partida que talvez ainda esteja em
// andamento: só considera "atrasada" (e portanto pronta pra buscar) uma partida cuja
// data marcada já passou há pelo menos 2h (tempo de jogo + acréscimos + folga).
const dueBuffer = 2 * time.Hour

// maxPerCycle limita quantas partidas são atualizadas por ciclo, para não estourar a
// cota diária de um provedor com plano gratuito de uma vez só — o restante é pego no
// próximo ciclo (15 em 15 minutos).
const maxPerCycle = 20

type UpdateResult struct {
	Checked   int
	Finalized int
	StillOpen int
	Errors    int
}

func (u *UpdateUsecase) Run(ctx context.Context) (UpdateResult, error) {
	var result UpdateResult

	suspended, err := u.incidents.IsSuspended(ctx, u.provider.Name())
	if err != nil {
		return result, fmt.Errorf("erro ao checar suspensão do provedor: %w", err)
	}
	if suspended {
		slog.Warn("atualização pulada: provedor com incidente de saúde aberto", "provider", u.provider.Name())
		return result, nil
	}

	due, err := u.repo.ListDueForUpdate(ctx, dueBuffer, maxPerCycle)
	if err != nil {
		return result, fmt.Errorf("erro ao listar partidas pendentes: %w", err)
	}
	result.Checked = len(due)

	for _, d := range due {
		stats, err := u.provider.SyncFixtureStatistics(ctx, d.ExternalID)
		if err != nil {
			result.Errors++
			slog.Error("erro ao buscar estatísticas da partida", "provider", u.provider.Name(),
				"external_id", d.ExternalID, "error", err)
			continue
		}
		if stats.Status != statsprovider.FixtureFinished {
			result.StillOpen++
			continue
		}

		update := postgres.FixtureStatsUpdate{
			HomeGoals: stats.HomeGoals, AwayGoals: stats.AwayGoals,
			HomeCorners: stats.HomeCorners, AwayCorners: stats.AwayCorners,
			HomePossession: stats.HomePossessionPct, AwayPossession: stats.AwayPossessionPct,
			HomeShots: stats.HomeShots, AwayShots: stats.AwayShots,
			HomeShotsOnTarget: stats.HomeShotsOnTarget, AwayShotsOnTarget: stats.AwayShotsOnTarget,
			HomeYellowCards: stats.HomeYellowCards, AwayYellowCards: stats.AwayYellowCards,
			HomeRedCards: stats.HomeRedCards, AwayRedCards: stats.AwayRedCards,
			HomeShotsInsidebox: stats.HomeShotsInsidebox, AwayShotsInsidebox: stats.AwayShotsInsidebox,
			HomeShotsOutsidebox: stats.HomeShotsOutsidebox, AwayShotsOutsidebox: stats.AwayShotsOutsidebox,
			HomeBlockedShots: stats.HomeBlockedShots, AwayBlockedShots: stats.AwayBlockedShots,
			HomeFouls: stats.HomeFouls, AwayFouls: stats.AwayFouls,
			HomeOffsides: stats.HomeOffsides, AwayOffsides: stats.AwayOffsides,
			Referee: stats.Referee, Venue: stats.Venue,
		}
		if err := u.repo.FinalizeFixture(ctx, d.ExternalID, update); err != nil {
			result.Errors++
			slog.Error("erro ao gravar partida finalizada", "external_id", d.ExternalID, "error", err)
			continue
		}
		result.Finalized++
	}

	return result, nil
}
