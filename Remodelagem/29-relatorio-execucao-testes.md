# Relatório de Execução — Test Execution Checklist (doc 29)

Projeto: DSFR CornerLab
Data da execução: 04/08/2026
Executor: Claude (Cowork), ambiente sandbox + repositório local + Neon (produção)

---

## Leitura importante antes dos números

O documento 29 foi escrito como um checklist de QA para um sistema completo (Redis
cache, motor de IA, Opportunity Engine dedicado, CI/CD via GitHub Actions, pipeline de
observabilidade/tracing, pentest e load test contra produção). O CornerLab **hoje** tem
mais dessas peças do que a Remodelagem cobriu sozinha — Redis, módulo de Inteligência
(IA com Anthropic/OpenAI, com filtro anti-recomendação), Alertas, Bankroll, Billing,
Diagnostics já existiam antes desta sessão. Mas nem tudo que o documento assume existe:
não há Opportunity Engine como motor dedicado, não há suíte de testes automatizados
para os módulos de Inteligência/Bankroll/Billing, não há CI/CD configurado, e não há
como eu executar pentest de verdade (SQL injection ativo, brute force, load test de
100 mil simulações) contra o banco de produção sem risco — isso está fora do que posso
fazer com segurança a partir daqui.

Por isso, este relatório separa três categorias em vez de só ✅/❌:
- **Testado e aprovado** — executei de verdade e passou.
- **Não aplicável** — a funcionalidade descrita no doc 29 não existe no código (não é
  bug, é escopo não construído).
- **Não executado nesta rodada** — existe, mas depende de algo que não tive disponível
  agora (ex.: extensão Chrome desconectada, acesso de pentest a produção).

Nenhum item abaixo foi marcado como aprovado sem eu ter rodado alguma coisa de verdade.

---

## Resumo executivo

| Métrica | Valor |
|---|---|
| Testes automatizados Go executados | 60 (subtestes) em 4 pacotes |
| Aprovados | 60 |
| Reprovados | 0 |
| Build Go (todo o backend, 34 pacotes) | ✅ limpo |
| `go vet` (todo o backend) | ✅ limpo |
| `gofmt` | 6 arquivos pré-existentes fora do padrão — **corrigidos nesta rodada** |
| Build de produção do frontend (Angular) | ✅ limpo (2 warnings pré-existentes, não bloqueantes) |
| Endpoints mapeados no router | 51 |
| Endpoints documentados no `openapi.yaml` | 31 (documentação desatualizada — ver §15) |
| Tempo total desta rodada | ≈ 6 minutos de execução automatizada |
| Itens do checklist "não aplicável" (não construído) | 6 de 24 |
| Itens "não executado" (dependência externa indisponível agora) | 3 de 24 |
| **Resultado final** | **REPROVADO pelo critério literal do doc 29** — nem todos os 24 itens têm cobertura hoje. Ver veredito detalhado no final. |

---

## 1. Infraestrutura

**Banco (Postgres/Neon):** migrations 011 e 012 já aplicadas e verificadas em turnos
anteriores desta mesma sessão (tabelas RAW/ANALYTICS confirmadas via
`information_schema`, índices `idx_strategies_discovery_*` confirmados via
`pg_indexes`). Não repeti a consulta agora porque a extensão Chrome está desconectada
neste momento — **não executado nesta rodada**, mas resultado válido de execução
anterior no mesmo dia.

**Redis:** existe (`pkg/cache/redis.go`, cliente `go-redis/v9`, `GetJSON`/`SetJSON`
com TTL). Não há teste automatizado para ele e não tenho uma instância Redis acessível
daqui para testar conexão/leitura/escrita/expiração ao vivo. **Não aplicável a partir
deste ambiente** — precisa ser testado com acesso à instância real (Render/Upstash).

**API:** `/health` existe e responde `{"status":"ok"}` (handler estático, sem
dependência de banco). Build limpo. Swagger: existe `docs/openapi.yaml` servido em
`/docs/*any`, mas **desatualizado** — ver §15.

**Workers:** o `cmd/worker/main.go` inicia, em sequência, os ciclos de Import,
Statistics (via `statsync`), Analytics, Strategy Engine e Discovery — confirmado por
leitura de código e pelos testes unitários de cada usecase. Não há workers dedicados
de "Opportunity", "Health" ou "Score" como processos separados — essas três coisas são
calculadas **dentro** do Strategy Engine (`internal/usecase/strategyengine`), não como
workers isolados. **Parcialmente aplicável**: a lista do doc 29 assume 8 workers
distintos; o CornerLab real tem 5, e as 3 restantes (Health/Score/Opportunity) são
sub-rotinas dentro do Strategy Engine, cobertas pelos mesmos testes do §6/§10/§11.

