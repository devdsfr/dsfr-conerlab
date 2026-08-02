# 16-modelo-de-dados.md

Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Definir o modelo oficial de dados do CornerLab.

Toda persistência deverá seguir este documento.

Nenhuma tabela poderá ser criada fora deste padrão.

---

# Filosofia

Separar o banco em três camadas.

RAW

↓

NORMALIZED

↓

ANALYTICS

Nunca misturar dados importados com dados calculados.

---

# Camada RAW

Representa exatamente os dados recebidos da API.

Nenhuma alteração deverá ser realizada.

Tabelas

raw_matches

raw_statistics

raw_events

raw_lineups

raw_leagues

raw_teams

raw_players

---

# Camada NORMALIZED

Responsável pela normalização.

Tabela

matches

teams

statistics

fixtures

players

leagues

seasons

venues

coaches

referees

---

# Camada ANALYTICS

Responsável pelos cálculos.

team_metrics

league_metrics

strategy_metrics

match_metrics

trend_metrics

financial_metrics

health_metrics

score_metrics

---

# USERS

users

id

name

email

password

role

created_at

updated_at

---

# LEAGUES

id

external_id

country

name

type

logo

active

created_at

---

# SEASONS

id

league_id

year

current

start_date

end_date

---

# TEAMS

id

external_id

league_id

name

country

logo

founded

venue_id

active

---

# FIXTURES

id

external_id

league_id

season_id

home_team_id

away_team_id

status

date

venue

round

winner

created_at

---

# MATCH STATISTICS

id

fixture_id

team_id

corners

corners_against

shots

shots_on_target

possession

yellow_cards

red_cards

offsides

fouls

passes

created_at

---

# TEAM METRICS

id

team_id

season_id

average_corners

average_corners_home

average_corners_away

average_corners_against

last5

last10

last20

consistency

variance

trend

health

score

updated_at

---

# STRATEGIES

id

name

description

owner_id

active

favorite

visibility

created_at

---

# STRATEGY FILTERS

strategy_id

filter

operator

value

order

---

# BACKTESTS

id

strategy_id

games

wins

losses

void

roi

yield

ev

drawdown

profit

capital

confidence

created_at

---

# SIMULATIONS

id

strategy_id

stake

bankroll

win_rate

odd

simulations

expected_profit

expected_capital

drawdown

probability_positive

created_at

---

# HEALTH

strategy_id

health_score

trend

variation

updated_at

---

# DSFR SCORE

strategy_id

score

roi_score

ev_score

yield_score

drawdown_score

consistency_score

robustness_score

updated_at

---

# OPPORTUNITIES

id

team_id

strategy_id

priority

opportunity_score

reason

status

created_at

expires_at

---

# INSIGHTS

id

type

title

description

priority

status

created_at

---

# ALERTS

id

user_id

type

title

description

read

created_at

---

# DASHBOARD CACHE

id

user_id

payload

updated_at

---

# FAVORITES

user_id

team_id

strategy_id

league_id

---

# WORKERS

id

worker

status

started_at

finished_at

duration

processed

errors

---

# AUDIT

id

user_id

entity

operation

old_value

new_value

ip

created_at

---

# LOGS

id

service

level

message

payload

created_at

---

# RELACIONAMENTOS

League

↓

Season

↓

Fixture

↓

Statistics

↓

Analytics

↓

Strategy

↓

Backtesting

↓

Simulation

↓

Dashboard

---

# ÍNDICES

Criar índices para

fixture_id

team_id

league_id

season_id

strategy_id

score

health

updated_at

date

status

---

# PARTICIONAMENTO

Particionar

fixtures

statistics

logs

audit

por

temporada

---

# RETENÇÃO

RAW

Nunca apagar

NORMALIZED

Nunca apagar

ANALYTICS

Reprocessável

CACHE

7 dias

LOGS

180 dias

---

# VERSIONAMENTO

Toda métrica calculada deverá possuir

algorithm_version

Permitindo recalcular resultados antigos.

---

# CRITÉRIOS DE ACEITE

✅ Modelo normalizado.

✅ Sem duplicidade.

✅ Índices definidos.

✅ Particionamento.

✅ Versionamento.

✅ Auditoria.

✅ Escalável.

---

# Próximo Documento

17-api-rest.md
