# Alerts & Insights Engine

> Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Criar um mecanismo inteligente capaz de monitorar continuamente todos os indicadores estatísticos da plataforma e gerar insights automáticos para o usuário.

O sistema não recomendará apostas.

O sistema identificará mudanças estatísticas relevantes.

---

# Filosofia

O usuário não deve descobrir mudanças importantes manualmente.

O sistema deverá monitorar continuamente todos os indicadores.

Sempre que houver alteração significativa,

o usuário será informado.

---

# Tipos de Insight

Mudança de tendência

↓

Mudança de consistência

↓

Mudança de média

↓

Mudança de estratégia

↓

Mudança de ranking

↓

Mudança de Score

↓

Mudança de Health

↓

Mudança de ROI

↓

Mudança de Drawdown

---

# Insight 01

Aumento de Consistência

Exemplo

Flamengo

Últimos 10

76%

↓

Últimos 20

84%

↓

Insight

"A equipe apresentou aumento consistente na frequência de escanteios."

---

# Insight 02

Queda de desempenho

Palmeiras

Últimos 15

88%

↓

Últimos 10

73%

↓

Insight

"A estratégia perdeu eficiência nas últimas partidas."

---

# Insight 03

Mudança de Linha

Histórico

Over 5.5

84%

↓

Over 6.5

63%

↓

Insight

"A linha de 6.5 reduziu significativamente a eficiência da estratégia."

---

# Insight 04

Nova Estratégia

Discovery Engine encontrou

1.842 jogos

↓

ROI

18%

↓

Score

94

↓

Insight

"Foi encontrada uma nova estratégia estatisticamente consistente."

---

# Insight 05

Health

Score

95

↓

88

↓

Insight

"A estratégia apresenta deterioração gradual."

---

# Insight 06

Drawdown

Normal

8%

↓

Atual

21%

↓

Insight

"O risco da estratégia aumentou."

---

# Insight 07

Mudança Temporal

Últimos 3 anos

ROI

18%

↓

Últimos 60 dias

ROI

8%

↓

Insight

"O desempenho recente está abaixo da média histórica."

---

# Ranking

Todos os insights deverão possuir prioridade.

Baixa

Média

Alta

Crítica

---

# Explicação

Todo insight deverá explicar

O que mudou.

Por que mudou.

Qual indicador mudou.

Quando mudou.

Qual impacto esperado.

---

# IA

A IA deverá gerar explicações utilizando exclusivamente dados do banco.

Nunca utilizar opiniões.

Nunca prever resultados.

---

# Dashboard

Mostrar

Insights Recentes

↓

Alertas

↓

Mudanças

↓

Ranking

↓

Histórico

---

# Histórico

Guardar

Todos os insights gerados.

Permitir consulta.

---

# API

GET

/insights

Resposta

[
{
"type":"trend",

"title":"Queda de Consistência",

"priority":"high",

"team":"Flamengo",

"description":"A média de escanteios caiu de 7.8 para 6.4 nos últimos 30 dias."

}
]

---

# Critérios de Aceite

✅ Gerar insights automaticamente.

✅ Explicar mudanças.

✅ Classificar prioridade.

✅ Atualizar diariamente.

✅ Nunca recomendar apostas.
