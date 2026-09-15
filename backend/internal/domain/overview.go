package domain

import "time"

// UpcomingMatch é uma partida AGENDADO (ainda não disputada) de uma liga com dado
// real, usada pelo calendário da página "Visão Geral" — a tela inicial do app, que
// mostra ao usuário os próximos jogos mapeados antes mesmo de ele escolher time ou
// campeonato.
type UpcomingMatch struct {
	MatchID   int64     `json:"match_id"`
	MatchDate time.Time `json:"match_date"`
	LeagueID  int64     `json:"league_id"`

	// SeasonID é a temporada À QUAL ESTA PARTIDA PERTENCE.
	//
	// Sem ele, o calendário mandava o usuário para o Dashboard informando apenas
	// liga + equipe, e o Dashboard precisava ADIVINHAR a temporada (escolhia a de
	// maior ano). Quando o palpite não batia com a partida clicada, a equipe não
	// aparecia na lista daquela temporada e a tela trocava silenciosamente para
	// outra equipe — o usuário clicava no Athletic Club e chegava no Alavés.
	//
	// A temporada é um fato da partida, não uma preferência: quem sabe qual é, é
	// quem foi clicado.
	SeasonID int64 `json:"season_id"`

	LeagueName   string `json:"league_name"`
	Round        int    `json:"round"`
	HomeTeamID   int64  `json:"home_team_id"`
	HomeTeamName string `json:"home_team_name"`
	AwayTeamID   int64  `json:"away_team_id"`
	AwayTeamName string `json:"away_team_name"`
}
