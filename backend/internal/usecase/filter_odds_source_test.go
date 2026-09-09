package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
)

// Testes de AUD-001 — vazamento de odd sintética para dentro da validação
// quantitativa.
//
// O que está sendo protegido: a odd gravada em matches.corner_odds pelo
// pipeline atual não vem de mercado; é gerada por SyntheticCornerOdds a partir
// da média de escanteios do próprio lote sincronizado. Backtestar sobre ela e
// filtrar por "odd <= X" equivale a selecionar lotes de média alta — o acerto
// alto sai por construção, não por vantagem.
//
// A regra que estes testes fixam:
//   - RequireRealOdds = true (Discovery e reavaliação de estratégia) rejeita
//     synthetic e unknown, aceita real.
//   - RequireRealOdds = false (Simulador) aceita tudo, mas o resultado sai
//     marcado com a procedência e com FinancialsReliable = false.
//
// Um teste que passasse ao permitir odd sintética no Discovery estaria
// validando exatamente o defeito. Se o pipeline um dia passar a gravar odds
// reais, estes testes continuam válidos sem alteração.

// --- dublês de repositório -------------------------------------------------

type stubMatchRepo struct{ matches []domain.Match }

func (s *stubMatchRepo) TeamMatches(context.Context, repository.MatchFilter) ([]domain.TeamMatchView, error) {
	return nil, nil
}

func (s *stubMatchRepo) AllMatches(context.Context, int64, []int64) ([]domain.Match, error) {
	return s.matches, nil
}

func (s *stubMatchRepo) GetMatchTeams(context.Context, []int64) (map[int64]domain.Match, error) {
	return map[int64]domain.Match{}, nil
}

func (s *stubMatchRepo) ListUpcoming(context.Context) ([]domain.UpcomingMatch, error) {
	return nil, nil
}

type stubTeamRepo struct{ teams []domain.Team }

func (s *stubTeamRepo) List(context.Context, *int64, ...int64) ([]domain.Team, error) {
	return s.teams, nil
}

func (s *stubTeamRepo) GetByID(_ context.Context, id int64) (*domain.Team, error) {
	for i := range s.teams {
		if s.teams[i].ID == id {
			return &s.teams[i], nil
		}
	}
	return nil, nil
}

func (s *stubTeamRepo) Search(context.Context, string) ([]domain.Team, error) { return nil, nil }

type stubLeagueRepo struct{}

func (stubLeagueRepo) List(context.Context) ([]domain.League, error) { return nil, nil }
func (stubLeagueRepo) GetByID(context.Context, int64) (*domain.League, error) {
	return &domain.League{ID: 1, Name: "Liga Teste"}, nil
}
func (stubLeagueRepo) ListSeasons(context.Context, int64) ([]domain.Season, error) {
	return nil, nil
}

// --- fixtures --------------------------------------------------------------

// matchWithOdds monta uma partida que ACERTA a linha 8.5 (11 escanteios) e
// carrega uma odd nessa linha, com a procedência informada.
func matchWithOdds(id int64, source string) domain.Match {
	return domain.Match{
		ID:          id,
		LeagueID:    1,
		SeasonID:    1,
		MatchDate:   time.Date(2025, 3, int(id), 0, 0, 0, 0, time.UTC),
		HomeTeamID:  10,
		AwayTeamID:  20,
		HomeCorners: 6,
		AwayCorners: 5,
		CornerOdds:  map[string]float64{"8.5": 1.90},
		OddsSource:  source,
	}
}

func newFilterUsecaseWith(matches []domain.Match) *FilterUsecase {
	return NewFilterUsecase(
		&stubMatchRepo{matches: matches},
		&stubTeamRepo{teams: []domain.Team{
			{ID: 10, Name: "Mandante", Tier: "medio"},
			{ID: 20, Name: "Visitante", Tier: "medio"},
		}},
		stubLeagueRepo{},
	)
}

func cornersCriteria() FilterCriteria {
	return FilterCriteria{CornersThreshold: 8, Metric: "corners", Stake: 10}
}

// --- Discovery: só odd real vale para validar estratégia -------------------

func TestRequireRealOdds_RejeitaSintetica(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceSynthetic),
		matchWithOdds(2, domain.OddsSourceSynthetic),
	})

	c := cornersCriteria()
	c.RequireRealOdds = true

	res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.MatchCount != 0 {
		t.Fatalf("odd sintética entrou na validação do Discovery: MatchCount = %d, esperado 0", res.MatchCount)
	}
	if res.FinancialsReliable {
		t.Fatal("FinancialsReliable = true sem nenhuma odd real")
	}
}

