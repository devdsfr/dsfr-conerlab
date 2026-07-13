package postgres

import (
	"context"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BetRepo struct {
	db *pgxpool.Pool
}

func NewBetRepo(db *pgxpool.Pool) *BetRepo {
	return &BetRepo{db: db}
}

func (r *BetRepo) Create(ctx context.Context, b *domain.Bet) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO bets (user_id, match_label, league_id, market, odd, stake, status, profit_loss, event_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`,
		b.UserID, b.MatchLabel, b.LeagueID, b.Market, b.Odd, b.Stake, b.Status, b.ProfitLoss, b.EventDate).
		Scan(&b.ID, &b.CreatedAt)
}

func (r *BetRepo) List(ctx context.Context, userID int64) ([]domain.Bet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, match_label, league_id, market, odd, stake, status, profit_loss, event_date, created_at
		FROM bets WHERE user_id=$1 ORDER BY event_date DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []domain.Bet
	for rows.Next() {
		var b domain.Bet
		if err := rows.Scan(&b.ID, &b.UserID, &b.MatchLabel, &b.LeagueID, &b.Market, &b.Odd, &b.Stake, &b.Status, &b.ProfitLoss, &b.EventDate, &b.CreatedAt); err != nil {
			return nil, err
		}
		bets = append(bets, b)
	}
	return bets, rows.Err()
}

func (r *BetRepo) Update(ctx context.Context, b *domain.Bet) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bets SET match_label=$1, league_id=$2, market=$3, odd=$4, stake=$5, status=$6, profit_loss=$7, event_date=$8
		WHERE id=$9 AND user_id=$10`,
		b.MatchLabel, b.LeagueID, b.Market, b.Odd, b.Stake, b.Status, b.ProfitLoss, b.EventDate, b.ID, b.UserID)
	return err
}

func (r *BetRepo) Delete(ctx context.Context, id int64, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM bets WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}
