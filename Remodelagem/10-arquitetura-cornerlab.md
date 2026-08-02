# 10-arquitetura-cornerlab.md

> Projeto: DSFR CornerLab
>
> Documento: Arquitetura Oficial
>
> Versão: 1.0

---

# Objetivo

Definir a arquitetura oficial do CornerLab.

Toda funcionalidade deverá respeitar esta arquitetura.

O objetivo é garantir:

- Escalabilidade
- Manutenibilidade
- Baixo acoplamento
- Alta performance
- Facilidade para IA gerar código
- Facilidade para testes

---

# Princípios

O sistema seguirá:

- Clean Architecture
- SOLID
- DDD (Domain Driven Design)
- Repository Pattern
- Dependency Injection
- Event Driven
- Worker Pattern
- CQRS (onde fizer sentido)

---

# Stack Oficial

Frontend

Angular 20

Typescript

Tailwind

Angular Material

NGXS

RxJS

Charts

ECharts

---

Backend

Go

Gin

GORM

Zap

Validator

JWT

Swagger

---

Banco

PostgreSQL

---

Cache

Redis

---

Workers

Go Worker

Cron

Queue

---

IA

OpenAI

(Contexto gerado pelo Backend)

---

Dados

API-Football

Statistics Provider

---

# Arquitetura

                    Angular

                       │

                       ▼

                 REST API

                       │

             Authentication

                       │

                Controllers

                       │

                  Services

          ┌────────┼────────┐

          ▼        ▼        ▼

 Statistics   Strategy   Financial

          ▼        ▼        ▼

       Repository Layer

               │

               ▼

         PostgreSQL

               ▲

               │

            Workers

               │

               ▼

         API-Football

---

# Camadas

Presentation

Controllers

DTOs

Validators

Swagger

---

Application

Services

UseCases

Rules

---

Domain

Entities

Interfaces

Enums

Value Objects

Business Rules

---

Infrastructure

Repository

API Providers

Database

Redis

Workers

---

# Organização

cmd/

internal/

domain/

application/

infrastructure/

api/

workers/

pkg/

configs/

docs/

---

# Domain

teams

matches

fixtures

statistics

strategies

backtesting

financial

simulation

ai

alerts

dashboard

users

---

# Repository Pattern

Nunca acessar GORM diretamente dentro do Service.

Sempre utilizar interfaces.

Exemplo

StrategyRepository

TeamRepository

MatchRepository

BacktestRepository

SimulationRepository

---

# Provider Pattern

Todo provedor externo deverá implementar interfaces.

Exemplo

StatisticsProvider

Implementações

ApiFootballProvider

SportMonksProvider

MockProvider

FutureProvider

---

Nunca utilizar API-Football diretamente nos Services.

---

# Worker Layer

Workers independentes.

Import Worker

Atualiza partidas.

---

Statistics Worker

Atualiza estatísticas.

---

Ranking Worker

Recalcula rankings.

---

Backtesting Worker

Executa estratégias.

---

Discovery Worker

Procura estratégias.

---

Score Worker

Atualiza DSFR Score.

---

Health Worker

Atualiza Health Score.

---

Alert Worker

Dispara alertas.

---

# Banco

Separar tabelas

Operacionais

Analíticas

Cache

Logs

Auditoria

---

# Índices

Criar índices para

league_id

team_id

season

fixture_id

match_date

strategy_id

score

updated_at

---

# Redis

Utilizar para

Dashboard

Rankings

Top Estratégias

Equipes

IA

Filtros

Cache da API

---

# Segurança

JWT

Refresh Token

HTTPS obrigatório

Rate Limit

Audit Log

LGPD

Criptografia

---

# Logs

Toda ação importante deverá gerar log.

Importação

Erro

Worker

Login

Backtesting

Simulação

IA

---

# Auditoria

Registrar

Usuário

Data

Operação

Valor antigo

Valor novo

IP

---

# Performance

Nunca recalcular

EV

ROI

Yield

DSFR Score

durante requisições.

Tudo deverá ser pré-calculado pelos Workers.

---

# Eventos

Sempre que ocorrer

Nova partida

↓

Atualizar estatísticas

↓

Atualizar rankings

↓

Atualizar estratégias

↓

Atualizar Score

↓

Atualizar Dashboard

↓

Atualizar IA

---

# API

REST

JSON

Versionada

/api/v1

---

# Versionamento

v1

Escanteios

v2

Cartões

v3

Finalizações

v4

Gols

v5

xG

---

# Testes

Unitários

Integração

Carga

Benchmark

Backtesting

---

# Deploy

GitHub

↓

GitHub Actions

↓

Render

↓

PostgreSQL

↓

Redis

---

# Monitoramento

Health Check

Workers

Banco

Redis

Tempo de resposta

Consumo API

---

# Métricas

Tempo médio

Consultas

Importações

Estratégias

Backtests

Simulações

Uso IA

---

# Critérios de Aceite

✅ Clean Architecture.

✅ Repository Pattern.

✅ Provider Pattern.

✅ Worker Pattern.

✅ Cache Redis.

✅ JWT.

✅ Logs.

✅ Auditoria.

✅ Pré-processamento.

✅ Escalabilidade.

---

# Filosofia

O usuário nunca deverá esperar cálculos complexos.

Todos os indicadores deverão estar prontos.

A experiência do usuário deverá ser instantânea.
