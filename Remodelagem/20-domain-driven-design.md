# Domain Driven Design (DDD)

Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Definir o modelo de domínio oficial do CornerLab.

Todo o sistema deverá utilizar a mesma linguagem de negócio.

Nunca deverão existir duas interpretações diferentes para o mesmo conceito.

---

# Ubiquitous Language

Os seguintes termos possuem significado oficial.

Equipe

Organização participante de uma competição.

---

Liga

Competição oficial.

Exemplo

Brasileirão Série A

Premier League

Champions League

---

Temporada

Conjunto de partidas pertencentes a uma liga em determinado ano.

---

Partida

Evento esportivo.

Possui

Mandante

Visitante

Data

Status

Resultado

---

Estatística

Qualquer informação produzida durante uma partida.

Exemplo

Escanteios

Finalizações

Posse

Cartões

---

Métrica

Valor calculado.

Exemplo

Média

Consistência

Health

Score

ROI

---

Estratégia

Conjunto de regras que define uma análise.

Nunca representa uma aposta.

---

Backtesting

Execução histórica de uma estratégia.

---

Simulação

Execução probabilística de uma estratégia futura.

---

Insight

Mensagem gerada automaticamente após detectar mudança relevante.

---

Oportunidade

Mudança estatística significativa identificada automaticamente.

Nunca representa recomendação.

---

# Bounded Contexts

Authentication

Responsável

Usuários

Permissões

Sessões

JWT

---

Statistics

Responsável

Importação

Normalização

Persistência

---

Analytics

Responsável

Médias

Indicadores

Tendências

Consistência

---

Strategy

Responsável

Filtros

Estratégias

Backtesting

Comparações

---

Financial

Responsável

ROI

Yield

EV

Monte Carlo

Gestão de banca

---

Discovery

Responsável

Encontrar padrões

Validar estratégias

Rankings

---

AI

Responsável

Explicações

Comparações

Resumo

Insights

---

Dashboard

Responsável

Widgets

Cards

Resumo

KPIs

---

Administration

Responsável

Configuração

Logs

Auditoria

Workers

---

# Agregados

Team

Root

Team

Filhos

Statistics

Metrics

Trend

Health

---

Strategy

Root

Strategy

Filhos

Filters

Backtests

Simulations

Scores

Insights

---

League

Root

League

Filhos

Season

Teams

Fixtures

---

User

Root

User

Filhos

Favorites

Alerts

Settings

---

# Value Objects

Money

Stake

Odd

Percentage

Probability

ROI

Yield

HealthScore

DSFRScore

Trend

Confidence

OpportunityScore

---

Todos deverão ser imutáveis.

---

# Entidades

User

League

Season

Fixture

Team

Statistic

Strategy

Backtest

Simulation

Insight

Opportunity

Alert

Worker

---

# Domain Services

StrategyAnalyzer

FinancialCalculator

BacktestingExecutor

SimulationEngine

DiscoveryEngine

HealthCalculator

ScoreCalculator

InsightGenerator

OpportunityDetector

---

# Eventos de Domínio

MatchImported

↓

StatisticsUpdated

↓

AnalyticsCalculated

↓

BacktestFinished

↓

HealthChanged

↓

ScoreUpdated

↓

OpportunityDetected

↓

InsightGenerated

---

# Fluxo

MatchImported

↓

AnalyticsCalculated

↓

StrategyUpdated

↓

HealthUpdated

↓

OpportunityDetected

↓

DashboardUpdated

---

# Regras

Uma estratégia nunca conhece usuários.

Uma equipe nunca conhece estratégias.

Backtesting nunca altera estatísticas.

Workers nunca executam regras diretamente.

Toda regra pertence ao domínio.

---

# Anti-Corruption Layer

Toda comunicação com sistemas externos deverá passar por adaptadores.

Exemplo

API Football

↓

FootballProvider

↓

Domain

Nunca utilizar objetos externos diretamente.

---

# Linguagem Oficial

Nunca utilizar

Bet

Ticket

Slip

Tip

Prediction

Sempre utilizar

Strategy

Analysis

Statistics

Historical Data

Indicators

Simulation

---

# Critérios de Aceite

✅ Linguagem única.

✅ Contextos isolados.

✅ Agregados definidos.

✅ Eventos documentados.

✅ Value Objects imutáveis.
