# Backtesting Engine

> Documento: 06-backtesting-engine.md
>
> Projeto: DSFR CornerLab
>
> Versão: 1.0

---

# Objetivo

O Backtesting Engine será responsável por validar estratégias utilizando
dados históricos armazenados no PostgreSQL.

Nenhuma análise será realizada diretamente na API externa.

Todo processamento ocorrerá utilizando dados locais.

---

# Filosofia

O objetivo do Backtesting NÃO é provar que uma estratégia funciona.

O objetivo é medir:

- Robustez
- Consistência
- Sustentabilidade
- Rentabilidade

---

# Objetivos

Responder perguntas como

Se eu tivesse utilizado essa estratégia em 2024...

Quanto teria lucrado?

Qual teria sido meu ROI?

Quantas apostas venceriam?

Qual seria meu Drawdown?

Qual seria minha maior sequência negativa?

Quanto minha banca teria crescido?

---

# Fluxo

Usuário cria filtro

↓

Sistema busca partidas

↓

Executa estratégia

↓

Simula todas as apostas

↓

Calcula indicadores

↓

Retorna relatório

---

# Entrada

Campeonato

Temporada

Equipe

Casa

Fora

Linha de escanteios

Quantidade mínima

Quantidade máxima

Últimos jogos

Odd média

Stake

Modelo de banca

---

# Filtros

Permitir

Equipe

Adversário

Casa

Fora

Data

Temporada

Campeonato

Escanteios

Cartões

Posse

Finalizações

Dias de descanso

Ranking

Forma

---

# Estratégias

Exemplo

Últimos 10 jogos

↓

Mais de 5 escanteios

↓

Casa

↓

Odd mínima 1.40

↓

Odd máxima 1.70

↓

Stake fixa

---

# Processamento

Cada partida deverá ser tratada como uma aposta independente.

Resultado

Win

Loss

Void

---

# Resultado

Mostrar

Quantidade de jogos

Quantidade de apostas

Vitórias

Derrotas

Empates

Void

Win Rate

Loss Rate

ROI

Yield

EV

Lucro

Capital Final

Capital Máximo

Capital Mínimo

---

# Sequências

Calcular

Maior sequência positiva

Maior sequência negativa

Maior sequência de greens

Maior sequência de reds

Tempo para recuperação

---

# Drawdown

Calcular

Maior Drawdown

Drawdown médio

Drawdown percentual

Tempo de recuperação

---

# Crescimento da banca

Mostrar

Capital inicial

↓

Capital final

↓

Lucro

↓

ROI

↓

Yield

↓

Curva de crescimento

---

# Curva Financeira

Gerar gráfico

Capital

↓

Tempo

---

# Heatmap

Criar mapa

Vitórias

↓

Campeonato

↓

Mês

↓

Equipe

↓

Casa

↓

Fora

---

# Performance

Agrupar

Por campeonato

↓

Por equipe

↓

Por temporada

↓

Por treinador

↓

Por árbitro

↓

Por estádio

---

# Score de Confiança

Calcular

Baseado em

Quantidade de jogos

Win Rate

ROI

Drawdown

Variância

Consistência

Resultado

0

↓

100

---

# Classificação

0-40

Ruim

41-60

Regular

61-75

Boa

76-90

Excelente

91-100

Elite

---

# Comparador

Estratégia A

↓

Executar

↓

Resultado

---

Estratégia B

↓

Executar

↓

Resultado

---

Mostrar

Maior lucro

Maior ROI

Maior Yield

Maior EV

Maior Drawdown

Maior Win Rate

Maior estabilidade

---

# Dashboard

Resumo

Jogos

Estratégias

ROI

Yield

Lucro

Capital

DSFR Score

Confiança

---

# Exportação

Permitir

CSV

Excel

PDF

JSON

---

# API

POST

/backtesting/run

Entrada

{

"league":71,

"season":2026,

"team":33,

"line":5.5,

"lastGames":10,

"odd":1.55,

"stake":100

}

Resposta

{

"games":128,

"wins":103,

"losses":25,

"winRate":80.47,

"roi":18.3,

"yield":11.2,

"ev":22,

"profit":2840,

"capital":12840,

"drawdown":620,

"confidence":94

}

---

# Critérios de Aceite

✅ Executar estratégias históricas.

✅ Utilizar apenas PostgreSQL.

✅ Calcular ROI.

✅ Calcular Yield.

✅ Calcular EV.

✅ Calcular Drawdown.

✅ Calcular Win Rate.

✅ Gerar Score.

✅ Exportar resultados.

✅ Comparar estratégias.

---

# Regras

Nunca alterar dados históricos.

Nunca recalcular resultados antigos.

Todo processamento deverá ser determinístico.

Todos os cálculos deverão ser reproduzíveis.

---

# Próximo Documento

07-inteligencia-artificial.md
