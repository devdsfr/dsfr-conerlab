// Package formulas implementa o Formula Catalog do CornerLab (Remodelagem/27,
// "Documento Oficial", v1.0) — a fonte única da verdade para todos os cálculos
// matemáticos da plataforma.
//
// Regra de ouro do catálogo: nenhum indicador do CornerLab pode existir sem
// estar documentado no Formula Catalog. Toda nova métrica deve primeiro ser
// adicionada ao catálogo, revisada e testada; só então implementada aqui.
//
// Convenções deste pacote:
//
//   - Probabilidades são frações no intervalo [0,1] (0.80 = 80%). Funções que
//     retornam percentuais explicitam isso no nome (ex.: ROIPercent).
//   - Odds são decimais europeias (> 1.0). Odd ≤ 1 é entrada inválida.
//   - Funções com entradas potencialmente inválidas retornam (float64, error);
//     helpers triviais e composições internas retornam float64 direto.
//   - Precisão dos testes: 4 casas decimais (tolerância 1e-4), conforme os
//     "Casos de Teste Obrigatórios" do catálogo.
//   - Scores compostos (Consistency, DSFR, Health, Opportunity, ...) usam os
//     pesos definidos no catálogo v1.0; os pesos são constantes exportadas para
//     auditoria.
//
// Índice do catálogo → onde está implementado:
//
//	01 Probabilidade .............. probability.go  Probability
//	02 Probabilidade Implícita .... probability.go  ImpliedProbability
//	03 Fair Odds .................. probability.go  FairOdds
//	04 Break Even ................. probability.go  BreakEven
//	05 Edge ....................... probability.go  Edge
//	06 Expected Value ............. financial.go    ExpectedValue
//	07 ROI ........................ financial.go    ROIPercent
//	08 Yield ...................... financial.go    YieldPercent
//	09 Win Rate ................... rates.go        WinRatePercent
//	10 Loss Rate .................. rates.go        LossRatePercent
//	11 Push Rate .................. rates.go        PushRatePercent
//	12 Drawdown ................... risk.go         MaxDrawdown
//	13 Profit Factor .............. financial.go    ProfitFactor
//	14 Kelly Criterion ............ staking.go      Kelly
//	15 Stake Percentual ........... staking.go      StakePercent
//	16 Stake Fixa ................. staking.go      StakeFixed
//	17 Reinvestimento ............. staking.go      Reinvest
//	18 Juros Compostos ............ staking.go      CompoundInterest
//	19 Monte Carlo ................ montecarlo.go   RunMonteCarlo
//	20 Variância .................. risk.go         Variance
//	21 Desvio Padrão .............. risk.go         StdDev
//	22 Consistência ............... scores.go       ConsistencyIndex
//	23 Confidence Score ........... scores.go       ConfidenceScore
//	24 DSFR Score ................. scores.go       DSFRScore
//	25 Health Score ............... scores.go       HealthScore
//	26 Opportunity Score .......... scores.go       OpportunityScore
//	27 Lifecycle Score ............ scores.go       LifecycleStage
//	28 Ranking Score .............. scores.go       RankingScore
//	29 Trend Score ................ scores.go       TrendScore
//	30 Robustness Score ........... scores.go       RobustnessScore
//	31 Volatility Score ........... scores.go       VolatilityScore
//	32 Risk Score ................. scores.go       RiskScore
//	33 Recovery Factor ............ financial.go    RecoveryFactor
//	34 Sharpe Adaptado ............ risk.go         SharpeAdapted
//	35 Calmar Adaptado ............ risk.go         CalmarAdapted
//	36 Expectancy ................. financial.go    Expectancy
//	37 Expectancy % ............... financial.go    ExpectancyPercent
//	38 CAGR da banca .............. staking.go      CAGR
//	39 Max Consecutive Losses ..... rates.go        MaxConsecutiveLosses
//	40 Max Consecutive Wins ....... rates.go        MaxConsecutiveWins
package formulas

// Version é a versão do Formula Catalog implementada por este pacote.
const Version = "1.0"
