# Test Strategy

Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Definir a estratégia oficial de testes do CornerLab.

Toda funcionalidade deverá possuir testes automatizados antes de ser disponibilizada em produção.

Nenhum cálculo financeiro ou estatístico poderá ser liberado sem validação.

---

# Filosofia

Código sem teste é código incompleto.

O objetivo não é apenas testar telas.

O objetivo é garantir que todas as fórmulas matemáticas produzam resultados corretos.

---

# Pirâmide de Testes

                E2E
                 ▲
          Integration
                 ▲
              Unit Tests

Distribuição

70% Unitários

20% Integração

10% End-to-End

---

# Testes Unitários

Objetivo

Validar regras de negócio isoladamente.

Exemplos

✔ Cálculo de EV

✔ Cálculo de ROI

✔ Cálculo de Yield

✔ Fair Odds

✔ Edge

✔ Drawdown

✔ Monte Carlo

✔ Health Score

✔ DSFR Score

---

# Testes de Integração

Objetivo

Validar comunicação entre módulos.

Exemplos

API → Banco

API → Redis

Worker → PostgreSQL

Worker → API Football

Analytics → IA

---

# Testes E2E

Objetivo

Simular o comportamento do usuário.

Fluxos

Login

Criar estratégia

Executar Backtesting

Executar Simulação

Consultar Dashboard

Favoritar Estratégia

Perguntar para IA

---

# Testes Matemáticos

Todo cálculo deverá possuir testes conhecidos.

Exemplo

Odd

1.50

Win Rate

80%

Stake

100

Resultado esperado

EV = 20

---

Outro

Odd

1.60

Win Rate

75%

Stake

100

Resultado esperado

EV = 20

---

# Testes Financeiros

Validar

Reinvestimento

Stake fixa

Stake percentual

Estratégia DSFR

Kelly

---

# Testes de Monte Carlo

Executar

10.000

50.000

100.000 simulações

Validar

Distribuição

Percentis

Convergência

---

# Testes do Discovery Engine

Validar

Quantidade mínima

ROI

Yield

Score

Health

Overfitting

---

# Testes do Opportunity Engine

Validar

Mudanças

Alertas

Prioridade

Expiração

---

# Testes do Dashboard

Tempo máximo

2 segundos

Com Redis

Sem Redis

Com cache expirado

---

# Testes da API

Todos endpoints deverão validar

400

401

403

404

422

500

---

# Testes de Segurança

JWT inválido

JWT expirado

SQL Injection

XSS

CSRF

Rate Limit

---

# Testes de Performance

Dashboard

<2 segundos

Backtesting

<5 segundos

Simulação

<5 segundos

Importação

<30 segundos

---

# Testes de Carga

100 usuários

500 usuários

1000 usuários

5000 usuários

---

# Testes dos Workers

Falha de API

Retry

Timeout

Duplicidade

Importação parcial

---

# Testes da IA

A IA nunca poderá

Inventar estatísticas

Recomendar apostas

Responder sem contexto

Misturar temporadas

---

# Cobertura

Backend

>=90%

Domain

100%

Application

>=95%

Frontend

>=80%

---

# Ferramentas

Backend

Go Test

Testify

Mockery

---

Frontend

Jest

Cypress

Playwright

---

Performance

k6

---

# CI/CD

Todo Pull Request deverá executar

Lint

↓

Unit Tests

↓

Integration

↓

Build

↓

Security Scan

↓

Deploy Homologação

---

# Critérios de Aceite

✅ Todos os testes passando.

✅ Cobertura mínima atingida.

✅ Performance validada.

✅ Segurança validada.

✅ Nenhum teste ignorado.

---

# Definition of Done

Uma funcionalidade só será considerada pronta quando:

✔ Critérios de aceite aprovados

✔ Testes unitários

✔ Testes integração

✔ Testes E2E

✔ Performance aprovada

✔ Documentação atualizada

✔ Swagger atualizado

✔ Deploy homologado