func TestRequireRealOdds_RejeitaUnknown(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceUnknown),
		matchWithOdds(2, ""), // linha antiga, anterior à migration 013
	})

	c := cornersCriteria()
	c.RequireRealOdds = true

	res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.MatchCount != 0 {
		t.Fatalf("odd de procedência desconhecida entrou na validação: MatchCount = %d, esperado 0", res.MatchCount)
	}
}

func TestRequireRealOdds_PermiteReal(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceReal),
	})

	c := cornersCriteria()
	c.RequireRealOdds = true

	res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	// Cada partida vira 2 candidatos (mandante e visitante) quando não há TeamID.
	if res.MatchCount != 4 {
		t.Fatalf("odd real deveria ser aceita: MatchCount = %d, esperado 4", res.MatchCount)
	}
	if res.OddsSource != domain.OddsSourceReal {
		t.Fatalf("OddsSource = %q, esperado %q", res.OddsSource, domain.OddsSourceReal)
	}
	if !res.FinancialsReliable {
		t.Fatal("FinancialsReliable = false com 100% de odds reais")
	}
}

// Mistura: basta uma sintética para o conjunto deixar de ser medida de mercado.
func TestOddsSourceSummary_MisturaNaoEConfiavel(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceReal),
		matchWithOdds(2, domain.OddsSourceSynthetic),
	})

	res, err := u.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.OddsSource != domain.OddsSourceSynthetic {
		t.Fatalf("OddsSource = %q, esperado %q em lote misto", res.OddsSource, domain.OddsSourceSynthetic)
	}
	if res.FinancialsReliable {
		t.Fatal("FinancialsReliable = true em lote que contém odd sintética")
	}
}

// --- Simulador: aceita sintética, mas devolve o resultado marcado ---------

func TestSimulador_AceitaSinteticaMasMarcaResultado(t *testing.T) {
	u := newFilterUsecaseWith([]domain.Match{
		matchWithOdds(1, domain.OddsSourceSynthetic),
		matchWithOdds(2, domain.OddsSourceSynthetic),
	})

	// RequireRealOdds fica false — é o caminho do Simulador.
	res, err := u.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.MatchCount != 4 {
		t.Fatalf("Simulador deveria aceitar odd sintética: MatchCount = %d, esperado 4", res.MatchCount)
	}
	if res.OddsSource != domain.OddsSourceSynthetic {
		t.Fatalf("OddsSource = %q, esperado %q", res.OddsSource, domain.OddsSourceSynthetic)
	}
	if res.FinancialsReliable {
		t.Fatal("resultado sobre odd sintética não pode ser marcado como confiável")
	}
}

// --- AUD-012: sem odd, a partida não entra no backtest --------------------

func TestSemOdd_PartidaForaDoBacktest(t *testing.T) {
	m := matchWithOdds(1, domain.OddsSourceReal)
	m.CornerOdds = nil // sem odd registrada para nenhuma linha

	u := newFilterUsecaseWith([]domain.Match{m})

	res, err := u.RunBacktest(context.Background(), 1, nil, cornersCriteria(), 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.MatchCount != 0 {
		t.Fatalf("partida sem odd entrou no backtest: MatchCount = %d, esperado 0", res.MatchCount)
	}
	// Antes da correção, essas partidas entravam com odd = 1.0, produzindo
	// P/L = 0 no acerto e -stake no erro: prejuízo garantido e sem sentido.
	if res.Profit != 0 || res.TotalStaked != 0 {
		t.Fatalf("backtest vazio deveria ter financeiro zerado: Profit=%v Staked=%v", res.Profit, res.TotalStaked)
	}
}

// --- Métrica sem odd por partida: nunca é medida de mercado ---------------

func TestMetricaComOddFixa_NaoEConfiavel(t *testing.T) {
	m := matchWithOdds(1, domain.OddsSourceReal)
	m.HomeGoals, m.AwayGoals = 2, 1

	u := newFilterUsecaseWith([]domain.Match{m})

	c := FilterCriteria{CornersThreshold: 8, Metric: "goals", GoalsThreshold: 2, FixedOdd: 1.85, Stake: 10}
	res, err := u.RunBacktest(context.Background(), 1, nil, c, 0)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if res.OddsSource != "fixed" {
		t.Fatalf("OddsSource = %q, esperado \"fixed\"", res.OddsSource)
	}
	if res.FinancialsReliable {
		t.Fatal("odd fixa informada pelo usuário não é medida de mercado")
	}
}
