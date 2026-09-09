package discovery

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
)

// Testes do AUD-003 — múltiplas comparações e validação fora da amostra.
//
// O teste obrigatório da auditoria é o `TestCicloSobreRuidoPuroNaoPublicaNada`
// mais abaixo: dado ruído aleatório sem vantagem nenhuma, o ciclo completo tem
// que publicar zero estratégias. É a única forma de demonstrar que o motor
// deixou de premiar sorte — antes da correção ele testava 5.940 combinações
// contra o mesmo histórico e publicava as melhores, o que fabrica aprovações
// mesmo em dados puramente aleatórios.

// --- dublês -----------------------------------------------------------------

type fakeMatchRepo struct{ matches []domain.Match }

func (f *fakeMatchRepo) TeamMatches(context.Context, repository.MatchFilter) ([]domain.TeamMatchView, error) {
	return nil, nil
}
func (f *fakeMatchRepo) AllMatches(context.Context, int64, []int64) ([]domain.Match, error) {
	return f.matches, nil
}
func (f *fakeMatchRepo) GetMatchTeams(context.Context, []int64) (map[int64]domain.Match, error) {
	return map[int64]domain.Match{}, nil
}
func (f *fakeMatchRepo) ListUpcoming(context.Context) ([]domain.UpcomingMatch, error) {
	return nil, nil
}

type fakeTeamRepo struct{ teams []domain.Team }

func (f *fakeTeamRepo) List(context.Context, *int64, ...int64) ([]domain.Team, error) {
	return f.teams, nil
}
func (f *fakeTeamRepo) GetByID(_ context.Context, id int64) (*domain.Team, error) {
	for i := range f.teams {
		if f.teams[i].ID == id {
			return &f.teams[i], nil
		}
	}
	return nil, nil
}
func (f *fakeTeamRepo) Search(context.Context, string) ([]domain.Team, error) { return nil, nil }

type fakeLeagueRepo struct{}

func (fakeLeagueRepo) List(context.Context) ([]domain.League, error) {
	return []domain.League{{ID: 1, Name: "Liga Teste"}}, nil
}
func (fakeLeagueRepo) GetByID(context.Context, int64) (*domain.League, error) {
	return &domain.League{ID: 1, Name: "Liga Teste"}, nil
}
func (fakeLeagueRepo) ListSeasons(context.Context, int64) ([]domain.Season, error) {
	return []domain.Season{{ID: 1, LeagueID: 1, Year: 2025}}, nil
}

// fakeStrategyRepo registra o que o ciclo tentou publicar.
type fakeStrategyRepo struct {
	published []domain.Strategy
	nextID    int64
}

func (f *fakeStrategyRepo) UpsertDiscovered(_ context.Context, s *domain.Strategy) error {
	f.nextID++
	s.ID = f.nextID
	f.published = append(f.published, *s)
	return nil
}
func (f *fakeStrategyRepo) DeactivateDiscoveredExcept(context.Context, int64, []int64) (int, error) {
	return 0, nil
}
func (f *fakeStrategyRepo) Create(context.Context, *domain.Strategy) error { return nil }
func (f *fakeStrategyRepo) ListForUser(context.Context, int64) ([]domain.Strategy, error) {
	return nil, nil
}
func (f *fakeStrategyRepo) ListActive(context.Context) ([]domain.Strategy, error) { return nil, nil }
func (f *fakeStrategyRepo) GetByID(context.Context, int64) (*domain.Strategy, error) {
	return nil, nil
}
func (f *fakeStrategyRepo) SetFlags(context.Context, int64, bool, bool) error { return nil }
func (f *fakeStrategyRepo) Delete(context.Context, int64, int64) error        { return nil }
func (f *fakeStrategyRepo) InsertBacktest(context.Context, *domain.Backtest) error {
	return nil
}
func (f *fakeStrategyRepo) LastBacktests(context.Context, int64, int) ([]domain.Backtest, error) {
	return nil, nil
}
func (f *fakeStrategyRepo) UpsertHealth(context.Context, *domain.StrategyHealth) error { return nil }
func (f *fakeStrategyRepo) UpsertScores(context.Context, *domain.StrategyScores) error { return nil }
func (f *fakeStrategyRepo) GetHealth(context.Context, int64) (*domain.StrategyHealth, error) {
	return nil, nil
}
func (f *fakeStrategyRepo) GetScores(context.Context, int64) (*domain.StrategyScores, error) {
	return nil, nil
}
func (f *fakeStrategyRepo) ListDiscovered(context.Context, *int64, int) ([]repository.DiscoveredStrategy, error) {
	return nil, nil
}