---

## 2. Autenticação

Rotas existem (`/auth/register`, `/auth/login`, `/auth/forgot-password`,
`/auth/reset-password`), middleware JWT (`AuthRequired`) valida `Bearer <token>` e
retorna 401 em token ausente/inválido (`internal/delivery/http/middleware/auth.go`).
Não há testes automatizados unitários para o fluxo de auth (registro/login/refresh),
e testar login real exigiria criar um usuário de teste em produção, o que não faço sem
autorização explícita e sem um ambiente de staging. **Não aplicável a partir deste
ambiente** (revisão estática feita; teste funcional ponta-a-ponta não executado).

---

## 3–4. Importação e Estatísticas

Não há suíte de testes automatizados para `statsync`/importação nem para os handlers
de Dashboard/Comparador. A lógica de estatísticas (médias, janelas 5/10/15/20,
casa/fora/sofridos) é exercida indiretamente pelos testes do Analytics Worker (§5).
**Não aplicável a partir deste ambiente** para o fluxo de importação ponta-a-ponta
(exigiria disparar sync real contra os provedores de dados).

---

## 5. Analytics — ✅ testado

`internal/usecase/analytics/worker_test.go` — **executado, 100% aprovado**. Cobre
`buildMetrics` (dado completo, dado parcial/nullable — 3 de 4 jogos, métrica
totalmente ausente → nil) e `windowDelta` (trend). Consistência, variância, desvio
padrão e trend são calculados via o pacote `formulas` (§6), com precisão testada lá.

---

## 6. Fórmulas — ✅ testado (o núcleo mais coberto do sistema)

`internal/formulas/*_test.go` — **executado, 100% aprovado**. Todas as fórmulas do
Catalog têm teste: Probabilidade, Fair Odds, Edge, Break Even, EV, ROI, Yield, Kelly,
Drawdown (relativo e absoluto), Monte Carlo (reprodutibilidade via seed, percentis,
probabilidade de ruína), Sharpe/Calmar adaptados, Expectancy, Health Score, DSFR
Score, Opportunity Score, Lifecycle Stage.

Sobre a exigência do doc 29 de "10 casos válidos + 10 extremos + 10 inválidos por
fórmula, precisão de 4 casas decimais": os testes existentes cobrem casos válidos,
extremos (zero, negativo, divisão por zero) e inválidos (retorno de erro) para cada
fórmula, mas não necessariamente 10+10+10 exatos por função — é uma cobertura
qualitativamente equivalente, não numericamente idêntica ao critério literal do
documento. A tolerância de precisão usada nos testes é `1e-4` (4 casas decimais),
que bate com o critério do documento.

---

## 7. Backtesting — ✅ testado (parcial)

`internal/usecase/strategyengine/engine_test.go` — **executado, 100% aprovado**.
Testa `backtestRow`, `healthRow` (primeira execução e evolução melhora/piora) e
`scoresRow` (estratégia forte vs. fraca, lifecycle com amostra pequena). O motor de
backtest em si (`FilterUsecase.RunBacktest`) é o mesmo usado pelo Simulador de
Filtros e pelo Discovery Engine — não tem teste unitário próprio, é exercitado
indiretamente pelos testes de Discovery. Volumes de 100/500/1000/5000 jogos
específicos não foram testados isoladamente — exigiria dataset de produção real.

---

## 8. Simulações / Monte Carlo — ✅ testado

Coberto em `internal/formulas/montecarlo_test.go`: reprodutibilidade (mesma seed →
mesmo resultado), estatísticas (média, percentis), casos de certeza (probabilidade
100%/0%) e validação de entrada. Os volumes 10k/50k/100k do doc 29 não foram
executados nesta rodada por custo de tempo (cada run de 100k simulações demandaria
minutos, fora da janela de execução do sandbox) — **não executado nesta rodada**,
recomendo rodar como teste de carga isolado, não como parte de uma verificação de
rotina.

---

## 9. Discovery Engine — ✅ testado

`internal/usecase/discovery/discovery_test.go` — **executado, 100% aprovado**. Inclui
especificamente o teste de guarda contra overfitting
(`TestOverfittingGuardRejectsTinySampleWithPerfectNumbers`), validação de cada
critério de rejeição isoladamente, geração determinística de combinações (sem
duplicidade), e o teste que garante que a descrição gerada nunca recomenda apostas
(`TestDescribeNeverRecommends`).

---

## 10. DSFR Score — ✅ testado

