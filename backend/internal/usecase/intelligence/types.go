// Package intelligence implementa o "Módulo de Inteligência Estatística": indicadores
// derivados (consistência, tendência, estabilidade, score, compatibilidade, ranking,
// dashboard executivo) e explicações em linguagem analítica — todos calculados
// exclusivamente a partir de dados históricos armazenados. Este pacote NUNCA gera
// recomendação de aposta; qualquer texto produzido aqui descreve o que já aconteceu,
// não o que fazer a respeito.
package intelligence

import "time"

// Meta acompanha toda resposta do módulo para garantir rastreabilidade: período
// analisado, quantidade de jogos, campeonato, temporada e data de atualização.
type Meta struct {
	LeagueID      int64     `json:"league_id"`
	LeagueName    string    `json:"league_name"`
	SeasonIDs     []int64   `json:"season_ids,omitempty"`
	Period        string    `json:"period"`
	GamesAnalyzed int       `json:"games_analyzed"`
	UpdatedAt     time.Time `json:"updated_at"`
	Cached        bool      `json:"cached"`
}
