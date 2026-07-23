package domain

import "time"

// UpcomingMatch é uma partida AGENDADO (ainda não disputada) de uma liga com dado
// real, usada pelo calendário da página "Visão Geral" — a tela inicial do app, que
// mostra ao usuário os próximos jogos mapeados antes mesmo de ele escolher time ou
// campeonato.
type UpcomingMatch struct {
	MatchID      int64     `json:"match_id"`
	MatchDate    time.Time `json:"match_date"`
	LeagueID     int64     `json:"league_id"`
	LeagueName   string    `json:"league_name"`
	Round        int       `json:"round"`
	HomeTeamID   int64     `json:"home_team_id"`
	HomeTeamName string    `json:"home_team_name"`
	AwayTeamID   int64     `json:"away_team_id"`
	AwayTeamName string    `json:"away_team_name"`
}
