# Developer Guide

Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Definir padrões obrigatórios para desenvolvimento.

Todo código produzido deverá seguir este documento.

Não importa se foi escrito por

- Desenvolvedor
- Claude Code
- Cursor
- Windsurf
- ChatGPT
- GitHub Copilot

O padrão deverá ser sempre o mesmo.

---

# Filosofia

Código deve ser

Simples

↓

Legível

↓

Testável

↓

Escalável

↓

Documentado

Nunca otimizar antes da necessidade.

Nunca sacrificar legibilidade por poucas linhas.

---

# Stack Oficial

Frontend

Angular 20

Typescript

TailwindCSS

Angular Material

NGXS

RxJS

ECharts

---

Backend

Go 1.25+

Gin

GORM

Validator

Zap Logger

JWT

Swagger

---

Banco

PostgreSQL

Redis

---

Infra

Render

GitHub Actions

Docker

---

# Estrutura Backend

cmd/

configs/

docs/

internal/

pkg/

---

Dentro de internal

api/

application/

domain/

infrastructure/

workers/

shared/

---

Nunca acessar

Infrastructure

diretamente

a partir da API.

Sempre utilizar

Application.

---

# Domain

O domínio nunca poderá conhecer

Banco

Redis

API Football

OpenAI

Frameworks

---

Permitido

Entidades

Interfaces

Enums

Value Objects

Regras

---

# Application

Responsável por

Casos de uso

Orquestração

Validações

Fluxos

---

Nunca colocar regra de negócio

em Controllers.

---

# Infrastructure

Implementações

Repository

Redis

API Football

OpenAI

Email

Cache

---

# API

Controller

↓

Use Case

↓

Service

↓

Repository

↓

Database

---

# Controllers

Responsabilidades

Receber Request

Validar DTO

Chamar UseCase

Retornar Response

Nada além disso.

---

# Services

Toda regra de negócio.

Nunca acessar HTTP.

Nunca acessar Gin.

Nunca acessar Context.

---

# Repository

Somente persistência.

Nenhuma regra.

---

# DTO

Separar

Request

Response

Nunca utilizar Entity diretamente.

---

# Entity

Toda Entity deverá possuir

ID

CreatedAt

UpdatedAt

Version

---

Nunca utilizar

JSON Tags

na Entity.

---

# Value Objects

Criar para

Money

Odd

Probability

Percentage

Stake

Score

Health

---

# Naming

Interface

StrategyRepository

Implementação

PostgresStrategyRepository

---

UseCase

CreateStrategyUseCase

---

Service

StrategyService

---

Controller

StrategyController

---

Worker

StatisticsWorker

---

# Erros

Nunca retornar

panic

Responder

Erro estruturado

---

Formato

{

code

message

details

traceId

}

---

# Logging

Sempre utilizar

Zap

Campos obrigatórios

traceId

userId

operation

duration

worker

---

# Testes

Todo UseCase deverá possuir

Teste unitário

Cobertura mínima

80%

---

Repository

Teste integração

---

API

Teste E2E

---

# Documentação

Toda API

Swagger

Obrigatório.

---

Todo módulo

README.md

Obrigatório.

---

# Git

Branches

main

develop

feature/*

bugfix/*

hotfix/*

release/*

---

Commit

Conventional Commits

feat:

fix:

refactor:

docs:

test:

build:

---

# Pull Request

Obrigatório

Descrição

Checklist

Testes

Screenshots (Frontend)

---

# Segurança

Nunca armazenar

Secrets

no código.

Utilizar

.env

---

Nunca logar

Tokens

Senhas

JWT

---

# Banco

Migration obrigatória.

Nunca alterar tabela manualmente.

---

# Redis

Tempo de cache documentado.

Nunca utilizar cache sem expiração.

---

# API Football

Toda chamada deverá passar pelo Provider.

Nunca consumir diretamente.

---

# IA

Nunca enviar SQL.

Nunca enviar dados sensíveis.

Sempre enviar contexto estruturado.

---

# Performance

Meta

API

<300ms

Dashboard

<2s

Backtesting

<5s

---

# Observabilidade

Prometheus

Health

Logs

Métricas

Tracing

---

# Qualidade

GolangCI-Lint

Prettier

ESLint

EditorConfig

---

# Critérios de Aceite

✅ Código padronizado.

✅ Testado.

✅ Documentado.

✅ Versionado.

✅ Sem duplicação.

✅ Sem regras em Controller.

✅ Sem SQL na camada de negócio.

---

# Definição de Pronto

Uma tarefa só poderá ser encerrada quando

✔ Código revisado

✔ Testes passando

✔ Swagger atualizado

✔ README atualizado

✔ Logs implementados

✔ Métricas implementadas

✔ Critérios de aceite aprovados

✔ Deploy homologado
