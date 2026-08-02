# Strategy Health Engine (SHE)

> Documento: 12-strategy-health-engine.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

O Strategy Health Engine (SHE) será responsável por monitorar continuamente a saúde estatística de todas as estratégias cadastradas.

Seu objetivo é detectar automaticamente sinais de melhoria, estabilidade ou deterioração antes que o usuário perceba perdas de desempenho.

---

# Filosofia

Uma estratégia não deixa de funcionar de um dia para outro.

Ela normalmente apresenta pequenos sinais de deterioração.

O sistema deverá identificar esses sinais automaticamente.

---

# Objetivos

Responder perguntas como

Minha estratégia continua saudável?

Ela está melhorando?

Ela está piorando?

Vale continuar utilizando?

Quando ela começou a perder eficiência?

Qual indicador provocou a queda?

---

# Indicadores Monitorados

Win Rate

ROI

Yield

EV

Drawdown

Edge

DSFR Score

Consistência

Variância

Robustez

Quantidade de jogos

---

# Cálculo do Health Score

O Health Score representa a evolução da estratégia.

Enquanto o DSFR Score mede qualidade,

o Health mede tendência.

Escala

-100

↓

0

↓

+100

---

# Interpretação

+80

Melhora muito forte

---

+40

Melhora consistente

---

0

Estável

---

-30

Início de deterioração

---

-60

Queda relevante

---

-90

Estratégia crítica

---

# Fórmula Conceitual

Health Score =

ΔROI

+

ΔWinRate

+

ΔEV

+

ΔDrawdown

+

ΔConsistência

+

ΔVariância

Cada componente possuirá peso configurável.

---

# Janelas de Comparação

Últimos

7 dias

30 dias

60 dias

90 dias

180 dias

365 dias

---

# Exemplo

ROI

18%

↓

14%

↓

-4%

---

Win Rate

84%

↓

81%

↓

-3%

---

Drawdown

8%

↓

14%

↓

+6%

---

Resultado

Health

-28

---

# Diagnóstico

O sistema deverá gerar explicações automáticas.

Exemplo

"A estratégia apresentou redução de ROI nos últimos 60 dias acompanhada por aumento do Drawdown."

---

# Tendências

Detectar

Melhora

Estabilidade

Deterioração

Recuperação

Mudança estrutural

---

# Dashboard

Mostrar

Health Score

↓

Gráfico temporal

↓

Motivos

↓

Indicadores responsáveis

---

# Linha do Tempo

Exibir

Data

Evento

Mudança

Impacto

---

Exemplo

01/03

ROI aumentou

+3%

---

18/03

Win Rate caiu

-5%

---

02/04

Drawdown aumentou

+7%

---

# Alertas

Quando

Health < -30

↓

Gerar alerta

---

Quando

Health < -60

↓

Alerta crítico

---

Quando

Health > +30

↓

Informar melhora consistente

---

# Benchmark

Comparar

Estratégia atual

↓

Média histórica

↓

Últimos 30 dias

↓

Últimos 90 dias

---

# API

GET

/strategy/health

Resposta

{

"health":-28,

"trend":"falling",

"roiVariation":-4,

"drawdownVariation":6,

"consistencyVariation":-3,

"recommendation":"Monitorar"

}

---

# Critérios de Aceite

✅ Calcular Health Score.

✅ Atualizar diariamente.

✅ Registrar histórico.

✅ Explicar mudanças.

✅ Gerar alertas.

✅ Comparar períodos.

---

# Próximo Documento

13-Opportunity Engine
