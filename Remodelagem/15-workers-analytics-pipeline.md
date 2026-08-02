# 15-workers-analytics-pipeline.md

> Projeto: DSFR CornerLab
> Arquitetura Oficial
> Versão 1.0

---

# Objetivo

Este documento define toda a arquitetura de processamento do CornerLab.

Nenhuma informação complexa deverá ser calculada durante a navegação do usuário.

Toda informação deverá estar previamente processada.

---

# Filosofia

O usuário consulta.

Workers calculam.

Nunca o contrário.

---

Fluxo

API-Football

↓

Import Worker

↓

PostgreSQL

↓

Analytics Pipeline

↓

Redis

↓

Dashboard

↓

IA

---

# Worker 01

Import Worker

Responsabilidade

Buscar novas partidas

Atualizar partidas existentes

Importar estatísticas

Atualizar classificações

Periodicidade

A cada 15 minutos

---

# Worker 02

Statistics Worker

Responsabilidade

Calcular médias

Escanteios

Escanteios sofridos

Casa

Fora

Últimos jogos

Médias móveis

Periodicidade

Sempre após atualização de partidas

---

# Worker 03

Team Analytics Worker

Responsabilidade

Atualizar indicadores das equipes

Calcular

Consistência

Health

Score

Forma

Ranking

Última atualização

---

# Worker 04

Strategy Worker

Responsabilidade

Atualizar todas as estratégias cadastradas.

Recalcular

ROI

Yield

EV

Win Rate

Drawdown

Score

---

# Worker 05

Backtesting Worker

Executar automaticamente

Todas as estratégias favoritas

Atualizar

Resultados

Curvas

Lucro

Capital

---

# Worker 06

Discovery Worker

Responsabilidade

Executar mineração de dados.

Encontrar novas estratégias.

Eliminar estratégias inválidas.

---

# Worker 07

Health Worker

Atualizar

Health

Tendência

Lifecycle

Robustez

---

# Worker 08

Opportunity Worker

Responsabilidade

Detectar

Mudanças

↓

Gerar oportunidades

↓

Criar alertas

↓

Atualizar Dashboard

---

# Worker 09

Ranking Worker

Atualizar

Ranking Equipes

Ranking Estratégias

Ranking Ligas

Ranking Score

Ranking Health

---

# Worker 10

Cache Worker

Enviar

Dashboard

↓

Redis

↓

Top Estratégias

↓

Insights

↓

Oportunidades

↓

Ranking

---

# Worker 11

AI Context Worker

Responsabilidade

Criar contexto resumido.

Nunca enviar consultas SQL para IA.

A IA receberá apenas JSON estruturado.

---

Exemplo

{

"team":"Flamengo",

"score":94,

"health":91,

"roi":18,

"yield":11,

"averageCorners":7.4,

"lastUpdate":"2026-08-01"

}

---

# Pipeline

Importação

↓

Normalização

↓

Validação

↓

Analytics

↓

Backtesting

↓

Discovery

↓

Health

↓

Score

↓

Insights

↓

Redis

↓

Dashboard

---

# Eventos

Nova Partida

↓

Evento

MatchFinished

↓

Statistics Worker

↓

Analytics Worker

↓

Discovery Worker

↓

Health Worker

↓

Redis

↓

Dashboard atualizado

---

# Regras

Nunca executar cálculos durante uma requisição HTTP.

Nunca consultar API externa durante uma requisição do usuário.

Todo cálculo pesado deverá ocorrer em Workers.

---

# Redis

Armazenar

Dashboard

Ranking

Top Estratégias

Insights

Opportunity

Resumo Financeiro

Cards

---

# Reprocessamento

Sempre que

Nova partida

↓

Recalcular apenas

Equipe envolvida

Estratégias impactadas

Rankings impactados

Nunca recalcular toda a base.

---

# Observabilidade

Cada Worker deverá registrar

Tempo

Quantidade processada

Erros

Retries

Tempo médio

Fila

---

# Retry

Caso falhe

Tentar novamente

1 minuto

↓

5 minutos

↓

15 minutos

↓

1 hora

Após isso

Registrar erro crítico.

---

# Monitoramento

Criar Dashboard interno

Workers ativos

Tempo médio

Fila

CPU

RAM

Consumo API

Jobs executados

Jobs pendentes

---

# Critérios de Aceite

✅ Todos os Workers independentes.

✅ Processamento assíncrono.

✅ Redis atualizado automaticamente.

✅ Retry automático.

✅ Logs estruturados.

✅ Monitoramento completo.

---

# Próximo Documento

16-modelo-de-dados.md