type fakePersister struct{}

func (fakePersister) PersistResult(context.Context, int64, *usecase.BacktestResult) (*strategyengine.Evaluation, error) {
	return &strategyengine.Evaluation{}, nil
}

// --- geradores de histórico -------------------------------------------------

const (
	testTeams = 12

	// Escanteios por equipe sorteados em Uniform{minCorners..maxCorners}.
	// Independentes entre si e de tudo mais — time, mando, data, adversário.
	minCorners = 1
	maxCorners = 8
)

// fairOdds devolve, para cada linha do espaço de busca, a odd EXATAMENTE justa
// da distribuição geradora: 1 / P(total > linha).
//
// É o detalhe que faz o histórico ser ruído de verdade. Uma odd qualquer (2.00
// em todas as linhas, por exemplo) criaria vantagem ou desvantagem real, e o
// motor estaria certo em achar padrão. Com a odd justa, TODA combinação tem
// valor esperado exatamente zero — o que sobra é só variação amostral. Se
// alguma coisa for publicada, foi sorte.
//
// A probabilidade vem da distribuição que gera os dados, calculada
// analiticamente. Não é estimada da amostra: derivar a odd do próprio lote é
// justamente o defeito do AUD-001, e usá-lo aqui contaminaria o teste.
func fairOdds() map[string]float64 {
	const side = maxCorners - minCorners + 1
	counts := make(map[int]int) // total -> número de pares (a,b) que somam isso
	for a := minCorners; a <= maxCorners; a++ {
		for b := minCorners; b <= maxCorners; b++ {
			counts[a+b]++
		}
	}
	total := side * side

	odds := map[string]float64{}
	for _, line := range cornerLines {
		favoraveis := 0
		for sum, n := range counts {
			if sum > line { // o motor trata threshold como "total > N"
				favoraveis += n
			}
		}
		p := float64(favoraveis) / float64(total)
		odds[lineKey(line)] = 1 / p
	}
	return odds
}

func teamList() []domain.Team {
	out := make([]domain.Team, 0, testTeams)
	for i := 1; i <= testTeams; i++ {
		out = append(out, domain.Team{ID: int64(i), Name: string(rune('A'+i-1)) + " FC", Tier: "G12"})
	}
	return out
}

// noiseHistory gera um histórico onde o total de escanteios não tem nenhuma
// relação com nada: cada partida sorteia escanteios de uma distribuição fixa,
// independente de time, mando, data ou adversário. As odds são as justas dessa
// distribuição (ver fairOdds) e ficam marcadas como REAL, para que o pipeline
// do AUD-001 aceite processá-las.
//
// Nenhuma combinação do espaço de busca tem vantagem aqui, por construção: o
// valor esperado de toda aposta é exatamente zero.
func noiseHistory(n int, seed int64) []domain.Match {
	rng := rand.New(rand.NewSource(seed))
	base := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	odds := fairOdds()

	corners := func() int { return minCorners + rng.Intn(maxCorners-minCorners+1) }

	out := make([]domain.Match, 0, n)
	for i := 0; i < n; i++ {
		home := int64(rng.Intn(testTeams) + 1)
		away := int64(rng.Intn(testTeams) + 1)
		for away == home {
			away = int64(rng.Intn(testTeams) + 1)
		}
		out = append(out, domain.Match{
			ID:          int64(i + 1),
			LeagueID:    1,
			SeasonID:    1,
			MatchDate:   base.AddDate(0, 0, i), // uma partida por dia: datas distintas
			HomeTeamID:  home,
			AwayTeamID:  away,
			HomeCorners: corners(),
			AwayCorners: corners(),
			CornerOdds:  odds,
			OddsSource:  domain.OddsSourceReal,
		})
	}
	return out
}

func lineKey(line int) string {
	return map[int]string{6: "6.5", 7: "7.5", 8: "8.5", 9: "9.5", 10: "10.5"}[line]
}

func newTestEngine(matches []domain.Match, repo *fakeStrategyRepo, crit Criteria) *Engine {
	return NewEngine(
		&fakeMatchRepo{matches: matches},
		&fakeTeamRepo{teams: teamList()},
		fakeLeagueRepo{},
		repo,
		fakePersister{},
		Options{Criteria: crit},
	)
}

