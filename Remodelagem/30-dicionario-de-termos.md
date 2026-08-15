# Dicionário do CornerLab — termos e cálculos

Todo termo que aparece na tela, explicado em português comum, com a fórmula exata
usada no código. As referências entre parênteses (Catálogo NN) apontam para o número
da fórmula no documento 27 (Formula Catalog v1.0), que é a fonte única de verdade —
o código em `backend/internal/formulas/` é a implementação literal dele.

Convenção: **odd** é sempre decimal (2.20 = você recebe 2,20 por 1 apostado).
**Stake** é o valor de cada aposta. **Unidade** é uma stake — quando o app diz
"lucro de 13,80 unidades", significa 13,8 vezes a stake que você configurou.

---

## Índice

1. [Conceitos de base](#1-conceitos-de-base)
2. [Probabilidade e odds](#2-probabilidade-e-odds)
3. [Retorno financeiro](#3-retorno-financeiro)
4. [Risco](#4-risco)
5. [Gestão de banca (staking)](#5-gestão-de-banca-staking)
6. [Scores proprietários do CornerLab](#6-scores-proprietários-do-cornerlab)
7. [Termos do Discovery Engine](#7-termos-do-discovery-engine)
8. [Termos técnicos da arquitetura](#8-termos-técnicos-da-arquitetura)

---

## 1. Conceitos de base

### Amostra (ou "ocorrências", "partidas encontradas")
Quantos jogos históricos bateram com os critérios do filtro. É o número mais
importante da tela e o que mais gente ignora: 100% de acerto em 8 jogos não
significa nada; 89% em 114 jogos já é um sinal. Por isso o sistema recusa publicar
qualquer descoberta com menos de 50 jogos, mesmo que os números pareçam perfeitos.

### Taxa de acerto (win rate)
Percentual de vezes em que o critério se confirmou.

```
taxa de acerto = acertos / total de jogos × 100
```

Sozinha, ela engana: um critério com 95% de acerto pagando odd 1.02 dá prejuízo.
Por isso o CornerLab nunca ranqueia por taxa de acerto — é uma regra explícita do
motor de descobertas.

### Janela ("últimos 5/10/20 jogos")
Recorte temporal da análise. "Últimos 10" olha só as 10 partidas mais recentes de
cada time, em vez da temporada inteira. Serve pra capturar momento de forma: um time
pode ter média de 5 escanteios no ano e 8 nos últimos cinco jogos.

### Mando (casa / fora / qualquer)
Se a análise considera só jogos em que o time atuou em casa, só fora, ou os dois.
Muda bastante: times costumam pressionar mais em casa, o que puxa escanteios pra cima.

### Linha / limiar ("acima de 8.5")
O corte que define acerto. "Escanteios 8.5+" quer dizer: o jogo precisa ter 9 ou mais
escanteios no total pra contar como acerto. O ",5" existe pra eliminar empate —
não existe "8,5 escanteios", então ou fica abaixo ou fica acima.

### Tier de adversário (G6 / G12 / Z4)
Faixa do adversário na tabela: G6 = seis primeiros, G12 = doze primeiros, Z4 = quatro
últimos. Serve pra separar "esse time faz muitos escanteios" de "esse time faz muitos
escanteios *contra time fraco*".

---

## 2. Probabilidade e odds

### Probabilidade (Catálogo 01)
```
P = eventos favoráveis / eventos possíveis
```
Retorna de 0 a 1. Em 114 jogos com 102 acertos: 102/114 = 0,895 (89,5%).

### Probabilidade implícita (Catálogo 02)
A chance que a casa de apostas está embutindo na odd que ela oferece.
```
P implícita = 1 / odd
```
Odd 2.20 → 1/2,20 = 45,5%. Ou seja: a casa está dizendo que esse evento acontece em
45,5% das vezes. (Na prática ela embute a margem dela aí dentro, então a chance real
que ela estima é um pouco menor.)

### Odd justa (Catálogo 03)
O contrário: dada uma probabilidade, qual odd pagaria exatamente o valor justo.
```
odd justa = 1 / P
```
Se seu histórico mostra 89,5% de acerto, a odd justa é 1/0,895 = **1,12**. Se a casa
está pagando mais que isso, matematicamente há vantagem; se paga menos, não há.
O sistema nunca deixa cair abaixo de 1,01.

### Odd de equilíbrio / break-even (Catálogo 04)
A taxa de acerto mínima pra você não perder dinheiro apostando sempre naquela odd.
```
break-even = 1 / odd
```
Odd 2.20 → você precisa acertar 45,5% das vezes só pra empatar. Acertou menos, perdeu
dinheiro, por mais que "quase" tenha dado certo.

### Edge (vantagem) (Catálogo 05)
```
edge = probabilidade real − probabilidade implícita
```
Positivo significa que seu histórico aponta chance maior do que a odd sugere.
Importante: "probabilidade real" aqui é uma **estimativa a partir do passado**, não
um fato sobre o futuro.

---

## 3. Retorno financeiro

### EV — valor esperado (Catálogo 06)
Quanto você ganharia, em média, por aposta, se aquele cenário se repetisse muitas vezes.
```
EV = (P vitória × lucro) − (P derrota × perda)
lucro = stake × (odd − 1)     perda = stake
```
Exemplo: stake 100, odd 1.60, 75% de acerto →
`0,75 × 60 − 0,25 × 100 = 45 − 25 =` **EV +20**.

EV negativo é o sinal mais duro que existe: significa que, repetindo aquilo, a
tendência matemática é perder — mesmo com taxa de acerto alta.

### ROI — retorno sobre investimento (Catálogo 07)
```
ROI = lucro / investimento × 100
```
Lucro de 500 sobre 2.000 investidos = 25%.

### Yield (Catálogo 08)
```
yield = lucro / volume total apostado × 100
```
Parece igual ao ROI, mas mede outra coisa: ROI olha o capital que você colocou;
yield olha o **volume movimentado**. Se você aposta a mesma stake fixa em tudo, os
dois batem — é por isso que no Simulador eles costumam aparecer com o mesmo número.
Eles se separam quando a stake varia entre apostas.

### Profit Factor (Catálogo 13)
```
PF = lucro bruto / prejuízo bruto
```
Acima de 1 = ganhou mais do que perdeu. 2,0 significa que pra cada real perdido você
ganhou dois.

### Recovery Factor (Catálogo 33)
```
RF = lucro líquido / drawdown máximo
```
Mede se o lucro compensa o tamanho do tombo que você levou no caminho. Lucro de 100
com drawdown de 100 (RF = 1) é bem diferente de lucro de 100 com drawdown de 10 (RF = 10).

### Expectancy (Catálogo 36 e 37)
```
E = (taxa de acerto × ganho médio) − (taxa de erro × perda média)
E% = E / stake média × 100
```
É o EV calculado sobre o desempenho real observado, em vez de sobre uma probabilidade
teórica.

---

## 4. Risco

### Variância (Catálogo 20) e desvio padrão (Catálogo 21)
```
variância σ² = Σ(x − média)² / N
desvio padrão σ = √variância
```
Medem o quanto os valores oscilam em torno da média. Dois times com média de 5
escanteios são coisas diferentes se um faz sempre 5 e o outro alterna entre 1 e 9 —
o segundo tem desvio padrão alto. Desvio alto = menos previsível.

### Drawdown máximo (Catálogo 12)
A maior queda entre um pico e o vale seguinte na sua curva de banca.
```
drawdown = (pico − vale) / pico
```
Se sua banca chegou a 1.000 e caiu pra 750 antes de voltar a subir, o drawdown foi
25%. É o número que responde "qual o pior momento que eu teria que ter aguentado
sem desistir". No Simulador ele aparece em unidades de stake (drawdown absoluto);
no motor de descobertas é convertido pra % do capital movimentado.

### Sharpe adaptado (Catálogo 34)
```
Sharpe = (ROI médio − ROI livre de risco) / desvio padrão dos ROIs
```
Retorno ajustado ao risco: quanto de retorno você tira por unidade de oscilação.
"Adaptado" porque no mercado financeiro existe uma taxa livre de risco (tesouro);
no esporte não existe equivalente, então o sistema permite usar zero.

### Calmar adaptado (Catálogo 35)
```
Calmar = ROI / drawdown máximo
```
Mesma ideia do Sharpe, mas usando o pior tombo como medida de risco em vez do desvio
padrão.

### Monte Carlo
Simulação: em vez de calcular uma fórmula fechada, o sistema **sorteia** milhares de
sequências possíveis de apostas com a sua taxa de acerto e sua odd, e observa a
distribuição dos resultados. Responde perguntas como "em quantos por cento dos
cenários eu quebro?" (probabilidade de ruína) e "qual o resultado no cenário
mediano vs. no cenário ruim?". Usa semente fixa, então a mesma entrada sempre gera
o mesmo resultado.

---

## 5. Gestão de banca (staking)

### Kelly (Catálogo 14)
```
Kelly = (b × p − q) / b
b = odd − 1     p = taxa de acerto     q = 1 − p
```
Fração da banca que maximiza o crescimento no longo prazo. Resultado negativo
significa "não há vantagem, não aposte". Na prática, muita gente usa uma fração do
Kelly (meio Kelly, quarto de Kelly) porque o Kelly cheio oscila demais e depende de
a sua estimativa de probabilidade estar certa — se ela estiver otimista, o Kelly
cheio quebra a banca.

### Stake percentual (Catálogo 15) e stake fixa (Catálogo 16)
```
stake percentual = banca × percentual
stake fixa = sempre o mesmo valor
```
Percentual acompanha a banca (cresce quando ganha, encolhe quando perde);
fixa ignora a banca.

### Reinvestimento (Catálogo 17) e juros compostos (Catálogo 18)
```
nova stake = stake + lucro
capital final = capital inicial × (1 + taxa)^períodos
```
É a matemática por trás da página de Projeções.

### CAGR (Catálogo 38)
```
CAGR = (capital final / capital inicial)^(1/anos) − 1
```
Taxa de crescimento anual composta — permite comparar períodos de duração diferente.

---

## 6. Scores proprietários do CornerLab

Todos vão de 0 a 100 e combinam vários indicadores. Os pesos abaixo são os que estão
no código; mudá-los exige nova versão do Formula Catalog.

### Índice de consistência (Catálogo 22)
```
40% taxa de acerto + 20% (1 − variância) + 20% (1 − drawdown) + 20% robustez
```
Mede se o desempenho é regular ou aos trancos.

### Confiança (Catálogo 23)
Média simples de quatro coisas normalizadas: volume de jogos, consistência,
baixa variância e robustez temporal. Responde "o quanto dá pra levar esse número
a sério" — é essencialmente uma medida de quão sólida é a amostra.

### DSFR Score (Catálogo 24)
O score-resumo da plataforma. Média ponderada de oito componentes:

| Componente | Peso |
|---|---|
| ROI | 20% |
| EV | 20% |
| Taxa de acerto | 15% |
| Yield | 10% |
| Drawdown (invertido) | 10% |
| Tamanho da amostra | 10% |
| Consistência | 10% |
| Variância (invertida) | 5% |

"Invertido" significa que quanto menor o drawdown/variância, mais pontos. Repare que
taxa de acerto vale só 15% — de propósito, pra impedir que um critério de acerto alto
e retorno ruim suba no ranking.

**Faixas de classificação:** Elite 91-100 · Excelente 81-90 · Muito Boa 71-80 ·
Boa 61-70 · Regular 40-60 · Descartar abaixo de 40.

### Health Score (Catálogo 25)
Saúde **recente**, comparando o período atual com o anterior:
```
Health = 50 + 50 × média(ΔROI, ΔEV, −ΔDrawdown, ΔConsistência)
```
- **50** = estável, ou primeira execução (sem histórico pra comparar)
- **acima de 50** = melhorando
- **abaixo de 50** = piorando

O drawdown entra com sinal invertido: drawdown subindo derruba a saúde.

### Opportunity Score (Catálogo 26)
```
30% Health + 25% DSFR + 20% ROI + 15% taxa de acerto + 10% consistência
```
Prioriza o que merece atenção **agora** — por isso Health pesa mais que DSFR aqui,
ao contrário do ranking geral.

### Ciclo de vida (Catálogo 27)
Classificação em cinco estágios, a partir da amostra, da saúde e da tendência:

| Estágio | Quando |
|---|---|
| Nascimento | amostra ainda abaixo do mínimo |
| Crescimento | tendência acima de +0,15 |
| Maturidade | saudável e estável |
| Declínio | saúde abaixo de 45 ou tendência abaixo de −0,15 |
| Obsoleta | saúde abaixo de 25 |

### Ranking (Catálogo 28)
A chave de ordenação da lista de estratégias:
```
35% DSFR + 25% Health + 20% ROI + 10% Yield + 10% Confiança
```

### Tendência / Trend (Catálogo 29)
```
50% (variação nos últimos 5) + 30% (últimos 10) + 20% (últimos 20)
```
Resultado entre −1 e +1. Janelas recentes pesam mais, porque o objetivo é detectar
mudança de rumo, não média histórica.

### Robustez (Catálogo 30)
Média simples de cinco fatores normalizados: volume de jogos, consistência, baixa
variância, ROI e histórico temporal. É a "solidez estatística" do padrão.

### Volatilidade (Catálogo 31)
Média de: desvio padrão, oscilação de ROI e oscilação de EV.
**Quanto maior, pior** — indica resultado instável.

### Risco (Catálogo 32)
Média de: drawdown, variância, volatilidade e taxa de erro.
**Quanto maior, pior.**

---

## 7. Termos do Discovery Engine

### Critérios mínimos de aprovação
Uma combinação só é publicada na página Descobertas se passar em **todos**:

| Critério | Valor mínimo |
|---|---|
| Jogos na amostra | 100 (trava absoluta: nunca abaixo de 50) |
| Taxa de acerto | 75% |
| ROI | 10% |
| Yield | 5% |
| Drawdown máximo | até 20% do capital movimentado |
| DSFR Score | 40 |
| Teto por liga | 40 descobertas publicadas |

### Overfitting
Quando um padrão parece perfeito só porque a amostra é pequena demais. Com 6 jogos é
fácil achar "100% de acerto" por puro acaso — e isso não se repete. A trava de 50
jogos existe exatamente pra barrar isso, e há um teste automatizado no código
(`TestOverfittingGuardRejectsTinySampleWithPerfectNumbers`) que garante que amostra
minúscula com números perfeitos seja rejeitada.

### Motivos de rejeição
Registrados a cada ciclo pra saber qual critério mais barra: `amostra_insuficiente`,
`win_rate_baixo`, `roi_baixo`, `yield_baixo`, `ev_nao_positivo`, `drawdown_alto`,
`score_baixo`.

### Estratégia "descoberta" vs. estratégia do usuário
**Descoberta** (`origin=discovery`): criada pelo sistema, pública, sem dono, nome
gerado automaticamente a partir dos parâmetros. **Do usuário** (`origin=user`):
criada por você no Simulador via "Salvar como estratégia", privada, nome livre.

---

## 8. Termos técnicos da arquitetura

### Backtest
Rodar um critério contra o histórico e ver o que teria acontecido. Todo número da
plataforma vem daqui. **Não é previsão** — é reconstituição do passado.

### Camada RAW
A resposta bruta do provedor, guardada exatamente como chegou e nunca alterada.
Serve pra poder reprocessar tudo do zero se descobrirmos um erro de cálculo, sem
precisar pedir os dados de novo.

### Camada NORMALIZADA
Os mesmos dados já organizados em tabelas limpas: jogos, times, ligas, temporadas.

### Camada ANALYTICS
O resultado já calculado (métricas, scores, saúde), esperando parado no banco.
É de onde as telas leem.

### Worker
Processo que roda em segundo plano, sozinho, sem ninguém pedir. O princípio da
arquitetura é "workers calculam, usuário só lê" — por isso as telas abrem rápido
mesmo cruzando milhares de jogos.

### Cron Job
Agendamento que dispara o worker de tempos em tempos (de hora em hora para
sincronizar jogos, uma vez por dia para o Discovery Engine).

### Idempotente
Operação que pode ser repetida sem duplicar nada. A sincronização é idempotente:
rodar duas vezes o mesmo dia não cria jogos repetidos.

### `worker_runs`
Tabela de auditoria: registra cada execução de worker, quanto tempo levou, quantos
itens processou e quantos erros deu. É de onde a tela de Integrações tira o
"última sincronização".

---

## Aviso

Nada neste documento é recomendação de aposta. Todas as fórmulas descrevem
desempenho **histórico** de critérios aplicados a dados passados. Resultado passado
não garante resultado futuro, e nenhum score alto muda isso — inclusive porque a
própria amostra pode estar enviesada pelo período analisado.
