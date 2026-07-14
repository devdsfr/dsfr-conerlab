// Package sofascore é o esqueleto do provedor Sofascore, pedido como provedor
// inicial no critério de aceite do Módulo de Sincronização.
//
// IMPORTANTE (decisão registrada aqui de propósito, para não se perder): o Sofascore
// não oferece uma API pública/oficial com chave de assinatura como a API-Football ou a
// SportMonks. O único caminho para buscar dados reais deles hoje é consumir endpoints
// internos não documentados do site/app, o que:
//   - pode violar os Termos de Uso do Sofascore;
//   - pode mudar ou ser bloqueado sem aviso (sem contrato de estabilidade);
//   - é exatamente o tipo de acesso que este projeto evita fazer sem confirmação
//     explícita e deliberada.
//
// Por isso este cliente implementa statsprovider.StatisticsProvider por completo (a
// interface fica pronta, e o resto do sistema — workers, config, health check — já
// funciona com "sofascore" como valor de STATISTICS_PROVIDER), mas cada método
// devolve statsprovider.ErrNotImplemented em vez de simular dados falsos. Quando
// houver acesso legítimo (ex: um endpoint oficial for lançado, ou um acordo comercial
// for firmado), a implementação real entra aqui sem exigir nenhuma mudança no
// restante da arquitetura — só troca o "corpo" destes métodos.
//
// Enquanto isso, o provedor configurado de fato em produção é o api_football (ver
// APIFootballProvider em internal/integration/sportsdata/apifootball), que já tem
// chave real e já está em uso.
package sofascore

import (
	"context"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/statsprovider"
)

type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) Name() string { return "sofascore" }

func (c *Client) SyncCompetitions(ctx context.Context, name, country string) ([]statsprovider.Competition, error) {
	return nil, statsprovider.ErrNotImplemented
}

func (c *Client) SyncTeams(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.TeamInfo, error) {
	return nil, statsprovider.ErrNotImplemented
}

func (c *Client) SyncFixtures(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.Fixture, error) {
	return nil, statsprovider.ErrNotImplemented
}

func (c *Client) SyncFixtureStatistics(ctx context.Context, fixtureExternalID string) (*statsprovider.FixtureStatistics, error) {
	return nil, statsprovider.ErrNotImplemented
}

func (c *Client) SyncStandings(ctx context.Context, competitionExternalID string, season int) ([]statsprovider.StandingEntry, error) {
	return nil, statsprovider.ErrNotImplemented
}

// HealthCheck devolve OK=false de propósito: sem uma integração real, não há endpoint
// nenhum pra verificar. Isso garante que, se alguém apontar STATISTICS_PROVIDER=sofascore
// hoje, o Worker de Health Check já registra o incidente e os demais workers já
// suspendem o ciclo — em vez de silenciosamente não fazer nada.
func (c *Client) HealthCheck(ctx context.Context) (statsprovider.HealthResult, error) {
	return statsprovider.HealthResult{
		OK:           false,
		ResponseTime: 0 * time.Second,
		Message:      "SofascoreProvider ainda não tem integração real — ver comentário de pacote em internal/integration/statsprovider/sofascore/client.go",
	}, nil
}
