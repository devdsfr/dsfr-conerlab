# Strategy Discovery Engine (SDE)

> Documento: 08-strategy-discovery-engine.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

Descobrir automaticamente estratégias lucrativas utilizando mineração de dados (Data Mining) sobre todo o histórico armazenado no PostgreSQL.

O usuário não precisa criar filtros.

O próprio sistema irá encontrar padrões estatísticos relevantes.

---

# Filosofia

Hoje

Usuário

↓

Cria filtro

↓

Executa Backtesting

↓

Analisa resultado

---

CornerLab

↓

Analisa milhões de combinações

↓

Descobre padrões

↓

Valida

↓

Executa Backtesting

↓

Entrega somente estratégias consistentes

---

# Objetivos

Encontrar automaticamente

- Estratégias consistentes
- Estratégias lucrativas
- Estratégias estáveis
- Estratégias com baixo Drawdown
- Estratégias com alta repetibilidade

---

# Motor de Descoberta

O Engine deverá combinar automaticamente dezenas de filtros.

Exemplo

Equipe

↓

Casa

↓

Últimos 10 jogos

↓

Linha

↓

Forma

↓

Adversário

↓

Dias de descanso

↓

Competição

↓

Temporada

↓

Executar

Backtesting

---

# Variáveis

Equipe

Casa

Fora

Últimos 5

Últimos 10

Últimos 20

Média

Escanteios

Escanteios cedidos

Posse

Finalizações

Cartões

Ranking

Forma

Dias de descanso

Sequência

Árbitro

Clima (futuro)

---

# Geração de Estratégias

O sistema deverá gerar automaticamente combinações.

Exemplo

Casa

+

Média > 6

+

Adversário cede > 5

+

Últimos 10

+

Linha 5.5

↓

Executar

---

Depois

Casa

+

Últimos 5

+

Linha 6.5

↓

Executar

---

Milhões de combinações poderão ser analisadas.

---

# Validação

Toda estratégia encontrada deverá passar por

Quantidade mínima de jogos

ROI mínimo

Yield mínimo

Win Rate mínimo

Drawdown máximo

EV positivo

Consistência mínima

---

# Critérios mínimos

Jogos

>=100

ROI

>=10%

Yield

>=5%

EV

Positivo

Drawdown

<=20%

Win Rate

>=75%

---

# Overfitting

Toda estratégia deverá ser validada.

Se

Quantidade de jogos

<50

↓

Descartar

---

Se

Win Rate

95%

↓

Mas

Apenas

12 jogos

↓

Marcar

Amostra insuficiente.

---

# Ranking

Ordenar

Maior ROI

Maior Yield

Maior EV

Maior Win Rate

Maior Consistência

Menor Drawdown

Maior Lucro

---

# DSFR Score

Cada estratégia receberá um Score.

Componentes

ROI

20%

Yield

15%

EV

20%

Win Rate

20%

Drawdown

10%

Quantidade de Jogos

10%

Consistência

5%

Resultado

0

↓

100

---

# Classificação

91-100

Elite

---

81-90

Excelente

---

71-80

Muito Boa

---

61-70

Boa

---

40-60

Regular

---

0-39

Descartar

---

# Dashboard

Mostrar

Top Estratégias

↓

Score

↓

ROI

↓

Yield

↓

EV

↓

Jogos

↓

Lucro

↓

Drawdown

↓

Confiabilidade

---

# Comparação

Permitir comparar

Estratégia A

↓

Estratégia B

↓

Estratégia C

---

Mostrar

ROI

Yield

EV

Win Rate

Lucro

Drawdown

Score

---

# IA

Pergunta

Existe alguma estratégia melhor que a minha?

↓

Resposta

"Foram encontradas 18 estratégias superiores utilizando os critérios definidos."

---

Outra

Qual possui menor risco?

↓

Resposta

"A estratégia X apresentou o menor Drawdown dentre todas as estratégias analisadas."

---

# Atualização

Executar

Todos os dias

03:00

↓

Reprocessar estratégias

↓

Atualizar Ranking

↓

Atualizar Scores

---

# API

POST

/strategy/discovery

Entrada

{

"league":71,

"season":2026,

"market":"corners"

}

Resposta

{

"strategies":[...]

}

---

# Critérios de Aceite

✅ Gerar estratégias automaticamente.

✅ Validar ROI.

✅ Validar Yield.

✅ Validar EV.

✅ Validar Drawdown.

✅ Eliminar Overfitting.

✅ Gerar Ranking.

✅ Atualizar automaticamente.

---

# Regras

Nunca recomendar apostas.

Nunca mostrar estratégias com amostra insuficiente.

Nunca considerar apenas Win Rate.

Sempre utilizar múltiplos indicadores.

---

# Próximo Documento

09-DSFR Intelligence Score