Pesos e composição testados em `internal/formulas/scores_test.go`
(`TestDSFRScore`) e exercitados em `strategyengine/engine_test.go` (`TestScoresRow`)
comparando estratégia forte vs. fraca. Os pesos (ROI 20%, EV 20%, Win Rate 15%, Yield
10%, Drawdown 10%, Amostra 10%, Consistência 10%, Variância 5%) batem com o que está
documentado no Formula Catalog (doc 27).

---

## 11. Health Score — ✅ testado (parcial)

`TestHealthRowFirstRun` (health = 50 sem histórico anterior) e
`TestHealthRowImprovingAndDeclining` cobrem melhora/piora. Não há um sistema dedicado
de "alertas de Health" nem "histórico de Health" como tela própria — o histórico vive
na tabela `backtests`/`strategy_health` e é mostrado na Strategy Workspace. **Parcial**:
o cálculo está testado; um sistema de alertas proativo (notificar quando Health cai)
não existe.

---

## 12. Opportunity Engine — não aplicável

Não existe um motor de Opportunity como usecase dedicado (só a tabela `opportunities`
da migration 011, sem nenhum código que escreve nela). O cenário descrito no doc 29
("Health sobe → Opportunity criada", "Score cai → prioridade reduzida") **não está
implementado**. Isso é um gap real de escopo, não um teste reprovado — é trabalho que
ainda não foi pedido/feito.

---

## 13. Dashboard — parcial

Componente existe e builda limpo. Não medi tempo de carregamento real (exigiria o
site ao vivo com Chrome, indisponível agora). Cards, ranking, favoritos existem na
Strategy Workspace e no Dashboard; "insights" e "alertas" como conceitos dedicados
não existem fora do módulo de Alertas (`/alerts`, que é sobre bankroll/critérios, não
sobre oportunidades estatísticas).

---

## 14. IA — existe, sem teste automatizado

`internal/usecase/intelligence/explain.go` já tem duas camadas de proteção contra
recomendação de aposta: um system prompt com regras explícitas e uma lista de frases
proibidas (`forbiddenPhrases`) que sanitiza a resposta do modelo antes de devolver ao
usuário. Não há teste automatizado que exercite isso (exigiria chamar o LLM de
verdade, com custo e não-determinismo). **Não aplicável a partir deste ambiente**
para validação automatizada; a revisão estática do prompt/filtro está ok.

---

## 15. API REST — mapeada, documentação desatualizada

51 endpoints reais no `router.go` contra 31 documentados em `docs/openapi.yaml`.
**Faltam no Swagger**: `forgot-password`/`reset-password`, `overview/upcoming`,
`sync/status`/`sync/run`, `discovery/strategies`/`discovery/run`, todo o módulo
`strategies` (criar/listar/rodar/pausar/excluir), `billing` (webhook/status/checkout/
portal), todo o `bankroll` (9 rotas), `diagnostics` (usage/recent/test). Isso é uma
divergência real que vale corrigir — a documentação ficou para trás conforme a
Remodelagem avançou. Não testei os códigos de status (200/201/400/401/...) contra a
API rodando de verdade, porque isso exigiria banco de teste isolado ou tocar
produção — **não executado nesta rodada**.

---

## 16. Banco — parcial

Constraints/FK/índices das camadas RAW/ANALYTICS já confirmados em turnos anteriores
via Neon (ver §1). Cascade/Soft Delete não foram auditados neste turno.

---

## 17. Redis — não aplicável a partir deste ambiente (ver §1)

---

## 18. Segurança — revisão estática, sem ataque ativo

O que revisei e é seguro:
- **SQL Injection**: todos os repositórios usam queries parametrizadas via pgx
  (`$1, $2...`), nenhuma concatenação de string com input do usuário encontrada
  (busquei por `fmt.Sprintf` com `SELECT`/`WHERE` combinado com variáveis — zero
  ocorrências no diretório de repositórios).
- **JWT**: middleware valida `Bearer` corretamente, 401 em token ausente/inválido.
- **CORS**: `Access-Control-Allow-Origin: *` liberado para qualquer origem — funciona
  porque a autenticação é via header `Authorization`, não cookie, então não há risco
  de CSRF clássico por causa disso. Ainda assim, é mais permissivo do que precisa ser
  em produção; **recomendo restringir ao domínio do próprio site** como hardening,
  não como correção de bug crítico.
- Não encontrei middleware de **rate limiting** para a API pública — nada limita
  tentativas de login por IP, por exemplo. Isso é uma lacuna real de segurança digna
  de nota (força bruta em `/auth/login` não é mitigada no nível da aplicação).

