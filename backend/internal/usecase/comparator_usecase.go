package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

type ComparatorUsecase struct {
	matches repository.MatchRepository
	teams   repository.TeamRepository
}

func NewComparatorUsecase(matches repository.MatchRepository, teams repository.TeamRepository) *ComparatorUsecase {
	return &ComparatorUsecase{matches: matches, teams: teams}
}

// MatchPoint é um ponto da evolução COM IDENTIDADE.
//
// Antes a evolução era `Trend []int` e o gráfico rotulava o eixo X como 1..N —
// não havia como dizer contra quem foi, quando, nem se a equipe jogou em casa.
// Um ponto de gráfico que não identifica a observação não é auditável.
type MatchPoint struct {
	MatchID      int64     `json:"match_id"`
	Date         time.Time `json:"date"`
	OpponentID   int64     `json:"opponent_id"`
	OpponentName string    `json:"opponent_name"`
	IsHome       bool      `json:"is_home"`

	// Value é a métrica+perspectiva daquela partida. nil = o provedor não
	// publicou o dado para ESTA partida (não é zero).
	Value *int `json:"value"`

	GoalsFor     int `json:"goals_for"`
	GoalsAgainst int `json:"goals_against"`
}

type TeamComparisonSide struct {
	Team domain.Team `json:"team"`

	// SampleSize = partidas do recorte (liga+temporada+local).
	// MetricSampleSize = quantas delas têm a métrica escolhida.
	// Os dois divergem quando o provedor não publica a métrica em todo jogo —
	// impedimentos, por exemplo, existem em 9 de 28 jogos do Flamengo em 2026.
	SampleSize       int  `json:"sample_size"`
	MetricSampleSize int  `json:"metric_sample_size"`
	MetricAvailable  bool `json:"metric_available"`

	// Period descreve a amostra REAL desta equipe. Fica dentro do lado porque as
	// duas equipes podem ter amostras diferentes, e apresentá-las sob a mesma
	// frase esconde justamente o que torna a comparação delicada.
	Period string `json:"period"`

	Summary     StatSummary     `json:"summary"`
	Frequencies []FrequencyBand `json:"frequencies"`

	// Distribution é a distribuição OBSERVADA (valor -> nº de partidas), montada
	// sobre as mesmas observações de Summary. Denominador = MetricSampleSize.
	Distribution []DistributionBucket `json:"distribution"`

	Evolution []MatchPoint `json:"evolution"`
}

// H2HMatch é um confronto direto, com tudo que permite auditá-lo.
type H2HMatch struct {
	MatchID      int64     `json:"match_id"`
	Date         time.Time `json:"date"`
	LeagueID     int64     `json:"league_id"`
	SeasonID     int64     `json:"season_id"`
	HomeTeamID   int64     `json:"home_team_id"`
	HomeTeamName string    `json:"home_team_name"`
	AwayTeamID   int64     `json:"away_team_id"`
	AwayTeamName string    `json:"away_team_name"`
	HomeGoals    int       `json:"home_goals"`
	AwayGoals    int       `json:"away_goals"`

	// Métrica selecionada, orientada pelo mando. nil quando ausente na partida.
	HomeValue *int `json:"home_value"`
	AwayValue *int `json:"away_value"`
}

type HeadToHead struct {
	MatchCount int        `json:"match_count"`
	Matches    []H2HMatch `json:"matches"`
}

type ComparisonResult struct {
	// Period do conjunto é a janela PEDIDA, rotulada como pedido. A amostra real
	// de cada equipe está em TeamA.Period / TeamB.Period.
	Period         string `json:"period"`
	RequestedLimit int    `json:"requested_limit"`

	Venue       Venue       `json:"venue"`
	Metric      Metric      `json:"metric"`
	Perspective Perspective `json:"perspective"`

	// FrequenciesAvailable = false quando não existe definição de produto de
	// faixas para o par métrica+perspectiva. Ver frequencyThresholds: os limites
	// documentados valem para o TOTAL da partida; reaproveitá-los em produzido
	// ou concedido seria errado por ordem de grandeza.
	FrequenciesAvailable bool   `json:"frequencies_available"`
	FrequenciesNote      string `json:"frequencies_note,omitempty"`

	TeamA TeamComparisonSide `json:"team_a"`
	TeamB TeamComparisonSide `json:"team_b"`
	H2H   HeadToHead         `json:"h2h"`
}

// ComparatorQuery agrupa os parâmetros para a assinatura não virar uma fila de
// sete argumentos posicionais.
type ComparatorQuery struct {
	TeamAID, TeamBID int64
	LeagueID         *int64
	SeasonID         *int64
	Limit            int
	Venue            Venue
	Metric           Metric
	Perspective      Perspective
}

