# CORREÇÕES DA AUDITORIA — DSFR CornerLab

Registro das correções aplicadas sobre os achados de `AUDITORIA_COMPLETA_SISTEMA.md`.

**Regra de trabalho:** um problema por vez, na ordem P0 → P1 → P2 → P3, cada um passando
pelas fases A (investigação) → B (implementação) → C (teste) → D (validação) → E (registro).
Nenhum problema é iniciado antes do anterior estar registrado aqui.

**Documento vivo.** Cada correção acrescenta uma seção; nada é reescrito retroativamente.

---

## Índice de status

| ID | Severidade | Prioridade | Status | Data |
|----|------------|-----------|--------|------|
| AUD-001 | CRITICAL | P0 | ✅ RESOLVIDO (código) · ⏸ MIGRATION PENDENTE DE APROVAÇÃO | 2026-09-09 |
| AUD-012 | HIGH | P1 | ✅ RESOLVIDO (junto com AUD-001 — mesma linha de código) | 2026-09-09 |
| AUD-002 | CRITICAL | P0 | ✅ RESOLVIDO (Catálogo v1.1) | 2026-09-09 |
| AUD-003 | CRITICAL | P0 | ✅ RESOLVIDO | 2026-09-09 |
| AUD-004 | CRITICAL | P0 | ✅ RESOLVIDO (eixo removido) · ⏸ MIGRATION 014 PENDENTE | 2026-09-09 |

---

## Pipeline parado desde 24/08 — falha silenciosa no cliente da API-Football

**Data:** 2026-09-10 · **Status:** causa raiz identificada e corrigida no código
**Não é item da auditoria original.** Apareceu ao investigar por que a Champions League
não carregava.

### Sintoma

`sync_runs` mostrava o mesmo padrão em cinco ciclos seguidos:

| ciclo | alvos | achadas | gravadas | checadas | finalizadas | erros |
|---|---|---|---|---|---|---|
| 07/09 14:00 | 11 | 0 | 0 | 50 | 0 | 50 |
| 07/09 12:59 | 11 | 0 | 0 | 50 | 0 | 50 |
| 30/08 11:56 | 11 | 0 | 0 | 50 | 0 | 50 |
| 24/08 01:05 | 11 | 0 | 0 | 50 | 0 | 50 |
| 23/08 20:33 | 11 | 0 | 0 | 50 | 5 | 45 |
| **02/08 02:48** | 11 | **3.456** | **3.456** | 50 | 44 | **0** |

### O que a evidência mostrou — e o que ela derrubou

A hipótese óbvia era cota estourada. **Errada.** `api_usage_log` nos dias 24/08 e 30/08:

| dia | endpoint | success | status | chamadas |
|---|---|---|---|---|
| 30/08 | `fixtures.lookup` | **true** | **200** | 50 |
| 30/08 | `fixtures` | **true** | **200** | 11 |
| 24/08 | `fixtures.lookup` | **true** | **200** | 50 |
| 24/08 | `fixtures` | **true** | **200** | 11 |

A API respondeu **200 OK em todas as chamadas** e o worker mesmo assim gravou zero.
(Em 23/08 sim houve 45 respostas 429 — rate limit real, problema diferente e já
tratado com throttle.)

### Causa raiz

**Status 200 não significa sucesso nesta API.** A API-Football devolve HTTP 200 mesmo
quando recusa a requisição, colocando o motivo num campo `errors` **dentro do corpo**,
com `response` vazio:

```json
{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}
```

`Client.doGet` checava **apenas o status HTTP**. Consequências, ambas batendo com os
números observados:

- `/fixtures?league=X&season=Y` → 200 + `errors` → `response` vazio →
  `SyncFixtures` devolvia **0 partidas e nenhum erro** → `achadas=0`, sem erro registrado
- `/fixtures?id=N` → 200 + `errors` → `response` vazio →
  `"partida não encontrada na API-Football"` → `checadas=50, erros=50`

**Por que ficou duas semanas sem ninguém notar:** o `api_usage_log` registrava
`success=true, status=200`, e a tela de diagnóstico mostrava tudo verde. Um pipeline que
falha em silêncio é pior do que um que quebra — ninguém investiga o que parece estar
funcionando. E a mensagem da API, que dizia exatamente qual era o problema, era
descartada antes de chegar a qualquer log.

### Correção

`internal/integration/sportsdata/apifootball/client.go`:

- `apiErrorMessage()` lê o campo `errors` do corpo. Ele é **polimórfico** — array vazio
  em caso de sucesso, objeto com o motivo em caso de recusa —, então é lido como
  `RawMessage` e interpretado. Formato inesperado é devolvido cru: preferir ruído a
  silêncio é a lição do próprio incidente.
- `doGet` passa a tratar 200-com-`errors` como erro, **carregando a mensagem original da
  API**. Na próxima execução o log dirá o motivo exato da recusa.
- Recusa por ritmo/cota que chega como 200 entra no mesmo backoff do 429
  (`isRateLimitMessage`).
- `baseURLOverride` para os testes apontarem a um servidor local.

**Testes** (`client_test.go`, 8 casos): recusa com 200 vira erro; `SyncFixtures` não
devolve mais vazio silencioso; `SyncFixtureStatistics` deixa de reportar recusa de plano
como "partida não encontrada"; resposta legítima com zero jogos **continua não sendo
erro**; status HTTP de erro continua erro; mensagem estável com múltiplos motivos.

### O que a correção NÃO faz, e o que ficou em aberto

**Ela não faz o pipeline voltar a sincronizar.** Ela faz o pipeline *dizer por que* não
sincroniza. A mensagem que a API vinha mandando foi descartada em todas as execuções, e
sem ela não é possível afirmar a causa da recusa — a suspeita mais provável é a
temporada pedida não estar coberta pelo plano (as ligas europeias estão com `MAX(year) =
2025` e as sul-americanas com `2026`), mas **isso é hipótese, não conclusão**. O próximo
ciclo depois do deploy resolve a dúvida.

**Ponto sem explicação, registrado como aberto:** nos ciclos de 07/09 o `sync_runs`
marca `checadas=50, erros=50`, mas o `api_usage_log` **não tem nenhuma linha naquela
data** (a retenção é de 90 dias, então não é limpeza). Verifiquei `provider_incidents`
para testar a hipótese de o disjuntor de saúde ter suspendido o provedor: a tabela está
**vazia**, o disjuntor nunca disparou. Não tenho explicação sustentada por dado para
esses dois ciclos e não vou inventar uma.

---

## Fonte de odds reais — o que a API-Football consegue e o que não consegue

**Data:** 2026-09-09 · **Status:** investigado, decisão pendente do usuário
**Contexto:** é a pendência que trava o AUD-001 e o AUD-003 de terem efeito prático.

