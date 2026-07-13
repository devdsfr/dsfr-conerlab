// Package sportsdata define um contrato normalizado para provedores externos de
// dados esportivos (API-Football, SportMonks) usados para popular o CornerLab com
// jogos e estatísticas de escanteios reais. Cada provedor implementa Provider e
// devolve os mesmos tipos normalizados, para que o restante do sistema (sincronização,
// persistência) nunca precise conhecer o formato específico de cada API externa.
package sportsdata

import (
	"context"
	"errors"
	"time"
)

// Fixture representa uma partida normalizada, com dados suficientes para popular as
// tabelas leagues/seasons/teams/matches do CornerLab.
type Fixture struct {
	ExternalID         string
	LeagueExternalID   string
	LeagueName         string
	LeagueCountry      string
	SeasonYear         int
	Round              int
	MatchDate          time.Time
	HomeTeamExternalID string
	HomeTeamName       string
	AwayTeamExternalID string
	AwayTeamName       string
	HomeGoals          int
	AwayGoals          int
	// HomeCorners/AwayCorners ficam nil quando o provedor não retorna a estatística
	// junto da lista de partidas (ex: API-Football exige uma chamada extra por jogo).
	HomeCorners *int
	AwayCorners *int
}

// Provider é o contrato que cada integração externa (API-Football, SportMonks) deve
// implementar.
type Provider interface {
	Name() string

	// FetchFixtures busca todas as partidas de um campeonato/temporada. Quando o
	// provedor já inclui estatísticas de escanteios na própria listagem, os campos
	// HomeCorners/AwayCorners já vêm preenchidos.
	FetchFixtures(ctx context.Context, leagueName, country string, season int) ([]Fixture, error)

	// FetchCorners busca os escanteios de uma partida específica, para provedores que
	// exigem uma chamada separada por jogo (ex: API-Football). Retorna ok=false se a
	// estatística não estiver disponível para essa partida.
	FetchCorners(ctx context.Context, fixtureExternalID string) (homeCorners, awayCorners int, ok bool, err error)
}

var ErrProviderNotConfigured = errors.New("provedor de dados esportivos não configurado (chave de API ausente)")

// FallbackProvider tenta o provedor primário e, em caso de erro, tenta o secundário.
// Usado quando SPORTS_DATA_PROVIDER=fallback e ambas as chaves (API-Football e
// SportMonks) estão configuradas.
type FallbackProvider struct {
	Primary   Provider
	Secondary Provider
}

func (f *FallbackProvider) Name() string {
	return "fallback(" + safeName(f.Primary) + "->" + safeName(f.Secondary) + ")"
}

func safeName(p Provider) string {
	if p == nil {
		return "none"
	}
	return p.Name()
}

func (f *FallbackProvider) FetchFixtures(ctx context.Context, leagueName, country string, season int) ([]Fixture, error) {
	if f.Primary != nil {
		fixtures, err := f.Primary.FetchFixtures(ctx, leagueName, country, season)
		if err == nil {
			return fixtures, nil
		}
	}
	if f.Secondary != nil {
		return f.Secondary.FetchFixtures(ctx, leagueName, country, season)
	}
	return nil, ErrProviderNotConfigured
}

func (f *FallbackProvider) FetchCorners(ctx context.Context, fixtureExternalID string) (int, int, bool, error) {
	if f.Primary != nil {
		home, away, ok, err := f.Primary.FetchCorners(ctx, fixtureExternalID)
		if err == nil && ok {
			return home, away, ok, nil
		}
	}
	if f.Secondary != nil {
		return f.Secondary.FetchCorners(ctx, fixtureExternalID)
	}
	return 0, 0, false, ErrProviderNotConfigured
}
