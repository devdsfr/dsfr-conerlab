# Prompt de QA automatizado — Claude in Chrome

Cole o bloco abaixo numa conversa nova do Claude no Chrome, com o navegador aberto.
Ele roda uma bateria de testes de interface contra o site em produção e devolve um
relatório com aprovados, reprovados e evidências.

**Antes de rodar:** se quiser cobrir as áreas logadas (Estratégias, Gestão de Banca,
Projeções), faça login você mesmo no site antes de iniciar — o prompt instrui o Claude
a não tentar autenticar sozinho. Sem login, ele roda só a parte pública e marca o
resto como "não testado".

**Segurança:** o prompt é read-only por design. Ele proíbe explicitamente excluir
estratégia, alterar assinatura, submeter formulário de conta e disparar o Discovery
(que é caro). Se quiser incluir os testes de escrita, veja a seção opcional no fim.

---

## Prompt (copie a partir daqui)

```
Você é o QA do CornerLab (https://dsfrcornerlab.com.br). Execute a bateria de testes
abaixo usando as ferramentas do navegador e, no fim, entregue um relatório.

REGRAS DE EXECUÇÃO — leia antes de começar:
1. Somente leitura. NÃO exclua estratégias, NÃO altere assinatura, NÃO submeta
   formulários de criar conta/redefinir senha, NÃO clique em "Procurar agora" na
   página Descobertas (dispara centenas de backtests em produção).
2. NÃO tente fazer login. Se uma página exigir autenticação e a sessão não estiver
   ativa, marque o teste como NÃO TESTADO e siga em frente.
3. Antes de cada página, comece a capturar console e rede; depois de carregar,
   leia as mensagens de erro do console e as requisições para /api/v1.
4. Registre evidência objetiva de cada teste: o que você viu na tela, o status HTTP
   observado, o texto exato de qualquer erro.
5. Não invente resultado. Se não conseguiu verificar algo, escreva "não verificado"
   e explique por quê. Um teste sem evidência é um teste não executado.

--- BLOCO 1 · DISPONIBILIDADE E CONSOLE ---
Visite estas páginas, uma a uma:
/visao-geral, /dashboard, /comparador, /filtros, /descobertas, /estrategias,
/banca, /projecoes, /assinatura, /integracoes, /suporte

Para cada uma, verifique:
1.1 A página renderiza conteúdo (não fica em branco nem presa no loading).
1.2 Zero erros no console do navegador.
1.3 Todas as chamadas para /api/v1 retornam 2xx. Anote qualquer 4xx/5xx com a URL.
1.4 O título/cabeçalho da página corresponde ao item de menu clicado.

--- BLOCO 2 · NAVEGAÇÃO E LAYOUT ---
2.1 No desktop, o menu lateral esquerdo está fixo, com a marca CornerLab centralizada
    no topo (logo, nome e a frase "inteligência estatística para escanteios"
    empilhados e centralizados).
2.2 O item do menu correspondente à página atual fica destacado em verde.
2.3 Clique em cada item do menu e confirme que a URL muda e o conteúdo troca.
2.4 Redimensione a janela para 390x844 (mobile). A barra lateral deve sumir e dar
    lugar a uma barra superior com o botão de menu à esquerda e a logo centralizada.
2.5 Ainda no mobile, abra o menu pelo botão e confirme que a lista de páginas aparece
    e que clicar num item navega e fecha o menu.
2.6 Volte para desktop (1440x900) antes de continuar.

--- BLOCO 3 · POLÍTICA ADSENSE (regressão importante) ---
Contexto: o Google reprovou o site por exibir anúncio em tela sem conteúdo. A correção
foi mover o anúncio para dentro do bloco de resultado. Verifique se continua correto.

3.1 Abra /dashboard SEM clicar em Analisar. NÃO deve existir nenhum elemento
    <ins class="adsbygoogle"> renderizado na página. Confirme inspecionando o DOM.
3.2 Idem para /comparador sem comparar e /filtros sem executar backtest.
3.3 Em /filtros, clique em "Executar backtest". Depois que o resultado aparecer, o
    slot de anúncio PODE existir. Confirme que ele está abaixo dos cards de métricas,
    não acima do formulário.
3.4 Em /visao-geral, selecione no calendário um dia SEM jogos (o painel mostra
    "Nenhum jogo mapeado nesse dia"). Não deve haver slot de anúncio nesse estado.

--- BLOCO 4 · SIMULADOR DE FILTROS ---
4.1 Em /filtros, com os valores padrão, clique em "Executar backtest".
4.2 O resultado deve trazer: partidas encontradas, taxa de acerto, ROI/Yield, lucro,
    média, maiores sequências, drawdown e a tabela de ocorrências.
4.3 "Partidas encontradas" deve ser MAIOR QUE ZERO. Se vier 0, isso é uma FALHA —
    anote os filtros exatos que estavam selecionados (campeonato, temporadas, métrica,
    linha, mando, odds).
4.4 Confira a coerência: o número de acertos + erros deve bater com partidas
    encontradas, e a taxa de acerto deve corresponder a acertos ÷ total.
4.5 Troque a métrica para "Gols" e execute de novo. Deve funcionar sem erro.
4.6 Repita para "Chutes". (Já houve um bug em que Chutes retornava sempre 0 —
    se voltar a acontecer, é regressão.)

--- BLOCO 5 · DESCOBERTAS E REPRODUTIBILIDADE (regressão importante) ---
5.1 Abra /descobertas. Liste quantos padrões estão publicados.
5.2 Para o PRIMEIRO card, anote: nome completo, nº de ocorrências, taxa de acerto,
    ROI, yield, drawdown, DSFR e classificação.
5.3 Expanda "Entender este padrão" e verifique que o texto: cita o tamanho da amostra,
    cita o período, e NÃO contém linguagem de recomendação de aposta (procure por
    "aposte", "recomendamos", "garantido", "vai bater", "certeza").
5.4 Clique em "Conferir no Simulador". O Simulador deve abrir com os filtros
    pré-preenchidos E executar o backtest automaticamente.
5.5 TESTE CRÍTICO: o número de "partidas encontradas" no Simulador deve bater
    (ou ficar muito próximo) das "ocorrências" que o card mostrava. Se o Simulador
    mostrar 0 partidas enquanto o card dizia 100+, é FALHA GRAVE de reprodutibilidade
    — anote quais temporadas ficaram marcadas no campo "Temporadas (backtest)".
5.6 Confirme que aparece o aviso "Critérios carregados de uma estratégia descoberta".

--- BLOCO 6 · FRESCOR DOS DADOS (worker) ---
6.1 Abra /integracoes e leia "Última sincronização".
6.2 Calcule há quantos dias/horas foi. Se tiver mais de 24 horas, marque como FALHA e
    registre a data exata — significa que o worker automático não está rodando.
6.3 Anote se está marcada como "manual" ou "cron". "Manual" indica que só rodou por
    clique humano.
6.4 Anote o status de cada provedor (OpenAI, API-Football, SportMonks) e o total de
    chamadas.

--- BLOCO 7 · ACESSIBILIDADE E CONTEÚDO ---
7.1 Confirme que existe o link "Pular para o conteúdo" ao navegar com Tab a partir do
    topo da página.
7.2 Navegue por Tab em /filtros e confirme que todos os campos e botões recebem foco
    visível, na ordem lógica.
7.3 Confirme que o rodapé com o aviso "A plataforma nunca recomenda apostas" aparece
    em todas as páginas.
7.4 Em /assinatura, confirme que a página explica o que é o CornerLab antes de falar
    de preço, e que existe a seção "O que o CornerLab não faz".

--- BLOCO 8 · ÁREAS LOGADAS (só se a sessão já estiver ativa) ---
Se não houver sessão, pule tudo e marque como NÃO TESTADO.
8.1 /estrategias lista as estratégias e, ao clicar numa, mostra o painel com DSFR,
    Health, ciclo de vida, confiança, robustez, volatilidade, risco e ranking.
8.2 O histórico de execuções aparece com pelo menos uma linha.
8.3 O painel "O que mudou desde a execução anterior" aparece (pode estar zerado).
8.4 /banca e /projecoes carregam sem erro (podem exibir paywall — isso é esperado e
    não é falha).

--- RELATÓRIO FINAL ---
Entregue nesta estrutura:

1. Tabela: ID do teste | Resultado (PASSOU / FALHOU / NÃO TESTADO) | Evidência curta
2. Lista das FALHAS em ordem de gravidade, cada uma com: o que era esperado, o que
   aconteceu, como reproduzir passo a passo.
3. Todos os erros de console e respostas HTTP não-2xx encontrados, com a URL.
4. Contagem final: executados, aprovados, reprovados, não testados.
5. Veredito: APROVADO (zero falhas) ou REPROVADO (uma ou mais falhas), sem meio-termo.

Não suavize resultado. Se algo falhou, diga que falhou.
```