func (u *ComparatorUsecase) Compare(ctx context.Context, q ComparatorQuery) (*ComparisonResult, error) {
	if q.Limit <= 0 {
		q.Limit = 10
	}

	sideA, err := u.buildSide(ctx, q.TeamAID, q)
	if err != nil {
		return nil, fmt.Errorf("equipe A: %w", err)
	}
	sideB, err := u.buildSide(ctx, q.TeamBID, q)
	if err != nil {
		return nil, fmt.Errorf("equipe B: %w", err)
	}

	h2h, err := u.buildH2H(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("confronto direto: %w", err)
	}

	_, freqOK := frequencyThresholds(q.Metric, q.Perspective)
	nota := ""
	if !freqOK {
		nota = "As faixas de frequência estão definidas para o total da partida. " +
			"Não há definição para produzido/concedido isoladamente, e aplicar os " +
			"mesmos limites daria percentuais enganosos."
	}

	return &ComparisonResult{
		Period:               fmt.Sprintf("Janela pedida: últimos %d jogos", q.Limit),
		RequestedLimit:       q.Limit,
		Venue:                q.Venue,
		Metric:               q.Metric,
		Perspective:          q.Perspective,
		FrequenciesAvailable: freqOK,
		FrequenciesNote:      nota,
		TeamA:                *sideA,
		TeamB:                *sideB,
		H2H:                  *h2h,
	}, nil
}

// O LOCAL é aplicado por equipe, cada uma sob a própria perspectiva: "Casa"
// significa A mandante no lado A e B mandante no lado B. Cruzar "A em casa"
// contra "B fora" seria outra pergunta, e o enunciado do REV-P2 a trata
// explicitamente como decisão de produto separada.
func (u *ComparatorUsecase) buildSide(ctx context.Context, teamID int64, q ComparatorQuery) (*TeamComparisonSide, error) {
	team, err := u.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	views, err := u.matches.TeamMatches(ctx, repository.MatchFilter{
		TeamID:   teamID,
		LeagueID: q.LeagueID,
		SeasonID: q.SeasonID,
		Limit:    q.Limit,
		HomeOnly: q.Venue == VenueCasa,
		AwayOnly: q.Venue == VenueFora,
	})
	if err != nil {
		return nil, err
	}

	// Duas coleções distintas de propósito: `evolution` mantém TODAS as partidas
	// do recorte (inclusive as sem a métrica, com value nil, para o gráfico não
	// mentir sobre a sequência), enquanto `valores` só recebe o que existe — é
	// ele que alimenta média e frequências.
	evolution := make([]MatchPoint, 0, len(views))
	valores := make([]int, 0, len(views))

	for i := len(views) - 1; i >= 0; i-- { // cronológico: mais antigo -> mais recente
		v := views[i]
		val := metricValue(v, q.Metric, q.Perspective)
		evolution = append(evolution, MatchPoint{
			MatchID:      v.MatchID,
			Date:         v.MatchDate,
			OpponentID:   v.Opponent.ID,
			OpponentName: v.Opponent.Name,
			IsHome:       v.IsHome,
			Value:        val,
			GoalsFor:     v.GoalsFor,
			GoalsAgainst: v.GoalsAgainst,
		})
		if val != nil {
			valores = append(valores, *val)
		}
	}

	side := &TeamComparisonSide{
		Team:             *team,
		SampleSize:       len(views),
		MetricSampleSize: len(valores),
		MetricAvailable:  len(valores) > 0,
		Period:           describePeriod(len(views), q.Limit),
		Summary:          Summarize(valores),
		// `valores` já exclui as partidas sem a métrica, então ausência nunca
		// entra como zero e zero observado entra normalmente.
		Distribution: buildDistribution(valores),
		Evolution:    evolution,
	}

	if thresholds, ok := frequencyThresholds(q.Metric, q.Perspective); ok {
		side.Frequencies = buildFrequencies(valores, thresholds)
	}
	return side, nil
}

func (u *ComparatorUsecase) buildH2H(ctx context.Context, q ComparatorQuery) (*HeadToHead, error) {
	raw, err := u.matches.HeadToHead(ctx, q.TeamAID, q.TeamBID, q.LeagueID, q.SeasonID)
	if err != nil {
		return nil, err
	}

	// Nomes resolvidos uma vez, fora do laço — são só duas equipes, e buscar por
	// partida seria N+1 sem ganho nenhum.
	nomes := map[int64]string{}
	for _, id := range []int64{q.TeamAID, q.TeamBID} {
		if t, err := u.teams.GetByID(ctx, id); err == nil {
			nomes[id] = t.Name
		}
	}

	out := make([]H2HMatch, 0, len(raw))
	for _, m := range raw {
		home, away := matchMetricValues(m, q.Metric)
		out = append(out, H2HMatch{
			MatchID:      m.ID,
			Date:         m.MatchDate,
			LeagueID:     m.LeagueID,
			SeasonID:     m.SeasonID,
			HomeTeamID:   m.HomeTeamID,
			HomeTeamName: nomes[m.HomeTeamID],
			AwayTeamID:   m.AwayTeamID,
			AwayTeamName: nomes[m.AwayTeamID],
			HomeGoals:    m.HomeGoals,
			AwayGoals:    m.AwayGoals,
			HomeValue:    home,
			AwayValue:    away,
		})
	}

	// MatchCount é a contagem REAL. Zero confrontos devolve lista vazia e a
	// interface diz isso — nunca zeros simulando um confronto que não houve.
	return &HeadToHead{MatchCount: len(out), Matches: out}, nil
}
