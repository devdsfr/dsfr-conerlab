# Estratégia de monetização — CornerLab

## Resumo executivo

O CornerLab tem um ativo raro para monetização: tráfego qualificado (gente olhando odds e estatísticas de escanteio na hora de decidir uma aposta). Isso abre três caminhos — anúncios contextuais genéricos, afiliação com casas de apostas, e assinatura premium da própria ferramenta — mas o Brasil regulamentou pesado a publicidade de apostas em 2024/2025, e desde a Lei Complementar 224/2025 quem divulga uma casa não licenciada pode responder solidariamente pelos tributos dela. Isso muda a ordem de prioridade: não dá para simplesmente "colocar anúncios" sem checar quem está anunciando.

Recomendação de fundo: comece pela assinatura premium (menor risco, maior margem, reforça a marca) e trate anúncios/afiliação como uma segunda camada, só com operadores licenciados pela SPA. Os detalhes de cada rota estão abaixo.

## O que mudou na regulação (e por que importa para você)

Desde 2024/2025 a publicidade de apostas no Brasil segue o Anexo X do CONAR e a Portaria SPA/MF nº 1.231/2024: todo anúncio precisa do símbolo "+18", ninguém no anúncio pode parecer menor de 21 anos, e são proibidas promessas de ganho fácil ou termos como "aposte grátis".

O ponto que mais afeta o CornerLab é este: a obrigação de checar se o anunciante tem autorização da Secretaria de Prêmios e Apostas (SPA/Ministério da Fazenda) deixou de ser só da casa de apostas — se estende a afiliados, influenciadores e veículos que divulgam. E a Lei Complementar 224/2025 criou responsabilidade solidária: se você continuar promovendo uma bet não licenciada depois de notificado, pode ser cobrado pelos tributos dela. As sanções para o operador ilegal chegam a 20% do faturamento e cassação da licença — mas o risco para quem divulga é o que interessa aqui.

Na prática: antes de aceitar qualquer parceria de afiliados ou rede de anúncios voltada a apostas, é preciso confirmar que cada casa anunciada está na lista oficial de operadores autorizados da SPA. Redes de anúncios genéricas de iGaming (Adsterra, HilltopAds, PropellerAds e afins) servem anúncios programáticos de qualquer operador, licenciado ou não — usar uma dessas sem curadoria é o cenário de maior risco.

## A tensão com a marca do CornerLab

O rodapé do app já diz "a plataforma nunca recomenda apostas" — é a base de confiança do produto (você organiza dados, não empurra aposta). Colocar banners ou links de afiliado de casas de apostas dentro do próprio dashboard entra em tensão direta com essa promessa: para o usuário, vai parecer que a "ferramenta neutra" está sendo paga para indicar onde apostar.

Isso não inviabiliza anúncios — só significa que, se for por esse caminho, o ideal é isolar visualmente essa monetização (ex.: uma seção "parceiros" separada, nunca misturada com os números do dashboard) e manter a assinatura premium como o motor principal de receita, já que ela monetiza a ferramenta em si, sem depender de promover apostas.

## As rotas de monetização

| Rota | Receita potencial | Risco regulatório/marca | Esforço de implementação |
|---|---|---|---|
| Assinatura premium (freemium) | Médio, cresce com a base de usuários fiéis | Nenhum — não envolve apostas | Médio (pagamento, controle de acesso) |
| AdSense contextual (sem certificação de gambling) | Baixo, cresce só com muito tráfego | Baixo, se o conteúdo do site não for tratado como gambling pelo Google | Baixo |
| Afiliação CPA/RevShare com casas licenciadas | Alto, com tráfego qualificado | Alto se não houver curadoria; controlável com casas licenciadas + disclosure claro | Médio (curadoria manual, contratos) |
| Redes de anúncios de iGaming (Adsterra, HilltopAds etc.) | Alto CPM, mas formatos agressivos (pop, push) prejudicam UX | Alto — anúncio programático não garante que a casa é licenciada | Baixo |

