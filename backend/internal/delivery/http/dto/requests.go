package dto

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type FilterRunRequest struct {
	LeagueID  int64   `json:"league_id" binding:"required"`
	SeasonIDs []int64 `json:"season_ids"`

	TeamID           *int64  `json:"team_id"`
	LastNGames       int     `json:"last_n_games"`
	HomeAway         string  `json:"home_away"`
	CornersThreshold int     `json:"corners_threshold" binding:"required"`
	OpponentTier     string  `json:"opponent_tier"`
	MaxOdds          float64 `json:"max_odds"`
	Stake            float64 `json:"stake"`
}

type SaveFilterRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Definition  string `json:"definition" binding:"required"` // JSON serializado do FilterCriteria
}

type BetRequest struct {
	MatchLabel string  `json:"match_label" binding:"required"`
	LeagueID   *int64  `json:"league_id"`
	Market     string  `json:"market" binding:"required"`
	Odd        float64 `json:"odd" binding:"required"`
	Stake      float64 `json:"stake" binding:"required"`
	Status     string  `json:"status"`
	EventDate  string  `json:"event_date" binding:"required"` // formato 2006-01-02
}
