# Fundamentos Matemáticos

> Documento: 02-fundamentos-matematicos.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

Definir todas as bases matemáticas utilizadas pelo Betting Strategy Engine (BSE).

Todos os cálculos financeiros, estatísticos e probabilísticos deverão utilizar exclusivamente os conceitos descritos neste documento.

Nenhum cálculo poderá utilizar fórmulas diferentes das especificadas aqui.

---

# Filosofia

O objetivo do CornerLab não é prever o futuro.

O objetivo é medir se uma estratégia possui vantagem matemática suficiente para gerar lucro no longo prazo.

O sistema nunca trabalhará com certeza.

Sempre trabalhará com probabilidades.

---

# Conceitos Fundamentais

Uma aposta possui quatro componentes fundamentais.

## Odd

Representa o retorno oferecido pela casa de apostas.

Exemplo

Odd

1.60

Significa

Para cada R$ 1 apostado

Retorno

R$ 1,60

Lucro

R$ 0,60

---

## Stake

Valor investido.

Exemplo

Stake

R$ 150

---

## Probabilidade

Chance estimada de sucesso.

Representada em porcentagem.

Exemplo

80%

Ou

0.80

---

## Resultado

Vitória

ou

Derrota

Não existe meio resultado para os cálculos do Engine.

---

# Conversões

Todo percentual deverá possuir equivalente decimal.

| Percentual | Decimal |
|------------|---------|
| 50% | 0.50 |
| 60% | 0.60 |
| 70% | 0.70 |
| 75% | 0.75 |
| 80% | 0.80 |
| 85% | 0.85 |
| 90% | 0.90 |

---

# Conceito de Probabilidade

A probabilidade representa a frequência esperada de sucesso ao longo de muitas tentativas.

Exemplo

80%

não significa

Acertará 8 das próximas 10 apostas.

Significa

Que em milhares de apostas semelhantes

espera-se aproximadamente

80% de sucesso.

---

# Independência

Cada aposta deverá ser considerada independente.

Exemplo

Flamengo

80%

Palmeiras

80%

A probabilidade conjunta será

0.80 × 0.80

=

0.64

64%

Nunca utilizar

80%

para representar duas apostas simultâneas.

---

# Probabilidade Conjunta

Fórmula

P(A e B)

=

P(A)

×

P(B)

Exemplo

85%

×

85%

=

72,25%

---

Outro exemplo

90%

×

90%

=

81%

---

Outro

75%

×

75%

=

56,25%

---

# Probabilidade Implícita

Toda odd possui uma probabilidade implícita.

Fórmula

Probabilidade

=

1

/

Odd

---

Exemplo

Odd

2.00

Probabilidade

50%

---

Odd

1.50

Probabilidade

66,67%

---

Odd

1.60

Probabilidade

62,50%

---

Odd

1.80

Probabilidade

55,56%

---

Odd

2.20

Probabilidade

45,45%

---

# Break Even

Representa a taxa mínima de acerto para não perder dinheiro.

É exatamente igual à probabilidade implícita da odd.

Exemplo

Odd

1.50

Break Even

66,67%

---

Odd

1.60

Break Even

62,50%

---

Odd

1.30

Break Even

76,92%

---

# Exemplo Real

Sua estratégia

Odd

1.50

Necessário

66,67%

Se sua estratégia acerta

80%

Existe vantagem matemática.

Se acerta

60%

Existe prejuízo esperado.

---

# Valor Esperado

Ainda não será calculado neste documento.

Mas toda estratégia será classificada.

EV Positivo

Quando

Probabilidade Real

>

Probabilidade Implícita.

---

EV Negativo

Quando

Probabilidade Real

<

Probabilidade Implícita.

---

# Exemplo

Odd

1.60

Probabilidade Implícita

62,50%

Histórico

75%

Resultado

Existe Edge.

---

Outro

Odd

1.40

Probabilidade Implícita

71,43%

Histórico

68%

Resultado

Não existe vantagem.

---

# Lei dos Grandes Números

Todos os cálculos assumem que

quanto maior a quantidade de apostas

mais os resultados tendem ao esperado.

Por isso

10 apostas

não validam estratégia.

100 apostas

ainda representam amostra pequena.

500 apostas

já permitem análises melhores.

1000+

representam excelente base estatística.

---

# Variância

Mesmo estratégias lucrativas apresentam perdas.

Isso é esperado.

O sistema nunca deverá considerar uma sequência negativa como prova de falha.

Sempre analisar

longo prazo.

---

# Premissas

Todo cálculo utilizará

Eventos independentes.

Odds decimais.

Probabilidades reais.

Stake fixa (salvo quando configurado reinvestimento).

---

# Regras de Negócio

Nunca calcular EV utilizando porcentagens inteiras.

Sempre converter

80%

↓

0.80

---

Nunca armazenar probabilidades como string.

Sempre utilizar decimal.

---

Toda fórmula deverá utilizar precisão mínima de quatro casas decimais.

---

Todo resultado financeiro deverá ser arredondado apenas na apresentação.

Nunca durante os cálculos.

---

# Casos de Uso

## Caso 1

Odd

1.60

Probabilidade

75%

Resultado esperado

Estratégia possui vantagem.

---

## Caso 2

Odd

1.30

Probabilidade

70%

Resultado esperado

Estratégia possui expectativa negativa.

---

## Caso 3

Múltipla

80%

×

85%

Resultado esperado

68%

---

# Critérios de Aceite

✅ Converter corretamente porcentagem para decimal.

✅ Calcular probabilidade conjunta.

✅ Calcular probabilidade implícita.

✅ Calcular Break Even.

✅ Identificar quando existe vantagem matemática.

✅ Trabalhar com precisão mínima de quatro casas decimais.

✅ Não arredondar durante cálculos.

✅ Validar independência entre eventos.

✅ Suportar odds entre 1.01 e 1000.

---

# Próximo Documento

03-probabilidade-e-odds.md

Neste documento será implementado

- Odds Justas
- Odds de Valor
- Edge
- Overround da Casa
- Fair Odds
- Cálculo da Margem da Casa
- Eficiência Matemática
