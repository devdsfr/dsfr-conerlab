// Package aidocs gera o documento de contexto do CornerLab em Markdown — o
// arquivo que o usuário baixa no painel Integrações para dar a outra IA (ChatGPT,
// Claude, Gemini) tudo que ela precisa saber para trabalhar com a plataforma.
//
// Por que GERADO e não um .md estático no repositório: um arquivo escrito à mão
// desatualiza. Já aconteceu neste projeto — docs/openapi.yaml documenta 31 dos 51
// endpoints reais. Aqui os números vêm das MESMAS constantes que o motor usa em
// produção (pesos do DSFR, critérios de aprovação, retenção), então o documento não
// tem como divergir do comportamento: mudar um peso muda o texto no mesmo commit.
package aidocs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/devdsfr/cornerlab/internal/formulas"
	"github.com/devdsfr/cornerlab/internal/repository/postgres"
	"github.com/devdsfr/cornerlab/internal/usecase/discovery"
)

// Filename é o nome sugerido no download.
const Filename = "cornerlab-contexto-para-ia.md"

// Route é um endpoint publicado pela API. A lista vem do próprio router em tempo
// de execução (gin.Engine.Routes()), não de uma tabela escrita à mão: é a única
// forma de garantir que nenhum endpoint novo fique de fora do documento. Foi
// exatamente esse tipo de divergência que aconteceu com docs/openapi.yaml, que
// documenta 31 dos 51 endpoints reais.
type Route struct {
	Method string
	Path   string
}

