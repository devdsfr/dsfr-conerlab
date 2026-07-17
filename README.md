# CornerLab

Plataforma de inteligência estatística, backtesting e gestão de estratégia para o
mercado de **escanteios**. A plataforma **nunca recomenda apostas** — organiza dados
históricos, calcula estatísticas, simula cenários financeiros hipotéticos e mede o
desempenho de filtros definidos pelo usuário.

## Escopo desta entrega

Fundação completa (backend, banco de dados, auth, Docker, Swagger), os três módulos
centrais do MVP e o **Módulo de Inteligência Estatística** completo no backend:

- **Módulo 1 — Dashboard Principal**: estatísticas de escanteios por equipe (média,
  desvio padrão, mediana, moda, frequências "acima de N", tendência, casa x fora).
- **Módulo 2 — Comparador**: comparação estatística entre duas equipes.
- **Módulo 3 — Simulador de Filtros**: motor de backtesting configurável (últimos N
  jogos, casa/fora, "acima de X escanteios", força do adversário, odds máximas),
  com taxa de acerto, ROI, yield, lucro, sequências e drawdown.
- **Módulo 5 (parcial) — Gestão de apostas**: cadastro de apostas e dashboard
  financeiro básico (Win Rate, ROI, Yield, lucro líquido/bruto).
- **Módulo de Inteligência Estatística** (motor completo no backend, endpoints
  `/api/v1/intelligence/*`; telas de frontend ainda não implementadas):
  - Índice de consistência por linha de escanteios (threshold).
  - Análise de tendência (últimos 5 x últimos 10 jogos), com detecção de queda.
  - Desempenho casa x fora.
  - Análise de adversário e posição no ranking de escanteios sofridos da liga.
  - Índice de Estabilidade (0–100).
  - Cruzamento estatístico (compatibilidade) entre duas equipes.
  - Score Estatístico ponderado (0–100): consistência 35%, forma recente 20%,
    equilíbrio casa/fora 15%, força do adversário 15%, escanteios sofridos pelo
    adversário 15%.
  - Explicações em linguagem analítica via IA (Anthropic), restritas exclusivamente
    aos dados armazenados, com filtro de segurança em duas camadas (prompt +
    verificação de frases proibidas) contra qualquer linguagem de recomendação.
  - Rankings automáticos (mais consistentes, mais/menos escanteios sofridos,
    maior crescimento, maior queda) e Dashboard Executivo (top 10 por dimensão).
  - Alertas Inteligentes (regras por frequência da equipe ou média do adversário,
    avaliação sob demanda).
  - Histórico da Estratégia: toda execução de filtro/backtest com um usuário
    autenticado é registrada automaticamente.
  - Exportação CSV/Excel de Dashboard, Comparador, Backtest e Ranking.
  - Cache Redis (TTL 24h) e rastreabilidade (`meta`: liga, temporadas, jogos
    analisados, data de atualização, se veio de cache) em toda resposta.
  - Integração com dados reais via **API-Football** e **SportMonks** (com
    fallback entre os dois), comando `cmd/sync`.
- **Autenticação JWT**, catálogo de campeonatos/temporadas/equipes, Swagger/OpenAPI.

Os módulos restantes do documento de requisitos original (Simulador Financeiro com
Monte Carlo, Estatísticas Avançadas, telas de frontend para o Módulo de Inteligência)
ainda não foram implementados — a arquitetura (Clean Architecture no backend,
componentes standalone no frontend) foi pensada para que sejam adicionados sem
retrabalho.

## Restrições respeitadas pelo Módulo de Inteligência Estatística

O sistema **não** recomenda apostas, **não** prevê resultados futuros, **não**
promete lucros e **não** usa linguagem de garantia — em nenhum endpoint, incluindo as
explicações geradas por IA. Todo insight é fundamentado exclusivamente em dados
históricos armazenados, e cada resposta traz o período/jogos analisados para
rastreabilidade.

## Stack

- **Backend**: Go 1.25, Gin, Clean Architecture, PostgreSQL (pgx), Redis, JWT,
  Swagger/OpenAPI, integração com Anthropic (explicações) e API-Football/SportMonks
  (dados reais).
