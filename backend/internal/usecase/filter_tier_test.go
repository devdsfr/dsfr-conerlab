package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
)

// Testes do AUD-004 — filtro por força do adversário.
//
// O teste obrigatório da auditoria é: "filtro por tier deve retornar apenas
// partidas cuja classificação vigente na data era a informada".
//
// Esse teste NÃO É EXECUTÁVEL hoje, e é importante dizer por quê em vez de
// escrever algo que passe. Ele pressupõe que exista uma classificação com
// dimensão temporal — "vigente na data" —, e o CornerLab não tem isso: teams.tier
// é um valor único por equipe, sem temporada e sem data, e o que estava lá dentro
// nem classificação era (constante 'G12' em 228 equipes, número da divisão em
// 110, zero em G6 e Z4).
//
// Enquanto a classificação por temporada não existir, o que se pode e se deve
// garantir é o comportamento seguro: o sistema RECUSA o filtro em vez de aplicar
// um recorte sem significado. É isso que está testado aqui.
//
// Quando a classificação point-in-time for implementada, o teste obrigatório
// original passa a fazer sentido e deve substituir estes.

func TestTierRecusadoNoBacktest(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceReal),
	})

	c := cornersCriteria()
	c.OpponentTier = "G6"

	_, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err == nil {
		t.Fatal("backtest com opponent_tier deveria falhar: a classificação não existe")
	}
	if !strings.Contains(err.Error(), "força do adversário") {
		t.Errorf("mensagem de erro não explica o motivo ao usuário: %q", err)
	}
}

// Todos os valores que a tela oferecia precisam ser recusados — inclusive G12,
// que era o único com equipes atribuídas e por isso o mais perigoso: devolvia
// resultado com cara de válido.
func TestTodosOsTiersSaoRecusados(t *testing.T) {
	for _, tier := range []string{"G6", "G12", "Z4", "1", "qualquer-coisa"} {
		c := cornersCriteria()
		c.OpponentTier = tier
		if err := c.Validate(); err == nil {
			t.Errorf("tier %q foi aceito", tier)
		}
	}
}

// A recusa não pode virar um bloqueio geral: sem tier, tudo continua funcionando.
func TestSemTierOBacktestRodaNormalmente(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceReal),
	})

	res, err := u.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatalf("backtest sem tier deveria funcionar: %v", err)
	}
	// REV-P3 (AUD-021): esperado passou de 4 para 2. São 2 PARTIDAS e a regra é
	// match-level; o engine contava cada uma duas vezes. A expectativa mudou
	// porque a regra mudou, não para o teste passar.
	if res.MatchCount != 2 {
		t.Errorf("MatchCount = %d, esperado 2", res.MatchCount)
	}
}

// Guarda contra a volta silenciosa do defeito: mesmo que alguém repovoe
// teams.tier, o motor não pode voltar a filtrar por ele sem que a classificação
// por temporada exista. A recusa está em Validate(), antes de qualquer leitura
// de equipe — este teste falha se o filtro for reintroduzido no laço.
func TestTierDeEquipeNaoInfluenciaResultado(t *testing.T) {
	matches := []domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceReal),
	}

	semClassificacao := NewFilterUsecase(
		&stubMatchRepo{matches: matches},
		&stubTeamRepo{teams: []domain.Team{
			{ID: 10, Name: "Mandante", Tier: ""},
			{ID: 20, Name: "Visitante", Tier: ""},
		}},
		stubLeagueRepo{},
	)
	comClassificacao := NewFilterUsecase(
		&stubMatchRepo{matches: matches},
		&stubTeamRepo{teams: []domain.Team{
			{ID: 10, Name: "Mandante", Tier: "G6"},
			{ID: 20, Name: "Visitante", Tier: "Z4"},
		}},
		stubLeagueRepo{},
	)

	a, err := semClassificacao.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := comClassificacao.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatal(err)
	}

	roiA, roiB := "n/a", "n/a"
	if a.ROI != nil {
		roiA = fmt.Sprintf("%.2f", *a.ROI)
	}
	if b.ROI != nil {
		roiB = fmt.Sprintf("%.2f", *b.ROI)
	}
	if a.MatchCount != b.MatchCount || a.HitRate != b.HitRate || roiA != roiB {
		t.Errorf("o valor de teams.tier alterou o resultado do backtest:\n  sem: %d jogos, %.2f%% acerto, ROI %s\n  com: %d jogos, %.2f%% acerto, ROI %s",
			a.MatchCount, a.HitRate, roiA, b.MatchCount, b.HitRate, roiB)
	}
}
