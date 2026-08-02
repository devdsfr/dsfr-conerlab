# Probability, Odds & Edge Engine

> Documento: 03-probabilidade-e-odds.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

Este documento define todas as regras utilizadas para calcular:

- Probabilidade Implícita
- Probabilidade Histórica
- Fair Odds
- Value Bet
- Edge
- Overround
- Eficiência da Estratégia

Todos esses cálculos serão utilizados pelo Betting Strategy Engine.

---

# Conceitos

Uma odd representa apenas o preço que a casa está oferecendo.

Ela NÃO representa necessariamente a probabilidade real do evento.

O objetivo do CornerLab é descobrir quando a probabilidade real é maior que a probabilidade implícita da odd.

Quando isso acontece existe vantagem matemática.

---

# Tipos de Probabilidade

O sistema deverá trabalhar com três probabilidades.

---

## Probabilidade Implícita

Calculada a partir da odd.

Fórmula

Pimplícita = 1 / Odd

Exemplo

Odd

1.50

Resultado

66,67%

---

## Probabilidade Histórica

Obtida através do banco de dados.

Exemplo

Últimos 20 jogos

Acima de 5 escanteios

17 jogos

Resultado

85%

---

## Probabilidade Ajustada

Probabilidade histórica ajustada utilizando filtros.

Exemplo

Últimos 10 jogos

Casa

Contra equipes Top 10

Últimos 90 dias

Resultado

81%

---

# Hierarquia

O sistema sempre utilizará

Probabilidade Ajustada

↓

Probabilidade Histórica

↓

Probabilidade Implícita

---

# Fair Odds

Fair Odd representa a odd justa.

Sem margem da casa.

Fórmula

Fair Odd

=

1

/

Probabilidade Real

---

Exemplo

Probabilidade

80%

Fair Odd

1.25

---

Outro

Probabilidade

75%

Fair Odd

1.33

---

Outro

Probabilidade

65%

Fair Odd

1.54

---

# Interpretação

Se a casa oferece

Odd

1.50

e

Fair Odd

1.25

Existe valor.

---

Se

Casa

1.20

Fair

1.25

Não existe valor.

---

# Edge

Edge representa a vantagem matemática.

Fórmula

Edge

=

Probabilidade Real

-

Probabilidade Implícita

---

Exemplo

Odd

1.60

Probabilidade Implícita

62,5%

Histórico

78%

Edge

15,5%

---

Outro

Odd

1.50

Histórico

68%

Implícita

66,67%

Edge

1,33%

---

# Classificação

Edge

Menor que 0

Negativo

---

0 até 3%

Muito baixo

---

3 até 6%

Baixo

---

6 até 10%

Bom

---

10 até 15%

Excelente

---

Acima de 15%

Excepcional

---

# Value Bet

Uma aposta somente poderá ser considerada Value Bet quando

Probabilidade Real

>

Probabilidade Implícita

e

Edge

>

0

---

# Exemplo

Odd

1.55

Implícita

64,52%

Histórico

81%

Resultado

Value Bet

Sim

---

Outro

Odd

1.30

Implícita

76,92%

Histórico

74%

Resultado

Value Bet

Não

---

# Overround

Overround representa a margem da casa.

Fórmula

Soma das probabilidades implícitas

-

100%

---

Exemplo

Mercado

Time A

2.00

↓

50%

Empate

3.50

↓

28,57%

Time B

3.80

↓

26,31%

Soma

104,88%

Margem

4,88%

---

# Eficiência da Estratégia

Criar indicador

IES

Índice de Eficiência da Estratégia

Componentes

Edge

40%

Win Rate

30%

ROI

20%

Drawdown

10%

Resultado

0

até

100

---

# Classificação IES

0-40

Ruim

---

41-60

Regular

---

61-75

Boa

---

76-90

Excelente

---

91-100

Elite

---

# Probabilidade Conjunta

Múltiplas

Sempre utilizar multiplicação.

Exemplo

85%

×

82%

=

69,70%

Nunca somar probabilidades.

---

# Comparador

Estratégia A

Odd

1.60

Probabilidade

75%

Edge

12,5%

---

Estratégia B

Odd

1.50

Probabilidade conjunta

72%

Edge

5,3%

---

Resultado

Estratégia A

Maior Edge

Maior EV esperado

---

# Dashboard

Mostrar

Odd

Fair Odd

Edge

Probabilidade

Implícita

Histórica

Ajustada

Value Bet

IES

---

# Alertas

Quando

Edge

ficar abaixo

2%

Mostrar

"A vantagem matemática da estratégia caiu."

---

Quando

Odd

ficar abaixo da Fair Odd

Mostrar

"A estratégia deixou de possuir valor esperado."

---

# API

GET

/strategy/value

Resposta

{
 "odd":1.55,
 "historicalProbability":0.81,
 "implicitProbability":0.6452,
 "fairOdd":1.23,
 "edge":0.1648,
 "valueBet":true,
 "ies":93
}

---

# Critérios de Aceite

✅ Calcular Probabilidade Implícita

✅ Calcular Fair Odd

✅ Calcular Edge

✅ Calcular Overround

✅ Identificar Value Bet

✅ Calcular IES

✅ Comparar estratégias

✅ Alertar perda de valor

✅ Trabalhar com múltiplas

---

# Próximo Documento

04-valor-esperado-ev.md

Neste documento será implementado

- EV

- EV Financeiro

- EV Acumulado

- EV por banca

- EV por mês

- EV por ano

- Comparação de estratégias

- Simulações financeiras
