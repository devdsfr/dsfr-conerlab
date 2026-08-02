package discovery

import (
	"strings"
	"testing"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/usecase"
	"github.com/devdsfr/cornerlab/internal/usecase/strategyengine"
)

// backtest monta um BacktestResult mínimo com os campos que os critérios do
// doc 08 avaliam. dd é o drawdown em unidades de stake (como o motor devolve).
func backtest(games, hits int, roi, yield, dd float64) *usecase.BacktestResult {
	hitRate := 0.0
	if games > 0 {
		hitRate = 100 * float64(hits) / float64(games)
	}
	return &usecase.BacktestResult{
		MatchCount:  games,
		Hits:        hits,
		Misses:      games - hits,
		HitRate:     hitRate,
		ROI:         roi,
		Yield:       yield,
		MaxDrawdown: dd,
		TotalStaked: float64(games), // stake 1 por entrada
	}
}

// approvedResult passa folgadamente em todos os critérios do doc 08.
func approvedResult() *usecase.BacktestResult {
	return backtest(150, 120, 14.0, 8.0, 15.0) // 80% acerto, drawdown 10%
}

func TestValidateAcceptsResultMeetingAllCriteria(t *testing.T) {
	if reason := DefaultCriteria().withDefaults().validate(approvedResult()); reason != "" {
		t.Fatalf("resultado dentro de todos os critérios foi rejeitado por %q", reason)
	}
}

func TestValidateRejectsEachCriterion(t *testing.T) {
	crit := DefaultCriteria().withDefaults()

	cases := []struct {
		name   string
		result *usecase.BacktestResult
		want   rejection
	}{
		// doc 08: amostra insuficiente é o primeiro corte.
		{"amostra abaixo do mínimo", backtest(80, 64, 14, 8, 8), rejectSample},
		{"win rate abaixo de 75%", backtest(150, 100, 14, 8, 15), rejectWinRate},
		{"roi abaixo de 10%", backtest(150, 120, 4, 8, 15), rejectROI},
		{"yield abaixo de 5%", backtest(150, 120, 14, 2, 15), rejectYield},
		{"drawdown acima de 20%", backtest(150, 120, 14, 8, 45), rejectDrawdown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := crit.validate(tc.result); got != tc.want {
				t.Errorf("esperava rejeição %q, veio %q", tc.want, got)
			}
		})
	}
}

// Regra explícita do doc 08 ("Overfitting"): win rate altíssimo com amostra
// pequena NUNCA pode ser publicado, mesmo com ROI e drawdown excelentes.
func TestOverfittingGuardRejectsTinySampleWithPerfectNumbers(t *testing.T) {
	perfectButTiny := backtest(12, 12, 95, 60, 0)
	if reason := DefaultCriteria().withDefaults().validate(perfectButTiny); reason != rejectSample {
		t.Fatalf("12 jogos com 100%% de acerto deveriam ser barrados por amostra, veio %q", reason)
	}
}

// A trava anti-overfitting não pode ser afrouxada por configuração.
func TestWithDefaultsEnforcesAbsoluteMinimumSample(t *testing.T) {
	loose := Criteria{MinGames: 5}.withDefaults()
	if loose.MinGames != absoluteMinimumGames {
		t.Errorf("MinGames=5 deveria ser elevado para %d, veio %d", absoluteMinimumGames, loose.MinGames)
	}
	if reason := loose.validate(backtest(20, 20, 90, 50, 0)); reason != rejectSample {
		t.Errorf("20 jogos deveriam continuar sendo barrados, veio %q", reason)
	}
}

func TestWithDefaultsKeepsExplicitOverrides(t *testing.T) {
	c := Criteria{MinGames: 200, MinROI: 25, MaxPerLeague: 5}.withDefaults()
	if c.MinGames != 200 || c.MinROI != 25 || c.MaxPerLeague != 5 {
		t.Errorf("overrides explícitos foram perdidos: %+v", c)
	}
	if c.MinYield != DefaultMinYield || c.MinWinRate != DefaultMinWinRate {
		t.Errorf("campos não informados deveriam cair no padrão do doc 08: %+v", c)
	}
}

