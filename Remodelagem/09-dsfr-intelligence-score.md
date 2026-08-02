# DSFR Intelligence Score (DIS)

> Documento: 09-dsfr-intelligence-score.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

Criar um indicador proprietário capaz de resumir toda a qualidade estatística
de uma estratégia em um único Score.

O Score deverá variar entre

0

↓

100

Quanto maior o Score

maior a confiança matemática da estratégia.

---

# Filosofia

O usuário não deverá precisar interpretar

ROI

Yield

EV

Drawdown

Variância

Win Rate

Consistência

Individualmente.

O sistema deverá resumir todos esses indicadores em um único número.

---

# Objetivos

Responder

Essa estratégia é boa?

Ela piorou?

Ela melhorou?

Vale continuar utilizando?

Ela perdeu eficiência?

---

# Componentes

O DSFR Score será composto por

ROI

20%

---

EV

20%

---

Win Rate

15%

---

Yield

10%

---

Drawdown

10%

---

Quantidade de Jogos

10%

---

Consistência

10%

---

Variância

5%

---

Robustez Temporal

10%

---

Total

100%

---

# ROI Score

ROI

<0

↓

0

---

0%

↓

40

---

10%

↓

60

---

20%

↓

80

---

30%

↓

100

---

# EV Score

EV Negativo

↓

0

---

0

↓

40

---

5

↓

60

---

10

↓

80

---

20+

↓

100

---

# Win Rate

Menor

60%

↓

0

---

70%

↓

50

---

75%

↓

70

---

80%

↓

85

---

85%

↓

100

---

# Yield

Negativo

↓

0

---

5%

↓

60

---

10%

↓

80

---

15%

↓

100

---

# Drawdown

Maior que

40%

↓

0

---

30%

↓

40

---

20%

↓

70

---

10%

↓

90

---

5%

↓

100

---

# Quantidade de Jogos

Menor que

30

↓

0

---

100

↓

40

---

300

↓

70

---

500

↓

85

---

1000+

↓

100

---

# Consistência

Desvio padrão

↓

Consistência

↓

Score

---

Muito Alto

↓

20

---

Alto

↓

50

---

Médio

↓

75

---

Baixo

↓

100

---

# Robustez Temporal

Analisar

Últimos

3 anos

↓

O ROI permaneceu positivo?

↓

Sim

↓

100

---

Oscilou

↓

70

---

Negativo

↓

20

---

# Fórmula

DSFR Score

=

ROI × 0.20

+

EV × 0.20

+

WinRate × 0.15

+

Yield × 0.10

+

Drawdown × 0.10

+

Jogos × 0.10

+

Consistência × 0.10

+

Variância × 0.05

+

Robustez × 0.10

---

# Classificação

0-30

Ruim

★

---

31-50

Fraca

★★

---

51-70

Regular

★★★

---

71-85

Boa

★★★★

---

86-94

Excelente

★★★★★

---

95-100

Elite

🏆

---

# Dashboard

Mostrar

DSFR Score

★★★★★

92

---

Explicação

Estratégia extremamente consistente.

ROI elevado.

Baixo Drawdown.

Grande quantidade de jogos.

Baixa variância.

---

# Histórico

Guardar

Score diário

↓

Semanal

↓

Mensal

↓

Anual

---

# Evolução

Mostrar gráfico

DSFR Score

↓

Tempo

---

# Alertas

Quando

Score

↓

5 pontos

Mostrar

"A estratégia perdeu eficiência."

---

Quando

Score

↓

10 pontos

Mostrar

"Recomenda-se revisar os filtros."

---

Quando

Score

>

90

Mostrar

"Estratégia extremamente consistente."

---

# Comparador

Estratégia A

92

★★★★★

---

Estratégia B

81

★★★★

---

Estratégia C

63

★★★

---

Ordenar

Maior Score

↓

Menor Score

---

# API

GET

/strategy/score

Resposta

{

"score":92,

"classification":"Excellent",

"stars":5,

"roi":18,

"yield":11,

"ev":19,

"drawdown":7,

"confidence":96

}

---

# Critérios de Aceite

✅ Calcular Score.

✅ Explicar Score.

✅ Atualizar automaticamente.

✅ Comparar estratégias.

✅ Armazenar histórico.

✅ Exibir evolução.

---

# Observação

O DSFR Score nunca deverá substituir os indicadores individuais.

Ele será um resumo executivo.

O usuário poderá abrir cada componente e visualizar exatamente como o Score foi calculado.

---

# Próximo Documento

10-Strategy Health Engine
