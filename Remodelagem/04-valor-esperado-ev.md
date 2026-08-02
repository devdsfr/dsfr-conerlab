# Expected Value Engine (EV)

> Documento: 04-valor-esperado-ev.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

Calcular o Valor Esperado (Expected Value - EV) de qualquer estratégia cadastrada no CornerLab.

O EV será o principal indicador utilizado para determinar se uma estratégia possui vantagem matemática.

Nenhuma estratégia poderá ser considerada sustentável sem apresentar EV positivo.

---

# Definição

Valor Esperado representa o lucro médio esperado por aposta caso a estratégia seja repetida centenas ou milhares de vezes.

O EV não prevê o resultado da próxima aposta.

Ele mede a qualidade matemática da estratégia.

---

# Conceitos

Toda aposta possui dois possíveis resultados.

Vitória

ou

Derrota

Logo

EV depende apenas de

Probabilidade de vitória

Probabilidade de derrota

Lucro quando ganha

Perda quando perde

---

# Fórmula Oficial

EV

=

(Pwin × Lucro)

-

(Ploss × Perda)

Onde

Pwin

Probabilidade de vitória

Ploss

1 - Pwin

Lucro

Stake × (Odd - 1)

Perda

Stake

---

# Exemplo 1

Stake

100

Odd

1.60

Probabilidade

75%

Lucro quando ganha

60

Perda

100

EV

=

0.75 × 60

-

0.25 × 100

=

45

-

25

=

20

Resultado

EV = +20

---

Interpretação

A cada aposta de R$100

espera-se ganhar

R$20

no longo prazo.

---

# Exemplo 2

Stake

100

Odd

1.30

Probabilidade

70%

Lucro

30

Perda

100

EV

=

0.70 × 30

-

0.30 × 100

=

21

-

30

=

-9

Resultado

EV

Negativo

---

Interpretação

Mesmo acertando

70%

essa estratégia perde dinheiro.

---

# Classificação

EV < 0

Estratégia Perdedora

---

EV = 0

Break Even

---

0 até 5

Margem Muito Baixa

---

5 até 10

Boa

---

10 até 20

Excelente

---

Acima de 20

Elite

---

# EV Acumulado

Lucro Esperado

=

EV

×

Quantidade de Apostas

---

Exemplo

EV

20

Quantidade

100

Lucro Esperado

2000

---

Outro

EV

12

Quantidade

500

Lucro Esperado

6000

---

# EV Financeiro

Exemplo

Stake

500

Odd

1.55

Win Rate

82%

Lucro

275

EV

=

0.82 × 275

-

0.18 × 500

=

225,50

-

90

=

135,50

---

Resultado

Cada aposta

gera

R$135,50

de valor esperado.

---

# EV por mês

Usuário informa

Stake

500

Apostas

40

EV

135,50

Resultado

Lucro Esperado

R$5.420

---

# EV por Ano

Lucro Mensal

5420

↓

Anual

65040

---

# EV por banca

O sistema deverá calcular

150

500

1000

2000

5000

10000

20000

Sempre utilizando o mesmo EV.

---

# Comparador

Estratégia A

Odd

1.60

Win Rate

75%

Stake

100

EV

20

---

Estratégia B

Odd

1.50

85%

×

85%

=

72,25%

Lucro

50

EV

=

0.7225 × 50

-

0.2775 × 100

=

36,12

-

27,75

=

8,37

---

Resultado

Estratégia A

EV

20

Estratégia B

EV

8,37

Mesmo possuindo maior assertividade individual,

a múltipla gera menor valor esperado.

---

# Dashboard

Mostrar

Stake

Odd

Win Rate

EV

Lucro Esperado

Lucro Mensal

Lucro Anual

Classificação

---

# Alertas

Quando

EV < 0

Mostrar

"A estratégia possui expectativa matemática negativa."

---

Quando

EV cair mais de 20%

Mostrar

"A estratégia perdeu eficiência."

---

# Simulações

Executar

10 apostas

100

500

1000

10000

Mostrar

Lucro esperado

Lucro mínimo

Lucro máximo

ROI

Yield

Drawdown

---

# API

GET

/strategy/ev

Resposta

{
 "stake":100,
 "odd":1.60,
 "winRate":0.75,
 "ev":20,
 "classification":"Excellent",
 "monthlyExpected":800,
 "yearExpected":9600
}

---

# Regras

Nunca calcular EV utilizando porcentagem inteira.

Sempre utilizar decimal.

Nunca arredondar durante cálculos.

Sempre calcular utilizando Stake líquida.

---

# Critérios de Aceite

✅ Calcular EV.

✅ Classificar EV.

✅ Calcular EV acumulado.

✅ Calcular EV mensal.

✅ Calcular EV anual.

✅ Comparar estratégias.

✅ Gerar alertas.

✅ Simular qualquer quantidade de apostas.

---

# Observação Importante

EV positivo NÃO garante lucro em poucas apostas.

Uma estratégia com EV positivo pode apresentar prejuízo em 20, 30 ou até 100 apostas.

O EV representa expectativa matemática de longo prazo.

Quanto maior a amostra, maior a tendência dos resultados convergirem para o valor esperado.

---

# Próximo Documento

05-simulador-financeiro.md

Implementará

- Simulação completa da banca

- Crescimento composto

- Estratégia de 3 vitórias

- Retirada de lucro

- Evolução automática

- Comparação entre bancas

- Cenários

- Monte Carlo Financeiro