- **Frontend**: Angular 20 (standalone components), Angular Material, Tailwind CSS, Chart.js.
- **Infra**: Docker Compose, migrations SQL puro (sem ORM).

## Como rodar (Docker Compose)

```bash
docker-compose up --build
```

- Backend: http://localhost:8080 (health check em `/health`, documentação em `/docs`)
- Frontend: http://localhost:4200
- Postgres: localhost:5432 (usuário/senha/banco: `cornerlab`)

O schema é criado automaticamente na primeira subida do Postgres (via
`backend/migrations`). Para popular o banco com dados de exemplo (times, um
campeonato fictício, 4 temporadas, ~1.500 partidas com escanteios e odds
sintéticas):

```bash
docker-compose exec backend /app/seed
```

Isso também cria um usuário de teste: `demo@cornerlab.app` / senha `demo12345`
(necessário apenas para salvar filtros, registrar apostas, criar alertas e
consultar o histórico de estratégia — Dashboard, Comparador, backtest e todos os
endpoints de `/intelligence` são públicos, sem exigir login).

## Como rodar localmente (sem Docker)

**Backend** (requer Go 1.25+ e um Postgres acessível):

```bash
cd backend
cp .env.example .env   # ajuste DATABASE_URL e, se quiser, as chaves de API
go run ./cmd/api
go run ./cmd/seed       # popula dados de exemplo
```

**Frontend** (requer Node 20+):

```bash
cd frontend
npm install
npm start   # http://localhost:4200, aponta para o backend em localhost:8080
```

## Variáveis de ambiente (backend/.env)