### 1. Assinatura premium — recomendação principal

Você já tem os dois módulos que sustentam um tier pago: Gestão de Banca (evolutiva) e a nova página de Cálculo de Projeções. Um modelo comum e testado é:

Grátis: Dashboard básico, Comparador, Simulador de Filtros com histórico limitado (ex.: últimos 90 dias).
Premium (assinatura mensal): histórico completo, Gestão de Banca evolutiva, Cálculo de Projeções, exportação de dados, alertas personalizados.

Isso não exige nenhuma parceria externa nem análise de licenciamento — só checkout (Stripe tem suporte a BRL e Pix via parceiros locais) e uma trava de acesso no backend, que já tem autenticação pronta (ver `AuthService`/`auth.interceptor.ts`).

### 2. AdSense contextual

Se o conteúdo do CornerLab for enquadrado pelo Google como "estatística esportiva" (não como "apostas"), anúncios contextuais comuns (não a vertical de gambling) podem rodar sem precisar da certificação especial de gambling — mas a receita de AdSense é proporcional a page views, e um site de nicho como esse provavelmente vai gerar um CPM baixo até ter volume relevante de tráfego. Vale como complemento, não como base.

### 3. Afiliação com casas de apostas licenciadas

Maior potencial de receita (CPA de R$ 25 a R$ 250 por depósito qualificado, RevShare de até 30-50% em alguns programas), mas exige processo manual: checar a lista de operadores autorizados pela SPA antes de fechar qualquer parceria, incluir o símbolo "+18" e disclosure claro de que é conteúdo patrocinado, e isolar essa área do dashboard analítico (ver seção acima sobre tensão de marca). Recomendo tratar isso como fase 2, depois que a base de usuários já existir e a curadoria puder ser feita com calma.

### 4. Redes de anúncios de iGaming

Tecnicamente a rota mais rápida de implementar (um script, formatos automáticos), mas é a que concentra o risco: essas redes servem operadores variados, muitos sem checagem de licença, e os formatos mais rentáveis (pop-under, push, social bar) tendem a degradar a experiência do usuário no meio de um dashboard analítico. Se for testar, comece só com Ezoic (parceiro certificado do Google, sem os formatos mais agressivos) e evite as redes puramente de push/pop.

## Pré-requisito técnico: medir tráfego

Hoje o CornerLab não tem nenhum analytics instalado (nem Google Analytics, nem qualquer tag de mensuração) — confirmei isso no código. Sem isso você não sabe quantos usuários tem, quais páginas engajam mais, nem consegue negociar com redes de anúncios ou programas de afiliados (todos pedem número de visitantes/mês). Esse é o primeiro passo prático, antes de qualquer uma das rotas acima.

## Roadmap sugerido

Fase 1 (agora): instalar Google Analytics 4 (ou Plausible/Umami, mais leve e sem depender de cookies de terceiros) para começar a medir tráfego, sessões e páginas mais usadas.

Fase 2 (2-4 semanas): desenhar e lançar o tier premium (Gestão de Banca + Projeções como diferenciais pagos), com checkout via Stripe.

Fase 3 (paralelo, se o volume de tráfego justificar): ativar AdSense contextual como receita complementar passiva.

Fase 4 (só depois que a base de usuários existir): avaliar 2-3 programas de afiliados com casas licenciadas pela SPA, com uma seção "parceiros" separada do dashboard e disclosure claro.

## O que eu preciso de você para refinar os números

Não tenho seu volume de tráfego atual (sessões/mês, usuários únicos) nem taxa de conversão esperada — sem isso, qualquer estimativa de receita em R$ seria um chute. Se você tiver esses números (mesmo que aproximados) ou quiser que eu ajude a configurar o Analytics primeiro para começar a coletar, consigo voltar com uma projeção de receita mais concreta por rota.
