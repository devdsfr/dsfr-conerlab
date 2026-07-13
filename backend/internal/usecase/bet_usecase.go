package usecase

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

type BetUsecase struct {
	bets repository.BetRepository
}

func NewBetUsecase(bets repository.BetRepository) *BetUsecase {
	return &BetUsecase{bets: bets}
}

func (u *BetUsecase) Register(ctx context.Context, b *domain.Bet) error {
	computeProfitLoss(b)
	return u.bets.Create(ctx, b)
}

func (u *BetUsecase) Update(ctx context.Context, b *domain.Bet) error {
	computeProfitLoss(b)
	return u.bets.Update(ctx, b)
}

func (u *BetUsecase) List(ctx context.Context, userID int64) ([]domain.Bet, error) {
	return u.bets.List(ctx, userID)
}

func (u *BetUsecase) Delete(ctx context.Context, id, userID int64) error {
	return u.bets.Delete(ctx, id, userID)
}

func computeProfitLoss(b *domain.Bet) {
	switch b.Status {
	case domain.BetStatusWon:
		b.ProfitLoss = round2(b.Stake * (b.Odd - 1))
	case domain.BetStatusLost:
		b.ProfitLoss = -b.Stake
	case domain.BetStatusVoid, domain.BetStatusPending:
		b.ProfitLoss = 0
	}
}

// FinancialDashboard agrega as métricas do Módulo 5 / Módulo 8.
type FinancialDashboard struct {
	TotalBets   int     `json:"total_bets"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	WinRate     float64 `json:"win_rate"`
	ROI         float64 `json:"roi"`
	Yield       float64 `json:"yield"`
	NetProfit   float64 `json:"net_profit"`
	GrossProfit float64 `json:"gross_profit"`
	AverageOdd  float64 `json:"average_odd"`
	HighestOdd  float64 `json:"highest_odd"`
	LowestOdd   float64 `json:"lowest_odd"`
	BiggestWin  float64 `json:"biggest_win"`
	BiggestLoss float64 `json:"biggest_loss"`
}

func (u *BetUsecase) FinancialDashboard(ctx context.Context, userID int64) (*FinancialDashboard, error) {
	bets, err := u.bets.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	d := &FinancialDashboard{}
	if len(bets) == 0 {
		return d, nil
	}

	totalStaked := 0.0
	totalOdds := 0.0
	settledCount := 0
	d.LowestOdd = bets[0].Odd

	for _, b := range bets {
		d.TotalBets++
		totalOdds += b.Odd
		if b.Odd > d.HighestOdd {
			d.HighestOdd = b.Odd
		}
		if b.Odd < d.LowestOdd {
			d.LowestOdd = b.Odd
		}
		if b.Status == domain.BetStatusWon || b.Status == domain.BetStatusLost {
			settledCount++
			totalStaked += b.Stake
			d.NetProfit += b.ProfitLoss
			if b.ProfitLoss > 0 {
				d.GrossProfit += b.ProfitLoss
			}
			if b.ProfitLoss > d.BiggestWin {
				d.BiggestWin = b.ProfitLoss
			}
			if b.ProfitLoss < d.BiggestLoss {
				d.BiggestLoss = b.ProfitLoss
			}
		}
		if b.Status == domain.BetStatusWon {
			d.Wins++
		}
		if b.Status == domain.BetStatusLost {
			d.Losses++
		}
	}

	if settledCount > 0 {
		d.WinRate = round2(100 * float64(d.Wins) / float64(settledCount))
	}
	if totalStaked > 0 {
		d.ROI = round2(100 * d.NetProfit / totalStaked)
		d.Yield = d.ROI
	}
	d.AverageOdd = round2(totalOdds / float64(d.TotalBets))
	d.NetProfit = round2(d.NetProfit)
	d.GrossProfit = round2(d.GrossProfit)
	d.BiggestWin = round2(d.BiggestWin)
	d.BiggestLoss = round2(d.BiggestLoss)
	return d, nil
}