---

## Extensão opcional — testes de escrita

Só rode isso se estiver logado e aceitar criar/apagar dados de teste em produção.
Cole como mensagem de follow-up **depois** que a bateria principal terminar:

```
Agora os testes de escrita. Estou ciente de que isso cria dados reais em produção e
autorizo. Mantenha-se estritamente ao roteiro:

E1. Em /filtros, execute um backtest, digite o nome "QA TESTE — pode apagar" no campo
    "Salvar como estratégia" e clique em Salvar. Confirme a mensagem de sucesso.
E2. Vá em /estrategias e confirme que "QA TESTE — pode apagar" aparece na lista.
E3. Abra a estratégia e clique em "Executar agora". Confirme que o histórico de
    execuções ganha uma linha nova e que os scores são preenchidos.
E4. PARE AQUI. Não exclua a estratégia — me avise para eu apagar manualmente, com
    o nome exato que ficou salvo.
```

---

## Quando rodar

- Depois de todo deploy (é o smoke test de release).
- Antes de pedir revisão no AdSense — o bloco 3 é justamente o que foi reprovado.
- Depois de criar os Cron Jobs no Render — o bloco 6 é o que confirma se o worker
  voltou a rodar sozinho.

## Falhas conhecidas nesta data (04/08/2026)

Se aparecerem, não são novidade — já estão mapeadas:

| Bloco | Falha esperada | Situação |
|---|---|---|
| 6.2 / 6.3 | Última sincronização parada e marcada como "manual" | Corrigido no repo (Dockerfile + render.yaml), falta criar os Cron Jobs no Render |
| 5.5 | Simulador mostrando 0 partidas ao vir de uma descoberta | Corrigido no repo, aguardando deploy |
