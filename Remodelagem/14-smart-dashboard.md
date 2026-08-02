# 14-smart-dashboard.md

> Projeto: DSFR CornerLab
>
> Módulo: Smart Dashboard
>
> Versão: 1.0

---

# Objetivo

O Smart Dashboard será a principal tela do sistema.

Sua responsabilidade é apresentar, de forma clara e priorizada, todas as informações relevantes para análise estatística das estratégias.

O Dashboard nunca deverá ser apenas uma coleção de gráficos.

Ele deverá responder:

"O que aconteceu?"

"O que mudou?"

"O que merece minha atenção?"

---

# Filosofia

O usuário deverá entender a situação atual da plataforma em menos de 30 segundos.

Todas as informações deverão ser organizadas por prioridade.

Nunca mostrar dezenas de indicadores sem contexto.

Sempre mostrar primeiro o que mudou.

---

# Estrutura

Header

↓

Resumo Executivo

↓

Insights do Dia

↓

Oportunidades

↓

Minhas Estratégias

↓

Mercado

↓

Equipes

↓

Alertas

↓

Histórico

---

# Header

Mostrar

Nome do usuário

↓

Data

↓

Última sincronização

↓

Quantidade de partidas atualizadas

↓

Status dos Workers

↓

Status da API

---

# Card 1

Resumo Executivo

Mostrar

Estratégias Monitoradas

↓

Estratégias Saudáveis

↓

Estratégias em Risco

↓

Oportunidades

↓

Alertas

↓

Score Médio

---

# Card 2

Resumo Financeiro

Mostrar

ROI Médio

↓

Yield Médio

↓

EV Médio

↓

Drawdown Médio

↓

Health Médio

↓

DSFR Score Médio

---

# Card 3

Resumo Estatístico

Mostrar

Jogos Processados

↓

Campeonatos

↓

Equipes

↓

Temporadas

↓

Estratégias

↓

Backtests

---

# Insights do Dia

Mostrar

Mudanças relevantes

Exemplo

Flamengo aumentou a média de escanteios.

↓

Liverpool perdeu consistência.

↓

Nova estratégia validada.

↓

Health da estratégia X caiu.

---

# Oportunidades

Mostrar

Opportunity Score

Equipe

↓

Liga

↓

Motivo

↓

Health

↓

Score

↓

Confiabilidade

---

Ordenar

Maior Opportunity Score

↓

Menor

---

# Minhas Estratégias

Mostrar

Nome

↓

DSFR Score

↓

Health

↓

ROI

↓

Yield

↓

EV

↓

Última atualização

↓

Status

---

Status

🟢 Saudável

🟡 Atenção

🔴 Crítica

---

# Mercado

Mostrar

Ranking por Liga

↓

Ranking por Estratégia

↓

Ranking por ROI

↓

Ranking por Score

↓

Ranking por Health

---

# Equipes

Mostrar

Top 10

Maior média

↓

Maior consistência

↓

Maior evolução

↓

Maior queda

↓

Maior Opportunity

---

# Alertas

Mostrar

Prioridade

Crítico

↓

Alto

↓

Médio

↓

Baixo

---

Cada alerta deverá conter

Título

Descrição

Data

Motivo

Ação sugerida

---

# Histórico

Mostrar

Últimos eventos

↓

Atualizações

↓

Novas estratégias

↓

Mudanças

↓

Alertas

---

# Pesquisa Global

Permitir pesquisar

Equipe

Liga

Estratégia

Temporada

Jogador (futuro)

Mercado

---

# Favoritos

Usuário poderá favoritar

Equipe

↓

Liga

↓

Estratégia

↓

Dashboard personalizado

---

# Widgets

Resumo Financeiro

↓

Resumo Estatístico

↓

Top Estratégias

↓

Health

↓

Insights

↓

Alertas

↓

Opportunity

↓

Ranking

---

# Dashboard Inteligente

Ao entrar

Mostrar

Bom dia, Daniel.

Hoje foram processadas

18 partidas.

↓

3 estratégias melhoraram.

↓

2 perderam eficiência.

↓

1 nova oportunidade foi encontrada.

↓

Nenhuma estratégia entrou em estado crítico.

---

# IA

Botão

Pergunte ao CornerLab

Exemplos

Por que minha estratégia caiu?

↓

Compare Flamengo e Palmeiras.

↓

Explique esse Score.

↓

Mostre estratégias semelhantes.

---

# Responsividade

Desktop

Tablet

Mobile

---

# Performance

Tempo máximo

2 segundos

Todas as informações deverão vir do Redis.

Nunca executar cálculos durante o carregamento.

---

# API

GET

/dashboard

Resposta

{

"user":"Daniel",

"score":92,

"health":91,

"strategies":18,

"alerts":2,

"opportunities":4,

"insights":7,

"workers":"OK"

}

---

# Critérios de Aceite

✅ Dashboard carregado em menos de 2 segundos.

✅ Dados provenientes do Redis.

✅ Informações priorizadas.

✅ Layout responsivo.

✅ Widgets configuráveis.

✅ Dashboard personalizado.

---

# Próximo Documento

15-Workers & Analytics Pipeline
