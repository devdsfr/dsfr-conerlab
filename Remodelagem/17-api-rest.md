# API REST

Projeto: DSFR CornerLab

Versão: v1

Arquitetura

REST

JSON

JWT

HTTPS

Versionamento

/api/v1

---

# Objetivo

Definir todos os endpoints públicos da plataforma.

Toda comunicação entre Frontend e Backend deverá utilizar exclusivamente esta API.

---

# Convenções

GET

Consultar

POST

Criar

PUT

Atualizar

PATCH

Atualização parcial

DELETE

Excluir

---

# Status HTTP

200

OK

201

Criado

204

Sem conteúdo

400

Requisição inválida

401

Não autenticado

403

Sem permissão

404

Não encontrado

409

Conflito

422

Erro de validação

500

Erro interno

---

# AUTENTICAÇÃO

POST

/auth/login

Entrada

{
 email
 password
}

Resposta

{
 accessToken
 refreshToken
 expires
 user
}

---

POST

/auth/refresh

---

POST

/auth/logout

---

GET

/auth/me

---

# LEAGUES

GET

/leagues

Lista campeonatos.

---

GET

/leagues/{id}

Detalhes.

---

GET

/leagues/{id}/teams

Equipes.

---

GET

/leagues/{id}/seasons

Temporadas.

---

# TEAMS

GET

/teams

---

GET

/teams/{id}

---

GET

/teams/{id}/statistics

---

GET

/teams/{id}/matches

---

GET

/teams/{id}/analytics

---

GET

/teams/{id}/health

---

GET

/teams/{id}/score

---

# FIXTURES

GET

/fixtures

Filtros

Liga

Temporada

Equipe

Data

Status

---

GET

/fixtures/{id}

---

GET

/fixtures/{id}/statistics

---

GET

/fixtures/{id}/events

---

# STATISTICS

GET

/statistics/corners

---

GET

/statistics/teams

---

GET

/statistics/leagues

---

GET

/statistics/trends

---

# ANALYTICS

GET

/analytics/team/{id}

---

GET

/analytics/league/{id}

---

GET

/analytics/strategy/{id}

---

GET

/analytics/dashboard

---

# STRATEGIES

GET

/strategies

---

POST

/strategies

Criar estratégia.

---

PUT

/strategies/{id}

---

DELETE

/strategies/{id}

---

GET

/strategies/{id}

---

GET

/strategies/{id}/backtest

---

GET

/strategies/{id}/simulation

---

GET

/strategies/{id}/score

---

GET

/strategies/{id}/health

---

# BACKTEST

POST

/backtesting/run

Entrada

Filtros

↓

Liga

↓

Equipe

↓

Mercado

↓

Linha

↓

Últimos jogos

↓

Odd

↓

Stake

---

Resposta

Jogos

Wins

Losses

ROI

Yield

EV

Lucro

Capital

Drawdown

---

# SIMULATOR

POST

/simulator/run

Entrada

Stake

Odd

WinRate

Quantidade

Modelo

---

Resposta

Capital

Lucro

ROI

Yield

Drawdown

Monte Carlo

---

# DISCOVERY

POST

/discovery/run

---

GET

/discovery/results

---

GET

/discovery/top

---

# OPPORTUNITIES

GET

/opportunities

---

GET

/opportunities/{id}

---

# INSIGHTS

GET

/insights

---

GET

/insights/latest

---

# ALERTS

GET

/alerts

---

PATCH

/alerts/{id}/read

---

# FAVORITES

GET

/favorites

---

POST

/favorites

---

DELETE

/favorites/{id}

---

# DASHBOARD

GET

/dashboard

---

GET

/dashboard/cards

---

GET

/dashboard/summary

---

GET

/dashboard/financial

---

GET

/dashboard/opportunities

---

# IA

POST

/ai/analyze

---

POST

/ai/compare

---

POST

/ai/explain

---

POST

/ai/chat

---

# WORKERS

GET

/workers

---

GET

/workers/status

---

POST

/workers/import

---

POST

/workers/recalculate

---

# ADMIN

GET

/admin/users

---

GET

/admin/logs

---

GET

/admin/audit

---

GET

/admin/jobs

---

POST

/admin/cache/clear

---

POST

/admin/reindex

---

# HEALTH CHECK

GET

/health

Resposta

API

Banco

Redis

Workers

Provider

Status

---

# PAGINAÇÃO

Todos endpoints

?page=1

&pageSize=20

&sort=name

&order=asc

---

# FILTROS

Todos endpoints

?league=

?season=

?team=

?home=

?away=

?date=

---

# VERSIONAMENTO

/api/v1

/api/v2

---

# SWAGGER

Toda API deverá possuir

OpenAPI 3

Swagger

Exemplos

Schemas

Validações

---

# Critérios de Aceite

✅ REST.

✅ JSON.

✅ JWT.

✅ Swagger.

✅ Paginação.

✅ Filtros.

✅ Versionamento.

✅ Health Check.
