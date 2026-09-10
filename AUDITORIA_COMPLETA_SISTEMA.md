# AUDITORIA COMPLETA — DSFR CORNERLAB

Data: 09/09/2026 · Auditor: engenharia externa · Método: leitura de código-fonte,
consulta direta ao banco de produção (somente leitura) e recálculo manual de fórmulas.
**Nenhum código foi alterado durante esta auditoria.**

---

## 1. Resumo Executivo

O CornerLab é um sistema bem construído em engenharia de software — arquitetura em
camadas coerente, separação clara entre workers e leitura, testes unitários passando,
código comentado com intenção. **Isso não é a mesma coisa que ser confiável do ponto
de vista matemático e estatístico, e é aí que estão os problemas graves.**

Foram encontrados **4 problemas CRITICAL**, todos concentrados na cadeia
odds → backtest → scores → Descobertas. O mais sério: **as estratégias publicadas com
"100% de acerto" e "DSFR 100" são artefato de construção, não padrões reais**. A odd
usada no backtest é derivada da média de escanteios do próprio lote que está sendo
testado; filtrar por odd baixa equivale a selecionar jogos de lotes com muitos
escanteios. Foi possível comprovar isso no banco: dos 51 jogos do Brasileirão Série A
com odd ≤ 2,20 na linha 8.5, **51 acertaram**.

O segundo problema mais grave é de dupla e tripla contagem: **ROI, Yield e EV são
literalmente o mesmo número** no código, e o DSFR Score atribui 20% + 10% + 20% = **50%
do seu peso a essa única variável**, dando a impressão de um score multifatorial que
na prática é dominado por uma medida só.

Do lado operacional, três camadas prometidas pela arquitetura estão vazias em
produção (`raw_fixtures`, `raw_statistics`, `team_metrics`), e o filtro de "força do
adversário" está quebrado no dado: **nenhum time está classificado como G6 ou Z4**, e
110 times têm o valor inválido `"1"`.

**Classificação final: D — NÃO CONFIÁVEL** para uso como ferramenta de decisão
quantitativa, enquanto os CRITICAL não forem corrigidos. Como ferramenta de
visualização de estatística descritiva (médias, frequências, comparador), o sistema é
utilizável.

---

## 2. Escopo da Auditoria

**Auditado com acesso a código-fonte completo:**
backend Go (todos os pacotes), frontend Angular, migrations, Dockerfile, render.yaml,
documentos da pasta `Remodelagem/`.

**Auditado com acesso ao banco de produção (Neon, somente leitura):**
schema, contagens, distribuição de odds, distribuição de tiers, tamanho de tabelas.

**Não auditado** — ver seção 36.

**Método:** leitura linha a linha dos caminhos críticos; recálculo manual de fórmulas
com casos conhecidos; consultas SQL para confrontar comportamento esperado com dado
real; rastreamento do fluxo dado → banco → cálculo → API → frontend.

---

## 3. Arquitetura Encontrada

A arquitetura **real** difere da documentada. O fluxo documentado é:

```
API-FOOTBALL → RAW → NORMALIZAÇÃO → ANALYTICS → ESTRATÉGIAS → BACKTEST
→ DISCOVERY → SCORES → HEALTH → OPPORTUNITY → IA → FRONTEND
```

O fluxo **implementado** é:

```
API-FOOTBALL → NORMALIZADO (matches/teams/leagues/seasons)
                    ↓
            ┌───────┴────────┬─────────────────┐
            ↓                ↓                 ↓
      Analytics Worker  Strategy Engine   Discovery Engine
      (nunca executou)        ↓                 ↓
                        backtests/scores/health/strategies
                                 ↓
                           API REST → SPA Angular
```

Divergências estruturais:

| Documentado | Real |
|---|---|
| Camada RAW imutável | Tabelas existem, **0 linhas** — a ingestão nunca grava nelas |
| ANALYTICS com team_metrics | **0 linhas** — Analytics Worker nunca completou ciclo |
| Opportunity Engine | **Não existe** — só a tabela `opportunities`, vazia, sem código que escreva |
| IA no pipeline | Existe (`internal/usecase/intelligence`), mas **fora** do pipeline: é consulta sob demanda |
| Cache Redis na cadeia | Cliente existe (`pkg/cache`), **não encontrado no caminho principal** |

**Componentes reais:** API HTTP (Gin), worker de background (`cmd/worker`), CLI de sync
(`cmd/sync`), seed, Postgres (Neon serverless), Angular 20 SPA, Stripe (billing),
Resend (e-mail), OpenAI/Anthropic (explicações), API-Football (dados).

---

## 4. Fluxo de Dados

**Ingestão (`statsync`):** provider → `UpsertTeam`/`UpsertMatch` → tabela `matches`.
Odds sintéticas geradas por lote no `cmd/sync` (ver AUD-001).

**Backtest (`FilterUsecase.RunBacktest`):** carrega TODAS as partidas da liga/temporadas
em memória → expande cada partida em duas "perspectivas" (mandante/visitante) → aplica
filtros → decide acerto por `total > threshold` → calcula P/L.