// permissiveDocCriteria afrouxa APENAS os limiares do doc 08 (win rate, ROI,
// yield, DSFR) para que o ruído consiga chegar até o mecanismo do AUD-003.
//
// Por que isso é necessário e por que NÃO é "relaxar filtro para gerar
// oportunidade": com os limiares de produção, o ruído é barrado lá atrás por
// "win_rate_baixo" e nem chega à correção de múltiplos testes. Um teste nessas
// condições passaria mesmo com o AUD-003 totalmente por corrigir — provaria
// que o doc 08 funciona, não que a correção funciona. Foi exatamente o que
// aconteceu na primeira versão deste arquivo, e o `Tested == 0` abaixo existe
// para que isso nunca volte a passar despercebido.
//
// As travas do AUD-003 (holdout, FDR, amostra mínima) NÃO são tocadas aqui —
// são justamente o que está sob teste. E nada disto altera produção:
// DefaultCriteria() continua com os valores do doc 08.
func permissiveDocCriteria() Criteria {
	return Criteria{
		MinGames:    AbsoluteMinimumGames,
		MinWinRate:  0.001,
		MinROI:      -1000,
		MinYield:    -1000,
		MaxDrawdown: 100,
		MinDSFR:     0.001,
	}
}

// --- TESTE OBRIGATÓRIO DA AUDITORIA ----------------------------------------

// TESTE OBRIGATÓRIO — ruído puro tem que publicar zero, com o mecanismo do
// AUD-003 efetivamente exercitado.
//
// Os limiares do doc 08 ficam permissivos de propósito (ver
// permissiveDocCriteria) para que as combinações cheguem até a correção de
// múltiplos testes e a validação fora da amostra. A asserção `Tested > 0`
// garante que o teste não vire vácuo: se um dia o ruído voltar a ser barrado
// antes de chegar lá, este teste falha em vez de passar sem provar nada.
func TestCicloSobreRuidoPuroNaoPublicaNada(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 2024, 99991} {
		repo := &fakeStrategyRepo{}
		eng := newTestEngine(noiseHistory(700, seed), repo, permissiveDocCriteria())

		res, err := eng.RunLeague(context.Background(), 1, nil)
		if err != nil {
			t.Fatalf("seed %d: RunLeague: %v", seed, err)
		}

		if res.Tested == 0 {
			t.Fatalf("seed %d: nenhuma combinação chegou ao teste de significância "+
				"(combinações=%d, rejeições=%v). O teste não exercitou o AUD-003 — "+
				"corrija o cenário antes de confiar neste resultado.",
				seed, res.Combinations, res.Rejections)
		}
		if res.Published != 0 || len(repo.published) != 0 {
			t.Errorf("seed %d: ciclo publicou %d estratégias sobre ruído puro (esperado 0).\n"+
				"combinações=%d testadas=%d limiar_fdr=%.6g significativas=%d validadas=%d\n"+
				"rejeições=%v",
				seed, res.Published, res.Combinations, res.Tested,
				res.FDRThreshold, res.Significant, res.Validated, res.Rejections)
		}
	}
}