### O achado que muda o planejamento

Da documentação oficial do endpoint `/odds` (Odds Pre-Match), citação direta:

> *"We provide pre-match odds between 1 and 14 days before the fixture."*
> *"We keep a 7-days history (The availability of odds may vary according to the
> leagues, seasons, fixtures and bookmakers)."*

**Não existe arquivo histórico de odds.** A consequência é dura e precisa estar clara
antes de qualquer estimativa de prazo:

1. **As 3.547 partidas já no banco nunca terão odd real.** Não é questão de plano pago
   ou de esforço de implementação — o dado não existe mais do lado do provedor.
2. **O histórico de odds reais começa do zero** no dia em que a coleta entrar no ar, e
   cresce uma rodada por vez.
3. **Até acumular 100 jogos por combinação** (mínimo do doc 08), o Discovery continua
   publicando zero. Corretamente.

Ou seja: integrar odds não é uma tarefa que "destrava" o Discovery em uma semana. É
ligar uma coleta e esperar uma temporada.

### O que ainda não sei, e por que não chutei

Três coisas não se respondem lendo documentação, e nenhuma delas eu vou afirmar sem
medir:

- **Existe mercado de escanteios na API?** Se só houver 1X2 e over/under de gols, a
  integração não resolve o AUD-001 — precificaria gols, não escanteios, que é a
  métrica central do produto. A lista de mercados é devolvida em tempo de execução
  (`/odds/bets`), não está publicada na documentação.
- **A cota do plano aguenta?** Odds vêm paginadas de 10 em 10.
- **Qual casa usar?** Odd de casa diferente muda o break-even e, portanto, o p-valor
  do teste de significância do AUD-003.

### Ferramenta criada: `cmd/oddsprobe`

Responde as três com dados, contra a API real, usando a chave já configurada.
**Só lê — não grava nada, em lugar nenhum.**

```
go run ./cmd/oddsprobe
go run ./cmd/oddsprobe -league 71 -season 2026
```

Verifica plano e cota, procura mercados de escanteios, lista as casas e busca uma
amostra real de odds, imprimindo a leitura de cada resultado.

### Ferramenta criada: `cmd/findleague`

Não é sobre odds, mas nasceu do mesmo problema — depender de um número que ninguém
confirmou. Resolve o `external_id` de um campeonato perguntando ao provedor, em vez
de procurar no painel ou chutar:

```
go run ./cmd/findleague -name "Europa League" -season 2025
```

Chutar id é pior do que parece: um id errado não deixa a liga vazia, ele carrega
dados de **outra** competição sob o nome que você escolheu, e ninguém percebe.

### Defeito encontrado durante esta investigação (corrigido)

`SyncRepo.UpsertMatch` gravava odd sintética **sem rótulo**: linha nova caía no
`DEFAULT 'unknown'`, descrevendo como "origem desconhecida" algo de origem
perfeitamente conhecida. Pior, no `ON CONFLICT` ele sobrescreveria odd **real** com
sintética mantendo o rótulo `'real'` — lavando dado fabricado como se fosse de
mercado, exatamente o que o AUD-001 existe para impedir.

A migration 013 marcou os dados que **já existiam**; o **produtor** continuava sem
correção. Agora grava `'synthetic'` explicitamente e nunca sobrescreve odd marcada
como `'real'`.

---

## AUD-001 — Odds sintéticas derivadas da própria amostra causam leakage

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Estatística / Backtest
**Status:** RESOLVIDO no código · migration `013` **não aplicada** (aguarda aprovação — ver Fase D)
**Data:** 2026-09-09

### Fase A — Investigação

**Hipótese da auditoria:** a odd usada no backtest não é de mercado; é derivada da média de
escanteios do próprio lote que depois será testado.

**Confirmada.** Rastreamento completo de quem escreve `matches.corner_odds`:

| Rota | Arquivo | Escreve odd? | Origem |
|------|---------|--------------|--------|
| Worker de sincronização (statsync) | `repository/postgres/statsync_repo.go:17` | **Não** | — o cabeçalho do arquivo declara explicitamente que nenhuma query ali toca `corner_odds` |
| `cmd/sync` | `repository/postgres/sync_repo.go:68,75` (`corner_odds = EXCLUDED.corner_odds`) | Sim | `usecase.SyntheticCornerOdds(batchMu)` |
| `cmd/seed` | idem | Sim | `usecase.SyntheticCornerOdds(batchMu)` |

`sync_usecase.go:69-82` calcula `batchMu` como a média de escanteios **de todo o lote** e
aplica **um único mapa de odds a todas as fixtures** desse lote. `oddsgen.go` deriva a odd
por aproximação normal (desvio 3.0, margem 1.08) sobre essa média.

Conclusão: **nenhuma odd real existe em nenhum ponto do sistema.** Não há fonte de mercado
integrada. Toda odd em produção é derivada da amostra.

**Evidência em produção (consulta de leitura, 2026-09-09):**

| Métrica | Valor |
|---------|-------|
| Partidas totais | 3.547 |
| Partidas com `corner_odds` preenchida | 2.558 |
| Partidas com odd de mercado | **0** |
| Estratégias `origin='discovery'` | 5 (todas `active = true`) |

Somado à evidência já registrada na auditoria — 5 ligas europeias com **exatamente 1 odd
distinta** para a linha 8.5, e Brasileirão Série A com **51/51 (100%)** de acerto entre os
jogos com odd ≤ 2,20 — o mecanismo do artefato fica explícito: filtrar por `MaxOdds` não
seleciona jogos com preço favorável, seleciona **lotes de média alta**. O acerto alto sai
por construção.

### Fase B — Implementação

Cinco mudanças, todas aditivas. Nenhum dado histórico apagado.

**1. `backend/migrations/013_odds_source.sql`** *(criada, NÃO aplicada)*
- `matches.odds_source TEXT NOT NULL DEFAULT 'unknown'` + CHECK `('real','synthetic','unknown')`
- Backfill: todo `corner_odds` não vazio → `'synthetic'` (justificado pelo rastreamento da Fase A)
- Índice parcial `idx_matches_real_odds ... WHERE odds_source = 'real'` — a consulta do
  Discovery precisa ser barata mesmo quando (como hoje) o resultado é vazio
- `UPDATE strategies SET active = false WHERE origin='discovery' AND active=true` —
  **despublica sem apagar**: linha, backtests, scores e health permanecem para auditoria

**2. `internal/domain/entities.go`** — campo `OddsSource`, constantes
`OddsSourceReal|Synthetic|Unknown`, método `HasRealOdds()`.

