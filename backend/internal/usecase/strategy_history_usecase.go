package usecase

import (
	"context"
	"encoding/json"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// StrategyHistoryUsecase registra cada execução de backtest para permitir comparação
// de desempenho ao longo do tempo — regra geral do MVP ("O sistema deverá manter
// histórico completo de simulações e backtests para comparação ao longo do tempo")
// e seção "Histórico da Estratégia" do módulo de Inteligência Estatística.
type StrategyHistoryUsecase struct {
	history repository.StrategyHistoryRepository
}

func NewStrategyHistoryUsecase(history repository.StrategyHistoryRepository) *StrategyHistoryUsecase {
	return &StrategyHistoryUsecase{history: history}
}

func (u *StrategyHistoryUsecase) RecordRun(ctx context.Context, userID int64, savedFilterID *int64, leagueID int64, seasonIDs []int64, criteria FilterCriteria, result *BacktestResult) error {
	defBytes, err := json.Marshal(criteria)
	if err != nil {
		return err
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	entry := &domain.StrategyHistoryEntry{
		UserID:        userID,
		SavedFilterID: savedFilterID,
		Definition:    string(defBytes),
		LeagueID:      &leagueID,
		SeasonIDs:     seasonIDs,
		Result:        string(resultBytes),
	}
	return u.history.Create(ctx, entry)
}

func (u *StrategyHistoryUsecase) List(ctx context.Context, userID int64) ([]domain.StrategyHistoryEntry, error) {
	return u.history.List(ctx, userID)
}