// Contraprova: sem a correção do AUD-003, o MESMO ruído produziria aprovações.
//
// Sem este teste, o anterior seria compatível com "os critérios do doc 08 já
// bastavam" — e a correção inteira poderia ser inútil sem ninguém notar. Aqui
// se mede quantas combinações passariam por todos os filtros ANTERIORES ao
// AUD-003, isto é, o que o motor teria publicado antes desta correção.
func TestSemCorrecaoORuidoProduziriaAprovacoes(t *testing.T) {
	totalAprovadasAntes := 0

	for _, seed := range []int64{1, 7, 42, 2024, 99991} {
		repo := &fakeStrategyRepo{}
		eng := newTestEngine(noiseHistory(700, seed), repo, permissiveDocCriteria())
		res, err := eng.RunLeague(context.Background(), 1, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		// res.Tested é o número de combinações que passaram por amostra, win
		// rate, ROI, yield, drawdown e DSFR — tudo que existia antes do AUD-003.
		totalAprovadasAntes += res.Tested
	}

	if totalAprovadasAntes == 0 {
		t.Fatal("nenhuma combinação passou pelos filtros antigos: o cenário de ruído não " +
			"reproduz o problema que o AUD-003 corrige, e o teste de ruído acima não vale nada")
	}
	t.Logf("sem o AUD-003, este ruído teria aprovado %d combinações no total "+
		"(todas falsas por construção)", totalAprovadasAntes)
}

// --- Não vazamento ----------------------------------------------------------

// A janela de descoberta e a de validação não podem compartilhar nenhuma
// partida, nem sequer uma data. Se compartilhassem, a "validação" estaria
// olhando dado que a busca já usou.
func TestJanelasNaoSeSobrepoem(t *testing.T) {
	matches := noiseHistory(500, 3)
	train, holdout, ok := splitHistory(matches, DefaultTrainFraction)
	if !ok {
		t.Fatal("split deveria ser possível com 500 partidas em datas distintas")
	}

	if train.To == nil {
		t.Fatal("janela de descoberta precisa de limite superior")
	}
	if !train.To.Equal(holdout.From) {
		t.Fatalf("corte inconsistente: treino vai até %v, validação começa em %v", train.To, holdout.From)
	}

	var nTrain, nHoldout, nAmbas int
	for _, m := range matches {
		inTrain := !m.MatchDate.Before(train.From) && m.MatchDate.Before(*train.To)
		inHoldout := !m.MatchDate.Before(holdout.From)
		switch {
		case inTrain && inHoldout:
			nAmbas++
		case inTrain:
			nTrain++
		case inHoldout:
			nHoldout++
		}
	}
	if nAmbas != 0 {
		t.Errorf("%d partidas caíram nas duas janelas", nAmbas)
	}
	if nTrain+nHoldout != len(matches) {
		t.Errorf("partidas perdidas no corte: %d + %d != %d", nTrain, nHoldout, len(matches))
	}
	// 70/30 com tolerância — o corte respeita a fronteira de data, então não é exato.
	frac := float64(nTrain) / float64(len(matches))
	if frac < 0.65 || frac > 0.75 {
		t.Errorf("fração de treino = %.3f, esperado ~%.2f", frac, DefaultTrainFraction)
	}
}

func TestSplitRecusaHistoricoInsuficiente(t *testing.T) {
	casos := []struct {
		nome    string
		matches []domain.Match
	}{
		{"vazio", nil},
		{"uma partida", noiseHistory(1, 1)},
	}
	for _, c := range casos {
		if _, _, ok := splitHistory(c.matches, DefaultTrainFraction); ok {
			t.Errorf("%s: split não deveria ser possível", c.nome)
		}
	}

	// Todas as partidas no mesmo dia: não há passado e futuro para separar.
	mesmoDia := noiseHistory(200, 5)
	d := mesmoDia[0].MatchDate
	for i := range mesmoDia {
		mesmoDia[i].MatchDate = d
	}
	if _, _, ok := splitHistory(mesmoDia, DefaultTrainFraction); ok {
		t.Error("histórico com uma única data não deveria ser divisível")
	}
}

// Liga sem janela de validação possível não publica nada — e o motivo fica
// registrado, em vez de o ciclo simplesmente não achar nada e parecer normal.
func TestLigaSemJanelaDeValidacaoNaoPublica(t *testing.T) {
	curta := noiseHistory(1, 1)
	repo := &fakeStrategyRepo{}
	eng := newTestEngine(curta, repo, DefaultCriteria())

	res, err := eng.RunLeague(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("RunLeague: %v", err)
	}
	if res.Published != 0 {
		t.Errorf("publicou %d sem janela de validação", res.Published)
	}
	if res.Rejections[string(rejectNotSplittable)] == 0 {
		t.Errorf("motivo %q não foi registrado: %v", rejectNotSplittable, res.Rejections)
	}
}

// --- Configuração não pode desligar a proteção -----------------------------

func TestConfiguracaoNaoDesligaValidacao(t *testing.T) {
	// Tentativas de desativar as travas via configuração.
	c := Criteria{TrainFraction: 1.0, FDRq: 1.0, HoldoutAlpha: 1.0, HoldoutMinGames: 0}.withDefaults()

	if c.TrainFraction != DefaultTrainFraction {
		t.Errorf("TrainFraction = %v; minerar 100%% do histórico não pode ser configurável", c.TrainFraction)
	}
	if c.FDRq != DefaultFDRq {
		t.Errorf("FDRq = %v; aceitar qualquer p-valor não pode ser configurável", c.FDRq)
	}
	if c.HoldoutAlpha != DefaultHoldoutAlpha {
		t.Errorf("HoldoutAlpha = %v", c.HoldoutAlpha)
	}
	if c.HoldoutMinGames != DefaultHoldoutMinGames {
		t.Errorf("HoldoutMinGames = %v", c.HoldoutMinGames)
	}
	if c.FDRMethod == "" {
		t.Error("FDRMethod vazio deveria cair no padrão conservador")
	}
}

// --- Componentes da validação ----------------------------------------------

func TestBreakEvenProbabilityVemDasOdds(t *testing.T) {
	r := &usecase.BacktestResult{Entries: []usecase.BacktestEntry{
		{Odd: 2.00}, {Odd: 2.00}, {Odd: 4.00}, {Odd: 4.00},
	}}
	got, ok := breakEvenProbability(r)
	if !ok {
		t.Fatal("deveria ser calculável")
	}
	// média de (0.5, 0.5, 0.25, 0.25) = 0.375
	if got < 0.3749 || got > 0.3751 {
		t.Errorf("break-even = %v, esperado 0.375", got)
	}

	// Sem odd utilizável não há hipótese nula.
	if _, ok := breakEvenProbability(&usecase.BacktestResult{Entries: []usecase.BacktestEntry{{Odd: 1}}}); ok {
		t.Error("odd = 1 não sustenta hipótese nula")
	}
	if _, ok := breakEvenProbability(&usecase.BacktestResult{}); ok {
		t.Error("resultado sem entradas não sustenta hipótese nula")
	}
}

// Taxa de acerto alta com odd baixa NÃO é vantagem — é o preço justo. O p-valor
// precisa enxergar isso, senão o motor volta a premiar linha fácil.
func TestPValorNaoPremiaAcertoAltoComOddBaixa(t *testing.T) {
	// 90 acertos em 100, odd 1.11 → break-even 90%. Exatamente o esperado.
	semVantagem := resultadoSintetico(100, 90, 1.11)
	p, ok := pValue(semVantagem)
	if !ok {
		t.Fatal("deveria ser calculável")
	}
	if p < 0.30 {
		t.Errorf("p = %.4f: acertar 90%% numa odd que embute 90%% não deveria parecer significativo", p)
	}

	// 65 acertos em 100 numa odd 2.00 (break-even 50%) — isso sim é vantagem.
	comVantagem := resultadoSintetico(100, 65, 2.00)
	p2, ok := pValue(comVantagem)
	if !ok {
		t.Fatal("deveria ser calculável")
	}
	if p2 > 0.01 {
		t.Errorf("p = %.4f: 65%% de acerto numa odd de break-even 50%% deveria ser significativo", p2)
	}
}

func TestCheckHoldoutReprovaCadaMotivo(t *testing.T) {
	casos := []struct {
		nome string
		r    *usecase.BacktestResult
		want rejection
	}{
		{"amostra pequena", resultadoSintetico(10, 9, 2.00), rejectHoldoutSample},
		{"sem lucro", resultadoSintetico(50, 20, 2.00), rejectHoldoutProfit},
		{"lucro mas não significativo", resultadoSintetico(50, 26, 2.00), rejectHoldoutSignificance},
		{"nil", nil, rejectHoldoutSample},
	}
	for _, c := range casos {
		v := checkHoldout(c.r, DefaultHoldoutMinGames, DefaultHoldoutAlpha)
		if v.Passed {
			t.Errorf("%s: deveria reprovar", c.nome)
			continue
		}
		if v.Reason != c.want {
			t.Errorf("%s: motivo %q, esperado %q", c.nome, v.Reason, c.want)
		}
	}

	// E um caso que passa: 40 acertos em 50 na odd 2.00.
	bom := resultadoSintetico(50, 40, 2.00)
	if v := checkHoldout(bom, DefaultHoldoutMinGames, DefaultHoldoutAlpha); !v.Passed {
		t.Errorf("resultado forte fora da amostra deveria passar: motivo %q", v.Reason)
	}
}

// resultadoSintetico monta um BacktestResult coerente (entradas, acertos, lucro
// e ROI) para exercitar os testes de significância sem passar pelo motor.
func resultadoSintetico(n, hits int, odd float64) *usecase.BacktestResult {
	const stake = 1.0
	entries := make([]usecase.BacktestEntry, 0, n)
	profit := 0.0
	for i := 0; i < n; i++ {
		hit := i < hits
		pl := -stake
		if hit {
			pl = stake * (odd - 1)
		}
		profit += pl
		entries = append(entries, usecase.BacktestEntry{Hit: hit, Odd: odd, ProfitLoss: pl})
	}
	staked := float64(n) * stake
	roi := 100 * profit / staked
	return &usecase.BacktestResult{
		MatchCount: n, Hits: hits, Misses: n - hits,
		HitRate: 100 * float64(hits) / float64(n),
		Entries: entries, Profit: profit, TotalStaked: staked,
		ROI: roi, Yield: roi,
	}
}