| Variável | Obrigatória | Descrição |
|---|---|---|
| `PORT`, `DATABASE_URL`, `REDIS_ADDR`, `REDIS_PASSWORD`, `JWT_SECRET`, `ENVIRONMENT` | Sim | Configuração básica da API. |
| `ANTHROPIC_API_KEY` | Não | Habilita `POST /api/v1/intelligence/explain`. Sem ela, o endpoint responde `503` com uma mensagem clara em vez de quebrar o resto da aplicação. |
| `SPORTS_DATA_PROVIDER` | Não (padrão `fallback`) | `api_football`, `sportmonks` ou `fallback` (tenta API-Football e depois SportMonks). Usado por `cmd/sync`. |
| `API_FOOTBALL_KEY` | Não | Chave da [API-Football](https://www.api-football.com/). |
| `SPORTMONKS_KEY` | Não | Chave da [SportMonks](https://www.sportmonks.com/). |
| `STRIPE_SECRET_KEY`, `STRIPE_PRICE_ID` | Não (obrigatórias para a Assinatura Premium) | Habilitam a Assinatura Premium (Stripe Checkout + Billing Portal). Sem elas, `GET /api/v1/billing/status` retorna `configured:false` e o frontend mostra "em configuração" no lugar do botão de assinar; os endpoints `/billing/*` respondem `503`. `STRIPE_PRICE_ID` é o Price recorrente (ex.: R$ 29,90/mês). |
| `STRIPE_WEBHOOK_SECRET` | Não (obrigatória para ativar acesso após o pagamento) | Signing Secret do endpoint de webhook (`<sua-api>/api/v1/billing/webhook`, eventos `checkout.session.completed`, `customer.subscription.updated/deleted`). Sem ela o status do assinante nunca é atualizado após o Checkout. |
| `STRIPE_TRIAL_DAYS` | Não (padrão `7`) | Dias de teste grátis aplicados na assinatura. |
| `FRONTEND_URL` | Não (padrão `http://localhost:4200`) | URL pública do frontend, usada nas URLs de sucesso/cancelamento do Checkout e retorno do Billing Portal. Em produção: `https://dsfrcornerlab.com.br`. |
| `DEV_PREMIUM_EMAILS` | Não | Libera Premium manualmente para e-mails específicos (lista separada por vírgula), sem passar pelo Stripe. Uso interno/QA. |

Sem as chaves de dados esportivos configuradas, o sistema continua funcionando
normalmente com os dados de exemplo gerados por `cmd/seed`.

## Sincronização com dados reais (`cmd/sync`)

Popula liga/temporadas/times/partidas reais a partir de API-Football e/ou
SportMonks (upsert idempotente — pode ser rodado repetidamente). Configurado por
padrão para o **Brasileirão Série A**, últimas 3 temporadas:

```bash
cd backend
go run ./cmd/sync -league "Brasileirão Série A" -country Brazil -seasons 2024,2025,2026
```

Flags: `-league`, `-country`, `-seasons` (anos separados por vírgula, obrigatório),
`-provider` (sobrescreve `SPORTS_DATA_PROVIDER` para essa execução). Escanteios por
partida são buscados do provedor quando disponíveis; quando ausentes, o sistema
gera odds sintéticas a partir da média de escanteios totais do lote sincronizado
(mesma técnica usada pelo seed), deixando claro nos logs quantas partidas ficaram
sem essa informação (`CornersMissing`).

## Dados de exemplo

O comando `seed` gera um campeonato fictício ("Brasileirão Série A (exemplo)"), 20
equipes com tendências de escanteios distintas (classificadas em G6/G12/Z4, usado
no filtro "contra equipes"), 4 temporadas (2022–2025) com turno-returno completo, e
odds sintéticas por linha de escanteios (4.5 a 10.5) calculadas por aproximação
estatística — para que ROI/yield no Simulador de Filtros tenham significado.
**Nenhum dado do seed representa jogos reais.** Para dados reais, use `cmd/sync`
(seção acima).

## Estrutura

```
cornerlab/
├── docker-compose.yml
├── backend/
│   ├── cmd/api          # entrypoint da API HTTP
│   ├── cmd/seed         # gerador de dados de exemplo
│   ├── cmd/sync         # sincronização com API-Football / SportMonks
│   ├── internal/domain            # entidades (inclui alertas e histórico de estratégia)
│   ├── internal/usecase            # regras de negócio (stats, dashboard, comparador, filtros, auth, apostas)
│   ├── internal/usecase/intelligence  # motor do Módulo de Inteligência Estatística
│   ├── internal/integration/llm        # cliente Anthropic (explicações)
│   ├── internal/integration/sportsdata # provedores API-Football / SportMonks + fallback
│   ├── internal/repository         # interfaces + implementação Postgres
│   ├── internal/delivery/http       # handlers, DTOs, router, middleware JWT
│   ├── pkg/cache        # cliente Redis + helpers de cache JSON
│   ├── pkg/export       # geração de CSV/XLSX
│   ├── migrations       # schema SQL
│   └── docs/openapi.yaml
└── frontend/
    └── src/app/
        ├── core           # models + serviço de API
        ├── shared         # componente de gráfico (Chart.js)
        └── features/{dashboard,comparator,filters}
```

## Regras gerais respeitadas

- Nenhuma recomendação de aposta é exibida em nenhuma tela ou resposta de API,
  incluindo as explicações geradas por IA (filtro de segurança em duas camadas).
- Todo resultado do Simulador de Filtros e todo endpoint de `/intelligence` exibe o
  período/critérios usados junto com os números (campeonato, temporadas, jogos
  analisados, data de atualização).
- Todos os cálculos estatísticos (média, desvio padrão, mediana, moda, consistência,
  estabilidade, score, ROI, yield, drawdown, sequências) são determinísticos e
  reproduzíveis — ver `backend/internal/usecase/stats.go`, `filter_usecase.go` e
  `backend/internal/usecase/intelligence/`.
- O resultado do backtest inclui um aviso explícito de que é baseado em dados
  históricos e não constitui previsão de resultados futuros.
- Respostas de `/intelligence` são cacheadas no Redis por 24h (atualização
  automática diária), com a flag `meta.cached` indicando se vieram do cache.

## Próximos passos sugeridos

1. Autenticação no frontend (tela de login, guarda de rotas para filtros salvos/apostas/alertas).
2. Telas de frontend para o Módulo de Inteligência Estatística (hoje só o backend
   está pronto — endpoints em `/api/v1/intelligence/*`).
3. Módulo 4 (Simulador Financeiro com Monte Carlo) e Módulo 9 (Estatísticas Avançadas).
4. Job agendado (cron) para rodar `cmd/sync` automaticamente e manter os dados reais
   sempre atualizados.