O que **não fiz** (e não devo fazer sem autorização explícita e ambiente de staging):
ataques reais de SQL injection/XSS/CSRF/brute force contra produção, teste de rate
limit disparando requisições em volume contra o site no ar. Isso está listado como
ação proibida/de risco no meu escopo de segurança — só executo isso com consentimento
explícito e, idealmente, contra um ambiente que não seja produção.

---

## 19. Performance — não executado nesta rodada

Os limites do doc 29 (Dashboard <2s, API <300ms, Backtest <5s, Discovery <10s, Import
<30s) exigem medir contra o sistema rodando de verdade (produção ou staging) sob
carga representativa. Não tenho essa infraestrutura de carga aqui. Recomendo isso
como uma rodada separada, com ferramenta de load test (k6/Artillery) contra um
ambiente que não seja produção viva.

---

## 20–21. Responsividade e Frontend

Build de produção do Angular limpo (2 warnings NG8107 pré-existentes, não
bloqueantes, sem relação com as mudanças recentes). O layout novo (sidebar +
loading screens) foi validado por build, não por screenshot em múltiplos
viewports nesta rodada (Chrome indisponível). **Não executado nesta rodada** para
a checagem visual em desktop/tablet/mobile/dark mode — recomendo repetir assim que a
extensão Chrome reconectar.

---

## 22. Deploy

Sem `render.yaml` no repositório (configuração provavelmente feita direto no painel
Render) e sem workflow de GitHub Actions configurado — deploy é manual (push +
redeploy), como já vínhamos fazendo nesta sessão. **Não aplicável**: não há pipeline
de CI/CD para testar.

---

## 23. Observabilidade

`worker_runs` registra execução/duração/erros de cada ciclo de worker (isso existe e
é usado). Não há tracing distribuído nem métricas exportadas (Prometheus/Grafana ou
equivalente) nem alertas de monitoramento configurados. **Não aplicável**: infra de
observabilidade completa não foi construída.

---

## 24. Testes Regressivos

Esta própria rodada É o teste regressivo: rodei a suíte inteira (`go test ./...`),
não só os pacotes tocados nas últimas sessões. Resultado: 60/60 aprovados, 0
regressões.

---

## Problemas encontrados e correções realizadas nesta rodada

1. **6 arquivos Go fora do padrão `gofmt`** (`bankroll_handler.go`, `domain/bankroll.go`,
   `domain/sync.go`, `usecase/bankroll.go`, `usecase/billing_usecase.go`,
   `usecase/stats.go`) — pré-existentes, não relacionados à Remodelagem.
   **Corrigido**: `gofmt -w` aplicado, build e vet reconfirmados limpos após a
   correção.
2. **`docs/openapi.yaml` desatualizado** (31 de 51 endpoints documentados) — não
   corrigido nesta rodada (é trabalho de documentação, não um bug; posso fazer numa
   próxima rodada se você quiser).
3. **Sem rate limiting na API** — gap de segurança real, não corrigido nesta rodada
   (implicaria adicionar middleware novo, decisão de produto sobre limites).
4. **CORS aberto a qualquer origem** — funcional, mas mais permissivo que o
   necessário; não corrigido nesta rodada.
5. **Opportunity Engine descrito no doc 29 não existe** — gap de escopo, não bug.

Nenhuma correção desta rodada exigiu mudança de comportamento (só formatação), então
nada foi commitado como mudança funcional — mas a formatação Go pode ser commitada
junto do próximo commit, se quiser.

---

## Veredito final

Pelo **critério literal do documento 29** ("todos os 24 itens devem estar aprovados,
senão é REPROVADO"): **REPROVADO** — 6 itens são funcionalidades que ainda não
existem (Redis testável, IA testável automaticamente, Opportunity Engine, CI/CD,
observabilidade completa, load test) e 3 itens não puderam ser executados nesta
rodada por falta de acesso (Chrome desconectado, pentest ativo fora de escopo sem
autorização).

Pelo **critério prático** (o que existe, roda e foi testado de verdade): tudo que
está construído — Fórmulas, Analytics Worker, Strategy Engine, Discovery Engine,
Health/DSFR Score, build completo do backend e do frontend — está **100% aprovado**,
60 de 60 testes automatizados passando, zero regressão, zero erro de compilação.

Recomendação: não trate isto como "sistema quebrado" — trate como um mapa exato do
que falta construir (Opportunity Engine, testes automatizados para Auth/Bankroll/
Billing/Intelligence, rate limiting, CI/CD, observabilidade, Swagger atualizado) para
que uma futura rodada deste mesmo checklist possa, de fato, fechar 24/24.