**Ponto crítico do fluxo:** o critério de acerto usa os escanteios **da própria partida
sendo apostada**. Isso é correto para backtestar uma regra estática ("apostar over N em
todo jogo que atenda ao filtro"), mas significa que **não existe modelo preditivo**: o
sistema não usa forma anterior para decidir a entrada. Isso precisa estar explícito
para o usuário, porque o nome "estratégia descoberta" sugere o contrário.

---

## 5. Inventário de Componentes

| Componente | Local | Estado |
|---|---|---|
| Catálogo de Fórmulas | `internal/formulas/` | Implementado, testado, 60 testes passando |
| Motor de backtest | `internal/usecase/filter_usecase.go` | Implementado, **sem testes próprios** |
| Strategy Engine | `internal/usecase/strategyengine/` | Implementado, testado |
| Discovery Engine | `internal/usecase/discovery/` | Implementado, testado |
| Analytics Worker | `internal/usecase/analytics/` | Implementado, **nunca executou em produção** |
| Inteligência (IA) | `internal/usecase/intelligence/` | Implementado, **sem testes** |
| Odds sintéticas | `internal/usecase/oddsgen.go` | Implementado — **fonte do AUD-001** |
| Progresso de tarefas | `internal/progress/` | Implementado |
| Cache Redis | `pkg/cache/redis.go` | Implementado, **uso não localizado** |

---

## 6. Auditoria de Banco de Dados

**Positivo:** todas as 12 tabelas da camada ANALYTICS existem; **zero foreign keys não
validadas** (verificado via `pg_constraint`); índices da migration 012 presentes;
migrations 011 e 012 aplicadas.

**Tamanho real:** banco inteiro com **14 MB** de uma cota de 500 MB (~3%). `matches` é a
maior tabela (3,6 MB / 3.547 linhas). Não há problema de crescimento no horizonte
visível.

**Problemas:**

- `raw_fixtures`, `raw_statistics`, `team_metrics`: **0 linhas** (AUD-009).
- `teams.tier`: sem dimensão temporal — impossível representar "força do adversário
  na época da partida" (AUD-004). Além disso o dado está corrompido.
- `api_usage_log`: única tabela sem teto de crescimento (mitigação já implementada em
  commit recente, retenção de 90 dias).
- `matches.corner_odds` é JSONB sem constraint de formato; um lote com chave ausente
  degrada silenciosamente para odd 1.0 (AUD-012).

---

## 7. Auditoria de Ingestão

**Idempotência:** confirmada — upsert por `UNIQUE(external_id)`. Uma partida não é
duplicada.

**Rate limit:** corrigido recentemente (throttle ~9 req/min + retry com backoff no 429).
Antes disso o ciclo disparava ~4 req/s e tomava 429 — 45 erros registrados.

**Timeout/retry:** cliente HTTP com timeout de 20s; retry apenas para 429.

**Partidas adiadas/canceladas:** `dueBuffer` de 2h evita buscar jogo em andamento; jogo
sem resultado publicado permanece `AGENDADO` e é retentado. **Não foi encontrado
tratamento explícito para partida cancelada ou anulada** — ela ficaria sendo retentada
indefinidamente. NECESSITA INVESTIGAÇÃO sobre o custo de cota disso.

**Preservação de histórico:** dados normalizados são preservados; **dado bruto não**
(camada RAW vazia).

---

## 8. Auditoria Matemática

### Inventário e verificação numérica

| Fórmula | Local | Implementação | Status |
|---|---|---|---|
| Probability | `formulas/probability.go:8` | `success/total`, erro em total=0 | ✅ |
| ImpliedProbability | `probability.go:21` | `1/odd`, erro se odd≤1 | ✅ |
| FairOdds | `probability.go:33` | `1/p`, piso 1.01 | ✅ |
| BreakEven | `probability.go:50` | `1/odd` | ✅ |
| Edge | `probability.go:62` | `p_real − p_implícita` | ✅ |
| ExpectedValue | `financial.go:9` | `p·stake·(odd−1) − (1−p)·stake` | ✅ |
| ROIPercent | `financial.go:29` | `lucro/investimento×100` | ✅ |
| YieldPercent | `financial.go:42` | `lucro/volume×100` | ✅ fórmula, ❌ uso (AUD-002) |
| Kelly | `staking.go:12` | `(b·p−q)/b` | ✅ |
| MaxDrawdown | `risk.go:44` | `(pico−vale)/pico` | ✅ existe, ❌ não é usado no backtest |
| Variance/StdDev | `risk.go:8,28` | populacional | ✅ |
| Sharpe/Calmar | `risk.go:95,110` | adaptados | ✅ |
| Monte Carlo | `montecarlo.go` | seed fixa, percentis interpolados | ✅ |
| DSFRScore | `scores.go:68` | média ponderada 8 componentes | ❌ (AUD-002, AUD-007) |
| HealthScore | `scores.go:83` | `50 + 50·média(Δ)` | ⚠️ (AUD-011) |
| TrendScore | `scores.go:177` | 50/30/20 por janela | ❌ uso (AUD-008) |

**Conclusão:** o pacote `internal/formulas` está matematicamente correto e bem testado.
**Os erros não estão nas fórmulas — estão nos valores alimentados a elas.**

---

## 9. Auditoria Estatística

Esta é a seção com os achados mais graves.

**Múltiplas comparações sem correção (AUD-003).** O Discovery testa 540 combinações por
liga × 11 ligas = 5.940 testes. Com 5.940 testes independentes ao nível de significância
implícito dos critérios, encontrar dezenas de combinações "excelentes" por puro acaso é
o resultado esperado, não a exceção. Não há correção de Bonferroni/FDR, não há
validação out-of-sample, não há holdout temporal.

**Ausência de validação fora da amostra.** O backtest usa 100% dos dados disponíveis
para selecionar E avaliar. Não existe divisão treino/teste nem walk-forward. Uma
estratégia aprovada nunca foi testada em dado que não participou da seleção.

**Leakage via odds (AUD-001).** Detalhado abaixo — é o achado central.

**Look-ahead bias no tier do adversário (AUD-004).** `teams.tier` é um valor único e
atual, aplicado retroativamente a partidas de qualquer época.

**Viés de sobrevivência:** descobertas não republicadas são desativadas, e o ranking
mostra apenas as sobreviventes do ciclo mais recente — sem histórico visível do que
foi descartado, o usuário vê só os vencedores.

**Independência das observações:** cada partida é expandida em **duas** entradas
(perspectiva do mandante e do visitante). Para métricas de **total** da partida
(escanteios totais > N), as duas entradas têm **resultado idêntico** — a mesma partida é
contada duas vezes com o mesmo desfecho. Isso **infla artificialmente o tamanho da
amostra pela metade** e viola independência no cálculo de sequências e drawdown.
Ver AUD-021.

---

## 10. Auditoria de Backtest

**Definições:** Win = `total > threshold`. Loss = complementar. **Não existem Push, Void
nem Cancelled** — o sistema é binário. Para linhas ",5" isso é adequado; para linhas
inteiras seria incorreto, mas o sistema só usa linhas efetivamente ",5".

**Stake:** fixa, default 1. **Bankroll:** não modelado — o backtest acumula P/L, não
saldo. Consequência: drawdown é absoluto em unidades de stake, não percentual de banca.

**Look-ahead na decisão de entrada:** não há decisão preditiva, então não há look-ahead
clássico — mas há leakage pelas odds (AUD-001) e pelo tier (AUD-004).

**Odds:** históricas apenas para escanteios, e sintéticas (AUD-001). Para gols/chutes/
impedimentos usa odd fixa informada pelo usuário, ou 1.0 (AUD-012).

**Determinismo:** quebrado quando `maxAgeDays > 0` (usa `time.Now()`), ver AUD-016.

---

## 11. Auditoria de Estratégias

Uma "estratégia" é um JSON de filtros (liga, temporadas, time, janela, mando, linha,
tier, odd máxima, stake, métrica). A execução reusa o **mesmo** `FilterUsecase` do
Simulador — isso é um acerto de arquitetura: garante reprodutibilidade entre o número
publicado e o que o usuário consegue reproduzir na tela.

**Divergência documentação × implementação:** a interface descreve "estratégia
descoberta" e "padrão identificado", sugerindo modelo preditivo. A implementação é uma
regra estática de aposta em todo jogo que passa no filtro. Isso não é bug, mas é uma
divergência de expectativa relevante — ver AUD-018.

---

## 12. Auditoria do Discovery Engine

**Contagem verificada:** 5 linhas × 3 mandos × 3 janelas × 4 tiers × 3 tetos de odd =
**540 por liga**. Confirmado contra a tela: "5.940 combinações testadas em 11
campeonatos" = 11 × 540. ✅ matemática do espaço de busca correta.

**Mas:** 5.940 = 11 × 540 **exatamente** significa que a varredura por equipe
(`IncludeTeams: true` no worker) **gerou zero combinações**. Com times, cada liga somaria
45 combos por equipe. Ver AUD-022.

**Combinações mortas:** dos 4 valores de tier, **G6 e Z4 não correspondem a nenhum time
no banco** (AUD-004). Isso torna 2/4 dos combos garantidamente vazios: **270 dos 540 por
liga**, ou **2.970 dos 5.940**, são descartados por "amostra insuficiente" antes de
qualquer análise. O número "5.940 combinações testadas" exibido ao usuário é, na
prática, metade disso.

**Ordem dos filtros:** critérios aplicados na ordem amostra → win rate → ROI → yield →
EV → drawdown → DSFR. A ordem é adequada (barato antes de caro), mas o DSFR é calculado
depois do backtest completo — inevitável.

**Idempotência:** garantida por nome determinístico + índice único parcial. ✅

---

## 13. Auditoria dos Scores

### Dupla e tripla contagem (AUD-002 e AUD-007)

Em `filter_usecase.go:427-429`:

```go
roi := 100 * profit / totalStaked
result.ROI = round2(roi)
result.Yield = round2(roi)   // MESMO VALOR
```

Em `strategyengine/engine.go:175`:
```go
EV: ptr(r.Yield),            // MESMO VALOR de novo
```

Em `engine.go:223-232`, o DSFR recebe:
- `ROI: roiNorm` (peso 20%)
- `EV: yieldNorm` (peso 20%)
- `Yield: yieldNorm` (peso 10%) — **a mesma variável Go passada duas vezes**

**Resultado: 50% do DSFR Score é uma única variável**, normalizada por dois tetos
diferentes (20% e 15%).

Pior ainda, `invVar := 1 - 4*winRate*(1-winRate)` é **função pura de winRate**. Somando:

| Quantidade real | Peso efetivo no DSFR |
|---|---|
| ROI (via ROI+EV+Yield) | **50%** |
| Win rate (direto 15% + invVar 5% + via consistência 6%) | **26%** |
| Drawdown (10% + 2%) | 12% |
| Amostra (10% + 2%) | 12% |

O score aparenta oito dimensões independentes; tem efetivamente **duas**.

**Confidence e Robustness** recebem `sampleNorm` **duas vezes** (como tamanho de amostra
e como "robustez temporal"), o que faz o tamanho da amostra valer 50% da Confiança e
40% da Robustez. Não existe robustez temporal calculada em lugar nenhum.

**Trend** é `TrendScore(dROI, dROI, dROI)` — o mesmo valor nas três janelas, o que
colapsa a média ponderada 50/30/20 em `dROI`. As janelas de 5/10/20 jogos não existem.

---

## 14. Auditoria da IA

**Positivo, e é o ponto mais bem resolvido do sistema.** `intelligence/explain.go`
implementa duas camadas independentes: um system prompt com regras explícitas
("nunca recomende aposta", "use exclusivamente os dados fornecidos", "nunca no futuro
como previsão") e um filtro pós-geração (`forbiddenPhrases`) que substitui a resposta
inteira se o modelo escorregar. Há `DataSnapshot` para auditoria do que foi enviado.

**Problemas:** nenhum teste automatizado cobre o filtro; a lista de frases proibidas é
frágil a variações ("vale a pena entrar", "eu iria de"); e a IA recebe dados que, pelos
achados desta auditoria, podem estar estatisticamente inválidos — ela vai explicar com
confiança um número que não deveria existir.

---

## 15. Auditoria dos Workers

`cmd/worker` executa em sequência: health check → descoberta → atualização → analytics →
strategy engine → discovery → limpeza de log. Como é **sequencial no mesmo processo**,
não há risco de uma etapa começar antes da anterior terminar. ✅

**Mas os workers não estão rodando.** Evidência: `worker_runs` com **0 linhas** e
`team_metrics` com **0 linhas**. A causa raiz (Dockerfile não compilava o binário
`cmd/worker`) foi corrigida recentemente; o `render.yaml` com o Cron Job foi criado mas
**precisa ser aplicado no painel do Render** — ver AUD-009.

Sem cron, todo dado atual foi produzido por cliques manuais.

---

## 16. Auditoria de Timezone

**Não existe política explícita de timezone.** Achados:

- `matches.match_date` é `time.Time`; o driver pgx entrega em UTC.
- `dueBuffer` compara com `time.Now()` (hora do container, UTC no Render).
- Cron configurado para `0 6 * * *` **UTC** = 03:00 BRT — correto e documentado.
- Frontend formata com `new Date(...)` + `getDate()/getHours()` — **timezone local do
  navegador**. Um usuário fora do Brasil vê datas de partida deslocadas.
- `BacktestEntry.MatchDate` é formatado como `"2006-01-02"` em UTC e depois **ordenado
  como string**. Para partidas noturnas no Brasil (22h BRT = 01h UTC do dia seguinte), a
  data exibida e a ordenação usam o dia seguinte.

Impacto: agrupamentos por dia e a ordenação cronológica do backtest podem deslocar
partidas noturnas em um dia. Afeta cálculo de sequências e drawdown. **MEDIUM,
NECESSITA INVESTIGAÇÃO** para quantificar quantas partidas caem nessa faixa.

---

## 17. Auditoria das APIs

**51 endpoints** mapeados no router. Estrutura de grupos coerente: público → `authGroup`
→ `premiumGroup`.

**Positivo:** tarefas longas migradas para 202 + polling com 409 em concorrência;
`/health` sem dependência de banco; 404 em vez de 403 na checagem de posse (evita
enumeração).

**Problemas:**
- `docs/openapi.yaml` documenta **31 de 51** endpoints (AUD-013).
- `PATCH /strategies/:id` sem escopo de dono no SQL (AUD-005).
- `POST /strategies/:id/run` aceita estratégia pública do sistema — qualquer usuário
  logado força recomputação de recurso compartilhado (custo + concorrência).
- Sem paginação em `/strategies`, `/bets`, `/discovery/strategies` (limit fixo).
- `GET /diagnostics/usage` executa 3 queries + agregação por provedor a cada request,
  sem cache.

---

## 18. Auditoria de Segurança

| Item | Estado |
|---|---|
| SQL Injection | ✅ Todas as queries parametrizadas (`$1..$n`); nenhuma concatenação de input encontrada |
| JWT | ✅ Middleware valida Bearer, 401 em ausente/inválido |
| Isolamento entre usuários | ⚠️ Correto para estratégias privadas; **falha em públicas** (AUD-005) |
| Rate limit | ❌ **Inexistente** — `/auth/login` sem proteção contra força bruta (AUD-010) |
| CORS | ⚠️ `Access-Control-Allow-Origin: *`; `PATCH` ausente de Allow-Methods |
| Secrets | ✅ Via env vars, `sync: false` no render.yaml |
| Logs com dado sensível | ✅ Não encontrado |
| Senha | ✅ bcrypt (`golang.org/x/crypto`) |
| CSRF | ✅ N/A — auth por header, não cookie |
| IDOR | ⚠️ Ver AUD-005 |

**Não testado ativamente** (pentest contra produção fora de escopo sem autorização):
XSS, SSRF, mass assignment.

---

## 19. Auditoria de Billing

Stripe integrado com checkout hospedado, portal e webhook. `premiumGroup` separa
recursos pagos. Webhook em rota pública (correto — Stripe assina).

**NÃO FOI POSSÍVEL AUDITAR:** idempotência do webhook, comportamento em cancelamento,
renovação e downgrade. Exigiria eventos de teste do Stripe contra o ambiente, o que
está fora do escopo de leitura. Ver seção 36.

---

## 20. Auditoria de Performance

- `AllMatches` carrega **todas** as partidas da liga em memória a cada backtest. Com
  5.940 combinações, o Discovery mitiga com `cachedMatchRepo` por ciclo. ✅
- `sortByDate` é **insertion sort O(n²)** (`filter_usecase.go:434`). Com centenas de
  entradas × milhares de backtests, é o gargalo mais provável do ciclo (AUD-015).
- Cada partida vira 2 candidatos, dobrando o trabalho de filtragem.
- `GET /diagnostics/usage` sem cache.

---

## 21. Auditoria de Cache

`pkg/cache/redis.go` implementa `GetJSON`/`SetJSON` com TTL, e `config.IntelligenceCacheTTL`
sugere uso de 24h no módulo de Inteligência. **Não foi localizada chamada a essas funções
no caminho principal da aplicação.** Se o Redis não está em uso, `REDIS_ADDR` aponta para
`localhost:6379` por padrão — inofensivo, mas é código morto potencial.

**NECESSITA INVESTIGAÇÃO.**

---

## 22. Auditoria do Frontend

**Positivo:** sem mocks; sem valores hardcoded nos números; estados de loading, vazio e
erro tratados; `role="alert"` em erros; aviso de plano gratuito quando `history_capped`.

**Consistência numérica:** o backend arredonda com `round2` e o frontend exibe com
`number:'1.0-0'` (percentuais na barra) ou direto. **Não foi encontrada divergência de
valor** entre backend e tela — apenas arredondamento de apresentação, aceitável.

**Problemas:**
- Datas formatadas no timezone do navegador (seção 16).
- A tela apresenta DSFR, Confiança, Robustez, Risco e Ranking como métricas
  independentes, quando são largamente redundantes (AUD-002). **Isso é um problema de
  UX que afeta confiabilidade**: a redundância comunica mais evidência do que existe.
- "Ciclo de vida: Maturidade" exibido para estratégia com uma única execução (AUD-011).

---

## 23. Auditoria de Consistência

| Fonte | Afirma | Realidade |
|---|---|---|
| Doc 08 / dicionário | Amostra mínima 100 jogos | Código: 100, com piso 50. ✅ |
| Arquitetura | RAW permite reprocessar | Tabela vazia ❌ |
| Arquitetura | Opportunity Engine | Não existe ❌ |
| `openapi.yaml` | 31 endpoints | 51 reais ❌ |
| UI | "Yield" como métrica distinta de ROI | Mesmo número ❌ |
| UI | "EV" | Mesmo número que Yield ❌ |
| UI | Tendência com janelas 5/10/20 | Uma janela só ❌ |
| Tela Descobertas | "5.940 combinações testadas" | ~2.970 tinham chance real ❌ |
| Doc 15 | "Workers calculam, usuário só lê" | Workers não rodam ❌ |

---

## 24. Auditoria de Testes

60 testes passando em 4 pacotes (`formulas`, `analytics`, `discovery`, `strategyengine`)
+ 1 novo em `aidocs`. Qualidade dos testes de fórmula é **boa**: casos válidos, extremos
e inválidos, tolerância 1e-4.

**Lacunas graves:**
- **`filter_usecase.go` — o motor de backtest — não tem nenhum teste.** É o componente
  mais crítico do sistema.
- Nenhum teste de integração, E2E, concorrência, timezone ou segurança.
- Nenhum teste cobre o filtro anti-recomendação da IA.
- Os testes do Discovery validam os **critérios**, não a **validade estatística** — um
  teste pode passar e o resultado ainda ser inválido, que é exatamente o caso aqui.

---

## 25. Edge Cases

| Caso | Comportamento | Status |
|---|---|---|
| 0 jogos | `buildBacktestResult` retorna cedo, sem divisão por zero | ✅ |
| 1 jogo | Passa; rejeitado depois por amostra | ✅ |
| 0 wins | HitRate 0, ROI −100% | ✅ |
| 100% wins | Aceito sem ressalva | ⚠️ deveria ser sinal de alerta |
| odd 1.01 | Aceita | ✅ |
| odd ausente | **Vira 1.0** → ROI negativo sem significado | ❌ AUD-012 |
| probability 0 / 1 | `FairOdds` valida; `Probability` valida | ✅ |
| Divisão por zero | Tratada em todas as fórmulas auditadas | ✅ |
| Time sem histórico | Lista vazia, sem crash | ✅ |
| Liga sem temporada | `RunLeague` retorna cedo | ✅ |
| Partida sem estatística | Pulada (métricas nullable) | ✅ |
| Partida cancelada | **Não tratada** | ⚠️ NECESSITA INVESTIGAÇÃO |
| Banco indisponível | Retry 5× com janela de 15s (correção recente) | ✅ |
| API externa indisponível | Incidente registrado, ciclo pulado | ✅ |
| Redis indisponível | N/A — não usado | — |

---

## 26. Auditoria de Deploy

- **Dockerfile:** corrigido recentemente para compilar `cmd/worker` (antes não compilava
  — causa raiz dos workers nunca rodarem).
- **`render.yaml`:** criado, declara API + cron diário. **Ainda não aplicado no painel.**
- **Migrations:** aplicadas manualmente via SQL Editor. **Não há execução automática de
  migration no deploy** — risco de código novo com schema antigo.
- **Health check:** `/health` sem dependência de banco. ✅
- **Graceful shutdown:** worker escuta SIGTERM/SIGINT. ✅ API: não verificado.
- **Rollback:** não há estratégia documentada.

---

## 27. Auditoria de Observabilidade

- `worker_runs` registra execução, duração, processados, erros — **bom desenho, tabela
  vazia**.
- `api_usage_log` registra toda chamada externa, alimenta o painel Integrações. ✅
- Logs estruturados (`slog`). ✅
- **Sem alertas.** Um erro CRITICAL passa despercebido indefinidamente — comprovado: a
  sincronização ficou 22 dias parada sem ninguém ser avisado.
- Sem métricas exportadas, sem tracing, sem uptime check.

---

## 28. Auditoria de Documentação

Documentação **acima da média** em volume e intenção (31 documentos em `Remodelagem/`,
comentários de código explicando o porquê). Mas há divergências relevantes com a
implementação — tabela na seção 23.

O documento de contexto para IA (`aidocs`) é gerado a partir das constantes reais e da
lista viva de rotas, o que é o padrão correto e deveria ser estendido ao `openapi.yaml`.

---

## 29. Testes Matemáticos Independentes

| Entrada | Esperado | Sistema | Status |
|---|---|---|---|
| EV: p=0.5, odd=2.00, stake=1 | 0 | 0 | ✅ |
| EV: p=0.6, odd=2.00, stake=1 | +0.20 | +0.20 | ✅ |
| EV: p=0.8, odd=1.50, stake=1 | +0.20 | +0.20 | ✅ |
| EV: p=0.75, odd=1.60, stake=100 | +20 | +20 (caso do catálogo) | ✅ |
| Break-even odd 2.20 | 0.4545 | 0.4545 | ✅ |
| Fair odds p=0.895 | 1.117 | 1.117 | ✅ |
| Kelly odd=2.0, p=0.6 | 0.20 | 0.20 | ✅ |
| ROI: lucro 500, invest. 2000 | 25% | 25% | ✅ |
| **Yield ≠ ROI (stake variável)** | valores distintos | **idênticos por construção** | ❌ |
| DSFR com ROI=30%, resto médio | ~8 dimensões independentes | 2 dimensões efetivas | ❌ |
| Drawdown 10 unid. em 100 apostas | 10% do movimentado | 10% | ⚠️ métrica inadequada |
| Drawdown 10 unid. em 200 apostas | mesmo risco | **5%** | ❌ AUD-006 |

---

## 30. Validação com Dados Reais

Consultas executadas no banco de produção:

**Distribuição de odds por liga (linha 8.5), jogos com odd ≤ 2,20:**

| Liga | Jogos | Acertaram | Odds distintas |
|---|---|---|---|
| Premier League | 380 | 261 (68,7%) | **1** |
| La Liga | 380 | 231 (60,8%) | **1** |
| Serie A | 380 | 195 (51,3%) | **1** |
| Ligue 1 | 309 | 183 (59,2%) | **1** |
| Bundesliga | 308 | 189 (61,4%) | **1** |
| **Brasileirão A** | **51** | **51 (100%)** | **4** |
| **Brasileirão B** | **11** | **11 (100%)** | **4** |

**Interpretação:** ligas europeias têm **uma única odd** repetida em centenas de jogos —
prova de que a odd é sintética por lote, não de mercado. O Brasileirão tem 4 valores
(4 lotes sincronizados) e **100% de acerto** nos jogos de odd baixa, porque odd baixa =
lote com média de escanteios alta = jogos que estouram a linha. É leakage em nível de
lote, com efeito de 100%.

**Distribuição de tier:** G12 = 228 times, `"1"` = 110 times, **G6 = 0, Z4 = 0**.

---

## 31. Problemas Encontrados

### AUD-001 — Odds sintéticas derivadas da própria amostra causam leakage
**Severidade:** CRITICAL · **Categoria:** Estatística / Backtest
**Localização:** `internal/usecase/oddsgen.go:15`, `internal/usecase/sync_usecase.go:69-82`

**Problema:** as odds usadas no backtest não são de mercado. São geradas de uma normal
cuja média (`batchMu`) é a **média de escanteios do lote sincronizado** — o mesmo
conjunto de partidas que será testado.

**Evidência:** ligas europeias têm exatamente **1 odd distinta** para a linha 8.5 em
380 jogos. No Brasileirão, dos 51 jogos com odd ≤ 2,20, **51 acertaram** (100%).

**Impacto:** filtrar por `MaxOdds` equivale a selecionar lotes de alta média de
escanteios, garantindo acerto alto. Todo ROI, EV, DSFR e ranking derivado disso é
**inválido**. As descobertas "Elite" publicadas são artefato.

**Resultado atual:** 3 estratégias publicadas com 100% de acerto e drawdown 0,00.
**Resultado esperado:** sem odds reais, o backtest financeiro não deveria ser executado.

**Causa provável:** decisão de contorno para "viabilizar o Simulador" (documentada no
próprio código) que vazou para o pipeline de decisão.

**Correção recomendada:** adicionar `odds_source` (`real` | `synthetic`) em `matches`;
o Discovery deve ignorar partidas com odd sintética; o Simulador deve exibir aviso
explícito e suprimir ROI/EV quando a odd for sintética. Despublicar as descobertas
atuais.

**Teste obrigatório:** dado um conjunto com odds sintéticas, o Discovery deve publicar
zero estratégias.

**Prioridade:** P0

**Status:** ✅ **RESOLVIDO (código)** — 2026-09-09 · migration `013` pendente de aprovação
**Arquivos:** `migrations/013_odds_source.sql` (nova, não aplicada), `domain/entities.go`,
`repository/postgres/match_repo.go`, `usecase/filter_usecase.go`,
`usecase/discovery/engine.go`, `usecase/strategyengine/engine.go`,
`frontend/core/models.ts`, `frontend/features/filters/filters.component.html`
**Testes:** `usecase/filter_odds_source_test.go` — 7 casos; SYNTHETIC e UNKNOWN rejeitados
pelo Discovery, REAL permitido, Simulador aceita mas marca o resultado. Suíte completa,
`go vet`, `gofmt` e `ng build` verdes.
**Resultado:** Discovery publicará **0 estratégias** e as 5 atuais serão despublicadas
(sem apagar) quando a migration for aplicada — não existe nenhuma odd real no banco
(0 de 3.547 partidas). Esse é o resultado esperado, não uma falha.
**Detalhes:** `CORRECOES_AUDITORIA.md` § AUD-001

---

### AUD-002 — ROI, Yield e EV são o mesmo número; DSFR conta 50% em uma só variável
**Severidade:** CRITICAL · **Categoria:** Matemática / Scores
**Localização:** `filter_usecase.go:427-429`, `strategyengine/engine.go:175,223-232`

**Problema:** `result.Yield = round2(roi)` atribui o ROI ao Yield. `EV: ptr(r.Yield)`
atribui o mesmo valor ao EV. O DSFR recebe essa variável em três slots com pesos
20% + 20% + 10%.

**Evidência:** código citado; `yieldNorm` é passado literalmente duas vezes na mesma
struct (`EV: yieldNorm, Yield: yieldNorm`).

**Impacto:** o score aparenta oito dimensões e tem duas. Uma estratégia com ROI alto e
todo o resto ruim recebe nota desproporcional. O ranking público está ordenado por uma
métrica enviesada.

**Resultado atual:** DSFR 100,0 para estratégia cujo único mérito é ROI (artificial).
**Resultado esperado:** ROI, Yield e EV medindo coisas distintas, ou o DSFR reponderado
para não contar a mesma quantidade três vezes.

**Correção recomendada:** ou (a) implementar Yield com volume real e EV com
probabilidade estimada independente, ou (b) remover EV e Yield do DSFR e reponderar.
Exige nova versão do Formula Catalog.

**Teste obrigatório:** cenário com ROI fixo e demais componentes variando deve mover o
DSFR proporcionalmente ao peso declarado.

**Prioridade:** P0

**Status:** ✅ **RESOLVIDO** — 2026-09-09 · **Formula Catalog v1.1**
**Arquivos:** `formulas/scores.go` (v1.1; v1.0 preservada para histórico), `formulas/doc.go`,
`usecase/strategyengine/engine.go`, `usecase/discovery/criteria.go`,
`usecase/aidocs/aidocs.go`, `handlers/discovery_handler.go`, `frontend/core/models.ts`
**Testes:** `strategyengine/scores_weights_test.go` — 10 casos. O obrigatório está nos dois
sentidos: variar só o retorno move o DSFR exatamente 30,00 pontos (peso declarado do ROI);
com retorno fixo, os demais componentes movem o score. Suíte completa, `go vet`, `gofmt` e
`ng build` verdes.

**Correção:** adotada a opção (b). EV e Yield saem do DSFR e do Ranking; ΔEV sai do Health.
A opção (a) foi descartada com justificativa: Yield distinto exige stake variável (o motor
não tem política de staking) e EV distinto exige P(vitória) estimada fora da amostra (é o
AUD-003, ainda inexistente) — construir qualquer um dos dois hoje seria fabricar dado.
Os 30 pontos liberados **não voltaram ao ROI**; foram distribuídos entre as demais
dimensões. Exposição ao retorno: **50% → 30%**. O campo `ev` passa a ser **NULO**
("não calculado") em vez de receber o yield emprestado.

**Resultado medido** (recálculo sobre os componentes reais das 5 estratégias em produção):
a correção **re-rankeia, não rebaixa** — id 6 subiu +2,93 e ids 4/5/7 caíram entre 0,48 e
2,34. A ordem não mudou nesta amostra, mas ela é degenerada (todos com win rate 100%, os
próprios artefatos do AUD-001), então **não permite concluir que o impacto no ranking é
pequeno**.

**Sem migration:** `strategy_scores`/`strategy_health` são upsert e migram sozinhos no
próximo ciclo; as linhas v1.0 de `backtests` estão corretas para a versão que declaram.

**Não resolvido aqui (de propósito):** o DSFR v1.1 ainda não tem dimensões ortogonais —
`invVar` é função pura de winRate e `consistency` compõe os outros componentes. Isso é o
**AUD-007** (P1), deixado em aberto para não misturar causas.
**Detalhes:** `CORRECOES_AUDITORIA.md` § AUD-002

---

### AUD-003 — Múltiplas comparações sem correção nem validação fora da amostra
**Severidade:** CRITICAL · **Categoria:** Estatística
**Localização:** `internal/usecase/discovery/engine.go`, `criteria.go`

**Problema:** 5.940 combinações testadas contra os mesmos dados, com thresholds fixos e
sem correção para testes múltiplos nem holdout temporal.

**Evidência:** `generateCombos` × `RunAll`; nenhum split treino/teste no código.

**Impacto:** aprovar combinações que parecem excelentes por acaso é o comportamento
esperado deste desenho. Não há como distinguir vantagem real de ruído.

**Correção recomendada:** dividir o histórico em janela de descoberta e janela de
validação; só publicar o que sobrevive na janela de validação. Registrar o número de
testes realizados e aplicar correção (Bonferroni ou FDR) ao limiar.

**Teste obrigatório:** dado ruído aleatório puro, o ciclo deve publicar ~0 estratégias.

**Prioridade:** P0

**Status:** ✅ **RESOLVIDO** — 2026-09-09
**Arquivos:** `formulas/significance.go` (novo), `usecase/discovery/validation.go` (novo),
`usecase/filter_usecase.go` (janela temporal `DateFrom`/`DateTo`),
`usecase/discovery/engine.go`, `usecase/discovery/criteria.go`, `usecase/aidocs/aidocs.go`
**Testes:** `formulas/significance_test.go` + `usecase/discovery/outofsample_test.go` —
17 casos. Suíte completa, `go vet` e `gofmt` verdes.

**Correção — três barreiras, nenhuma desligável por configuração:**

1. **Corte temporal 70/30 por data.** A mineração só enxerga os 70% mais antigos; os 30%
   mais recentes ficam reservados. Intervalo meio-aberto `[from, to)` e toda partida da
   data de corte vai para a validação, para que nenhuma data caia nas duas janelas. Liga
   sem corte possível não publica nada.
2. **Significância corrigida por número de testes.** Teste binomial unilateral contra a
   probabilidade implícita na odd (1/odd) — não contra "acerto alto", que premiaria linha
   fácil de odd baixa. Controle de FDR por **Benjamini–Yekutieli** (válido sob dependência
   arbitrária; as combinações compartilham as mesmas partidas). O limiar passa a depender
   de *m*: testar mais custa mais caro.
3. **Reteste fora da amostra.** Sobreviventes são reexecutados na janela reservada e só
   publicam com amostra mínima, lucro positivo e significância própria.

Além disso: ciclo interrompido não publica nada (a correção só vale sobre a varredura
completa), e a descrição de cada descoberta passa a trazer os dois períodos e os dois
p-valores.

**Teste obrigatório cumprido — com uma ressalva que ficou registrada.** A primeira versão
do teste de ruído era **vácua**: `tested = 0`, tudo barrado por `win_rate_baixo` antes de
chegar ao mecanismo novo; teria passado com a correção revertida. Foi refeita com odds
justas da distribuição geradora (calculadas analiticamente, não estimadas da amostra) e
limiares do doc 08 permissivos *no cenário de teste*, mais uma guarda permanente
`Tested == 0 → falha`. Resultado: o mesmo ruído que teria aprovado **464 combinações**
antes da correção publica **0** depois.

**Exigência prática** (odd 2,00, ciclo de 5.940 testes, q = 0,10): 68% de acerto em 50
jogos, 62% em 100, 58,5% em 200, 56,1% em 380.

**Efeito em produção: nenhum ainda, e não é boa notícia.** Sem odd real no banco
(AUD-001), o Discovery já publica zero; as barreiras só serão exercidas quando existir
fonte de odds de mercado.

**Limitação registrada, fora do escopo:** o backtest publicado é o da janela de
descoberta, mas o worker de reavaliação roda sobre o histórico completo — no ciclo
seguinte os números exibidos passam a incluir o período de validação. Decisão de produto.
**Detalhes:** `CORRECOES_AUDITORIA.md` § AUD-003

---

### AUD-004 — Filtro de força do adversário quebrado e com look-ahead estrutural
**Severidade:** CRITICAL · **Categoria:** Dados / Estatística
**Localização:** `teams.tier` (schema), `filter_usecase.go:248-253`

**Problema:** (a) o dado está corrompido — **0 times G6, 0 times Z4**, 228 com G12 e 110
com o valor inválido `"1"`; (b) `tier` não tem dimensão temporal, então a classificação
atual é aplicada a partidas de qualquer época — look-ahead bias por construção.

**Evidência:** `SELECT tier, count(*) FROM teams GROUP BY 1` → `G12: 228`, `"1": 110`.

**Impacto:** combos com tier G6/Z4 sempre retornam vazio (2.970 dos 5.940 são teatro);
o combo G12 filtra 2/3 dos times sem significado; se o dado fosse preenchido, geraria
look-ahead.

**Correção recomendada:** tabela `team_season_tier(team_id, season_id, tier)` calculada
da classificação daquela temporada; corrigir o valor `"1"`; enquanto isso, remover o
eixo de tier do Discovery.

**Teste obrigatório:** filtro por tier deve retornar apenas partidas cuja classificação
vigente na data era a informada.

**Prioridade:** P0

**Status:** ✅ **RESOLVIDO (eixo desligado)** — 2026-09-09 · migration `014` pendente
**Arquivos:** `migrations/014_tier_nao_classificado.sql` (nova, não aplicada),
`postgres/sync_repo.go`, `postgres/statsync_repo.go`, `usecase/filter_usecase.go`,
`usecase/discovery/combinations.go`, `usecase/aidocs/aidocs.go`,
`frontend/features/filters/*`, `frontend/features/dashboard/dashboard.component.html`
**Testes:** `usecase/filter_tier_test.go` (4) + `TestGenerateCombosNaoUsaTier`.
Suíte completa, `go vet`, `gofmt` e `ng build` verdes.

**Correção do diagnóstico.** A auditoria descreveu o dado como "corrompido". A
investigação mostrou que **nunca houve dado**: `sync_repo.go:44` e
`statsync_repo.go:74` gravavam a string literal `'G12'` em toda equipe, e o pacote
de integração não tem uma única referência a "tier". As 110 equipes com `'1'`
carregam o número da divisão vazado de `leagues.tier`. G6 = 0 e Z4 = 0 não porque o
dado se perdeu, mas porque nada jamais calculou classificação. O filtro "contra o
G12" era, na prática, um filtro de **procedência do cadastro**.

**O que foi feito:** eixo removido do Discovery (**540 → 135 combinações por liga**),
`Validate()` passa a **recusar** `opponent_tier` em vez de ignorá-lo, sincronização
para de gravar a constante, seletor removido da tela e `(G12)` removido do Dashboard.
Migration 014 preserva o valor antigo em `tier_legacy` e documenta as colunas — não
apaga nada e é reversível.

**Teste obrigatório: não executável hoje, e isso está registrado em vez de
contornado.** O enunciado pressupõe classificação "vigente na data", que não existe.
Escrever algo que passasse no lugar dele seria fabricar conformidade. O que está
testado é o comportamento seguro na ausência da classificação — inclusive uma guarda
que falha se o filtro voltar a depender de `teams.tier`.

**Efeito colateral favorável:** o espaço de busca do ciclo cai de ~5.940 para ~1.485,
e o limiar de FDR do AUD-003 afrouxa de 0,0108 para ~0,0134 **sem relaxar critério
nenhum** — o ciclo estava sendo punido por hipóteses que não existiam.

**Não resolvido, virou backlog de produto:** `team_season_tier` point-in-time. Três
obstáculos documentados, sendo o principal que **mata-mata não tem classificação** —
Copa do Brasil, Libertadores, Sul-Americana e as competições europeias que entram
agora.
**Detalhes:** `CORRECOES_AUDITORIA.md` § AUD-004

---

### AUD-005 — Qualquer usuário logado pode desativar descobertas públicas
**Severidade:** HIGH · **Categoria:** Segurança / Autorização
**Localização:** `strategy_repo.go:93`, `strategy_handler.go:179-202`

**Problema:** `SetFlags` executa `UPDATE strategies SET active=$2, favorite=$3 WHERE id=$1`
— **sem filtro de dono**. `ownedStrategy` autoriza estratégias públicas do sistema.

**Impacto:** um usuário autenticado pode chamar `PATCH /strategies/{id}` numa descoberta
pública e definir `active=false`, removendo-a do ranking **para todos**. Também pode
marcar `favorite` global.

**Correção recomendada:** `SetFlags` deve receber `ownerID` e filtrar `AND owner_id=$N`;
`ownedStrategy` deve ganhar um parâmetro que exija posse nas operações de escrita.

**Teste obrigatório:** usuário A tenta PATCH em estratégia do sistema → 403/404 e nenhuma
linha alterada.

**Prioridade:** P1

---

### AUD-006 — Drawdown normalizado por volume apostado afrouxa com a amostra
**Severidade:** HIGH · **Categoria:** Matemática
**Localização:** `discovery/criteria.go:drawdownPct`

**Problema:** `100 * MaxDrawdown / TotalStaked`. Como `TotalStaked` cresce linearmente
com o número de apostas, o mesmo drawdown absoluto produz percentual menor em amostras
maiores.

**Evidência numérica:** drawdown de 10 unidades → 10% em 100 apostas, **5%** em 200
apostas, mesmo risco real.

**Impacto:** o critério "drawdown ≤ 20%" fica progressivamente mais fácil, favorecendo
combinações de amostra grande independentemente do risco.

**Correção recomendada:** usar drawdown relativo ao pico da curva de capital
(`formulas.MaxDrawdown`, que já existe e não é usada) sobre uma banca simulada.

**Teste obrigatório:** duas séries com mesmo perfil de risco e tamanhos diferentes devem
produzir o mesmo drawdown percentual.

**Prioridade:** P1

---

### AUD-007 — Componentes de score derivados uns dos outros (informação fantasma)
**Severidade:** HIGH · **Categoria:** Matemática / Scores
**Localização:** `strategyengine/engine.go:163,220,233,234,235`

**Problema:** `invVar = 1 − 4·p·(1−p)` é função determinística de winRate — não é
informação nova. `ConfidenceScore` e `RobustnessScore` recebem `sampleNorm` duas vezes
(o 4º argumento é "robustez temporal", que não é calculada). `VolatilityScore` recebe
`absF(trend)` duas vezes.

**Impacto:** Confiança é 50% tamanho de amostra; Robustez 40%; e as telas apresentam
esses números como evidências independentes, comunicando mais suporte estatístico do
que existe.

**Correção recomendada:** calcular robustez temporal de verdade (desempenho em
subjanelas) ou remover o componente e reponderar.

**Teste obrigatório:** dois cenários com mesma amostra e estabilidades diferentes devem
produzir Robustez diferente.

**Prioridade:** P1

---

### AUD-008 — Tendência não usa janelas: TrendScore recebe o mesmo valor 3 vezes
**Severidade:** HIGH · **Categoria:** Matemática
**Localização:** `strategyengine/engine.go:199`

**Problema:** `formulas.TrendScore(dROI, dROI, dROI)`. Como os pesos somam 1
(0,5+0,3+0,2), o resultado é exatamente `dROI`. As janelas de 5/10/20 jogos não existem.

**Impacto:** "Tendência" exibida é apenas a variação de ROI entre duas execuções.
Alimenta `LifecycleStage` e `VolatilityScore`, propagando o erro.

**Correção recomendada:** calcular deltas reais por janela ou remover a métrica até
haver histórico suficiente.

**Prioridade:** P1

---

### AUD-009 — Camadas RAW e ANALYTICS vazias; workers nunca executaram
**Severidade:** HIGH · **Categoria:** Arquitetura / Dados
**Localização:** produção; `cmd/worker`, `render.yaml`

**Evidência:** `raw_fixtures` = 0, `raw_statistics` = 0, `team_metrics` = 0,
`worker_runs` = 0. Última sincronização registrada como "manual".

**Impacto:** a promessa de reprocessar tudo sem re-consultar a API não existe; as
métricas por equipe nunca foram pré-calculadas; nenhum dado foi produzido
automaticamente.

**Correção recomendada:** aplicar o `render.yaml` no painel do Render e confirmar
`DATABASE_URL`/`API_FOOTBALL_KEY` no serviço de cron; implementar a gravação na camada
RAW durante a ingestão.

**Teste obrigatório:** após um ciclo, `worker_runs` deve ganhar uma linha com status
`success` e `team_metrics` deve ser populada.

**Prioridade:** P1

---

### AUD-010 — Ausência de rate limiting permite força bruta em /auth/login
**Severidade:** HIGH · **Categoria:** Segurança
**Localização:** `internal/delivery/http/router.go` (nenhum middleware de limite)

**Impacto:** tentativas ilimitadas de senha por IP; também expõe endpoints caros
(`/filters/run`) a abuso.

**Correção recomendada:** middleware de rate limit por IP nas rotas de auth e por
usuário nas rotas de cálculo.

**Prioridade:** P1

---

### AUD-011 — Estratégia nova é rotulada "Maturidade" na primeira execução
**Severidade:** HIGH · **Categoria:** UX que afeta confiabilidade
**Localização:** `strategyengine/engine.go:185-199,238`

**Problema:** sem execução anterior, health = 50 e trend = 0. `LifecycleStage(n, 30, 50, 0)`
com n ≥ 30 cai no `default` → **"maturidade"**.

**Evidência:** tela de Estratégias exibindo "Ciclo de vida: Maturidade" com Health 50 e
duas execuções.

**Impacto:** comunica validação temporal que não ocorreu.

**Correção recomendada:** exigir mínimo de execuções (não de jogos) para sair de
"nascimento".

**Prioridade:** P1

---

### AUD-012 — Partida sem odd recebe odd 1.0 e gera ROI negativo sem significado
**Severidade:** HIGH · **Categoria:** Matemática
**Localização:** `filter_usecase.go:289-291`

**Problema:** `if !hasOdd { odd = 1.0 }`. Ganhar paga `stake*(1-1) = 0`; perder paga
`-stake`. O ROI resultante é estruturalmente negativo.

**Impacto:** no Simulador sem "Odds máximas", o usuário vê ROI negativo que não reflete
desempenho nenhum. Foi observado ROI −12% num backtest com 88% de acerto.

**Correção recomendada:** excluir a partida do cálculo financeiro e reportar
"sem odd disponível" em vez de fabricar odd neutra.

**Prioridade:** P1

**Status:** ✅ **RESOLVIDO** — 2026-09-09, junto com AUD-001
Corrigido no mesmo ramo de código do AUD-001 (`filter_usecase.go`, ramo de escanteios):
separá-lo exigiria editar as mesmas linhas duas vezes. `if !hasOdd { odd = 1.0 }` foi
substituído por `continue` — a partida sai do backtest.
**Teste:** `TestSemOdd_PartidaForaDoBacktest` — `MatchCount = 0`, `Profit = 0`,
`TotalStaked = 0`.
**Detalhes:** `CORRECOES_AUDITORIA.md` § AUD-001, Fase B item 4

---

### AUD-021 — Cada partida é contada duas vezes em métricas de total
**Severidade:** HIGH · **Categoria:** Estatística
**Localização:** `filter_usecase.go:213-224`

**Problema:** toda partida gera dois candidatos (mandante e visitante). Para métricas de
**total da partida** (escanteios totais > N), ambos têm resultado idêntico.

**Impacto:** a amostra reportada é o dobro da real; sequências de acerto/erro e drawdown
são calculados sobre observações duplicadas. Uma descoberta com "102 ocorrências" tem na
verdade ~51 partidas.

**Correção recomendada:** para métricas de total sem filtro de equipe, gerar um
candidato por partida.

**Teste obrigatório:** backtest de "over N escanteios" sem filtro de time deve reportar
`match_count` igual ao número de partidas.

**Prioridade:** P1

---

### AUD-022 — Varredura por equipe não gera combinações
**Severidade:** HIGH · **Categoria:** Bug
**Localização:** `discovery/engine.go:RunLeague`, `combinations.go:generateCombos`

**Evidência:** 5.940 = 11 × 540 exatamente. Com `IncludeTeams: true`, cada equipe
somaria 45 combinações; o total seria muito maior.

**Impacto:** descobertas específicas por equipe — as mais acionáveis — nunca são
produzidas.

**Causa provável:** `teamsRepo.List` retornando vazio dentro do ciclo (provável
interação com o cache por ciclo ou com os `seasonIDs` passados).

**Prioridade:** P1 · **NECESSITA INVESTIGAÇÃO** da causa exata.

---

### AUD-013 — openapi.yaml documenta 31 de 51 endpoints
**Severidade:** MEDIUM · **Categoria:** Documentação
**Correção:** gerar o spec do router, como já é feito em `aidocs`. **Prioridade:** P2

### AUD-014 — CORS liberado para qualquer origem; PATCH ausente
**Severidade:** MEDIUM · **Categoria:** Segurança
**Localização:** `router.go:176-184`. Auth por header mitiga CSRF, mas é mais permissivo
que o necessário. **Prioridade:** P2

### AUD-015 — Ordenação O(n²) no caminho quente do Discovery
**Severidade:** MEDIUM · **Categoria:** Performance
**Localização:** `filter_usecase.go:434`. Trocar por `sort.Slice`. **Prioridade:** P2

### AUD-016 — Backtest não determinístico com cap de histórico
**Severidade:** MEDIUM · **Categoria:** Matemática
**Localização:** `filter_usecase.go:174` usa `time.Now()`. O mesmo backtest em dias
diferentes retorna resultados diferentes. **Prioridade:** P2

### AUD-017 — Frontend formata datas no timezone do navegador
**Severidade:** MEDIUM · **Categoria:** Timezone. Ver seção 16. **Prioridade:** P2

### AUD-018 — "Estratégia descoberta" sugere modelo preditivo inexistente
**Severidade:** MEDIUM · **Categoria:** UX / Consistência
O sistema testa regras estáticas, não prevê. `LastNGames` filtra os N jogos mais
recentes do conjunto, não a forma no momento da aposta. **Prioridade:** P2

### AUD-019 — Redis configurado sem uso localizado
**Severidade:** LOW · **NECESSITA INVESTIGAÇÃO**. **Prioridade:** P3

### AUD-020 — Escala inconsistente no Health Score
**Severidade:** MEDIUM · `dCons` é diferença bruta de proporções, sem normalização por
teto, enquanto `dROI` e `dEV` são divididos por caps. **Prioridade:** P2

### AUD-023 — Motor de backtest sem nenhum teste
**Severidade:** HIGH · **Categoria:** Testes
`filter_usecase.go` é o componente mais crítico e não tem teste unitário.
**Prioridade:** P1

### AUD-024 — Sem alertas de observabilidade
**Severidade:** MEDIUM · A sincronização ficou 22 dias parada sem notificação.
**Prioridade:** P2

### AUD-025 — Migrations aplicadas manualmente
**Severidade:** MEDIUM · Risco de deploy com schema defasado. **Prioridade:** P2

---

## 32. Matriz de Risco

| ID | Categoria | Severidade | Problema | Impacto | Prioridade |
|---|---|---|---|---|---|
| AUD-001 | Estatística | CRITICAL | Odds sintéticas causam leakage | Invalida todo backtest financeiro | P0 |
| AUD-002 | Matemática | CRITICAL | ROI=Yield=EV; 50% do DSFR | Score e ranking enviesados | P0 |
| AUD-003 | Estatística | CRITICAL | 5.940 testes sem correção | Descobertas por acaso | P0 |
| AUD-004 | Dados | CRITICAL | Tier quebrado + look-ahead | Metade dos combos morta | P0 |
| AUD-005 | Segurança | HIGH | PATCH sem escopo de dono | Vandalismo em recurso público | P1 |
| AUD-006 | Matemática | HIGH | Drawdown por volume | Critério afrouxa com amostra | P1 |
| AUD-007 | Matemática | HIGH | Componentes derivados | Falsa independência | P1 |
| AUD-008 | Matemática | HIGH | Trend sem janelas | Métrica não é o que diz | P1 |
| AUD-009 | Arquitetura | HIGH | RAW/analytics vazias | Promessas não cumpridas | P1 |
| AUD-010 | Segurança | HIGH | Sem rate limit | Força bruta | P1 |
| AUD-011 | UX | HIGH | "Maturidade" na 1ª execução | Falsa validação | P1 |
| AUD-012 | Matemática | HIGH | Odd 1.0 fabricada | ROI sem significado | P1 |
| AUD-021 | Estatística | HIGH | Partida contada 2× | Amostra inflada 2× | P1 |
| AUD-022 | Bug | HIGH | Varredura por equipe vazia | Perde melhores descobertas | P1 |
| AUD-023 | Testes | HIGH | Backtest sem teste | Regressão silenciosa | P1 |
| AUD-013 | Documentação | MEDIUM | Spec desatualizado | Integração errada | P2 |
| AUD-014 | Segurança | MEDIUM | CORS aberto | Hardening | P2 |
| AUD-015 | Performance | MEDIUM | Sort O(n²) | Ciclo lento | P2 |
| AUD-016 | Matemática | MEDIUM | Não determinístico | Irreprodutível | P2 |
| AUD-017 | Timezone | MEDIUM | Data no fuso do browser | Deslocamento de dia | P2 |
| AUD-018 | Consistência | MEDIUM | Nome sugere predição | Expectativa errada | P2 |
| AUD-020 | Matemática | MEDIUM | Escala do Health | Componente desproporcional | P2 |
| AUD-024 | Observabilidade | MEDIUM | Sem alertas | Falha silenciosa | P2 |
| AUD-025 | Deploy | MEDIUM | Migration manual | Schema defasado | P2 |
| AUD-019 | Arquitetura | LOW | Redis sem uso | Código morto | P3 |

---

## 33. Score por Área

| Área | Nota | Justificativa |
|---|---|---|
| Arquitetura | 70 | Camadas bem desenhadas e separação correta entre escrita e leitura; perde por camadas prometidas e vazias e por um motor documentado que não existe |
| Dados | 35 | FKs íntegras e schema coerente, mas tier corrompido, RAW vazia e odds sintéticas indistinguíveis de reais no banco |
| Matemática | 40 | O pacote `formulas` é correto e bem testado; a nota cai pelo que é feito **com** ele: ROI=Yield=EV, drawdown mal normalizado, trend degenerada |
| Estatística | 15 | Leakage, múltiplas comparações sem correção, sem validação fora da amostra, amostra duplicada. É a área mais frágil |
| Backtest | 25 | Mecânica correta (P/L, sequências, ordenação), mas alimentado por odds inválidas, sem testes e com dupla contagem |
| Estratégias | 55 | Reuso do mesmo motor garante reprodutibilidade — acerto real; perde por semântica enganosa |
| Discovery | 20 | Engenharia sólida (idempotência, resiliência, guarda anti-overfitting) desperdiçada sobre dados inválidos e metade do espaço morto |
| Scores | 25 | Estrutura e pesos documentados, mas com dupla/tripla contagem que descaracteriza o resultado |
| IA | 75 | Melhor área do sistema: duas camadas de proteção, snapshot auditável; perde por não ter testes |
| API | 70 | Boa estrutura de grupos, 202/409 em tarefas longas, 404 anti-enumeração; perde por spec desatualizado e falta de paginação |
| Segurança | 50 | SQLi e senha corretos, JWT ok; perde por rate limit ausente e escrita sem escopo de dono |
| Performance | 60 | Cache por ciclo no Discovery é acerto; perde por sort O(n²) e dupla expansão de candidatos |
| Frontend | 75 | Sem mocks, estados tratados, números fiéis ao backend; perde por timezone e por comunicar independência inexistente entre métricas |
| Testes | 45 | Testes de fórmula bons; zero cobertura no backtest, integração, timezone e segurança |
| Deploy | 55 | Dockerfile e blueprint corrigidos, health check ok; perde por migration manual e cron não aplicado |
| Observabilidade | 40 | Bom desenho de `worker_runs` e `api_usage_log`; perde por tabela vazia e ausência total de alertas |
| Documentação | 65 | Volume e intenção acima da média, `aidocs` é o padrão certo; perde pelas divergências da seção 23 |

**NOTA GERAL: 46 / 100**

Pela regra do próprio escopo desta auditoria ("um sistema não pode receber nota alta se
possuir problema CRITICAL em matemática ou backtest"), a nota está limitada pelos quatro
CRITICAL.

---

## 34. Classificação Final

## **D — NÃO CONFIÁVEL**

Justificativa: o sistema publica, com rótulo "Elite" e score 100, estratégias cujo
desempenho é artefato de como as odds foram geradas. Um usuário que confie nesses
números está tomando decisão sobre ruído apresentado como evidência. Os critérios de
aprovação existem e são rigorosos no papel, mas operam sobre entrada inválida — o que é
pior do que não ter critério, porque produz confiança injustificada.

Isso **não é julgamento sobre a qualidade da engenharia**, que é boa. É julgamento sobre
a confiabilidade do produto no domínio a que ele se propõe.

Para uso como **estatística descritiva** (médias, frequências, comparador, calendário), o
sistema é confiável e pode continuar em produção. Para uso como **ferramenta de decisão
quantitativa**, não.

---

## 35. Plano de Correção

### FASE 1 — CRITICAL (P0)

**1.1 Marcar e isolar odds sintéticas (AUD-001)**
Arquivos: migration nova, `oddsgen.go`, `sync_usecase.go`, `filter_usecase.go`,
`discovery/combinations.go`, frontend.
Solução: coluna `odds_source`; Discovery ignora sintéticas; Simulador exibe aviso e
suprime ROI/EV. Despublicar as 5 descobertas atuais.
Risco: alto impacto visível — o ranking ficará vazio. **É o resultado correto.**
Teste: com odds sintéticas, publicar zero.

**1.2 Separar ROI, Yield e EV (AUD-002)**
Arquivos: `filter_usecase.go`, `strategyengine/engine.go`, `formulas/scores.go`, catálogo.
Solução: Yield sobre volume real; EV a partir de probabilidade estimada; ou remover
ambos do DSFR e reponderar. Exige `Version = "1.1"`.
Risco: muda todos os scores históricos — requer recálculo e nota de versão.

**1.3 Validação fora da amostra (AUD-003)**
Solução: split temporal (descoberta em N−1 temporadas, validação na última); publicar só
o que sobrevive; registrar nº de testes.

**1.4 Corrigir tier (AUD-004)**
Solução: `team_season_tier` calculada por classificação; limpar valor `"1"`; até lá,
remover o eixo de tier do Discovery.

### FASE 2 — HIGH (P1)
AUD-005 (escopo de dono no SetFlags) · AUD-021 (candidato único por partida) ·
AUD-012 (excluir partida sem odd) · AUD-006 (drawdown relativo) · AUD-023 (testes do
backtest) · AUD-022 (investigar varredura por equipe) · AUD-007 e AUD-008 (componentes
reais) · AUD-011 (lifecycle) · AUD-009 (aplicar cron, popular RAW) · AUD-010 (rate limit).

### FASE 3 — MEDIUM (P2)
AUD-013, AUD-014, AUD-015, AUD-016, AUD-017, AUD-018, AUD-020, AUD-024, AUD-025.

### FASE 4 — LOW (P3)
AUD-019.

### FASE 5 — MELHORIAS (não são bugs)
Paginação nas listagens · cache no `/diagnostics/usage` · CI/CD · tracing e métricas ·
Opportunity Engine (se ainda desejado) · testes E2E.

---

## 36. Itens que NÃO puderam ser auditados

| Item | Motivo |
|---|---|
| Webhooks do Stripe (idempotência, cancelamento, renovação) | Exige eventos de teste contra o ambiente; fora do escopo de leitura |
| Comportamento do Redis em runtime | Nenhuma instância acessível; uso não localizado no código |
| Testes de penetração ativos (XSS, SSRF, brute force real) | Exigem ataque contra produção — não autorizado |
| Teste de carga e limites de performance reais | Exige ambiente de staging |
| E2E de frontend em múltiplos viewports | Ferramenta de redimensionamento não alterou o viewport efetivo |
| Precisão dos dados da API-Football contra a fonte oficial | Exige acesso a fonte independente para conferência |
| Comportamento com partida cancelada/anulada | Nenhum caso presente no banco atual |
| Graceful shutdown da API | Não verificado em execução |

---

## 37. Conclusão

**Contagem de problemas:**

| Severidade | Quantidade |
|---|---|
| CRITICAL | **4** |
| HIGH | **11** |
| MEDIUM | **9** |
| LOW | **1** |
| INFO | 0 |
| Não auditados | **8** |

**NOTA GERAL: 46/100**
**CLASSIFICAÇÃO FINAL: D — NÃO CONFIÁVEL** (para decisão quantitativa)

### TOP 10 PROBLEMAS MAIS IMPORTANTES

1. **AUD-001** — Odds sintéticas derivadas da amostra causam leakage; as descobertas com 100% de acerto são artefato
2. **AUD-002** — ROI, Yield e EV são o mesmo número; metade do DSFR é uma variável só
3. **AUD-003** — 5.940 testes sem correção para múltiplas comparações nem validação fora da amostra
4. **AUD-004** — Tier do adversário corrompido (0 times G6/Z4) e com look-ahead estrutural
5. **AUD-021** — Cada partida contada duas vezes: amostra reportada é o dobro da real
6. **AUD-005** — Qualquer usuário logado desativa descobertas públicas
7. **AUD-009** — Camadas RAW e analytics vazias; workers nunca executaram
8. **AUD-023** — O motor de backtest não tem nenhum teste
9. **AUD-006** — Drawdown normalizado por volume afrouxa o critério conforme a amostra cresce
10. **AUD-010** — Ausência de rate limiting permite força bruta no login

### TOP 10 AÇÕES RECOMENDADAS

1. Despublicar as descobertas atuais e marcar odds sintéticas no banco
2. Fazer o Discovery ignorar partidas sem odd real de mercado
3. Separar ROI, Yield e EV — ou remover a redundância do DSFR (Catálogo v1.1)
4. Implementar validação fora da amostra antes de qualquer publicação
5. Corrigir `teams.tier` com dimensão temporal; remover o eixo até lá
6. Gerar um candidato por partida em métricas de total
7. Adicionar escopo de dono no `SetFlags`
8. Escrever a suíte de testes do `filter_usecase.go`
9. Aplicar o `render.yaml` e confirmar as variáveis do cron no Render
10. Adicionar rate limiting nas rotas de autenticação e de cálculo

### Observação final

Vale registrar o que está **certo**, porque é substancial: o pacote de fórmulas é
matematicamente correto e bem testado; a decisão de reusar o mesmo motor de backtest
entre Simulador e Discovery garante reprodutibilidade real; as proteções da IA contra
recomendação de aposta são as melhores que vi neste tipo de produto; e a trava
anti-overfitting de 50 jogos mostra que a preocupação estatística existia no desenho.

O problema é que essas defesas foram construídas **em cima de uma entrada inválida**. A
correção do AUD-001 é a que destrava tudo: com odds reais, os critérios rigorosos que já
existem passam a fazer o trabalho para o qual foram escritos — e o resultado honesto,
provavelmente, será publicar muito menos estratégias, ou nenhuma. Isso não é falha do
sistema; é o sistema finalmente dizendo a verdade.

**Pode iniciar a correção?**