**3. `internal/repository/postgres/match_repo.go`** — `AllMatches` passa a ler
`odds_source` e `created_at`.

**4. `internal/usecase/filter_usecase.go`**
- `FilterCriteria.RequireRealOdds bool` — `json:"-"`, **não exposto ao usuário**: é ligado
  internamente por quem valida estratégia, não é um filtro de tela
- Ramo de escanteios: descarta a partida quando `RequireRealOdds && !HasRealOdds()`
- `BacktestResult.OddsSource` + `BacktestResult.FinancialsReliable`, preenchidos por
  `oddsSourceSummary()`: só é `true` quando **100%** das odds usadas são reais — uma única
  sintética no lote e o conjunto vira cenário hipotético
- **AUD-012 corrigido junto** (mesma linha de código, separá-lo exigiria tocar o ramo duas
  vezes): partida sem odd deixa de entrar com `odd = 1.0`. Fabricar 1.0 produzia P/L = 0 no
  acerto e −stake no erro — prejuízo estrutural sem significado financeiro

**5. Discovery e reavaliação passam a exigir odd real**
- `internal/usecase/discovery/engine.go:250` — `RequireRealOdds: true` na mineração
- `internal/usecase/strategyengine/engine.go:86` — `RequireRealOdds: true` na reavaliação
  de estratégia publicada (reavaliar é validação quantitativa, mesma exigência)

**6. Frontend — o Simulador continua rodando, mas diz o que está mostrando**
- `core/models.ts` — `odds_source` e `financials_reliable` no `BacktestResult`
- `features/filters/filters.component.html` — aviso âmbar quando `odds_source === 'synthetic'`:
  as odds foram estimadas do próprio histórico, ROI/yield/lucro são cenário e não desempenho
  observado

### Fase C — Testes

`backend/internal/usecase/filter_odds_source_test.go` (novo, 7 testes):

| Teste | Garante |
|-------|---------|
| `TestRequireRealOdds_RejeitaSintetica` | SYNTHETIC → rejeitado pelo Discovery (`MatchCount = 0`) |
| `TestRequireRealOdds_RejeitaUnknown` | UNKNOWN e string vazia (linha pré-013) → rejeitados |
| `TestRequireRealOdds_PermiteReal` | REAL → aceito, `OddsSource='real'`, `FinancialsReliable=true` |
| `TestOddsSourceSummary_MisturaNaoEConfiavel` | lote misto real+sintético → não confiável |
| `TestSimulador_AceitaSinteticaMasMarcaResultado` | Simulador roda, mas resultado sai marcado |
| `TestSemOdd_PartidaForaDoBacktest` | AUD-012 — sem odd, fora do backtest; financeiro zerado |
| `TestMetricaComOddFixa_NaoEConfiavel` | odd fixa do usuário nunca é medida de mercado |

**Resultado:**

```
go build ./...                    OK
go vet ./...                      OK (sem apontamentos)
gofmt -l internal cmd pkg         OK (nenhum arquivo)
go test ./...                     ok  formulas, usecase, aidocs, analytics, discovery, strategyengine
npx tsc --noEmit                  OK
npx ng build --configuration production   OK (só warnings pré-existentes)
```

Nenhum teste existente foi alterado para passar.

### Fase D — Validação

**Efeito medido sobre a produção atual, se a migration for aplicada:**

| Item | Antes | Depois |
|------|-------|--------|
| Partidas marcadas `synthetic` | — | 2.558 |
| Partidas marcadas `unknown` | — | 989 |
| Partidas marcadas `real` | — | **0** |
| Estratégias de descoberta ativas | 5 | **0** |
| Estratégias de descoberta preservadas no banco | 5 | 5 |
| Estratégias que o ciclo publicará | — | **0**, até existir odd real |

