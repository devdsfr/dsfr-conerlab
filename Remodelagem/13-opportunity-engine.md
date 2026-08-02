# Opportunity Engine (OE)

> Documento: 13-opportunity-engine.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

O Opportunity Engine é responsável por identificar automaticamente
mudanças estatísticas relevantes em todas as equipes monitoradas.

O objetivo NÃO é recomendar apostas.

O objetivo é detectar padrões que merecem atenção.

---

# Filosofia

Enquanto o usuário dorme,

o sistema continua trabalhando.

Todos os dias milhares de partidas serão reprocessadas.

Quando uma oportunidade estatística surgir,

ela aparecerá automaticamente no Dashboard.

---

# Objetivos

Responder perguntas como

Quais equipes melhoraram?

Quais equipes pioraram?

Quais estratégias cresceram?

Quais estratégias perderam eficiência?

Quais campeonatos mudaram?

Existe alguma oportunidade nova?

---

# Conceito

Uma oportunidade NÃO significa

"Aposte."

Significa

"Existe um comportamento estatístico diferente do histórico."

---

# Tipos

## Tendência de Alta

Média crescente

↓

Score crescente

↓

Health positivo

↓

ROI crescente

---

## Tendência de Queda

Média caindo

↓

Health negativo

↓

Drawdown aumentando

---

## Nova Estratégia

Discovery Engine encontrou nova estratégia válida.

---

## Recuperação

Estratégia voltou a apresentar resultados positivos.

---

## Mudança Estrutural

Mudança significativa no comportamento estatístico.

---

# Fontes

Statistics Engine

↓

Analytics Engine

↓

Discovery Engine

↓

Health Engine

↓

DSFR Score

↓

Backtesting

---

# Critérios

Uma oportunidade somente poderá ser criada quando

Pelo menos

3 indicadores

mudarem simultaneamente.

---

Exemplo

Win Rate ↑

+

ROI ↑

+

Health ↑

↓

Criar oportunidade.

---

Outro

Drawdown ↑

+

ROI ↓

+

Score ↓

↓

Criar alerta.

---

# Classificação

Informativa

↓

Baixa

↓

Média

↓

Alta

↓

Crítica

---

# Priorização

Calcular

Impacto

×

Confiabilidade

×

Quantidade de Jogos

×

Persistência

=

Priority Score

---

# Opportunity Score

Criar indicador

Opportunity Score

0

↓

100

---

Componentes

Health

30%

---

DSFR Score

25%

---

ROI

15%

---

Win Rate

10%

---

Consistência

10%

---

Quantidade de Jogos

10%

---

# Classificação

91-100

Oportunidade Excepcional

---

81-90

Muito Forte

---

71-80

Boa

---

61-70

Moderada

---

Abaixo de 60

Não exibir.

---

# Dashboard

Página

Oportunidades

Mostrar

Equipe

↓

Liga

↓

Health

↓

Score

↓

ROI

↓

Última atualização

↓

Motivo

↓

Opportunity Score

---

# Exemplo

Flamengo

Health

+18

Score

94

ROI

22%

Opportunity Score

96

Motivo

"Aumento consistente na média de escanteios nas últimas 8 partidas."

---

# Explicação

Toda oportunidade deverá responder

O que mudou?

Quando mudou?

Qual indicador mudou?

Qual impacto?

Qual confiança?

---

# Histórico

Guardar

Data

Equipe

Motivo

Score

Health

Status

---

# Ciclo

Nova

↓

Confirmada

↓

Monitorando

↓

Encerrada

---

# Expiração

Uma oportunidade expira quando

Health voltar para neutro.

ou

Score cair abaixo do mínimo.

---

# API

GET

/opportunities

Resposta

{

"team":"Flamengo",

"league":"Serie A",

"score":96,

"health":18,

"roi":22,

"reason":"Aumento consistente da média de escanteios.",

"createdAt":"2026-08-01"

}

---

# Critérios de Aceite

✅ Detectar mudanças automaticamente.

✅ Gerar Opportunity Score.

✅ Explicar o motivo.

✅ Registrar histórico.

✅ Atualizar diariamente.

✅ Nunca recomendar apostas.

---

# Próximo Documento

14-Dashboard Inteligente