func TestDrawdownPctIsRelativeToStakedCapital(t *testing.T) {
	r := backtest(200, 160, 12, 7, 30) // 30 unidades sobre 200 movimentadas
	if got := drawdownPct(r); got != 15 {
		t.Errorf("drawdown deveria ser 15%%, veio %v", got)
	}
	// Sem capital movimentado não há percentual a calcular (evita divisão por zero).
	if got := drawdownPct(&usecase.BacktestResult{}); got != 0 {
		t.Errorf("resultado vazio deveria ter drawdown 0, veio %v", got)
	}
}

func TestClassifyFollowsDocBands(t *testing.T) {
	cases := []struct {
		score float64
		want  Classification
	}{
		{95, ClassElite}, {91, ClassElite},
		{90, ClassExcelent}, {81, ClassExcelent},
		{80, ClassVeryGood}, {71, ClassVeryGood},
		{70, ClassGood}, {61, ClassGood},
		{60, ClassRegular}, {40, ClassRegular},
		{39, ClassDiscard}, {0, ClassDiscard},
	}
	for _, tc := range cases {
		if got := Classify(tc.score); got != tc.want {
			t.Errorf("score %v: esperava %q, veio %q", tc.score, tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Espaço de busca
// ---------------------------------------------------------------------------

func TestGenerateCombosCoversFullLeagueGrid(t *testing.T) {
	combos := generateCombos(nil, false)
	want := len(cornerLines) * len(homeAwayOptions) * len(windowOptions) *
		len(opponentTierOptions) * len(maxOddsOptions)
	if len(combos) != want {
		t.Fatalf("esperava %d combinações de liga, veio %d", want, len(combos))
	}
	for _, c := range combos {
		if c.teamID != nil {
			t.Fatal("varredura de liga não deveria conter combinação por equipe")
		}
	}
}

func TestGenerateCombosAddsReducedTeamGrid(t *testing.T) {
	teams := []domain.Team{{ID: 1, Name: "Palmeiras"}, {ID: 2, Name: "Flamengo"}}
	base := len(generateCombos(nil, false))
	combos := generateCombos(teams, true)

	perTeam := len(cornerLines) * len(homeAwayOptions) * len(maxOddsOptions)
	if want := base + len(teams)*perTeam; len(combos) != want {
		t.Fatalf("esperava %d combinações, veio %d", want, len(combos))
	}

	// Cada equipe precisa aparecer com o próprio ID — um ponteiro compartilhado
	// pelo laço faria todas apontarem para a última equipe.
	seen := map[int64]bool{}
	for _, c := range combos {
		if c.teamID != nil {
			seen[*c.teamID] = true
		}
	}
	if !seen[1] || !seen[2] {
		t.Errorf("cada equipe deveria ter seu próprio team_id, vistos: %v", seen)
	}
}

// Idempotência do ciclo (migration 012): combinações iguais geram nomes iguais e
// combinações diferentes geram nomes diferentes.
func TestComboNameIsDeterministicAndUnique(t *testing.T) {
	combos := generateCombos([]domain.Team{{ID: 1, Name: "Palmeiras"}}, true)

	names := map[string]bool{}
	for _, c := range combos {
		n := c.name("Brasileirão Série A")
		if n != c.name("Brasileirão Série A") {
			t.Fatalf("nome não determinístico: %q", n)
		}
		if names[n] {
			t.Fatalf("nome duplicado para combinações distintas: %q", n)
		}
		names[n] = true

		if len([]rune(n)) > strategyNameMaxLen {
			t.Fatalf("nome excede o limite da coluna (%d runas): %q", strategyNameMaxLen, n)
		}
	}
}

func TestComboNameTruncatesLongLeagueNames(t *testing.T) {
	long := strings.Repeat("Campeonato Muito Longo ", 20)
	n := combo{line: 8, maxOdds: 2.0}.name(long)
	if len([]rune(n)) > strategyNameMaxLen {
		t.Fatalf("nome deveria ser truncado em %d runas, veio %d", strategyNameMaxLen, len([]rune(n)))
	}
}

func TestComboDefinitionMatchesFilterFormat(t *testing.T) {
	teamID := int64(7)
	c := combo{teamID: &teamID, line: 9, homeAway: "home", window: 10, tier: "G6", maxOdds: 2.20}

	raw, err := c.definition(18, []int64{26, 27})
	if err != nil {
		t.Fatalf("definition falhou: %v", err)
	}

	// A definição precisa ser reinterpretável pelo Strategy Engine sem conversão.
	d, err := strategyengine.ParseDefinition(raw)
	if err != nil {
		t.Fatalf("definição gerada não é válida para o Strategy Engine: %v", err)
	}
	if d.LeagueID != 18 || len(d.SeasonIDs) != 2 || d.CornersThreshold != 9 ||
		d.HomeAway != "home" || d.LastNGames != 10 || d.OpponentTier != "G6" ||
		d.MaxOdds != 2.20 || d.Metric != "corners" {
		t.Errorf("definição não reflete a combinação: %+v", d)
	}
	if d.TeamID == nil || *d.TeamID != 7 {
		t.Errorf("team_id perdido na serialização: %+v", d.TeamID)
	}
}

// Toda combinação precisa exigir odd registrada, senão ROI/EV não têm
// significado (ver comentário de maxOddsOptions).
func TestEveryComboRequiresRegisteredOdds(t *testing.T) {
	for _, c := range generateCombos([]domain.Team{{ID: 1, Name: "X"}}, true) {
		if c.maxOdds <= 0 {
			t.Fatalf("combinação sem teto de odd permitiria jogos sem odd real: %+v", c)
		}
	}
}

// ---------------------------------------------------------------------------
// Explicação publicada
// ---------------------------------------------------------------------------

func TestDescribeCarriesSampleAndDisclaimer(t *testing.T) {
	c := candidate{
		combo:  combo{line: 8, homeAway: "home", window: 10, maxOdds: 2.20},
		result: approvedResult(),
		dsfr:   72.5,
	}
	text := describe(c, "Brasileirão Série A")

	// Rastreabilidade: amostra e período precisam acompanhar os números.
	if !strings.Contains(text, "150 ocorrências") {
		t.Errorf("descrição deveria informar a amostra analisada: %q", text)
	}
	if !strings.Contains(text, "Brasileirão Série A") {
		t.Errorf("descrição deveria informar a liga: %q", text)
	}
	if !strings.Contains(text, "não constituem recomendação de aposta") {
		t.Errorf("descrição deveria conter o disclaimer: %q", text)
	}
}

// Regra inegociável da plataforma: nenhuma linguagem de recomendação/garantia.
func TestDescribeNeverRecommends(t *testing.T) {
	forbidden := []string{
		"aposte", "apostar em", "recomendamos", "recomendo", "sugerimos",
		"garantido", "lucro certo", "vai acontecer", "tendência para o próximo",
		"oportunidade de entrada",
	}
	for _, tier := range []string{"", "G6"} {
		for _, team := range []string{"", "Palmeiras"} {
			c := candidate{
				combo:  combo{line: 9, homeAway: "away", tier: tier, teamName: team, maxOdds: 1.70},
				result: approvedResult(),
				dsfr:   88,
			}
			text := strings.ToLower(describe(c, "Premier League"))
			for _, term := range forbidden {
				if strings.Contains(text, term) {
					t.Errorf("descrição contém linguagem proibida %q: %s", term, text)
				}
			}
		}
	}
}