**Este é o resultado correto, não uma falha.** Foi previsto na auditoria e autorizado
explicitamente na instrução de correção ("se encontrar 0 estratégias, isso também pode ser
um resultado válido"). As 5 estratégias publicadas hoje têm 100% de acerto e drawdown 0,00
porque foram mineradas sobre odd derivada da própria amostra — mantê-las no ar seria
apresentar artefato como oportunidade.

**O que NÃO foi feito, e por quê:**
- **A migration 013 não foi aplicada em produção.** Ela altera a tabela `strategies`
  (despublicação). Alteração com efeito visível ao usuário exige aprovação explícita —
  fica pendente.
- **Não foi criada nenhuma odd real.** Fabricar odd para "fazer o sistema funcionar" é
  exatamente o defeito auditado, em outra roupagem.
- **Nenhum critério foi relaxado** para compensar o resultado zero.

**Pendência de produto que esta correção expõe (não é bug, é falta de insumo):** o CornerLab
não tem fonte de odds de mercado integrada. Enquanto não tiver, o Discovery é estruturalmente
incapaz de validar estratégia financeira, e o Simulador só pode operar como ferramenta de
cenário. Isso precisa entrar no backlog como item próprio.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-001 e AUD-012 marcados
- Arquivos alterados:
  - `backend/migrations/013_odds_source.sql` *(novo, não aplicado)*
  - `backend/internal/domain/entities.go`
  - `backend/internal/repository/postgres/match_repo.go`
  - `backend/internal/usecase/filter_usecase.go`
  - `backend/internal/usecase/discovery/engine.go`
  - `backend/internal/usecase/strategyengine/engine.go`
  - `backend/internal/usecase/filter_odds_source_test.go` *(novo)*
  - `frontend/src/app/core/models.ts`
  - `frontend/src/app/features/filters/filters.component.html`

**Ações pendentes do usuário:**
1. Aprovar e aplicar a migration `013_odds_source.sql` em produção (despublica 5 estratégias)
2. Decidir a fonte de odds reais antes de esperar qualquer publicação do Discovery

---

## AUD-002 — ROI, Yield e EV são o mesmo número; DSFR conta 50% em uma só variável

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Matemática / Scores
**Status:** RESOLVIDO — Formula Catalog **v1.1**
**Data:** 2026-09-09

### Fase A — Investigação

**Hipótese da auditoria confirmada, e a causa é mais estrutural do que o relatório sugeria.**

O relatório descrevia `result.Yield = round2(roi)` como se fosse uma atribuição
equivocada. Não é um deslize: **sob a mecânica atual do motor, ROI e Yield são a mesma
quantidade por definição.** O backtest aposta stake constante e liquida toda entrada,
então "investimento" (Catálogo 07) e "volume apostado" (Catálogo 08) são o mesmo
denominador. Escrever `Yield = ROI` é o cálculo correto — o erro está em depois tratá-los
como duas evidências.

O EV é outra história: **nunca foi calculado.** `strategyengine/engine.go` gravava
`EV: ptr(r.Yield)` — o campo recebia o ROI emprestado.

**Mapa da concentração no DSFR v1.0** (`engine.go:229-238`, pesos de `formulas/scores.go`):

| Slot | Valor recebido | Peso |
|------|----------------|------|
| ROI | `roiNorm` = ROI/20 | 20% |
| EV | `yieldNorm` = ROI/15 | 20% |
| Yield | `yieldNorm` — **a mesma variável Go, de novo** | 10% |

**50% do score em uma quantidade só**, normalizada por dois tetos diferentes para
disfarçar. Verificado executando a v1.0 preservada:
`DSFRScore{ROI:1, EV:1, Yield:1} = 50,00` (teste `TestDSFRV10_RetornoValia50PorCento`).

**Mesmo defeito em mais três lugares, todos dentro do escopo do AUD-002:**

| Local | Colapso |
|-------|---------|
| `RankingScore` | ROI 20% + Yield 10% sobre a mesma quantidade — em cima de um DSFR (35%) que já era 50% dela |
| `healthRow` | `ΔEV = (Yield − prevYield)/yieldCapPct` = o próprio ΔROI reescalado → metade da "saúde" era uma variável |
| `discovery/criteria.go` | Gates de ROI (≥10%), Yield (≥5%) e "EV" (>0) incidem todos sobre lucro/volume. Com os limiares padrão **só o de ROI chega a barrar** — os outros dois nunca são atingidos. A regra do doc 08 ("nunca considerar apenas Win Rate; sempre múltiplos indicadores") estava sendo satisfeita com três cópias de um indicador |

**Por que os testes existentes não pegaram:** `TestScoresRow` e `TestHealthRow*` são
direcionais ("forte > fraca") e passam identicamente antes e depois. Nenhum afirmava peso.

### Fase B — Implementação

**Decisão: opção (b) da auditoria — remover EV e Yield do DSFR e reponderar.**
A opção (a) foi descartada, e o motivo importa:

- **Yield distinto de ROI** exige stake variável por entrada. O motor não tem política de
  staking. Inventar uma só para diferenciar as métricas seria fabricar dado.
- **EV distinto** exige P(vitória) de um modelo independente, estimado **fora da amostra**.
  Com a taxa de acerto do próprio lote e a odd média dele, o EV colapsa algebricamente no
  ROI realizado — o mesmo número com outro nome. Estimativa fora da amostra é o AUD-003 e
  ainda não existe.

**1. `internal/formulas` — Catálogo v1.1** (`scores.go`, `doc.go`)

Novas funções, com as da v1.0 **preservadas e testadas**: linhas de `backtests` gravadas
antes carregam `algorithm_version = "1.0"` e só podem ser reproduzidas pela fórmula da
época. Score de versões diferentes não é comparável.

| Fórmula | v1.0 | v1.1 |
|---------|------|------|
| `DSFRScoreV11` | ROI 20 · EV 20 · WR 15 · Yield 10 · DD 10 · Amostra 10 · Consist. 10 · Var. 5 | **ROI 30 · WR 20 · DD 15 · Amostra 15 · Consist. 15 · Var. 5** |
| `RankingScoreV11` | DSFR 35 · Health 25 · ROI 20 · Yield 10 · Conf. 10 | **DSFR 40 · Health 25 · ROI 20 · Conf. 15** |
| `HealthScoreV11` | média de 4 deltas (ΔROI, ΔEV, ΔDD, ΔCons) | **média de 3** (ΔEV era ΔROI) |

Os 30 pontos liberados por EV e Yield **não voltaram para o ROI** — isso apenas
reconcentraria a mesma quantidade. Foram distribuídos entre as dimensões restantes.
Exposição total ao retorno: **50% → 30%**.

**2. `strategyengine/engine.go`**
- `EV: nil` — o campo passa a sair NULO. "Não calculado" é a informação verdadeira; um
  número emprestado de outra métrica não é.
- `scoresRow` e `healthRow` usam as funções v1.1
- `components` e `variation` (JSON exibido na tela de detalhe) perdem as chaves `ev` e
  `yield`: não eram componentes, eram o `roiNorm` reescalado

**3. `discovery/criteria.go`** — `rejectEV` (`"ev_nao_positivo"`) renomeado para
`rejectNonPositiveProfit` (`"lucro_nao_positivo"`). **Comportamento idêntico** — a condição
`r.Yield <= 0` continua exatamente igual. Os gates de ROI e Yield **não foram removidos**:
removê-los afrouxaria a validação em configurações não padrão (`MinYield` é configurável).
O que muda é o rótulo deixar de mentir, e um comentário registrando quais são os
indicadores realmente independentes da função.

**4. Propagação da verdade para fora do backend**
- `usecase/aidocs` (documento vivo lido por outra IA): tabela de pesos atualizada e um
  aviso explícito de que ROI e Yield são o mesmo número e o EV vem nulo
- `handlers/discovery_handler.go`: `EV float64` → `EV *float64`. `derefFloat(nil)` produzia
  `0.0`, e a tela exibiria "EV 0%" como se fosse medição
- `frontend/core/models.ts`: `ev: number | null`

**Nenhuma migration.** `strategy_scores` e `strategy_health` são upsert por estratégia —
o próximo ciclo do worker regrava tudo em v1.1 sozinho. `backtests` é histórico
append-only e suas linhas v1.0 estão **corretas para a versão que declaram**; reescrevê-las
seria alterar dado histórico.

### Fase C — Testes

`internal/usecase/strategyengine/scores_weights_test.go` (novo, 10 testes). O teste
obrigatório da auditoria feito **nos dois sentidos**, porque só o par flagra a regressão:

| Teste | Garante |
|-------|---------|
| `TestDSFR_RetornoNaoPodeUltrapassarSeuPesoDeclarado` | variar só o retorno move o DSFR exatamente 30,00 pontos (= peso declarado). Sob a v1.0 movia ~50 |
| `TestDSFR_ComRetornoFixoOsDemaisComponentesMovemOScore` | com retorno fixo, os demais componentes movem o score — e dentro do teto de 70 |
| `TestDSFRV11_PesosSomamUmECadaSlotValeOQueDeclara` | cada slot isolado devolve o próprio peso; pesos somam 1 |
| `TestDSFRV10_PreservadaParaHistorico` | v1.0 continua reproduzindo 20/20/10 |
| `TestDSFRV10_RetornoValia50PorCento` | prova executável do defeito original |
| `TestRankingV11_SemYieldEPesosSomamUm` | ranking sem slot de yield |
| `TestHealthV11_TresDeltasNaoQuatro` | `HealthScoreV11(0,−1,0)` = 50 + 50/3, não 50 + 50/4 |
| `TestBacktestRow_EVFicaNulo` | EV nulo; ROI e Yield seguem gravados |
| `TestScoresRow_ComponentesSemEVeYield` | JSON de componentes com 6 chaves, sem `ev`/`yield` |
| `TestHealthRow_VariacaoSemChaveEV` | JSON de variação com 3 deltas |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  formulas, usecase, aidocs, analytics,
                                              discovery, strategyengine  (25 testes)
npx ng build --configuration production    OK
```

Nenhum teste existente foi alterado. Os cinco de `engine_test.go` continuam passando sem
tocar em nada — o que confirma o diagnóstico da Fase A sobre a fraqueza deles.

### Fase D — Validação

Recalculei o DSFR das **5 estratégias reais em produção** a partir dos componentes gravados
em `strategy_scores`, usando as duas versões da fórmula. A v1.0 reimplementada reproduz o
valor gravado em todas (divergência máxima de 0,02, do arredondamento dos componentes a 3
casas no JSON) — o que valida o modelo antes de comparar.

| id | Estratégia | DSFR v1.0 | DSFR v1.1 | Δ |
|----|-----------|-----------|-----------|---|
| 3 | Escanteios 8.5+ · odd ≤ 2,20 | 100,00 | 100,00 | 0,00 |
| 4 | Escanteios 7.5+ · odd ≤ 2,20 | 99,04 | 98,56 | −0,48 |
| 5 | Escanteios 6.5+ · odd ≤ 2,20 | 95,29 | 92,95 | −2,34 |
| 6 | Escanteios 7.5+ · odd ≤ 1,70 | 84,17 | **87,10** | **+2,93** |
| 7 | Escanteios 8.5+ · odd ≤ 3,50 | 76,22 | 74,19 | −2,03 |

**Leitura honesta destes números — três observações, uma delas incômoda:**

1. **A correção re-rankeia, não rebaixa.** A estratégia 6 **subiu** 2,93. Seu retorno era
   fraco (`roi_norm` 0,57) mas todos os demais componentes eram 1,0; a v1.0 lhe dava peso
   extra por retorno através dos slots de EV/Yield (que usam teto 15 em vez de 20, inflando
   o valor), enquanto a v1.1 lhe dá peso pelos componentes que ela de fato tem. Quem
   ganha e quem perde depende de onde estava o mérito.

2. **A ordem não mudou aqui, e isso não é reconforto.** Os cinco casos têm
   `sample = consistency = inv_drawdown = 1,0`, ou seja, quase não há variação nos
   componentes não-retorno — justamente porque são os artefatos do AUD-001 (win rate 100%).
   Com uma carteira real e diversa, a reordenação seria maior. **Esta amostra é pequena
   demais para concluir que o impacto no ranking é pequeno.**

3. **O efeito prático imediato é nulo, por outro motivo:** essas 5 estratégias serão
   despublicadas pelo AUD-001 assim que a migration 013 for aplicada. Todas continuam acima
   do gate `MinDSFR = 40` sob a v1.1, então **a correção do AUD-002 sozinha não elimina
   nenhuma estratégia** — ela não foi feita para isso.

**O que esta correção NÃO faz, e precisa ficar claro:** o DSFR v1.1 ainda **não tem seis
dimensões independentes**. `InvVariance = 1 − 4p(1−p)` é função pura do win rate, e
`Consistency` é uma composição dos outros quatro componentes. Isso é o **AUD-007** (P1) e
foi deliberadamente deixado em aberto — corrigir dois problemas na mesma passagem
impediria atribuir efeito a causa. O comentário no topo da v1.1 em `scores.go` registra a
pendência para quem ler a fórmula antes de eu chegar lá.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-002 marcado
- Arquivos alterados:
  - `backend/internal/formulas/scores.go` (v1.1; v1.0 preservada)
  - `backend/internal/formulas/doc.go` (`Version = "1.1"` + histórico)
  - `backend/internal/usecase/strategyengine/engine.go`
  - `backend/internal/usecase/discovery/criteria.go`
  - `backend/internal/usecase/aidocs/aidocs.go`
  - `backend/internal/delivery/http/handlers/discovery_handler.go`
  - `backend/internal/usecase/strategyengine/scores_weights_test.go` *(novo)*
  - `frontend/src/app/core/models.ts`
- Limpeza: removidos ~50 arquivos `*.go.<número>` (lixo de edição do mount Windows) que
  poluíam o repositório. Nenhum era código-fonte.

**Ações pendentes do usuário:** nenhuma para o AUD-002. Os scores em produção migram
sozinhos para v1.1 no próximo ciclo do worker.

---

## AUD-003 — Múltiplas comparações sem correção nem validação fora da amostra

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Estatística
**Status:** RESOLVIDO
**Data:** 2026-09-09

### Fase A — Investigação

**Confirmado, sem atenuantes.** `generateCombos` monta 540 combinações por liga
(5 linhas × 3 mandos × 3 janelas × 4 tiers × 3 tetos de odd); com `IncludeTeams`, mais
45 por equipe. `mine` roda backtest de cada uma **contra o mesmo histórico**, aplica
limiares fixos e `publish` fica com as melhores por DSFR.

Não existe no código: nenhum split treino/teste, nenhum registro do número de testes,
nenhuma correção de limiar por multiplicidade. `FilterCriteria` não tinha sequer como
expressar uma janela temporal — só `maxAgeDays`, que é relativo a `time.Now()` e
portanto muda de significado a cada execução.

**Por que isso é fatal e não apenas imperfeito.** Testar muitas hipóteses contra um
único conjunto de dados e ficar com as vencedoras é o procedimento que *fabrica*
falsos positivos. Com 5.940 testes a 5% de significância, ~297 combinações "aprovam"
mesmo que nenhuma tenha vantagem alguma. O desenho não tinha como distinguir vantagem
de ruído — aprovar sorte era o comportamento esperado dele.

**Um agravante que o relatório não citava:** o critério de acerto (`MinWinRate = 75%`)
não é comparado com nada. Uma linha de escanteios baixa acerta 90% e paga odd 1,05 —
prejuízo garantido. "Taxa de acerto alta" só significa alguma coisa contra a
probabilidade que a odd já embutia.

### Fase B — Implementação

Três mecanismos. Os três são obrigatórios e nenhum é desligável por configuração.

**1. Janela temporal no motor de backtest** — `usecase/filter_usecase.go`

`FilterCriteria.DateFrom` / `DateTo`, intervalo **meio-aberto `[From, To)`**, `json:"-"`
(interno, o Simulador não muda de comportamento). Meio-aberto de propósito: descoberta
e validação usam a mesma data de corte nas duas pontas, e um intervalo fechado dos dois
lados colocaria as partidas do dia do corte nos dois conjuntos. Aplicado **antes** de
`LastNGames`, para que "últimos N jogos" signifique "os N mais recentes dentro da
janela" — se fosse depois, a janela de descoberta poderia selecionar jogos que só
existem porque a de validação foi lida.

**2. Corte temporal do histórico** — `usecase/discovery/validation.go` (novo)

`splitHistory` divide por **data**, no quantil da *contagem* de partidas (70/30). Não
por calendário: ligas têm pausas e temporadas de tamanhos diferentes, e cortar "na
metade do tempo" desbalancearia as amostras. Todas as partidas da data de corte vão
para a validação. Quando o histórico não permite um corte com significado (menos de 2
partidas, ou todas no mesmo dia), a liga **não publica nada** — motivo registrado
`liga_sem_janela_de_validacao`. Sem janela de validação não há como saber se o padrão
sobrevive fora da amostra, e publicar sem saber é o defeito original.

**3. Significância corrigida por número de testes** — `formulas/significance.go` (novo)

- `BinomialAtLeast(n, k, p)` — p-valor unilateral exato, somado em espaço logarítmico
  (log-gama). Não aproximação normal: com n de algumas dezenas e p perto de 1, a
  normal erra justamente na cauda que interessa.
- A hipótese nula é a **probabilidade implícita na odd** (1/odd, Catálogo 02), média
  sobre as entradas do backtest. A pergunta deixa de ser "acerta muito?" e passa a ser
  "acerta mais do que a odd já pagava?".
- `FDRThreshold(pvalues, q, método)` — Benjamini–Hochberg e Benjamini–Yekutieli.
  **Padrão: Benjamini–Yekutieli**, que vale sob dependência arbitrária. As combinações
  compartilham fortemente as mesmas partidas ("8.5+ casa e fora" e "8.5+ mandante" são
  o mesmo jogo visto duas vezes), então a hipótese de dependência positiva do BH não é
  demonstrável aqui; com estrutura de dependência desconhecida, o procedimento
  conservador é a escolha honesta — erra publicando de menos.
- Limiar 0 significa **"não publique nada"**, nunca "sem limiar". Explicitado no código
  porque tratar 0 como ausência de limiar publicaria tudo.

**4. Reteste fora da amostra** — `Engine.validate`

Cada sobrevivente é reexecutado na janela de validação. Exigências: mínimo de 30
ocorrências, lucro positivo e p-valor ≤ 0,05. Os critérios **não** são os do doc 08, e
isso é deliberado: a janela de validação é menor por construção, e exigir os mesmos 100
jogos reprovaria tudo por tamanho de amostra — a validação viraria teatro. O que se
exige aqui é consistência. Sem correção de multiplicidade nesta etapa, porque o conjunto
de candidatos já foi fixado *antes* de olhar esses dados, que é exatamente a condição
que a correção existe para restaurar.

**Duas decisões que fecham portas dos fundos:**

- `withDefaults` **não deixa desligar as travas**: `TrainFraction = 1.0` (minerar tudo)
  ou `FDRq = 1.0` (aceitar qualquer p-valor) voltam ao padrão em vez de valer. Tornar
  essas duas configuráveis seria devolver o defeito por outro caminho.
- **Ciclo interrompido não publica nada.** Antes, `mine` devolvia o que já tinha
  minerado ao receber shutdown. A correção de multiplicidade só é válida sobre a
  varredura completa — corrigir sobre um prefixo usaria um *m* menor que o real e
  afrouxaria o limiar.

**5. O usuário passa a ver a evidência** — `describeValidation`

A descrição de cada descoberta publicada traz o período em que o padrão foi procurado,
o período em que se sustentou, e as duas probabilidades de o resultado ter saído por
acaso. Um número de acerto sem "quantas hipóteses foram testadas para chegar nele"
comunica mais confiança do que a evidência sustenta.

**6. Observabilidade** — `LeagueResult` ganha `TrainUntil`, `HoldoutFrom`, `Tested`,
`FDRThreshold`, `Significant`, `Validated`. "Publiquei 3" não significa nada sem "de
quantos testes" e "validadas contra qual período".

**7. Documento vivo** (`usecase/aidocs`) — seção nova explicando as três barreiras, com
os parâmetros lidos das constantes reais.

**Nenhuma migration.** A correção é toda de pipeline; não altera schema nem dado
histórico.

### Fase C — Testes

`formulas/significance_test.go` (8 testes) e `usecase/discovery/outofsample_test.go`
(9 testes, ciclo completo com repositórios falsos).

**O teste obrigatório da auditoria — e o erro que quase o tornou inútil.**

A primeira versão do teste de ruído passava, mas eu instrumentei antes de confiar nela.
Diagnóstico: `tested = 0`, `rejeições = {amostra_insuficiente: 400, win_rate_baixo: 140}`.
**O ruído era barrado pelos critérios do doc 08 e nunca chegava ao mecanismo do AUD-003.**
O teste teria passado com a correção inteira revertida — provaria que o `MinWinRate` de
75% funciona, não que a correção funciona.

Duas mudanças resolveram:

- **Odds justas.** O ruído usava odd 2,00 fixa em todas as linhas, o que cria vantagem
  ou desvantagem *real* — nesse caso o motor estaria certo em achar padrão. Agora as
  odds são as exatamente justas da distribuição geradora (1 ÷ P(total > linha),
  calculada analiticamente, **não** estimada da amostra — estimá-la seria reintroduzir
  o AUD-001 dentro do teste). Assim toda aposta tem valor esperado exatamente zero e o
  que sobra é só variação amostral.
- **Limiares do doc 08 permissivos** no cenário de teste, para que o ruído alcance a
  correção. Não é "afrouxar filtro para gerar oportunidade": as travas do AUD-003
  continuam nos valores de produção, e `DefaultCriteria()` não foi tocada. É isolar o
  mecanismo sob teste.
- **Guarda anti-vácuo permanente:** `if res.Tested == 0 { t.Fatalf(...) }`. Se algum dia
  o ruído voltar a ser barrado antes da correção, o teste falha em vez de passar sem
  provar nada.

| Teste | Garante |
|-------|---------|
| `TestCicloSobreRuidoPuroNaoPublicaNada` | 5 sementes × 700 partidas de ruído justo → **0 publicadas**, com `Tested > 0` |
| `TestSemCorrecaoORuidoProduziriaAprovacoes` | contraprova: o mesmo ruído teria aprovado **464 combinações** antes do AUD-003 |
| `TestJanelasNaoSeSobrepoem` | nenhuma partida nas duas janelas; nenhuma perdida; fração ~70% |
| `TestSplitRecusaHistoricoInsuficiente` | vazio, 1 partida e "tudo no mesmo dia" não são divisíveis |
| `TestLigaSemJanelaDeValidacaoNaoPublica` | publica 0 e registra o motivo |
| `TestConfiguracaoNaoDesligaValidacao` | `TrainFraction=1`, `FDRq=1`, `alpha=1` voltam ao padrão |
| `TestPValorNaoPremiaAcertoAltoComOddBaixa` | 90% de acerto em odd 1,11 → p ≈ 0,4 (não significativo); 65% em odd 2,00 → p < 0,01 |
| `TestCheckHoldoutReprovaCadaMotivo` | cada motivo de reprovação fora da amostra |
| `TestFDRThreshold_ApertaComMaisTestes` | p = 0,004 sobrevive entre 3 testes, **não** sobrevive entre 1.000 |
| `TestFDRThreshold_NadaSignificativoDevolveZero` | limiar 0 não deixa passar nada |
| `TestBinomialAtLeast_*` | valores conhecidos, monotonicidade, entradas inválidas |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  todos os pacotes (42 testes)
```

Nenhum teste existente foi alterado.

### Fase D — Validação

**Quanto o limiar aperta com o tamanho do espaço de busca** (q = 0,10, Benjamini–Yekutieli):

| Combinações testadas | H(m) | Limiar máximo |
|---|---|---|
| 100 | 5,19 | 0,0193 |
| 540 (uma liga) | 6,87 | 0,0146 |
| 1.000 | 7,49 | 0,0134 |
| 5.940 (ciclo completo) | 9,27 | **0,0108** |

**O que isso exige na prática**, para uma estratégia em odd 2,00 (break-even 50%),
dentro de um ciclo de 5.940 testes:

| Amostra | Acerto necessário para ser significativa |
|---|---|
| 50 jogos | 68,0% |
| 100 jogos | 62,0% |
| 200 jogos | 58,5% |
| 380 jogos | 56,1% |

Amostra maior compra significância mais barato — que é o incentivo correto, e o oposto
do que o motor fazia antes (onde combos de amostra pequena eram os mais fáceis de
"aprovar" por sorte).

**Efeito sobre produção: nenhum efeito observável hoje, por um motivo que não é bom.**
Com o AUD-001, não existe nenhuma odd real no banco (0 de 3.547 partidas), então o
Discovery já publica zero. As três barreiras do AUD-003 estão implementadas e testadas,
mas **só serão exercidas em produção quando houver fonte de odds de mercado**. Não é
possível medir aqui a queda real de publicações — a medição vive no cenário sintético
da Fase C, onde a queda foi de 464 aprovações para 0.

**Efeito colateral intencional:** a mineração passa a ver 70% do histórico, então menos
combinações alcançam os 100 jogos do doc 08. Isso é um **aperto, não um problema a
compensar** — a trava de amostra mínima não foi e não deve ser reduzida para recuperar
volume.

**Limitação conhecida, registrada e não corrigida aqui:** o backtest persistido na
publicação é o da **janela de descoberta**, mas o worker de reavaliação
(`strategyengine.RunStrategy`) roda a definição sobre o histórico **completo**, sem
janela. Ou seja, no ciclo seguinte os números exibidos passam a incluir o período de
validação. Para monitorar degradação isso é razoável; para "os números que fundamentaram
a publicação", não. Corrigir exige decidir se a estratégia publicada mostra o número da
descoberta ou o número corrente — é decisão de produto, não de auditoria, e está fora
do escopo do AUD-003.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-003 marcado
- Arquivos alterados:
  - `backend/internal/formulas/significance.go` *(novo)*
  - `backend/internal/formulas/significance_test.go` *(novo)*
  - `backend/internal/usecase/discovery/validation.go` *(novo)*
  - `backend/internal/usecase/discovery/outofsample_test.go` *(novo)*
  - `backend/internal/usecase/filter_usecase.go` (janela `DateFrom`/`DateTo`)
  - `backend/internal/usecase/discovery/engine.go`
  - `backend/internal/usecase/discovery/criteria.go`
  - `backend/internal/usecase/aidocs/aidocs.go`

**Ações pendentes do usuário:** nenhuma para o AUD-003. A correção só produz efeito
visível quando existir fonte de odds reais (pendência do AUD-001).

---

## AUD-004 — Filtro de força do adversário quebrado e com look-ahead estrutural

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Dados / Estatística
**Status:** RESOLVIDO no código · migration `014` **não aplicada** (aguarda aprovação)
**Data:** 2026-09-09

### Fase A — Investigação

**O relatório chamou o dado de "corrompido". Não é. É pior: nunca houve dado.**

Rastreamento de quem escreve `teams.tier`:

| Rota | Valor gravado |
|------|---------------|
| `repository/postgres/sync_repo.go:44` | `'G12'` — **string literal no SQL** |
| `repository/postgres/statsync_repo.go:74` | `'G12'` — **string literal no SQL** |
| `internal/integration/**` | nada: **zero** referências a "tier" em todo o pacote de integração |

O provedor não devolve esse campo e nenhum ponto do sistema jamais calculou a
posição de nenhuma equipe. `'G12'` é uma constante, não uma medida.

**Estado em produção (09/09/2026):**

| Valor de `teams.tier` | Equipes |
|---|---|
| `G12` | 228 |
| `1` | 110 |
| `G6` | **0** |
| `Z4` | **0** |

As 110 com `'1'` carregam o **número da divisão da liga** (`leagues.tier` vale `'1'`
em 13 ligas e `'2'` em 8) — em algum momento esse valor foi copiado de `leagues`
para `teams`. Divisão não é grupo de classificação.

Também verifiquei que **todas as 338 equipes vieram do provedor** (`external_id`
preenchido em 338, zero semeadas). Ou seja: a hipótese "o `'1'` veio do seed
fictício" está descartada — as equipes do seed já não existem.

**O que o filtro realmente fazia.** "Contra o G12" não selecionava os doze
primeiros colocados: selecionava as equipes que entraram pelo caminho de
sincronização, e excluía as 110 que carregam o número da divisão. Um filtro de
**procedência de cadastro** com aparência de variável de força do adversário.
"Contra o G6" e "contra o Z4" devolviam vazio sempre.

**O segundo defeito, que sobreviveria mesmo se o dado existisse.** `teams.tier` é um
valor único por equipe, sem temporada e sem data. Aplicar a classificação de hoje a
uma partida de 2023 usa informação que não existia na data do jogo. E mesmo uma
classificação por temporada seria look-ahead se apurada da tabela final: na 5ª
rodada ninguém sabe quem termina no G6.

**Superfície de impacto medida antes de mexer:** 0 estratégias publicadas (de 5) e
0 filtros salvos (de 0) usam `opponent_tier`. Nada quebra.

### Fase B — Implementação

**Decisão: desligar o eixo, não remendar o dado.** A alternativa recomendada pela
auditoria — `team_season_tier` calculada da classificação da temporada — foi
descartada *nesta passagem* por três motivos, e fica especificada como trabalho
futuro (abaixo).

**1. `migrations/014_tier_nao_classificado.sql`** *(criada, NÃO aplicada)*
Não apaga nada: preserva o valor atual em `tier_legacy`, esvazia `tier` e
documenta as duas colunas com `COMMENT ON COLUMN`. Reversível com
`UPDATE teams SET tier = tier_legacy`. `leagues.tier` **não** é tocada — ali o
valor é o número da divisão, que é correto e usado como tal.

**2. Parar de gravar a constante** — `sync_repo.go` e `statsync_repo.go` passam a
gravar `''`. Vazio = não classificado, que é a verdade.

**3. O motor de backtest recusa em vez de ignorar** — `FilterCriteria.Validate()`
devolve erro explícito quando `OpponentTier` vem preenchido. Aceitar o parâmetro e
ignorá-lo em silêncio seria pior que um erro: o usuário leria o resultado como
"contra o G6". O bloco que comparava `opp.Tier` foi removido do laço.

**4. Eixo removido do Discovery** — `opponentTierOptions` saiu de
`combinations.go`. **540 → 135 combinações por liga.** As 405 que saíram não eram
hipóteses: eram a mesma hipótese contada quatro vezes, três delas garantidamente
vazias.

**5. Frontend** — seletor "Contra equipes" removido do Simulador; `(G12)` ao lado
do nome do adversário removido do Dashboard; o componente para de enviar
`opponent_tier` e ignora esse campo ao recarregar uma definição antiga.

**6. Documento vivo** (`aidocs`) — o termo "Tier de adversário" passa a explicar
que está desativado e por quê, com instrução explícita para outra IA ignorar
`opponent_tier` em definições antigas.

### Fase C — Testes

`internal/usecase/filter_tier_test.go` (novo, 4 testes) + 1 em `discovery_test.go`.

**Sobre o teste obrigatório da auditoria.** O enunciado era: *"filtro por tier deve
retornar apenas partidas cuja classificação vigente na data era a informada"*. Esse
teste **não é executável hoje**, e escrever algo que passasse no lugar dele seria
fabricar conformidade: ele pressupõe uma classificação com dimensão temporal
("vigente na data"), que o sistema não tem. O que está testado é o comportamento
seguro na ausência dela. Quando a classificação point-in-time existir, o teste
original passa a fazer sentido e deve substituir estes.

| Teste | Garante |
|-------|---------|
| `TestTierRecusadoNoBacktest` | backtest com `opponent_tier` falha, e a mensagem explica o motivo |
| `TestTodosOsTiersSaoRecusados` | G6, G12, Z4, "1" e valores arbitrários — todos recusados |
| `TestSemTierOBacktestRodaNormalmente` | a recusa não virou bloqueio geral |
| `TestTierDeEquipeNaoInfluenciaResultado` | mesmo repovoando `teams.tier`, o resultado não muda — guarda contra a volta silenciosa do filtro |
| `TestGenerateCombosNaoUsaTier` | nenhuma combinação nasce com tier preenchido |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  todos os pacotes
npx ng build --configuration production   OK
```

`TestGenerateCombosCoversFullLeagueGrid` foi ajustado (540 → 135) porque a grade
mudou de propósito — não para fazer o teste passar.

### Fase D — Validação

| Item | Antes | Depois |
|------|-------|--------|
| Combinações por liga | 540 | **135** |
| Combinações que nunca podiam render nada (G6, Z4) | 270 | 0 |
| Combinações filtrando por constante (G12) | 135 | 0 |
| Equipes com classificação real | 0 | 0 *(inalterado — nunca houve)* |
| Estratégias publicadas afetadas | 0 de 5 | 0 |
| Filtros salvos afetados | 0 de 0 | 0 |

**Interação favorável com o AUD-003:** a correção de múltiplos testes penaliza pelo
número de hipóteses testadas. Com 5.940 combinações no ciclo, o limiar de
Benjamini–Yekutieli era 0,0108; com o eixo removido o espaço cai para ~1.485 e o
limiar afrouxa para ~0,0134 — **sem relaxar critério nenhum**. O ciclo estava sendo
punido por hipóteses que não existiam.

**O que esta correção NÃO faz.** Não devolve a capacidade de analisar força do
adversário. Essa análise é legítima e valiosa — foi desligada porque o que estava no
lugar dela era falso, não porque a ideia seja ruim.

**Trabalho futuro especificado (não é AUD, é backlog de produto):**
`team_season_tier(team_id, season_id, as_of_date, tier)` apurada **incrementalmente**
a partir dos jogos anteriores a cada data. Três obstáculos que precisam de decisão
antes de implementar:

1. **Custo e complexidade:** exige recomputar a tabela de classificação a cada
   rodada de cada temporada, e o sistema não importa classificação — teria que
   derivá-la dos resultados (`home_goals`/`away_goals`), que existem.
2. **Mata-mata não tem tabela.** Copa do Brasil, Libertadores, Sul-Americana — e a
   Champions/Europa League que estão para entrar — não têm classificação. O eixo só
   é definível em competições de pontos corridos, e isso precisa estar no desenho
   desde o começo.
3. **Rodadas iniciais são ruído.** Na 3ª rodada a "classificação vigente" é quase
   aleatória. Provavelmente é preciso um mínimo de rodadas antes de o tier valer.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-004 marcado
- Arquivos alterados:
  - `backend/migrations/014_tier_nao_classificado.sql` *(novo, não aplicado)*
  - `backend/internal/repository/postgres/sync_repo.go`
  - `backend/internal/repository/postgres/statsync_repo.go`
  - `backend/internal/usecase/filter_usecase.go`
  - `backend/internal/usecase/discovery/combinations.go`
  - `backend/internal/usecase/discovery/discovery_test.go`
  - `backend/internal/usecase/aidocs/aidocs.go`
  - `backend/internal/usecase/filter_tier_test.go` *(novo)*
  - `frontend/src/app/features/filters/filters.component.html` e `.ts`
  - `frontend/src/app/features/dashboard/dashboard.component.html`

**Ações pendentes do usuário:**
1. Aprovar e aplicar a migration `014_tier_nao_classificado.sql` (não destrutiva,
   reversível). Sem ela o banco continua afirmando "G12" para 228 equipes, mas
   nenhum código lê esse valor.
2. Decidir se `team_season_tier` entra no backlog.

---