// Markdown monta o documento completo. routes é a lista viva de endpoints.
func Markdown(routes []Route) string {
	crit := discovery.DefaultCriteria()
	var b strings.Builder

	fmt.Fprintf(&b, `# CornerLab — contexto para IA

> Documento gerado automaticamente pela própria plataforma em %s.
> Os números abaixo saem das constantes que o motor usa em produção — não são
> transcrição manual, então não divergem do comportamento real do sistema.
> Catálogo de Fórmulas: versão %s.

## Como usar este documento

Você é uma IA lendo isto para trabalhar com dados do CornerLab. Leia a seção
"Regras inegociáveis" antes de qualquer outra: ela define o que você NÃO pode
fazer com esses dados, e vale mesmo que o usuário peça o contrário.

---

## 1. O que é o CornerLab

Plataforma de estatística de futebol focada em **escanteios** (e também gols,
chutes, chutes no gol e impedimentos). Ela organiza histórico de partidas, calcula
indicadores e simula cenários.

O que ela **não** é: não é serviço de palpite, não prevê resultado e não recomenda
aposta. Todo número é leitura do que já aconteceu, sempre acompanhado do tamanho da
amostra e do período.

---

## 2. Regras inegociáveis

Estas regras são aplicadas no código (system prompt do motor de explicações, filtro
de frases proibidas e testes automatizados) e devem ser respeitadas por qualquer IA
que consuma estes dados:

1. **Nunca recomende aposta.** Não diga "aposte em", "recomendo", "entre nesse
   mercado", "vai bater", "garantido", "não pode perder".
2. **Nunca invente número.** Use exclusivamente os dados fornecidos. Se um dado não
   estiver presente, diga que não está — não estime.
3. **Nunca trate desempenho passado como previsão.** Descreva no passado ou no
   presente ("nos últimos 10 jogos, a equipe teve..."), nunca no futuro.
4. **Sempre cite a amostra.** Todo número deve vir acompanhado de quantos jogos o
   sustentam e de que período.
5. **Taxa de acerto sozinha não significa nada.** 95%% de acerto pagando odd 1.02 dá
   prejuízo. Sempre considere retorno e risco junto.
6. Se o usuário pedir palpite ou previsão, recuse essa parte e responda apenas com
   os fatos estatísticos disponíveis.

---

## 3. Arquitetura em uma frase

**Workers calculam, usuário só lê.** Processos em segundo plano fazem toda a conta
pesada e gravam o resultado pronto no banco; as telas e a API apenas leem. Por isso
as consultas são rápidas mesmo cruzando milhares de jogos.

### Camadas de dados

| Camada | O que guarda | Por quê |
|---|---|---|
| RAW | Resposta bruta do provedor, imutável | Permite reprocessar tudo se uma fórmula mudar, sem re-consultar a API |
| NORMALIZADO | Partidas, equipes, ligas, temporadas | Base de todas as consultas |
| ANALYTICS | Métricas, backtests, scores, saúde | Resultado já calculado, esperando ser lido |

### Ciclo automático (1x por dia, 03:00 BRT)

1. Confere se o provedor de dados está de pé
2. Descobre partidas novas (status AGENDADO)
3. Finaliza partidas cuja data já passou, buscando as estatísticas
4. Recalcula métricas por equipe (médias, janelas, consistência, tendência)
5. Reexecuta o backtest de cada estratégia ativa e recalcula saúde e scores
6. Roda a varredura de descobertas
7. Registra a execução em worker_runs

**Exceção à regra:** o Simulador de Filtros e os botões "Procurar agora" /
"Executar agora" calculam sob demanda — são os únicos pontos onde há espera.

---

## 4. Conceitos de leitura obrigatória

- **Amostra**: quantos jogos históricos bateram com o critério. É o número mais
  importante e o mais ignorado. 100%% de acerto em 8 jogos não significa nada.
- **Linha / limiar**: o corte que define acerto. "Escanteios 8.5+" = o jogo precisa
  ter 9 ou mais escanteios no total. O ",5" existe para não haver empate.
- **Janela**: recorte temporal (últimos 5, 10, 15 ou 20 jogos). Captura momento de
  forma, em oposição à média da temporada inteira.
- **Mando**: casa, fora ou qualquer. Times costumam pressionar mais em casa.
- **Tier de adversário**: DESATIVADO. A ideia era separar "faz muitos escanteios"
  de "faz muitos escanteios contra time fraco", com G6 (seis primeiros), G12 (doze
  primeiros) e Z4 (quatro últimos). Nunca funcionou: a coluna teams.tier continha
  uma constante gravada no código, não classificação. O filtro foi removido da
  tela, do motor de backtest e do espaço de busca. Se você receber opponent_tier
  em alguma definição antiga, **ignore** — o backtest recusa esse parâmetro.
- **Odd**: sempre decimal (2.20 = recebe 2,20 por 1 apostado).
- **Unidade**: uma stake. "Lucro de 13,8 unidades" = 13,8 vezes a stake configurada.

---

## 5. Fórmulas (Catálogo v%s)

### Probabilidade e odds

- Probabilidade = eventos favoráveis ÷ eventos possíveis
- Probabilidade implícita = 1 ÷ odd (a chance que a casa está embutindo)
- Odd justa = 1 ÷ probabilidade (piso de 1.01)
- Break-even = 1 ÷ odd (acerto mínimo para não perder naquela odd)
- Edge = probabilidade real − probabilidade implícita

### Retorno

- ROI = lucro ÷ investimento × 100
- Yield = lucro ÷ volume apostado × 100
- EV = (P vitória × lucro) − (P derrota × perda), onde lucro = stake × (odd − 1)

> **Leia isto antes de usar ROI, Yield ou EV como três indicadores.**
> O motor de backtest aposta stake constante e liquida toda entrada. Logo
> "investimento" e "volume apostado" são a mesma quantidade, e **ROI e Yield são
> numericamente iguais** — não são duas evidências, são uma.
> **O EV não é calculado.** A fórmula acima está no catálogo, mas exigiria
> P(vitória) estimada por um modelo independente e fora da amostra, que o
> CornerLab ainda não tem; com a taxa de acerto do próprio lote o EV colapsa no
> ROI realizado. O campo "ev" vem **nulo** e deve ser tratado como ausente, nunca
> como zero.
- Profit Factor = lucro bruto ÷ prejuízo bruto
- Recovery Factor = lucro líquido ÷ drawdown máximo
- Expectancy = (taxa acerto × ganho médio) − (taxa erro × perda média)

### Risco

- Variância = Σ(x − média)² ÷ N; Desvio padrão = √variância
- Drawdown máximo = (pico − vale) ÷ pico
- Sharpe adaptado = (ROI médio − ROI livre de risco) ÷ desvio padrão dos ROIs
- Calmar adaptado = ROI ÷ drawdown máximo
- Monte Carlo: sorteia milhares de sequências para obter distribuição de
  resultados e probabilidade de ruína (semente fixa, reprodutível)

### Gestão de banca

- Kelly = (b × p − q) ÷ b, com b = odd − 1, p = taxa de acerto, q = 1 − p
- Stake percentual = banca × percentual; Stake fixa = valor constante
- Juros compostos = capital × (1 + taxa)^períodos
- CAGR = (capital final ÷ capital inicial)^(1÷anos) − 1

---

## 6. Scores proprietários

Todos vão de 0 a 100. Os pesos abaixo são os que estão rodando agora.

### DSFR Score — resumo da qualidade de uma estratégia

| Componente | Peso |
|---|---|
| ROI | %.0f%% |
| Taxa de acerto | %.0f%% |
| Drawdown (invertido) | %.0f%% |
| Tamanho da amostra | %.0f%% |
| Consistência | %.0f%% |
| Variância (invertida) | %.0f%% |

Até a versão 1.0 do catálogo havia também um slot de EV (20%%) e um de Yield
(10%%), ambos alimentados pelo mesmo número do ROI — 50%% do score era uma
variável só. Foram removidos na v1.1; o peso foi distribuído entre os
componentes acima, não devolvido ao ROI.

**Faixas:** Elite 91–100 · Excelente 81–90 · Muito Boa 71–80 · Boa 61–70 ·
Regular 40–60 · Descartar abaixo de 40.

### Health Score — saúde recente

Health = 50 + 50 × média(ΔROI, −ΔDrawdown, ΔConsistência), comparando o
período atual com o anterior.

- **50** = estável, ou primeira execução (sem histórico para comparar)
- **acima de 50** = melhorando · **abaixo de 50** = piorando

### Demais scores

- **Consistência**: %.0f%% taxa de acerto + %.0f%% (1−variância) + %.0f%% (1−drawdown) + %.0f%% robustez
- **Ranking**: %.0f%% DSFR + %.0f%% Health + %.0f%% ROI + %.0f%% Confiança
- **Oportunidade**: %.0f%% Health + %.0f%% DSFR + %.0f%% ROI + %.0f%% acerto + %.0f%% consistência
- **Tendência** (−1 a +1): %.0f%% últimos 5 + %.0f%% últimos 10 + %.0f%% últimos 20
- **Confiança**: média de volume, consistência, baixa variância e robustez temporal
- **Robustez**: média de volume, consistência, baixa variância, ROI e histórico
- **Volatilidade** (quanto maior, pior): média de desvio padrão e oscilações de ROI e EV
- **Risco** (quanto maior, pior): média de drawdown, variância, volatilidade e taxa de erro

### Ciclo de vida

| Estágio | Quando |
|---|---|
| Nascimento | amostra abaixo do mínimo |
| Crescimento | tendência acima de +0,15 |
| Maturidade | saudável e estável |
| Declínio | saúde abaixo de 45 ou tendência abaixo de −0,15 |
| Obsoleta | saúde abaixo de 25 |

---

## 7. Discovery Engine — descobertas automáticas

O sistema combina automaticamente as variáveis abaixo e roda um backtest em cada
combinação, usando o MESMO motor do Simulador de Filtros (por isso qualquer número
publicado é reproduzível na tela).

**Espaço de busca por campeonato:** 5 linhas de escanteio × 3 mandos × 3 janelas ×
3 tetos de odd = 135 combinações. Eram 540 até 09/2026, quando o eixo de tier de
adversário saiu — ele multiplicava a grade por 4 sem acrescentar hipótese nenhuma
(ver "Tier de adversário" na seção de termos).

### Critérios de aprovação — precisa passar em TODOS

| Critério | Limite | Motivo de rejeição |
|---|---|---|
| Jogos na amostra | mínimo %d (trava absoluta: %d) | amostra_insuficiente |
| Taxa de acerto | mínimo %.0f%% | win_rate_baixo |
| ROI | mínimo %.0f%% | roi_baixo |
| Yield | mínimo %.0f%% | yield_baixo |
| Lucro | maior que zero | lucro_nao_positivo |
| Drawdown máximo | até %.0f%% do capital movimentado | drawdown_alto |
| DSFR Score | mínimo %.0f | score_baixo |
| Teto por campeonato | %d publicadas | corte por ranking |

**Guarda contra overfitting:** a trava de %d jogos não é configurável. Existe porque
com amostra pequena é fácil achar "100%% de acerto" por acaso — e isso não se repete.

### Passar nos critérios acima NÃO basta

Passar em todos eles ainda é um resultado dentro da amostra. Como o motor testa
centenas de combinações contra o mesmo histórico, algumas passam por sorte. Há mais
duas barreiras, e as duas são obrigatórias:

**1. Corte temporal.** O histórico de cada campeonato é dividido por data: os %.0f%%
mais antigos formam a janela de DESCOBERTA e os %.0f%% mais recentes a janela de
VALIDAÇÃO. A mineração só enxerga a de descoberta. Toda partida da data de corte vai
para a validação, para que nenhuma data caia nas duas. Campeonato cujo histórico não
permite esse corte não publica nada (motivo: liga_sem_janela_de_validacao).

**2. Significância corrigida por número de testes.** Cada combinação vira um teste
binomial unilateral contra a probabilidade que a própria odd embutia (1 ÷ odd). Não
se pergunta "a taxa de acerto é alta?" — uma linha fácil acerta 90%% e paga 1.05, o
que é prejuízo. Pergunta-se se o acerto observado supera o que a odd já precificava.
Os p-valores de todas as combinações passam por controle de falsas descobertas
(Benjamini–Yekutieli, q = %.2f), então **o limiar depende de quantos testes foram
feitos**: testar mais custa mais caro. Motivo de rejeição: nao_sobreviveu_correcao_fdr.

**3. Reteste fora da amostra.** Quem sobrevive é reexecutado na janela de validação,
que nunca foi lida. Para publicar é preciso: mínimo de %d ocorrências lá, lucro
positivo e significância própria (α = %.2f). Motivos: validacao_amostra_insuficiente,
validacao_sem_lucro, validacao_nao_significativa.

Como consequência, a descrição de toda descoberta publicada traz o período em que o
padrão foi procurado, o período em que ele se sustentou e as duas probabilidades de
o resultado ter saído por acaso. Um ciclo interrompido no meio não publica nada — a
correção só é válida sobre a varredura completa.

**Idempotência:** cada combinação gera um nome determinístico; um novo ciclo
atualiza a descoberta equivalente em vez de duplicá-la. Descobertas não
republicadas no ciclo mais recente são desativadas.

**Zero descobertas é resultado válido.** Se nenhuma combinação passa nos critérios,
a lista fica vazia — isso é o sistema funcionando, não falha.

---

## 8. API REST

Base: `+"`https://dsfrcornerlab.com.br`"+`

A lista abaixo é lida do próprio router no momento em que este documento é gerado —
endpoint novo aparece aqui automaticamente, sem ninguém precisar lembrar de editar.

%s

**Autenticação:** exigem `+"`Authorization: Bearer <token>`"+` as rotas de estratégias,
apostas, alertas, gestão de banca, exportações, assinatura, e os disparos
`+"`POST /sync/run`"+` e `+"`POST /discovery/run`"+`. As demais são leitura pública.

**Tarefas longas:** `+"`/sync/run`"+` e `+"`/discovery/run`"+` respondem **202 Accepted** e rodam em
segundo plano; acompanhe pelos endpoints `+"`/progress`"+` correspondentes. Um segundo
disparo enquanto a tarefa roda devolve **409 Conflict**.

---

## 9. Limites operacionais

- **Provedor de dados**: chamadas espaçadas (~9 por minuto) para respeitar o limite
  da API-Football; um ciclo completo leva alguns minutos.
- **Plano gratuito**: backtests limitados aos últimos 90 dias de histórico. O plano
  Premium libera o período completo.
- **Retenção do log de chamadas**: %d dias.
- **Odds**: quando o provedor não publica odd de escanteios, o sistema pode gerar
  valor sintético a partir da média do lote. **Atenção:** números derivados de odd
  sintética não representam mercado real e não devem ser tratados como resultado
  válido de backtest.

---

## 10. Glossário rápido de siglas

EV (valor esperado) · ROI (retorno sobre investimento) · WR (win rate, taxa de
acerto) · BE (break-even) · DD (drawdown) · PF (profit factor) · RF (recovery
factor) · CAGR (crescimento anual composto) · MC (Monte Carlo) · DSFR (score
proprietário) · G6/G12/Z4 (faixas da tabela) · RAW (camada de dados brutos)

---

## Aviso final

Nada neste documento é recomendação de aposta. Todas as fórmulas descrevem
desempenho histórico de critérios aplicados a dados passados. Resultado passado não
garante resultado futuro, e nenhum score alto muda isso.
`,
		time.Now().Format("02/01/2006"),
		formulas.Version,
		formulas.Version,
		// Pesos do DSFR (v1.1 — sem EV e sem Yield, ver AUD-002)
		formulas.DSFRv11WROI*100, formulas.DSFRv11WWinRate*100,
		formulas.DSFRv11WDrawdown*100, formulas.DSFRv11WSampleSize*100,
		formulas.DSFRv11WConsistency*100, formulas.DSFRv11WVariance*100,
		// Consistência
		formulas.ConsistencyWWinRate*100, formulas.ConsistencyWVariance*100,
		formulas.ConsistencyWDrawdown*100, formulas.ConsistencyWRobustness*100,
		// Ranking
		formulas.Rankingv11WDSFR*100, formulas.Rankingv11WHealth*100,
		formulas.Rankingv11WROI*100, formulas.Rankingv11WConfidence*100,
		// Oportunidade
		formulas.OpportunityWHealth*100, formulas.OpportunityWDSFR*100, formulas.OpportunityWROI*100,
		formulas.OpportunityWWinRate*100, formulas.OpportunityWConsistency*100,
		// Tendência
		formulas.TrendWLast5*100, formulas.TrendWLast10*100, formulas.TrendWLast20*100,
		// Critérios do Discovery
		crit.MinGames, discovery.AbsoluteMinimumGames,
		crit.MinWinRate, crit.MinROI, crit.MinYield,
		crit.MaxDrawdown, crit.MinDSFR, crit.MaxPerLeague,
		discovery.AbsoluteMinimumGames,
		// AUD-003: corte temporal, correção de múltiplos testes e reteste.
		crit.TrainFraction*100, (1-crit.TrainFraction)*100,
		crit.FDRq, crit.HoldoutMinGames, crit.HoldoutAlpha,
		// Seção 8: lista viva de endpoints, lida do router
		routeTable(routes),
		// Retenção
		postgres.UsageLogRetentionDays,
	)

	return b.String()
}

// routeTable formata a lista de endpoints como tabela Markdown, ordenada por
// caminho para o documento ficar estável entre gerações (dois downloads seguidos
// produzem o mesmo texto, o que facilita comparar versões).
func routeTable(routes []Route) string {
	if len(routes) == 0 {
		return "_(lista de endpoints indisponível nesta geração)_"
	}

	ordenadas := make([]Route, len(routes))
	copy(ordenadas, routes)
	sort.Slice(ordenadas, func(i, j int) bool {
		if ordenadas[i].Path == ordenadas[j].Path {
			return ordenadas[i].Method < ordenadas[j].Method
		}
		return ordenadas[i].Path < ordenadas[j].Path
	})

	var t strings.Builder
	t.WriteString("| Método | Rota |\n|---|---|\n")
	for _, r := range ordenadas {
		fmt.Fprintf(&t, "| %s | `%s` |\n", r.Method, r.Path)
	}
	fmt.Fprintf(&t, "\n_%d endpoints publicados._", len(ordenadas))
	return t.String()
}
