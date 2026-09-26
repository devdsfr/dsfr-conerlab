# CORREÇÕES DA AUDITORIA — DSFR CornerLab

Registro das correções aplicadas sobre os achados de `AUDITORIA_COMPLETA_SISTEMA.md`.

**Regra de trabalho:** um problema por vez, na ordem P0 → P1 → P2 → P3, cada um passando
pelas fases A (investigação) → B (implementação) → C (teste) → D (validação) → E (registro).
Nenhum problema é iniciado antes do anterior estar registrado aqui.

**Documento vivo.** Cada correção acrescenta uma seção; nada é reescrito retroativamente.

---

## Índice de status

| ID | Severidade | Prioridade | Status | Data |
|----|------------|-----------|--------|------|
| AUD-001 | CRITICAL | P0 | ✅ RESOLVIDO (código + migration 013 aplicada — ver reconciliação de 13/09) | 2026-09-09 |
| AUD-012 | HIGH | P1 | ✅ RESOLVIDO (junto com AUD-001 — mesma linha de código) | 2026-09-09 |
| AUD-002 | CRITICAL | P0 | ✅ RESOLVIDO (Catálogo v1.1) | 2026-09-09 |
| AUD-003 | CRITICAL | P0 | ✅ RESOLVIDO | 2026-09-09 |
| AUD-004 | CRITICAL | P0 | ✅ RESOLVIDO (eixo removido + migration 014 aplicada — ver reconciliação de 13/09) | 2026-09-09 |
| REV-P0 | CRITICAL | P0 | ✅ RESOLVIDO — 12/12 da Definition of Done com evidência (ver passagem de 15/09) | 2026-09-15 |
| AUD-009 | HIGH | P1 | ✅ RESOLVIDO como efeito do REV-P0 — `worker_runs` = 6, `team_metrics` = 2.495 | 2026-09-15 |

---

## REV-P0 — Integridade e sincronização dos dados

**Data:** 2026-09-13 · **Origem:** Prompt Mestre de Revisão Funcional, prioridade P0.
Não é item da auditoria original; cruza com AUD-009 (workers nunca executaram).

### Diagnóstico — quatro causas independentes, não uma

O sintoma único ("dados desatualizados", `context deadline exceeded`) escondia quatro
defeitos distintos. Cada um sozinho já bastaria para parar o pipeline.

**Causa 1 — o Cron Job nunca existiu.** Verificado no painel do Render em 12/09: três
blueprints na conta (DIZIMAN, dsfr-finance, dsfr-global) e nenhum do dsfr-conerlab; nenhum
serviço do tipo cron. As 17 linhas de `sync_runs` têm `triggered_by = 'manual'`, sem
exceção. Todo dado que existe no banco foi produzido por alguém clicando num botão.
→ **Corrigido:** cron `cornerlab-worker` criado (crn-dair0e95efls73ek3080), 11:00 UTC.

**Causa 2 — o serviço web rodava com os limites do plano gratuito.**
`API_FOOTBALL_RATE_LIMIT_PER_MIN` e `SYNC_MAX_PER_CYCLE` existiam apenas no bloco do cron
do `render.yaml` — que nunca foi aplicado. O botão "Sincronizar agora" roda dentro do
serviço web e usava os padrões: 6,5 s entre chamadas e 50 partidas por ciclo.

Evidência em `api_usage_log` (média de `duration_ms` por endpoint):

| momento | fixtures.lookup | fixtures.statistics | resultado |
|---|---|---|---|
| 12/09 17h | 6.577 ms | 6.637 ms | 34 partidas, morreu em `context deadline exceeded` |
| 13/09 10h | 249 ms | 270 ms | 294 partidas, 0 erros, ciclo completo em 15min18s |

A latência real do provedor é ~250 ms. Os 6,5 s eram estrangulamento nosso, deliberado,
correto para o plano Free e errado para o plano Pro contratado.
→ **Corrigido:** as três variáveis configuradas no serviço `cornerlab-backend`.

> **Nota sobre o timeout.** Em 12/09 eu subi `syncTimeout` de 20 para 60 minutos
> configuráveis. A evidência acima mostra que **o timeout não era o fator limitante**: o
> ciclo saudável leva 15 min e caberia nos 20. O que matava o ciclo era o estrangulamento
> de 6,5 s. Mantive os 60 min como margem, mas registro que essa alteração **não foi a
> correção** — atribuí-la como tal seria transformar coincidência em causa.

**Causa 3 — ciclo que falha não deixava rastro.** `recordRun` só era chamado depois das
duas fases. O ciclo de 12/09 ~17:00 morreu no meio: `api_usage_log` tem as 17 chamadas
falhas daquele instante, `sync_runs` não tem linha nenhuma, e a tela seguiu anunciando
"última sincronização: 11/09 22:40". **Falhar deixava o sistema com aparência melhor do
que rodar mal.**
→ **Corrigido:** migration 016 (`status`, `error_message`), gravação no caminho de erro
tanto no handler quanto no worker, e exibição da falha na tela.

**Causa 4 — `DATABASE_URL` ausente vira "connection refused" em localhost.**
A primeira execução agendada do cron (13/09 11:00 UTC, gatilho `Scheduled`) morreu em 23 s:

```
failed to connect to `user=cornerlab database=cornerlab`:
127.0.0.1:5432 (localhost): connect: connection refused
❌ Your cronjob failed because of an error: Exited with status 1
```

`user=cornerlab`, `localhost:5432` é o **valor padrão** de `config.go`. A chave
`DATABASE_URL` existe no painel do cron, mas o valor não chegou ao contêiner, e `getEnv`
trocou o vazio pelo padrão de desenvolvimento sem dizer nada. O log resultante descreve um
sintoma a três passos da causa: quem o lê investiga rede e Neon, não variável de ambiente.
→ **Corrigido no código:** `config.Validate()` recusa, em `ENVIRONMENT=production`,
`DATABASE_URL` vazia ou apontando para localhost, e `JWT_SECRET` no valor de exemplo.
Falha na inicialização nomeando a variável.
→ **PENDENTE em produção:** o valor precisa ser recolado no painel. Não faço isso —
é um segredo.

### Arquivos alterados

| Arquivo | O quê |
|---|---|
| `migrations/016_sync_runs_status.sql` | novo — `status`, `error_message`, índice por origem |
| `internal/domain/sync.go` | `Status`, `ErrorMessage` e as constantes de origem/status |
| `internal/repository/postgres/sync_run_repo.go` | `LastRunBySource`, colunas centralizadas, gravação de falha |
| `internal/repository/interfaces.go` | `LastRunBySource` na interface |
| `internal/delivery/http/handlers/sync_handler.go` | registra ciclo interrompido; `/sync/status` separa origens |
| `cmd/worker/main.go` | erro das fases volta e vira `status='failed'`; `cfg.Validate()` |
| `cmd/api/main.go` | `cfg.Validate()` |
| `pkg/config/config.go` | `Validate()` e `SyncTimeoutMinutes` |
| `render.yaml` | nomes/planos reais; variáveis no serviço web; aviso de risco ao aplicar |
| `frontend/.../models.ts`, `integrations.component.{ts,html}` | origens separadas, falha visível |

### Testes adicionados

`handlers/sync_status_test.go` (6 casos) e `config/config_validate_test.go` (6 casos).
Cada um reproduz uma situação observada em produção, não uma hipótese.

**Estado da execução: NÃO EXECUTADOS.** O ambiente Linux desta sessão está inacessível
desde a atualização do Windows de 08/09 (7 falhas consecutivas de montagem). Os testes
foram escritos mas nunca compilados. Isto é uma pendência de verificação, não um
resultado — e a Fase E não pode ser considerada cumprida até alguém rodar
`go build ./... && go vet ./... && go test ./...`.

### Achados que NÃO foram corrigidos (registrados, não silenciados)

**Fila de atualização pode inanir os jogos mais antigos.** `ListDueForUpdate` usa
`ORDER BY match_date DESC LIMIT n`. Hoje há 325 partidas vencidas e o teto é 300: as 25
mais antigas não são alcançadas enquanto existirem 300 mais recentes. Drena sozinho
conforme a fila diminui, mas volta a acontecer sempre que a fila passar do teto. As mais
antigas são de 2026-03-15 (MLS). Não alterei — seria um segundo problema no mesmo ciclo.

**Partidas que nunca finalizam são re-consultadas para sempre.** Uma partida cujo
resultado o provedor nunca publica permanece `AGENDADO` e volta à fila a cada ciclo,
gastando até 2 requisições por execução, indefinidamente. Não existe marcação de
"tentada e inviável".

**Proveniência de 2.211 partidas não é auditável.** Das 3.157 finalizadas, 2.211 têm
escanteios preenchidos e `stats_synced_at` nulo — todas criadas em 2026-07-14, todas com
`odds_source = 'synthetic'`. Não há registro de quando ou como aquelas estatísticas
entraram.
Comparação das distribuições:

| grupo | n | média escanteios | desvio | média gols |
|---|---|---|---|---|
| legado (sem `stats_synced_at`) | 2.211 | 9,24 | 3,70 | 2,66 |
| verificado pelo worker | 946 | 9,98 | 3,63 | 2,76 |

São compatíveis com futebol real e **não há indício de fabricação**. Mas compatível não é
comprovado: **NA — NÃO FOI POSSÍVEL AUDITAR A PROVENIÊNCIA** desses registros a partir do
banco. Como são 70% da base histórica, isto merece item próprio.

**Paginação não é lida.** `fixturesResponse` não desserializa o campo `paging` da
API-Football. Não há evidência de truncamento (3.712 partidas para 12 ligas, ~310 por
liga, bem acima de um limite de 100 por página), mas a ausência de verificação significa
que um truncamento futuro passaria despercebido.

**`worker_runs` = 0 e `team_metrics` = 0** (AUD-009) continuam zerados — consequência
direta da Causa 1. Só serão populados quando o cron executar com sucesso.

**`ERR_BLOCKED_BY_CLIENT`**: classificado como **AMBIENTE DO CLIENTE**. O recurso é
`pagead2.googlesyndication.com`, bloqueado por extensão do navegador. Nenhuma causa no
CornerLab.

---

## REV-P0 — segunda passagem de fechamento (13/09/2026, tarde)

**Objetivo:** fechar formalmente o P0. **Resultado: REV-P0 permanece PARCIAL.**
Nenhum dos 12 itens da Definition of Done pôde ser marcado. Abaixo, o que foi apurado.

### Correção de um diagnóstico da passagem anterior

Na primeira passagem escrevi que `DATABASE_URL` "chegou vazia" ao contêiner e sugeri que o
valor estivesse em branco no painel. **A evidência nova mostra um mecanismo diferente**, e
registro isso em vez de reescrever o texto anterior:

1. A chave `DATABASE_URL` existe no cron e o campo de valor aparece preenchido (mascarado).
2. O botão de salvar variáveis do Cron Job é literalmente
   **"Save, rebuild, and apply on next run"** — em Cron Job, variável de ambiente só passa
   a valer depois de um **rebuild**.
3. A aba Builds do cron tinha **uma única build**: `First Build`, 12/09 17:05 — ou seja,
   **anterior** à adição das secrets.

Logo, a execução de 13/09 11:00 UTC rodou sobre uma imagem construída antes das variáveis
existirem, e o `getEnv` devolveu o padrão de desenvolvimento. Não era valor vazio: era
valor nunca aplicado. A diferença importa porque a ação corretiva é outra — recolar o
segredo não resolveria nada.

**Ação executada:** disparei um **Manual Build** (cache limpo), concluído em 55,3 s. A
imagem agora carrega o ambiente atual. Isto **não valida** a conexão: valida apenas que a
condição que a impedia deixou de existir.

**Não expus, copiei nem registrei o valor de `DATABASE_URL` em nenhum momento.**

### Migration 016 — NÃO APLICADA

Consulta ao schema de produção:

```
coluna: triggered_by (character varying)      ← única das três que existe
migrations aplicadas: 010, 011, 012, 013, 014, 015
```

`status` e `error_message` **não existem** em `sync_runs`. A 016 está escrita e embutida
via `go:embed`, e será aplicada pelo runner de migrations na primeira subida do código
novo — mas o código novo não foi publicado. Enquanto isso, o registro de ciclos que falham
continua inexistente em produção.

### Fase E — BLOQUEADA, não concluída

`go build ./...`, `go vet ./...`, `go test ./...` e o build do frontend **não foram
executados**. O ambiente Linux desta sessão falhou pela 8ª vez consecutiva com erro de
montagem (regressão de uma atualização do Windows de 08/09). Não é um resultado "passou com
ressalvas": é ausência de verificação.

### Execução automática real — a única que houve FALHOU

| campo | valor |
|---|---|
| gatilho | `Scheduled` (não manual) |
| início | 13/09/2026 11:00:05 UTC |
| término | 13/09/2026 11:00:26 UTC |
| duração | 23,0 s |
| ligas processadas | 0 |
| partidas encontradas / atualizadas / finalizadas | 0 / 0 / 0 |
| erros | processo encerrado com status 1 |
| linha em `sync_runs` | **nenhuma** |

O painel do cron exibe **"No successful runs yet."**

Tentei disparar uma execução manual apenas como **diagnóstico da conexão** — não como prova
do scheduler, que o escopo desta passagem proíbe. O botão "Trigger Run" não iniciou
execução nas três tentativas. Registro como limitação da minha sessão, não como defeito do
sistema.

### Itens 5 e 6 — não validáveis nesta passagem

A tela separando automático de manual e o alerta de desatualização estão **escritos mas não
publicados**. Validar comportamento real exige o deploy. Verificar apenas o código-fonte
seria trocar "validado" por "parece certo", que é o tipo de conclusão que esta revisão
existe para evitar.

### Definition of Done — estado real

| # | Item | Estado |
|---|---|---|
| 1 | `DATABASE_URL` válida no cron | ⏳ rebuild feito; não comprovado por execução |
| 2 | cron conecta ao banco | ❌ não comprovado |
| 3 | migration 016 aplicada | ❌ schema confirma que não |
| 4 | build passa | ❌ não executado |
| 5 | vet passa | ❌ não executado |
| 6 | testes passam | ❌ não executados |
| 7 | frontend passa | ❌ não executado |
| 8 | execução automática real ocorreu | ⚠️ ocorreu e **falhou** |
| 9 | `sync_runs` registrou a automática | ❌ nenhuma linha `cron` |
| 10 | falhas ficam registradas | ❌ código pronto, não publicado |
| 11 | tela diferencia automático/manual | ❌ código pronto, não publicado |
| 12 | alerta de desatualização validado | ❌ não validado |

**REV-P0 = PARCIAL.**

### Caminho crítico para fechar (nesta ordem)

1. `cd backend && go build ./... && go vet ./... && go test ./...` e
   `cd frontend && npx ng build --configuration production`.
2. Commit e push. O deploy do `cornerlab-backend` aplica a migration 016 na subida.
3. Confirmar no banco que `sync_runs` ganhou `status` e `error_message`.
4. Aguardar a execução agendada de **14/09 11:00 UTC** e conferir a linha
   `triggered_by = 'cron'`, `status = 'success'`.
5. Só então reavaliar os 12 itens.

Nada disso pode ser feito daqui: o passo 1 depende de um ambiente que compile, o passo 4
depende do relógio.

### Escopo preservado

Não foram tocados nesta passagem, conforme instrução: inanição da fila, partidas
eternamente `AGENDADO`, proveniência das 2.211 legadas, paginação, AUD-007, fonte de odds,
Dashboard. Nenhum deles bloqueou a validação do worker automático — o bloqueio é
ambiente de build e relógio.

---

## REV-P0 — terceira passagem (13/09/2026)

**REV-P0 continua PARCIAL.** Esta passagem não avançou nenhum item da Definition of Done,
e o motivo é um só.

### 1. Comandos executados

| Comando | Resultado |
|---|---|
| `go build ./...` | **NÃO EXECUTADO** — ambiente indisponível |
| `go vet ./...` | **NÃO EXECUTADO** — idem |
| `go test ./...` | **NÃO EXECUTADO** — idem |
| `npx ng build --configuration production` | **NÃO EXECUTADO** — idem |

Tentativa registrada às 12h (9ª falha consecutiva nesta sessão):

```
failed to mount ... /outputs is under Plan9 share "c" which is not mounted
```

O ambiente Linux desta sessão perdeu o acesso ao disco do usuário após uma atualização do
Windows de 08/09/2026. Não é lentidão nem timeout: o sistema de arquivos do projeto não é
montável. Sem compilador não há build, sem build não há push, sem push não há deploy, e
sem deploy os itens 3 a 12 permanecem inalcançáveis. **O caminho crítico inteiro está
atrás de um único bloqueio.**

### 2. Commit / deploy / migration

Nada. Nenhum commit, nenhum push, nenhum deploy. `016_sync_runs_status.sql` **continua não
aplicada** — reconfirmado no schema de produção: `sync_runs` tem `triggered_by` e não tem
`status` nem `error_message`; `schema_migrations` para em 015.

### 3. Cron

Nenhuma execução nova. A última continua sendo a de 13/09 11:00:05 UTC, `Scheduled`, 23,0 s,
encerrada com status 1 e sem linha em `sync_runs`. A próxima execução agendada é
**14/09 11:00 UTC** — depende do relógio, não de mim, e conforme a regra desta revisão
disparo manual não serve como prova.

O rebuild manual feito na segunda passagem (55,3 s, cache limpo) segue de pé: a imagem do
cron já carrega o ambiente atual. Isso remove a causa conhecida da falha das 11:00, mas
**não é evidência de sucesso** até uma execução agendada rodar.

### 4. UI e alerta

Não validáveis sem deploy. Sem alteração desde a segunda passagem.

### Revisão manual, no lugar da compilação

Como não pude compilar, revisei à mão os arquivos alterados pelo REV-P0, procurando
especificamente erros de compilação. **Isto não substitui `go build` e não está sendo
apresentado como se substituísse** — é apenas o que foi possível fazer.

Verificado: imports (`strings` e `fmt` em uso onde foram adicionados, `time` ainda em uso em
`sync_run_repo.go`); a interface anônima `interface{ Scan(...any) error }` é satisfeita por
`pgx.Row`; o `discoveryResult, err := ...` em `sync_handler.go` redeclara apenas `err`, o
que é válido por estarem no mesmo bloco; `repoFake` implementa os cinco métodos de
`SyncRunRepository`; os dois arquivos de teste novos são os únicos em seus pacotes, sem
colisão de nomes (`repoFake`, `chamarStatus`, `produção`, `contémTudo`); a chamada
`recordRun` do modo loop do worker descarta os dois retornos como statement, o que é legal.

Nenhum erro encontrado. **Nenhuma garantia oferecida.**

### Risco de ordenação a observar no deploy

`colunasSyncRun` passa a selecionar `status` e `error_message`. Se o binário novo servir
tráfego antes da migration 016 rodar, **toda leitura de `sync_runs` quebra** — é a mesma
classe de falha que derrubou produção três vezes com as migrations 013 e 015. O runner de
migrations roda na subida, antes de servir, justamente por isso. Vale conferir o log do
deploy: a linha da 016 deve aparecer antes do primeiro request.

### Definition of Done — estado real

| # | Item | Estado |
|---|---|---|
| 1 | `DATABASE_URL` válida no cron | ⏳ rebuild aplicado; sem execução que comprove |
| 2 | cron conecta ao banco | ❌ |
| 3 | migration 016 aplicada | ❌ |
| 4 | build backend passa | ❌ não executado |
| 5 | vet passa | ❌ não executado |
| 6 | testes backend passam | ❌ não executados |
| 7 | frontend build passa | ❌ não executado |
| 8 | execução automática real teve sucesso | ❌ a única falhou |
| 9 | `sync_runs` registrou a automática | ❌ |
| 10 | falhas são persistidas | ❌ não publicado |
| 11 | UI diferencia automático/manual | ❌ não publicado |
| 12 | alerta de desatualização validado | ❌ |

### Status final

**REV-P0 = PARCIAL.**

Três passagens chegaram ao mesmo ponto de bloqueio. Não há trabalho de análise restante
neste item: o diagnóstico está fechado e a correção está escrita. O que falta é execução
num ambiente que compile.

---

## REV-P0 — quarta passagem (13/09/2026, 18h UTC) — DESBLOQUEADO

O bloqueio das três passagens anteriores foi resolvido **pelo Daniel**, fora desta sessão:
build, push e deploy aconteceram. Esta passagem **verifica em produção** o que antes só
existia como código.

### 1. Validação de código

Meu ambiente Linux continua inacessível (10ª falha de montagem). Os quatro comandos
**não foram executados por mim**. O que a evidência de produção permite afirmar:

| Item | Evidência | Conclusão |
|---|---|---|
| `go build ./...` | deploy `dbb922a` concluído em 1m08s; o Dockerfile compila os 4 binários | ✅ compila |
| `npx ng build --configuration production` | frontend novo servindo e renderizando o template novo | ✅ compila |
| `go vet ./...` | nenhuma — o Dockerfile não roda vet | ⚠️ **não verificado** |
| `go test ./...` | nenhuma — o Dockerfile não roda testes | ⚠️ **não verificado** |

Um build de Docker bem-sucedido prova compilação; **não prova vet nem testes**. Registro
como não verificado em vez de inferir.

### 2. Deploy e migration — CONFIRMADOS

- `cornerlab-backend`: deploy `dbb922a`, Auto-Deploy, 1m08s.
- `cornerlab-worker`: build `dbb922a`, Auto-Deploy, 54,9s — o cron também está no código novo.

Schema de produção consultado às 17:58 UTC:

```
ultima_migration = 016_sync_runs_status.sql
```

| coluna | tipo | nulo | default |
|---|---|---|---|
| `triggered_by` | character varying | NÃO | — |
| `status` | text | NÃO | `'success'::text` |
| `error_message` | text | SIM | — |

**Migration 016 aplicada.** Nenhum incidente de ordenação: o runner rodou antes de servir,
como projetado.

### 3. Cron — ainda sem execução agendada

`SELECT count(*) FROM sync_runs WHERE triggered_by='cron'` → **0**.

A única execução agendada até aqui foi a de 13/09 11:00 UTC, que falhou (documentada na
segunda passagem) e é anterior tanto ao rebuild quanto ao deploy do código novo. A próxima
é **14/09 11:00 UTC**. Não há como antecipá-la sem usar disparo manual, que esta revisão
não aceita como prova.

### 4. UI em produção — VALIDADA

`https://dsfrcornerlab.com.br/integracoes`, texto real da página:

```
Última tentativa: 13/09 07:02 (manual, durou 15min 18s)
Último ciclo que trouxe dado: 13/09 07:02 (manual, durou 15min 18s)
Automático (agendado, 08:00): nunca executou
Manual (este botão): 13/09 07:02 · 15min 18s — 3712 jogos novos, 299 finalizados
A sincronização automática nunca rodou. [...]
```

`GET /api/v1/sync/status` em produção:

```json
{ "last_cron_run": null, "cron_never_ran": true, "cron_stale": true,
  "hours_since_cron": null, "stale": false, "hours_since_success": 7,
  "last_manual_run": { "status": "success", "fixtures_upserted": 3712,
                       "matches_finalized": 299 } }
```

Confere item a item: automática separada da manual; última tentativa automática (ausente,
e dito explicitamente); última automática bem-sucedida (ausente); status; duração; execução
manual em linha própria. O campo de erro só aparece quando há erro — e não há, porque
nenhum ciclo falhou **depois** do deploy.

**Alerta de desatualização — os dois lados validados:**
- `stale: false` com `hours_since_success: 7` → **nenhum alerta global**, correto: houve
  ciclo bem-sucedido há 7h e os dados estão em dia.
- `cron_stale: true` + `cron_never_ran: true` → **aviso persistente** na tela sobre a
  automação, correto e independente do primeiro.

É exatamente a distinção que a correção existia para criar: dado em dia e automação morta
podem ser verdade ao mesmo tempo, e agora a tela diz as duas coisas.

### Ajuste cosmético feito nesta passagem

O texto saía grudado — `"08:00):nunca executou"` — porque o Angular colapsa a quebra de
linha entre o `<span>` e o bloco `@if`. Corrigido com `&nbsp;` explícito nas duas linhas.
É marcação minha, do próprio REV-P0. **Ainda não publicado** — entra no próximo deploy e
não bloqueia nada.

### Definition of Done — estado real

| # | Item | Estado |
|---|---|---|
| 1 | `DATABASE_URL` válida no cron | ⏳ imagem reconstruída com o ambiente; sem execução que comprove |
| 2 | cron conecta ao banco | ⏳ aguarda 14/09 11:00 UTC |
| 3 | migration 016 aplicada | ✅ |
| 4 | build backend passa | ✅ (via Docker build do deploy) |
| 5 | vet passa | ⚠️ não verificado |
| 6 | testes backend passam | ⚠️ não verificado |
| 7 | frontend build passa | ✅ (servindo em produção) |
| 8 | execução automática real teve sucesso | ❌ aguarda 14/09 11:00 UTC |
| 9 | `sync_runs` registrou a automática | ❌ zero linhas `cron` |
| 10 | falhas são persistidas | ⏳ coluna e código no ar; nenhuma falha ocorreu desde o deploy |
| 11 | UI diferencia automático/manual | ✅ validado em produção |
| 12 | alerta de desatualização validado | ✅ validado nos dois estados |

**4 confirmados, 2 não verificados, 4 aguardando o relógio, 2 sem evidência.**

### Status final

**REV-P0 = PARCIAL.**

Saiu de "bloqueado em tudo" para "pendente de duas coisas": a saída de `go vet` e
`go test`, que só o Daniel tem como produzir, e a execução agendada de 14/09 11:00 UTC.
Nenhuma das duas depende de mais trabalho de análise.

---

## REV-P0 — quinta passagem (13/09/2026, 18h15 UTC) — reconciliação 013/014

Duas tarefas: reconciliar o estado das migrations 013 e 014, e fechar o que der do REV-P0.
**Nenhum dado foi modificado nesta passagem** — só medição.

### Reconciliação — a inconsistência era do índice, não do banco

O índice deste documento dizia "MIGRATION PENDENTE" para AUD-001 (013) e AUD-004 (014).
**Estava desatualizado.** As duas estão aplicadas em produção. O índice foi corrigido; os
registros históricos que diziam "pendente" permanecem intactos, como manda o formato
append-only.

`schema_migrations`:

| version | checksum | applied_at |
|---|---|---|
| `013_odds_source.sql` | `adotada-sem-verificacao` | 2026-09-12 11:31:36 UTC |
| `014_tier_nao_classificado.sql` | `adotada-sem-verificacao` | 2026-09-12 11:31:36 UTC |
| `015_result_odds.sql` | `adotada-sem-verificacao` | 2026-09-12 11:31:36 UTC |
| `016_sync_runs_status.sql` | `936515e6…0583` | 2026-09-13 12:26:46 UTC |

**Como ler esses carimbos, para não concluir errado.** `adotada-sem-verificacao` é o marcador
que o runner usa para linhas *adotadas* na criação do ledger — 013, 014 e 015 já tinham sido
executadas à mão antes de `schema_migrations` existir, e foram inscritas em bloco quando a
tabela foi criada. **Logo, `applied_at = 12/09 11:31:36` é a data em que o ledger foi
semeado, não a data em que o SQL rodou.** A 016 é o contraste: checksum real e `applied_at`
correspondente à execução pelo runner durante o deploy.

Quando as 013/014 realmente rodaram: o histórico do SQL Editor do Neon tem as entradas
`add odds source column and update matches table` e `add tier_legacy column and migrate tier
data`, ambas em **09/09/2026, 14:33 e 14:35 (GMT-3)** — compatíveis com aplicação manual pelo
editor. A 015 aparece em 12/09 08:25 e a criação do ledger em 12/09 08:31 (GMT-3 = 11:31
UTC, batendo exatamente com `applied_at`).

Resumindo com honestidade: **a migration originalmente aguardava aprovação e foi aplicada
manualmente via SQL Editor do Neon; a evidência disponível aponta 09/09/2026 para 013 e 014.
O banco sozinho não permite determinar o instante da execução** — apenas o da adoção no
ledger.

### Efeitos reais da migration 013

| medida | valor |
|---|---|
| `matches.odds_source = 'synthetic'` | **2.558** |
| `matches.odds_source = 'unknown'` | **3.003** |
| `matches.odds_source = 'real'` | **0** (a categoria não aparece) |
| soma | 5.561 = total de `matches`, sem nulos |

Zero odds reais em toda a base. Não é falha da migration: é a consequência, já documentada,
de a API-Football manter apenas 7 dias de histórico de odds. O rótulo está correto — nenhuma
odd sintética está se passando por real.

Estratégias:

| origem | ativa | quantidade |
|---|---|---|
| `discovery` | `false` | **5** |

É o único grupo existente — não há estratégia de outra origem nem nenhuma ativa. **As 5
estratégias antigas de Discovery estão despublicadas e continuam no banco.** É exatamente o
exigido pelo AUD-001: invalidar sem apagar. Nenhuma foi removida.

### Efeitos reais da migration 014

| medida | valor |
|---|---|
| `teams` total | 410 |
| `teams.tier` preenchido | **0** |
| `teams.tier_legacy` preenchido | **338** |

A coluna `tier_legacy` existe. O tier corrompido foi zerado em todas as 410 equipes, e o
valor legado foi preservado em 338 — **o dado não foi destruído, foi movido**. As 72 sem
`tier_legacy` são equipes que nunca tiveram tier atribuído.

### `go vet` e `go test` — continuam NÃO VERIFICADOS

Tentei novamente; 11ª falha consecutiva de montagem do ambiente Linux. Nada mudou. Não uso
o build do Docker como substituto: ele compila, não roda vet nem testes.

**Isso impede formalmente fechar o P0?** Sim, pelo critério escrito — os itens 5 e 6 da
Definition of Done exigem evidência e não há nenhuma. Mas é uma pendência de *verificação*,
não um defeito conhecido: não existe sintoma, falha ou suspeita associada. Basta a saída dos
dois comandos rodados em qualquer máquina com Go.

### Cron — ainda não ocorreu

Hora da consulta: **13/09/2026 18:15 UTC**. A execução agendada é 14/09 11:00 UTC, daqui a
~17 horas. `SELECT count(*) FROM sync_runs WHERE triggered_by='cron'` → **0**. Não antecipei
com disparo manual. `worker_runs` e `team_metrics` seguem em **0**, e só podem mudar depois
de um ciclo do worker.

### Persistência de falhas

Coluna e código publicados; nenhuma falha ocorreu depois do deploy. Não vou provocar falha
em produção para gerar evidência. Classificação: **implementado, aguardando evidência
natural**.

### Definition of Done

| # | Item | Estado |
|---|---|---|
| 1 | `DATABASE_URL` funcional no cron | ⏳ aguarda execução agendada |
| 2 | cron conecta ao banco | ⏳ aguarda execução agendada |
| 3 | migration 016 aplicada | ✅ |
| 4 | backend compila | ✅ |
| 5 | `go vet` passa | ⚠️ **NÃO VERIFICADO** |
| 6 | `go test` passa | ⚠️ **NÃO VERIFICADO** |
| 7 | frontend compila | ✅ |
| 8 | execução automática real tem sucesso | ⏳ 14/09 11:00 UTC |
| 9 | `sync_runs` registra `triggered_by='cron'` | ⏳ idem |
| 10 | mecanismo de falha implementado | ✅ implementado, aguardando evidência natural |
| 11 | UI diferencia automático/manual | ✅ |
| 12 | alerta de desatualização funciona | ✅ |

**REV-P0 = PARCIAL.** 5 verdes, 2 não verificados, 4 aguardando o relógio, 1 implementado
sem evidência natural.

Fora do REV-P0, esta passagem fechou uma dívida separada: **AUD-001 e AUD-004 deixam de ter
migration pendente** — as duas estão aplicadas e seus efeitos foram medidos e conferem com o
que a auditoria exigia.

---

## REV-P0 — sexta passagem (14/09/2026) — regressão introduzida por mim

### O que aconteceu

A execução agendada de 14/09 11:00 UTC falhou:

```
{"time":"2026-09-14T11:00:49Z","level":"ERROR","msg":"worker não vai rodar",
 "error":"configuração inválida para ENVIRONMENT=production:
          JWT_SECRET (vazio ou ainda no valor de exemplo)"}
❌ Your cronjob failed because of an error: Exited with status 1
```

**A causa é o `config.Validate()` que eu escrevi na quarta passagem.** Ele exigia
`JWT_SECRET` de todo binário. O worker não emite nem valida token — JWT é assunto exclusivo
da API — e o Cron Job, corretamente, não tem essa variável. A verificação criada para
impedir má configuração virou ela própria a indisponibilidade.

Não é um efeito colateral distante: é o defeito óbvio de validar a união de tudo que o
repositório usa em vez do que aquele processo usa.

### O lado bom, e ele é grande

A mensagem cita **apenas** `JWT_SECRET`. Como `Validate` acumula todas as pendências numa
mensagem só, o silêncio sobre `DATABASE_URL` é informação: **a credencial rotacionada está
presente, não vazia e não apontando para localhost.**

Isso fecha, por evidência indireta, a pergunta que estava aberta desde 12/09. O que ainda
falta provar é a conexão efetiva ao Postgres — o processo morreu antes de tentar.

### Correção

`config.go`: a validação foi separada por binário.

| função | valida | quem chama |
|---|---|---|
| `Validate()` | `DATABASE_URL` | `cmd/worker` (e qualquer binário) |
| `ValidateAPI()` | `Validate()` + `JWT_SECRET` | `cmd/api` |

`cmd/api/main.go` passou a chamar `ValidateAPI()`. `cmd/worker/main.go` segue em
`Validate()`, que agora não exige nada que o worker não use.

### Sobre os testes alterados

`TestProducaoRecusaJWTSecretDeExemplo` passou a chamar `ValidateAPI`, e
`TestValidateRelataTodosOsProblemasDeUmaVez` virou `TestValidateAPI...`.

**Isto não é alterar teste para fazê-lo passar.** Os testes codificavam uma regra errada —
"todo binário precisa de JWT_SECRET" —, e a regra é que mudou. O sintoma foi produzido em
produção antes de qualquer teste ser tocado.

Dois testes novos travam a regra correta nas duas direções:

- `TestWorkerSemJWTSecretEhValido` — reproduz o incidente: falharia com o código anterior.
- `TestAPISemJWTSecretEhInvalida` — garante que a exigência não foi simplesmente removida;
  a API continua recusando subir com o segredo de exemplo.

### Estado

Nenhum item da Definition of Done mudou de cor, com uma ressalva: o item 1
(`DATABASE_URL` funcional no cron) agora tem **evidência indireta forte** — passou na
validação. Continua ⏳ porque conexão efetiva não foi demonstrada.

O item 8 (execução automática com sucesso) permanece ❌: a de hoje falhou, agora por causa
desta regressão. A próxima oportunidade é **15/09 11:00 UTC**, e ela depende de o código
corrigido estar publicado até lá.

**REV-P0 = PARCIAL.**

---

## REV-P0 — sétima passagem (15/09/2026) — RESOLVIDO

Os dois bloqueios que restavam caíram no mesmo dia: o ambiente de build voltou e a
execução agendada rodou.

### 1. Validação de código — EXECUTADA

O ambiente Linux desta sessão voltou a montar o disco. Go não vinha instalado; instalei a
toolchain 1.25.1 no diretório do usuário (sem privilégio de root) e rodei de verdade:

```
$ go build ./...      → exit 0
$ go vet ./...        → exit 0
$ go test ./...       → exit 0   (9 pacotes com teste, 0 falhas)
```

Os 14 testes do REV-P0, verbosos:

```
ok  pkg/config                                  0.007s
    PASS TestProducaoRecusaDatabaseURLVazia
    PASS TestProducaoRecusaBancoEmLocalhost
    PASS TestProducaoRecusaJWTSecretDeExemplo
    PASS TestWorkerSemJWTSecretEhValido          ← regressão de 14/09
    PASS TestAPISemJWTSecretEhInvalida
    PASS TestDesenvolvimentoAceitaPadroesLocais
    PASS TestProducaoBemConfiguradaPassa
    PASS TestValidateAPIRelataTodosOsProblemasDeUmaVez
ok  internal/delivery/http/handlers              0.013s
    PASS TestStatusRevelaQueOCicloAutomaticoNuncaRodou
    PASS TestStatusNaoAlarmaComCicloAutomaticoRecente
    PASS TestStatusMarcaCicloAutomaticoAtrasado
    PASS TestCicloInterrompidoEhRegistradoComoFalha
    PASS TestCicloQueFalhouNaoContaComoBemSucedido
    PASS TestFalhaAoGravarHistoricoNaoPropaga
```

`gofmt -l` acusou `cmd/api/main.go` e `cmd/worker/main.go` — ordenação de import herdada de
quando `migrate` foi adicionado. Corrigido com `gofmt -w`; build, vet e test revalidados
depois, todos em exit 0. **Não commitado** (junto com este documento, são as únicas
alterações locais pendentes; nada substantivo).

**Frontend:** `npx ng build --configuration production` → **exit 0**. Só avisos NG8107
pré-existentes, em `dashboard.component.html` e num trecho de `integrations.component.html`
anterior ao REV-P0.

> Observação de método: o `node_modules` da pasta do usuário contém binários Windows
> (esbuild), que o Linux não executa. **Não rodei `npm ci` ali** — isso substituiria os
> binários dele e quebraria o ambiente local. Copiei o frontend para `/tmp` sem
> `node_modules`, instalei lá e compilei. A pasta do usuário ficou intocada.

### 2. Execução automática real — SUCESSO

Render, aba Runs: gatilho **`Scheduled`**, duração **17m33s**, 15/09 11:00 UTC.
Correspondência em `sync_runs`:

| campo | valor |
|---|---|
| id | 21 |
| `triggered_by` | **`cron`** |
| `status` | **`success`** |
| início (UTC) | 15/09 11:00 |
| fim (UTC) | 15/09 11:13 |
| duração | 775 s (12min 55s) |
| ligas processadas (`targets`) | 12 |
| fixtures encontradas | 3.712 |
| fixtures gravadas | 3.712 |
| partidas verificadas | 15 |
| partidas finalizadas | 9 |
| erros | **0** |

A diferença entre os 17m33s do Render e os 775 s da linha é esperada: `sync_runs` cronometra
descoberta + atualização; o processo ainda roda analytics, strategy engine e discovery
engine depois disso.

**`DATABASE_URL` validada indiretamente, sem exposição:** o processo subiu, passou pelo
`Validate()`, conectou ao Postgres, aplicou migrations e gravou a linha 21. Nenhum dos
quatro é possível com credencial ausente ou inválida. O valor nunca foi exibido nem
registrado.

**Por que o cron voltou a funcionar:** a correção do `ValidateAPI` foi publicada no commit
`fcf8c7d` (auto-deploy, build de 58,0 s). O Cron Job **não** tem `JWT_SECRET` configurada —
conferido — o que confirma que a solução foi a separação por binário, e não o paliativo de
espalhar o segredo.

### 3. Antes / depois

| medida | antes (13/09) | depois (15/09) |
|---|---|---|
| `worker_runs` | **0** | **6** |
| `team_metrics` | **0** | **2.495** |
| `sync_runs` | 17 | 21 |
| `sync_runs` com `triggered_by='cron'` | 0 | 2 |
| partidas `AGENDADO` vencidas | 325 | **6** |
| partidas `FINALIZADO` | 3.157 | 3.508 |

**AUD-009 fecha junto.** As camadas RAW/ANALYTICS estavam vazias porque nenhum worker jamais
tinha executado; agora executaram. `team_metrics` saiu de zero para 2.495 registros
pré-calculados. A fila de partidas vencidas caiu de 325 para 6 — a inanição descrita na
primeira passagem drenou sozinha, como previsto.

### 4. Interface em produção

```
Última tentativa: 15/09 08:13 (automática, durou 12min 55s)
Último ciclo que trouxe dado: 15/09 08:13 (automática, durou 12min 55s)
Automático (agendado, 08:00): 15/09 08:13 · 12min 55s — 3712 jogos novos, 9 finalizados
Manual (este botão): 14/09 09:37 · 12min 20s — 3712 jogos novos, 17 finalizados
```

Origens separadas, nenhum alerta indevido (o ciclo automático está fresco), e a palavra
"automática" aparecendo pela primeira vez desde que o projeto existe.

### Ressalva de leitura, registrada como backlog

`triggered_by='cron'` significa **"gravado por `cmd/worker`"**, não "disparado pelo
scheduler". A linha 19 (14/09) tem `triggered_by='cron'` mas veio de um *Trigger Run*
manual no Render. Sozinha, a coluna não distingue as duas coisas.

A prova do item 8 não dependeu disso: veio do cruzamento entre o gatilho `Scheduled`
registrado pelo Render e o horário da linha 21. Mas o rótulo é ambíguo e merece um campo
próprio um dia. **Não corrigido agora** — fora do escopo desta passagem.

### Definition of Done — FINAL

| # | Item | Estado | Evidência |
|---|---|---|---|
| 1 | `DATABASE_URL` funcional no cron | ✅ | processo conectou e gravou a linha 21 |
| 2 | cron conecta ao banco | ✅ | idem |
| 3 | migration 016 aplicada | ✅ | `schema_migrations`, checksum real, 13/09 12:26 UTC |
| 4 | backend compila | ✅ | `go build ./...` exit 0 |
| 5 | `go vet` passa | ✅ | `go vet ./...` exit 0 |
| 6 | `go test` passa | ✅ | `go test ./...` exit 0, 9 pacotes, 0 falhas |
| 7 | frontend compila | ✅ | `ng build --configuration production` exit 0 |
| 8 | execução automática real tem sucesso | ✅ | Render `Scheduled` 11:00 + linha 21 |
| 9 | `sync_runs` registra `triggered_by='cron'` | ✅ | id 19 e id 21 |
| 10 | mecanismo de falha implementado | ✅ | migration 016 + gravação nos dois caminhos, publicado |
| 11 | UI diferencia automático/manual | ✅ | texto de produção acima |
| 12 | alerta de desatualização funciona | ✅ | validado nos dois estados em 13/09 |

## REV-P0 = RESOLVIDO

Doze de doze com evidência. O item 10 está implementado e publicado; ainda não houve falha
natural que o exercite em produção — quando houver, a linha aparecerá com
`status='failed'` e a mensagem, em vez de sumir como sumiu em 12/09.

### REV-P1 começa aqui (ver seção própria ao final do documento)

### Pendências fora do REV-P0 (backlog, intocadas)

Inanição da fila sob backlog alto; partidas eternamente `AGENDADO` reconsultadas para
sempre; proveniência não auditável de 2.211 partidas legadas; paginação não lida;
ambiguidade de `triggered_by`; AUD-005, 006, 007, 008, 010, 021, 023; coleta de odds reais.

---

## Pipeline parado desde 24/08 — falha silenciosa no cliente da API-Football

**Data:** 2026-09-10 · **Status:** causa raiz identificada e corrigida no código
**Não é item da auditoria original.** Apareceu ao investigar por que a Champions League
não carregava.

### Sintoma

`sync_runs` mostrava o mesmo padrão em cinco ciclos seguidos:

| ciclo | alvos | achadas | gravadas | checadas | finalizadas | erros |
|---|---|---|---|---|---|---|
| 07/09 14:00 | 11 | 0 | 0 | 50 | 0 | 50 |
| 07/09 12:59 | 11 | 0 | 0 | 50 | 0 | 50 |
| 30/08 11:56 | 11 | 0 | 0 | 50 | 0 | 50 |
| 24/08 01:05 | 11 | 0 | 0 | 50 | 0 | 50 |
| 23/08 20:33 | 11 | 0 | 0 | 50 | 5 | 45 |
| **02/08 02:48** | 11 | **3.456** | **3.456** | 50 | 44 | **0** |

### O que a evidência mostrou — e o que ela derrubou

A hipótese óbvia era cota estourada. **Errada.** `api_usage_log` nos dias 24/08 e 30/08:

| dia | endpoint | success | status | chamadas |
|---|---|---|---|---|
| 30/08 | `fixtures.lookup` | **true** | **200** | 50 |
| 30/08 | `fixtures` | **true** | **200** | 11 |
| 24/08 | `fixtures.lookup` | **true** | **200** | 50 |
| 24/08 | `fixtures` | **true** | **200** | 11 |

A API respondeu **200 OK em todas as chamadas** e o worker mesmo assim gravou zero.
(Em 23/08 sim houve 45 respostas 429 — rate limit real, problema diferente e já
tratado com throttle.)

### Causa raiz

**Status 200 não significa sucesso nesta API.** A API-Football devolve HTTP 200 mesmo
quando recusa a requisição, colocando o motivo num campo `errors` **dentro do corpo**,
com `response` vazio:

```json
{"errors":{"plan":"Free plans do not have access to this season."},"response":[]}
```

`Client.doGet` checava **apenas o status HTTP**. Consequências, ambas batendo com os
números observados:

- `/fixtures?league=X&season=Y` → 200 + `errors` → `response` vazio →
  `SyncFixtures` devolvia **0 partidas e nenhum erro** → `achadas=0`, sem erro registrado
- `/fixtures?id=N` → 200 + `errors` → `response` vazio →
  `"partida não encontrada na API-Football"` → `checadas=50, erros=50`

**Por que ficou duas semanas sem ninguém notar:** o `api_usage_log` registrava
`success=true, status=200`, e a tela de diagnóstico mostrava tudo verde. Um pipeline que
falha em silêncio é pior do que um que quebra — ninguém investiga o que parece estar
funcionando. E a mensagem da API, que dizia exatamente qual era o problema, era
descartada antes de chegar a qualquer log.

### Correção

`internal/integration/sportsdata/apifootball/client.go`:

- `apiErrorMessage()` lê o campo `errors` do corpo. Ele é **polimórfico** — array vazio
  em caso de sucesso, objeto com o motivo em caso de recusa —, então é lido como
  `RawMessage` e interpretado. Formato inesperado é devolvido cru: preferir ruído a
  silêncio é a lição do próprio incidente.
- `doGet` passa a tratar 200-com-`errors` como erro, **carregando a mensagem original da
  API**. Na próxima execução o log dirá o motivo exato da recusa.
- Recusa por ritmo/cota que chega como 200 entra no mesmo backoff do 429
  (`isRateLimitMessage`).
- `baseURLOverride` para os testes apontarem a um servidor local.

**Testes** (`client_test.go`, 8 casos): recusa com 200 vira erro; `SyncFixtures` não
devolve mais vazio silencioso; `SyncFixtureStatistics` deixa de reportar recusa de plano
como "partida não encontrada"; resposta legítima com zero jogos **continua não sendo
erro**; status HTTP de erro continua erro; mensagem estável com múltiplos motivos.

### O que a correção NÃO faz, e o que ficou em aberto

**Ela não faz o pipeline voltar a sincronizar.** Ela faz o pipeline *dizer por que* não
sincroniza. A mensagem que a API vinha mandando foi descartada em todas as execuções, e
sem ela não é possível afirmar a causa da recusa — a suspeita mais provável é a
temporada pedida não estar coberta pelo plano (as ligas europeias estão com `MAX(year) =
2025` e as sul-americanas com `2026`), mas **isso é hipótese, não conclusão**. O próximo
ciclo depois do deploy resolve a dúvida.

**Ponto sem explicação, registrado como aberto:** nos ciclos de 07/09 o `sync_runs`
marca `checadas=50, erros=50`, mas o `api_usage_log` **não tem nenhuma linha naquela
data** (a retenção é de 90 dias, então não é limpeza). Verifiquei `provider_incidents`
para testar a hipótese de o disjuntor de saúde ter suspendido o provedor: a tabela está
**vazia**, o disjuntor nunca disparou. Não tenho explicação sustentada por dado para
esses dois ciclos e não vou inventar uma.

---

## Fonte de odds reais — o que a API-Football consegue e o que não consegue

**Data:** 2026-09-09 · **Status:** investigado, decisão pendente do usuário
**Contexto:** é a pendência que trava o AUD-001 e o AUD-003 de terem efeito prático.

### O achado que muda o planejamento

Da documentação oficial do endpoint `/odds` (Odds Pre-Match), citação direta:

> *"We provide pre-match odds between 1 and 14 days before the fixture."*
> *"We keep a 7-days history (The availability of odds may vary according to the
> leagues, seasons, fixtures and bookmakers)."*

**Não existe arquivo histórico de odds.** A consequência é dura e precisa estar clara
antes de qualquer estimativa de prazo:

1. **As 3.547 partidas já no banco nunca terão odd real.** Não é questão de plano pago
   ou de esforço de implementação — o dado não existe mais do lado do provedor.
2. **O histórico de odds reais começa do zero** no dia em que a coleta entrar no ar, e
   cresce uma rodada por vez.
3. **Até acumular 100 jogos por combinação** (mínimo do doc 08), o Discovery continua
   publicando zero. Corretamente.

Ou seja: integrar odds não é uma tarefa que "destrava" o Discovery em uma semana. É
ligar uma coleta e esperar uma temporada.

### O que ainda não sei, e por que não chutei

Três coisas não se respondem lendo documentação, e nenhuma delas eu vou afirmar sem
medir:

- **Existe mercado de escanteios na API?** Se só houver 1X2 e over/under de gols, a
  integração não resolve o AUD-001 — precificaria gols, não escanteios, que é a
  métrica central do produto. A lista de mercados é devolvida em tempo de execução
  (`/odds/bets`), não está publicada na documentação.
- **A cota do plano aguenta?** Odds vêm paginadas de 10 em 10.
- **Qual casa usar?** Odd de casa diferente muda o break-even e, portanto, o p-valor
  do teste de significância do AUD-003.

### Ferramenta criada: `cmd/oddsprobe`

Responde as três com dados, contra a API real, usando a chave já configurada.
**Só lê — não grava nada, em lugar nenhum.**

```
go run ./cmd/oddsprobe
go run ./cmd/oddsprobe -league 71 -season 2026
```

Verifica plano e cota, procura mercados de escanteios, lista as casas e busca uma
amostra real de odds, imprimindo a leitura de cada resultado.

### Ferramenta criada: `cmd/findleague`

Não é sobre odds, mas nasceu do mesmo problema — depender de um número que ninguém
confirmou. Resolve o `external_id` de um campeonato perguntando ao provedor, em vez
de procurar no painel ou chutar:

```
go run ./cmd/findleague -name "Europa League" -season 2025
```

Chutar id é pior do que parece: um id errado não deixa a liga vazia, ele carrega
dados de **outra** competição sob o nome que você escolheu, e ninguém percebe.

### Defeito encontrado durante esta investigação (corrigido)

`SyncRepo.UpsertMatch` gravava odd sintética **sem rótulo**: linha nova caía no
`DEFAULT 'unknown'`, descrevendo como "origem desconhecida" algo de origem
perfeitamente conhecida. Pior, no `ON CONFLICT` ele sobrescreveria odd **real** com
sintética mantendo o rótulo `'real'` — lavando dado fabricado como se fosse de
mercado, exatamente o que o AUD-001 existe para impedir.

A migration 013 marcou os dados que **já existiam**; o **produtor** continuava sem
correção. Agora grava `'synthetic'` explicitamente e nunca sobrescreve odd marcada
como `'real'`.

---

## AUD-001 — Odds sintéticas derivadas da própria amostra causam leakage

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Estatística / Backtest
**Status:** RESOLVIDO no código · migration `013` **não aplicada** (aguarda aprovação — ver Fase D)
**Data:** 2026-09-09

### Fase A — Investigação

**Hipótese da auditoria:** a odd usada no backtest não é de mercado; é derivada da média de
escanteios do próprio lote que depois será testado.

**Confirmada.** Rastreamento completo de quem escreve `matches.corner_odds`:

| Rota | Arquivo | Escreve odd? | Origem |
|------|---------|--------------|--------|
| Worker de sincronização (statsync) | `repository/postgres/statsync_repo.go:17` | **Não** | — o cabeçalho do arquivo declara explicitamente que nenhuma query ali toca `corner_odds` |
| `cmd/sync` | `repository/postgres/sync_repo.go:68,75` (`corner_odds = EXCLUDED.corner_odds`) | Sim | `usecase.SyntheticCornerOdds(batchMu)` |
| `cmd/seed` | idem | Sim | `usecase.SyntheticCornerOdds(batchMu)` |

`sync_usecase.go:69-82` calcula `batchMu` como a média de escanteios **de todo o lote** e
aplica **um único mapa de odds a todas as fixtures** desse lote. `oddsgen.go` deriva a odd
por aproximação normal (desvio 3.0, margem 1.08) sobre essa média.

Conclusão: **nenhuma odd real existe em nenhum ponto do sistema.** Não há fonte de mercado
integrada. Toda odd em produção é derivada da amostra.

**Evidência em produção (consulta de leitura, 2026-09-09):**

| Métrica | Valor |
|---------|-------|
| Partidas totais | 3.547 |
| Partidas com `corner_odds` preenchida | 2.558 |
| Partidas com odd de mercado | **0** |
| Estratégias `origin='discovery'` | 5 (todas `active = true`) |

Somado à evidência já registrada na auditoria — 5 ligas europeias com **exatamente 1 odd
distinta** para a linha 8.5, e Brasileirão Série A com **51/51 (100%)** de acerto entre os
jogos com odd ≤ 2,20 — o mecanismo do artefato fica explícito: filtrar por `MaxOdds` não
seleciona jogos com preço favorável, seleciona **lotes de média alta**. O acerto alto sai
por construção.

### Fase B — Implementação

Cinco mudanças, todas aditivas. Nenhum dado histórico apagado.

**1. `backend/migrations/013_odds_source.sql`** *(criada, NÃO aplicada)*
- `matches.odds_source TEXT NOT NULL DEFAULT 'unknown'` + CHECK `('real','synthetic','unknown')`
- Backfill: todo `corner_odds` não vazio → `'synthetic'` (justificado pelo rastreamento da Fase A)
- Índice parcial `idx_matches_real_odds ... WHERE odds_source = 'real'` — a consulta do
  Discovery precisa ser barata mesmo quando (como hoje) o resultado é vazio
- `UPDATE strategies SET active = false WHERE origin='discovery' AND active=true` —
  **despublica sem apagar**: linha, backtests, scores e health permanecem para auditoria

**2. `internal/domain/entities.go`** — campo `OddsSource`, constantes
`OddsSourceReal|Synthetic|Unknown`, método `HasRealOdds()`.

**3. `internal/repository/postgres/match_repo.go`** — `AllMatches` passa a ler
`odds_source` e `created_at`.

**4. `internal/usecase/filter_usecase.go`**
- `FilterCriteria.RequireRealOdds bool` — `json:"-"`, **não exposto ao usuário**: é ligado
  internamente por quem valida estratégia, não é um filtro de tela
- Ramo de escanteios: descarta a partida quando `RequireRealOdds && !HasRealOdds()`
- `BacktestResult.OddsSource` + `BacktestResult.FinancialsReliable`, preenchidos por
  `oddsSourceSummary()`: só é `true` quando **100%** das odds usadas são reais — uma única
  sintética no lote e o conjunto vira cenário hipotético
- **AUD-012 corrigido junto** (mesma linha de código, separá-lo exigiria tocar o ramo duas
  vezes): partida sem odd deixa de entrar com `odd = 1.0`. Fabricar 1.0 produzia P/L = 0 no
  acerto e −stake no erro — prejuízo estrutural sem significado financeiro

**5. Discovery e reavaliação passam a exigir odd real**
- `internal/usecase/discovery/engine.go:250` — `RequireRealOdds: true` na mineração
- `internal/usecase/strategyengine/engine.go:86` — `RequireRealOdds: true` na reavaliação
  de estratégia publicada (reavaliar é validação quantitativa, mesma exigência)

**6. Frontend — o Simulador continua rodando, mas diz o que está mostrando**
- `core/models.ts` — `odds_source` e `financials_reliable` no `BacktestResult`
- `features/filters/filters.component.html` — aviso âmbar quando `odds_source === 'synthetic'`:
  as odds foram estimadas do próprio histórico, ROI/yield/lucro são cenário e não desempenho
  observado

### Fase C — Testes

`backend/internal/usecase/filter_odds_source_test.go` (novo, 7 testes):

| Teste | Garante |
|-------|---------|
| `TestRequireRealOdds_RejeitaSintetica` | SYNTHETIC → rejeitado pelo Discovery (`MatchCount = 0`) |
| `TestRequireRealOdds_RejeitaUnknown` | UNKNOWN e string vazia (linha pré-013) → rejeitados |
| `TestRequireRealOdds_PermiteReal` | REAL → aceito, `OddsSource='real'`, `FinancialsReliable=true` |
| `TestOddsSourceSummary_MisturaNaoEConfiavel` | lote misto real+sintético → não confiável |
| `TestSimulador_AceitaSinteticaMasMarcaResultado` | Simulador roda, mas resultado sai marcado |
| `TestSemOdd_PartidaForaDoBacktest` | AUD-012 — sem odd, fora do backtest; financeiro zerado |
| `TestMetricaComOddFixa_NaoEConfiavel` | odd fixa do usuário nunca é medida de mercado |

**Resultado:**

```
go build ./...                    OK
go vet ./...                      OK (sem apontamentos)
gofmt -l internal cmd pkg         OK (nenhum arquivo)
go test ./...                     ok  formulas, usecase, aidocs, analytics, discovery, strategyengine
npx tsc --noEmit                  OK
npx ng build --configuration production   OK (só warnings pré-existentes)
```

Nenhum teste existente foi alterado para passar.

### Fase D — Validação

**Efeito medido sobre a produção atual, se a migration for aplicada:**

| Item | Antes | Depois |
|------|-------|--------|
| Partidas marcadas `synthetic` | — | 2.558 |
| Partidas marcadas `unknown` | — | 989 |
| Partidas marcadas `real` | — | **0** |
| Estratégias de descoberta ativas | 5 | **0** |
| Estratégias de descoberta preservadas no banco | 5 | 5 |
| Estratégias que o ciclo publicará | — | **0**, até existir odd real |

**Este é o resultado correto, não uma falha.** Foi previsto na auditoria e autorizado
explicitamente na instrução de correção ("se encontrar 0 estratégias, isso também pode ser
um resultado válido"). As 5 estratégias publicadas hoje têm 100% de acerto e drawdown 0,00
porque foram mineradas sobre odd derivada da própria amostra — mantê-las no ar seria
apresentar artefato como oportunidade.

**O que NÃO foi feito, e por quê:**
- **A migration 013 não foi aplicada em produção.** Ela altera a tabela `strategies`
  (despublicação). Alteração com efeito visível ao usuário exige aprovação explícita —
  fica pendente.
- **Não foi criada nenhuma odd real.** Fabricar odd para "fazer o sistema funcionar" é
  exatamente o defeito auditado, em outra roupagem.
- **Nenhum critério foi relaxado** para compensar o resultado zero.

**Pendência de produto que esta correção expõe (não é bug, é falta de insumo):** o CornerLab
não tem fonte de odds de mercado integrada. Enquanto não tiver, o Discovery é estruturalmente
incapaz de validar estratégia financeira, e o Simulador só pode operar como ferramenta de
cenário. Isso precisa entrar no backlog como item próprio.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-001 e AUD-012 marcados
- Arquivos alterados:
  - `backend/migrations/013_odds_source.sql` *(novo, não aplicado)*
  - `backend/internal/domain/entities.go`
  - `backend/internal/repository/postgres/match_repo.go`
  - `backend/internal/usecase/filter_usecase.go`
  - `backend/internal/usecase/discovery/engine.go`
  - `backend/internal/usecase/strategyengine/engine.go`
  - `backend/internal/usecase/filter_odds_source_test.go` *(novo)*
  - `frontend/src/app/core/models.ts`
  - `frontend/src/app/features/filters/filters.component.html`

**Ações pendentes do usuário:**
1. Aprovar e aplicar a migration `013_odds_source.sql` em produção (despublica 5 estratégias)
2. Decidir a fonte de odds reais antes de esperar qualquer publicação do Discovery

---

## AUD-002 — ROI, Yield e EV são o mesmo número; DSFR conta 50% em uma só variável

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Matemática / Scores
**Status:** RESOLVIDO — Formula Catalog **v1.1**
**Data:** 2026-09-09

### Fase A — Investigação

**Hipótese da auditoria confirmada, e a causa é mais estrutural do que o relatório sugeria.**

O relatório descrevia `result.Yield = round2(roi)` como se fosse uma atribuição
equivocada. Não é um deslize: **sob a mecânica atual do motor, ROI e Yield são a mesma
quantidade por definição.** O backtest aposta stake constante e liquida toda entrada,
então "investimento" (Catálogo 07) e "volume apostado" (Catálogo 08) são o mesmo
denominador. Escrever `Yield = ROI` é o cálculo correto — o erro está em depois tratá-los
como duas evidências.

O EV é outra história: **nunca foi calculado.** `strategyengine/engine.go` gravava
`EV: ptr(r.Yield)` — o campo recebia o ROI emprestado.

**Mapa da concentração no DSFR v1.0** (`engine.go:229-238`, pesos de `formulas/scores.go`):

| Slot | Valor recebido | Peso |
|------|----------------|------|
| ROI | `roiNorm` = ROI/20 | 20% |
| EV | `yieldNorm` = ROI/15 | 20% |
| Yield | `yieldNorm` — **a mesma variável Go, de novo** | 10% |

**50% do score em uma quantidade só**, normalizada por dois tetos diferentes para
disfarçar. Verificado executando a v1.0 preservada:
`DSFRScore{ROI:1, EV:1, Yield:1} = 50,00` (teste `TestDSFRV10_RetornoValia50PorCento`).

**Mesmo defeito em mais três lugares, todos dentro do escopo do AUD-002:**

| Local | Colapso |
|-------|---------|
| `RankingScore` | ROI 20% + Yield 10% sobre a mesma quantidade — em cima de um DSFR (35%) que já era 50% dela |
| `healthRow` | `ΔEV = (Yield − prevYield)/yieldCapPct` = o próprio ΔROI reescalado → metade da "saúde" era uma variável |
| `discovery/criteria.go` | Gates de ROI (≥10%), Yield (≥5%) e "EV" (>0) incidem todos sobre lucro/volume. Com os limiares padrão **só o de ROI chega a barrar** — os outros dois nunca são atingidos. A regra do doc 08 ("nunca considerar apenas Win Rate; sempre múltiplos indicadores") estava sendo satisfeita com três cópias de um indicador |

**Por que os testes existentes não pegaram:** `TestScoresRow` e `TestHealthRow*` são
direcionais ("forte > fraca") e passam identicamente antes e depois. Nenhum afirmava peso.

### Fase B — Implementação

**Decisão: opção (b) da auditoria — remover EV e Yield do DSFR e reponderar.**
A opção (a) foi descartada, e o motivo importa:

- **Yield distinto de ROI** exige stake variável por entrada. O motor não tem política de
  staking. Inventar uma só para diferenciar as métricas seria fabricar dado.
- **EV distinto** exige P(vitória) de um modelo independente, estimado **fora da amostra**.
  Com a taxa de acerto do próprio lote e a odd média dele, o EV colapsa algebricamente no
  ROI realizado — o mesmo número com outro nome. Estimativa fora da amostra é o AUD-003 e
  ainda não existe.

**1. `internal/formulas` — Catálogo v1.1** (`scores.go`, `doc.go`)

Novas funções, com as da v1.0 **preservadas e testadas**: linhas de `backtests` gravadas
antes carregam `algorithm_version = "1.0"` e só podem ser reproduzidas pela fórmula da
época. Score de versões diferentes não é comparável.

| Fórmula | v1.0 | v1.1 |
|---------|------|------|
| `DSFRScoreV11` | ROI 20 · EV 20 · WR 15 · Yield 10 · DD 10 · Amostra 10 · Consist. 10 · Var. 5 | **ROI 30 · WR 20 · DD 15 · Amostra 15 · Consist. 15 · Var. 5** |
| `RankingScoreV11` | DSFR 35 · Health 25 · ROI 20 · Yield 10 · Conf. 10 | **DSFR 40 · Health 25 · ROI 20 · Conf. 15** |
| `HealthScoreV11` | média de 4 deltas (ΔROI, ΔEV, ΔDD, ΔCons) | **média de 3** (ΔEV era ΔROI) |

Os 30 pontos liberados por EV e Yield **não voltaram para o ROI** — isso apenas
reconcentraria a mesma quantidade. Foram distribuídos entre as dimensões restantes.
Exposição total ao retorno: **50% → 30%**.

**2. `strategyengine/engine.go`**
- `EV: nil` — o campo passa a sair NULO. "Não calculado" é a informação verdadeira; um
  número emprestado de outra métrica não é.
- `scoresRow` e `healthRow` usam as funções v1.1
- `components` e `variation` (JSON exibido na tela de detalhe) perdem as chaves `ev` e
  `yield`: não eram componentes, eram o `roiNorm` reescalado

**3. `discovery/criteria.go`** — `rejectEV` (`"ev_nao_positivo"`) renomeado para
`rejectNonPositiveProfit` (`"lucro_nao_positivo"`). **Comportamento idêntico** — a condição
`r.Yield <= 0` continua exatamente igual. Os gates de ROI e Yield **não foram removidos**:
removê-los afrouxaria a validação em configurações não padrão (`MinYield` é configurável).
O que muda é o rótulo deixar de mentir, e um comentário registrando quais são os
indicadores realmente independentes da função.

**4. Propagação da verdade para fora do backend**
- `usecase/aidocs` (documento vivo lido por outra IA): tabela de pesos atualizada e um
  aviso explícito de que ROI e Yield são o mesmo número e o EV vem nulo
- `handlers/discovery_handler.go`: `EV float64` → `EV *float64`. `derefFloat(nil)` produzia
  `0.0`, e a tela exibiria "EV 0%" como se fosse medição
- `frontend/core/models.ts`: `ev: number | null`

**Nenhuma migration.** `strategy_scores` e `strategy_health` são upsert por estratégia —
o próximo ciclo do worker regrava tudo em v1.1 sozinho. `backtests` é histórico
append-only e suas linhas v1.0 estão **corretas para a versão que declaram**; reescrevê-las
seria alterar dado histórico.

### Fase C — Testes

`internal/usecase/strategyengine/scores_weights_test.go` (novo, 10 testes). O teste
obrigatório da auditoria feito **nos dois sentidos**, porque só o par flagra a regressão:

| Teste | Garante |
|-------|---------|
| `TestDSFR_RetornoNaoPodeUltrapassarSeuPesoDeclarado` | variar só o retorno move o DSFR exatamente 30,00 pontos (= peso declarado). Sob a v1.0 movia ~50 |
| `TestDSFR_ComRetornoFixoOsDemaisComponentesMovemOScore` | com retorno fixo, os demais componentes movem o score — e dentro do teto de 70 |
| `TestDSFRV11_PesosSomamUmECadaSlotValeOQueDeclara` | cada slot isolado devolve o próprio peso; pesos somam 1 |
| `TestDSFRV10_PreservadaParaHistorico` | v1.0 continua reproduzindo 20/20/10 |
| `TestDSFRV10_RetornoValia50PorCento` | prova executável do defeito original |
| `TestRankingV11_SemYieldEPesosSomamUm` | ranking sem slot de yield |
| `TestHealthV11_TresDeltasNaoQuatro` | `HealthScoreV11(0,−1,0)` = 50 + 50/3, não 50 + 50/4 |
| `TestBacktestRow_EVFicaNulo` | EV nulo; ROI e Yield seguem gravados |
| `TestScoresRow_ComponentesSemEVeYield` | JSON de componentes com 6 chaves, sem `ev`/`yield` |
| `TestHealthRow_VariacaoSemChaveEV` | JSON de variação com 3 deltas |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  formulas, usecase, aidocs, analytics,
                                              discovery, strategyengine  (25 testes)
npx ng build --configuration production    OK
```

Nenhum teste existente foi alterado. Os cinco de `engine_test.go` continuam passando sem
tocar em nada — o que confirma o diagnóstico da Fase A sobre a fraqueza deles.

### Fase D — Validação

Recalculei o DSFR das **5 estratégias reais em produção** a partir dos componentes gravados
em `strategy_scores`, usando as duas versões da fórmula. A v1.0 reimplementada reproduz o
valor gravado em todas (divergência máxima de 0,02, do arredondamento dos componentes a 3
casas no JSON) — o que valida o modelo antes de comparar.

| id | Estratégia | DSFR v1.0 | DSFR v1.1 | Δ |
|----|-----------|-----------|-----------|---|
| 3 | Escanteios 8.5+ · odd ≤ 2,20 | 100,00 | 100,00 | 0,00 |
| 4 | Escanteios 7.5+ · odd ≤ 2,20 | 99,04 | 98,56 | −0,48 |
| 5 | Escanteios 6.5+ · odd ≤ 2,20 | 95,29 | 92,95 | −2,34 |
| 6 | Escanteios 7.5+ · odd ≤ 1,70 | 84,17 | **87,10** | **+2,93** |
| 7 | Escanteios 8.5+ · odd ≤ 3,50 | 76,22 | 74,19 | −2,03 |

**Leitura honesta destes números — três observações, uma delas incômoda:**

1. **A correção re-rankeia, não rebaixa.** A estratégia 6 **subiu** 2,93. Seu retorno era
   fraco (`roi_norm` 0,57) mas todos os demais componentes eram 1,0; a v1.0 lhe dava peso
   extra por retorno através dos slots de EV/Yield (que usam teto 15 em vez de 20, inflando
   o valor), enquanto a v1.1 lhe dá peso pelos componentes que ela de fato tem. Quem
   ganha e quem perde depende de onde estava o mérito.

2. **A ordem não mudou aqui, e isso não é reconforto.** Os cinco casos têm
   `sample = consistency = inv_drawdown = 1,0`, ou seja, quase não há variação nos
   componentes não-retorno — justamente porque são os artefatos do AUD-001 (win rate 100%).
   Com uma carteira real e diversa, a reordenação seria maior. **Esta amostra é pequena
   demais para concluir que o impacto no ranking é pequeno.**

3. **O efeito prático imediato é nulo, por outro motivo:** essas 5 estratégias serão
   despublicadas pelo AUD-001 assim que a migration 013 for aplicada. Todas continuam acima
   do gate `MinDSFR = 40` sob a v1.1, então **a correção do AUD-002 sozinha não elimina
   nenhuma estratégia** — ela não foi feita para isso.

**O que esta correção NÃO faz, e precisa ficar claro:** o DSFR v1.1 ainda **não tem seis
dimensões independentes**. `InvVariance = 1 − 4p(1−p)` é função pura do win rate, e
`Consistency` é uma composição dos outros quatro componentes. Isso é o **AUD-007** (P1) e
foi deliberadamente deixado em aberto — corrigir dois problemas na mesma passagem
impediria atribuir efeito a causa. O comentário no topo da v1.1 em `scores.go` registra a
pendência para quem ler a fórmula antes de eu chegar lá.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-002 marcado
- Arquivos alterados:
  - `backend/internal/formulas/scores.go` (v1.1; v1.0 preservada)
  - `backend/internal/formulas/doc.go` (`Version = "1.1"` + histórico)
  - `backend/internal/usecase/strategyengine/engine.go`
  - `backend/internal/usecase/discovery/criteria.go`
  - `backend/internal/usecase/aidocs/aidocs.go`
  - `backend/internal/delivery/http/handlers/discovery_handler.go`
  - `backend/internal/usecase/strategyengine/scores_weights_test.go` *(novo)*
  - `frontend/src/app/core/models.ts`
- Limpeza: removidos ~50 arquivos `*.go.<número>` (lixo de edição do mount Windows) que
  poluíam o repositório. Nenhum era código-fonte.

**Ações pendentes do usuário:** nenhuma para o AUD-002. Os scores em produção migram
sozinhos para v1.1 no próximo ciclo do worker.

---

## AUD-003 — Múltiplas comparações sem correção nem validação fora da amostra

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Estatística
**Status:** RESOLVIDO
**Data:** 2026-09-09

### Fase A — Investigação

**Confirmado, sem atenuantes.** `generateCombos` monta 540 combinações por liga
(5 linhas × 3 mandos × 3 janelas × 4 tiers × 3 tetos de odd); com `IncludeTeams`, mais
45 por equipe. `mine` roda backtest de cada uma **contra o mesmo histórico**, aplica
limiares fixos e `publish` fica com as melhores por DSFR.

Não existe no código: nenhum split treino/teste, nenhum registro do número de testes,
nenhuma correção de limiar por multiplicidade. `FilterCriteria` não tinha sequer como
expressar uma janela temporal — só `maxAgeDays`, que é relativo a `time.Now()` e
portanto muda de significado a cada execução.

**Por que isso é fatal e não apenas imperfeito.** Testar muitas hipóteses contra um
único conjunto de dados e ficar com as vencedoras é o procedimento que *fabrica*
falsos positivos. Com 5.940 testes a 5% de significância, ~297 combinações "aprovam"
mesmo que nenhuma tenha vantagem alguma. O desenho não tinha como distinguir vantagem
de ruído — aprovar sorte era o comportamento esperado dele.

**Um agravante que o relatório não citava:** o critério de acerto (`MinWinRate = 75%`)
não é comparado com nada. Uma linha de escanteios baixa acerta 90% e paga odd 1,05 —
prejuízo garantido. "Taxa de acerto alta" só significa alguma coisa contra a
probabilidade que a odd já embutia.

### Fase B — Implementação

Três mecanismos. Os três são obrigatórios e nenhum é desligável por configuração.

**1. Janela temporal no motor de backtest** — `usecase/filter_usecase.go`

`FilterCriteria.DateFrom` / `DateTo`, intervalo **meio-aberto `[From, To)`**, `json:"-"`
(interno, o Simulador não muda de comportamento). Meio-aberto de propósito: descoberta
e validação usam a mesma data de corte nas duas pontas, e um intervalo fechado dos dois
lados colocaria as partidas do dia do corte nos dois conjuntos. Aplicado **antes** de
`LastNGames`, para que "últimos N jogos" signifique "os N mais recentes dentro da
janela" — se fosse depois, a janela de descoberta poderia selecionar jogos que só
existem porque a de validação foi lida.

**2. Corte temporal do histórico** — `usecase/discovery/validation.go` (novo)

`splitHistory` divide por **data**, no quantil da *contagem* de partidas (70/30). Não
por calendário: ligas têm pausas e temporadas de tamanhos diferentes, e cortar "na
metade do tempo" desbalancearia as amostras. Todas as partidas da data de corte vão
para a validação. Quando o histórico não permite um corte com significado (menos de 2
partidas, ou todas no mesmo dia), a liga **não publica nada** — motivo registrado
`liga_sem_janela_de_validacao`. Sem janela de validação não há como saber se o padrão
sobrevive fora da amostra, e publicar sem saber é o defeito original.

**3. Significância corrigida por número de testes** — `formulas/significance.go` (novo)

- `BinomialAtLeast(n, k, p)` — p-valor unilateral exato, somado em espaço logarítmico
  (log-gama). Não aproximação normal: com n de algumas dezenas e p perto de 1, a
  normal erra justamente na cauda que interessa.
- A hipótese nula é a **probabilidade implícita na odd** (1/odd, Catálogo 02), média
  sobre as entradas do backtest. A pergunta deixa de ser "acerta muito?" e passa a ser
  "acerta mais do que a odd já pagava?".
- `FDRThreshold(pvalues, q, método)` — Benjamini–Hochberg e Benjamini–Yekutieli.
  **Padrão: Benjamini–Yekutieli**, que vale sob dependência arbitrária. As combinações
  compartilham fortemente as mesmas partidas ("8.5+ casa e fora" e "8.5+ mandante" são
  o mesmo jogo visto duas vezes), então a hipótese de dependência positiva do BH não é
  demonstrável aqui; com estrutura de dependência desconhecida, o procedimento
  conservador é a escolha honesta — erra publicando de menos.
- Limiar 0 significa **"não publique nada"**, nunca "sem limiar". Explicitado no código
  porque tratar 0 como ausência de limiar publicaria tudo.

**4. Reteste fora da amostra** — `Engine.validate`

Cada sobrevivente é reexecutado na janela de validação. Exigências: mínimo de 30
ocorrências, lucro positivo e p-valor ≤ 0,05. Os critérios **não** são os do doc 08, e
isso é deliberado: a janela de validação é menor por construção, e exigir os mesmos 100
jogos reprovaria tudo por tamanho de amostra — a validação viraria teatro. O que se
exige aqui é consistência. Sem correção de multiplicidade nesta etapa, porque o conjunto
de candidatos já foi fixado *antes* de olhar esses dados, que é exatamente a condição
que a correção existe para restaurar.

**Duas decisões que fecham portas dos fundos:**

- `withDefaults` **não deixa desligar as travas**: `TrainFraction = 1.0` (minerar tudo)
  ou `FDRq = 1.0` (aceitar qualquer p-valor) voltam ao padrão em vez de valer. Tornar
  essas duas configuráveis seria devolver o defeito por outro caminho.
- **Ciclo interrompido não publica nada.** Antes, `mine` devolvia o que já tinha
  minerado ao receber shutdown. A correção de multiplicidade só é válida sobre a
  varredura completa — corrigir sobre um prefixo usaria um *m* menor que o real e
  afrouxaria o limiar.

**5. O usuário passa a ver a evidência** — `describeValidation`

A descrição de cada descoberta publicada traz o período em que o padrão foi procurado,
o período em que se sustentou, e as duas probabilidades de o resultado ter saído por
acaso. Um número de acerto sem "quantas hipóteses foram testadas para chegar nele"
comunica mais confiança do que a evidência sustenta.

**6. Observabilidade** — `LeagueResult` ganha `TrainUntil`, `HoldoutFrom`, `Tested`,
`FDRThreshold`, `Significant`, `Validated`. "Publiquei 3" não significa nada sem "de
quantos testes" e "validadas contra qual período".

**7. Documento vivo** (`usecase/aidocs`) — seção nova explicando as três barreiras, com
os parâmetros lidos das constantes reais.

**Nenhuma migration.** A correção é toda de pipeline; não altera schema nem dado
histórico.

### Fase C — Testes

`formulas/significance_test.go` (8 testes) e `usecase/discovery/outofsample_test.go`
(9 testes, ciclo completo com repositórios falsos).

**O teste obrigatório da auditoria — e o erro que quase o tornou inútil.**

A primeira versão do teste de ruído passava, mas eu instrumentei antes de confiar nela.
Diagnóstico: `tested = 0`, `rejeições = {amostra_insuficiente: 400, win_rate_baixo: 140}`.
**O ruído era barrado pelos critérios do doc 08 e nunca chegava ao mecanismo do AUD-003.**
O teste teria passado com a correção inteira revertida — provaria que o `MinWinRate` de
75% funciona, não que a correção funciona.

Duas mudanças resolveram:

- **Odds justas.** O ruído usava odd 2,00 fixa em todas as linhas, o que cria vantagem
  ou desvantagem *real* — nesse caso o motor estaria certo em achar padrão. Agora as
  odds são as exatamente justas da distribuição geradora (1 ÷ P(total > linha),
  calculada analiticamente, **não** estimada da amostra — estimá-la seria reintroduzir
  o AUD-001 dentro do teste). Assim toda aposta tem valor esperado exatamente zero e o
  que sobra é só variação amostral.
- **Limiares do doc 08 permissivos** no cenário de teste, para que o ruído alcance a
  correção. Não é "afrouxar filtro para gerar oportunidade": as travas do AUD-003
  continuam nos valores de produção, e `DefaultCriteria()` não foi tocada. É isolar o
  mecanismo sob teste.
- **Guarda anti-vácuo permanente:** `if res.Tested == 0 { t.Fatalf(...) }`. Se algum dia
  o ruído voltar a ser barrado antes da correção, o teste falha em vez de passar sem
  provar nada.

| Teste | Garante |
|-------|---------|
| `TestCicloSobreRuidoPuroNaoPublicaNada` | 5 sementes × 700 partidas de ruído justo → **0 publicadas**, com `Tested > 0` |
| `TestSemCorrecaoORuidoProduziriaAprovacoes` | contraprova: o mesmo ruído teria aprovado **464 combinações** antes do AUD-003 |
| `TestJanelasNaoSeSobrepoem` | nenhuma partida nas duas janelas; nenhuma perdida; fração ~70% |
| `TestSplitRecusaHistoricoInsuficiente` | vazio, 1 partida e "tudo no mesmo dia" não são divisíveis |
| `TestLigaSemJanelaDeValidacaoNaoPublica` | publica 0 e registra o motivo |
| `TestConfiguracaoNaoDesligaValidacao` | `TrainFraction=1`, `FDRq=1`, `alpha=1` voltam ao padrão |
| `TestPValorNaoPremiaAcertoAltoComOddBaixa` | 90% de acerto em odd 1,11 → p ≈ 0,4 (não significativo); 65% em odd 2,00 → p < 0,01 |
| `TestCheckHoldoutReprovaCadaMotivo` | cada motivo de reprovação fora da amostra |
| `TestFDRThreshold_ApertaComMaisTestes` | p = 0,004 sobrevive entre 3 testes, **não** sobrevive entre 1.000 |
| `TestFDRThreshold_NadaSignificativoDevolveZero` | limiar 0 não deixa passar nada |
| `TestBinomialAtLeast_*` | valores conhecidos, monotonicidade, entradas inválidas |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  todos os pacotes (42 testes)
```

Nenhum teste existente foi alterado.

### Fase D — Validação

**Quanto o limiar aperta com o tamanho do espaço de busca** (q = 0,10, Benjamini–Yekutieli):

| Combinações testadas | H(m) | Limiar máximo |
|---|---|---|
| 100 | 5,19 | 0,0193 |
| 540 (uma liga) | 6,87 | 0,0146 |
| 1.000 | 7,49 | 0,0134 |
| 5.940 (ciclo completo) | 9,27 | **0,0108** |

**O que isso exige na prática**, para uma estratégia em odd 2,00 (break-even 50%),
dentro de um ciclo de 5.940 testes:

| Amostra | Acerto necessário para ser significativa |
|---|---|
| 50 jogos | 68,0% |
| 100 jogos | 62,0% |
| 200 jogos | 58,5% |
| 380 jogos | 56,1% |

Amostra maior compra significância mais barato — que é o incentivo correto, e o oposto
do que o motor fazia antes (onde combos de amostra pequena eram os mais fáceis de
"aprovar" por sorte).

**Efeito sobre produção: nenhum efeito observável hoje, por um motivo que não é bom.**
Com o AUD-001, não existe nenhuma odd real no banco (0 de 3.547 partidas), então o
Discovery já publica zero. As três barreiras do AUD-003 estão implementadas e testadas,
mas **só serão exercidas em produção quando houver fonte de odds de mercado**. Não é
possível medir aqui a queda real de publicações — a medição vive no cenário sintético
da Fase C, onde a queda foi de 464 aprovações para 0.

**Efeito colateral intencional:** a mineração passa a ver 70% do histórico, então menos
combinações alcançam os 100 jogos do doc 08. Isso é um **aperto, não um problema a
compensar** — a trava de amostra mínima não foi e não deve ser reduzida para recuperar
volume.

**Limitação conhecida, registrada e não corrigida aqui:** o backtest persistido na
publicação é o da **janela de descoberta**, mas o worker de reavaliação
(`strategyengine.RunStrategy`) roda a definição sobre o histórico **completo**, sem
janela. Ou seja, no ciclo seguinte os números exibidos passam a incluir o período de
validação. Para monitorar degradação isso é razoável; para "os números que fundamentaram
a publicação", não. Corrigir exige decidir se a estratégia publicada mostra o número da
descoberta ou o número corrente — é decisão de produto, não de auditoria, e está fora
do escopo do AUD-003.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-003 marcado
- Arquivos alterados:
  - `backend/internal/formulas/significance.go` *(novo)*
  - `backend/internal/formulas/significance_test.go` *(novo)*
  - `backend/internal/usecase/discovery/validation.go` *(novo)*
  - `backend/internal/usecase/discovery/outofsample_test.go` *(novo)*
  - `backend/internal/usecase/filter_usecase.go` (janela `DateFrom`/`DateTo`)
  - `backend/internal/usecase/discovery/engine.go`
  - `backend/internal/usecase/discovery/criteria.go`
  - `backend/internal/usecase/aidocs/aidocs.go`

**Ações pendentes do usuário:** nenhuma para o AUD-003. A correção só produz efeito
visível quando existir fonte de odds reais (pendência do AUD-001).

---

## AUD-004 — Filtro de força do adversário quebrado e com look-ahead estrutural

**Severidade:** CRITICAL · **Prioridade:** P0 · **Categoria:** Dados / Estatística
**Status:** RESOLVIDO no código · migration `014` **não aplicada** (aguarda aprovação)
**Data:** 2026-09-09

### Fase A — Investigação

**O relatório chamou o dado de "corrompido". Não é. É pior: nunca houve dado.**

Rastreamento de quem escreve `teams.tier`:

| Rota | Valor gravado |
|------|---------------|
| `repository/postgres/sync_repo.go:44` | `'G12'` — **string literal no SQL** |
| `repository/postgres/statsync_repo.go:74` | `'G12'` — **string literal no SQL** |
| `internal/integration/**` | nada: **zero** referências a "tier" em todo o pacote de integração |

O provedor não devolve esse campo e nenhum ponto do sistema jamais calculou a
posição de nenhuma equipe. `'G12'` é uma constante, não uma medida.

**Estado em produção (09/09/2026):**

| Valor de `teams.tier` | Equipes |
|---|---|
| `G12` | 228 |
| `1` | 110 |
| `G6` | **0** |
| `Z4` | **0** |

As 110 com `'1'` carregam o **número da divisão da liga** (`leagues.tier` vale `'1'`
em 13 ligas e `'2'` em 8) — em algum momento esse valor foi copiado de `leagues`
para `teams`. Divisão não é grupo de classificação.

Também verifiquei que **todas as 338 equipes vieram do provedor** (`external_id`
preenchido em 338, zero semeadas). Ou seja: a hipótese "o `'1'` veio do seed
fictício" está descartada — as equipes do seed já não existem.

**O que o filtro realmente fazia.** "Contra o G12" não selecionava os doze
primeiros colocados: selecionava as equipes que entraram pelo caminho de
sincronização, e excluía as 110 que carregam o número da divisão. Um filtro de
**procedência de cadastro** com aparência de variável de força do adversário.
"Contra o G6" e "contra o Z4" devolviam vazio sempre.

**O segundo defeito, que sobreviveria mesmo se o dado existisse.** `teams.tier` é um
valor único por equipe, sem temporada e sem data. Aplicar a classificação de hoje a
uma partida de 2023 usa informação que não existia na data do jogo. E mesmo uma
classificação por temporada seria look-ahead se apurada da tabela final: na 5ª
rodada ninguém sabe quem termina no G6.

**Superfície de impacto medida antes de mexer:** 0 estratégias publicadas (de 5) e
0 filtros salvos (de 0) usam `opponent_tier`. Nada quebra.

### Fase B — Implementação

**Decisão: desligar o eixo, não remendar o dado.** A alternativa recomendada pela
auditoria — `team_season_tier` calculada da classificação da temporada — foi
descartada *nesta passagem* por três motivos, e fica especificada como trabalho
futuro (abaixo).

**1. `migrations/014_tier_nao_classificado.sql`** *(criada, NÃO aplicada)*
Não apaga nada: preserva o valor atual em `tier_legacy`, esvazia `tier` e
documenta as duas colunas com `COMMENT ON COLUMN`. Reversível com
`UPDATE teams SET tier = tier_legacy`. `leagues.tier` **não** é tocada — ali o
valor é o número da divisão, que é correto e usado como tal.

**2. Parar de gravar a constante** — `sync_repo.go` e `statsync_repo.go` passam a
gravar `''`. Vazio = não classificado, que é a verdade.

**3. O motor de backtest recusa em vez de ignorar** — `FilterCriteria.Validate()`
devolve erro explícito quando `OpponentTier` vem preenchido. Aceitar o parâmetro e
ignorá-lo em silêncio seria pior que um erro: o usuário leria o resultado como
"contra o G6". O bloco que comparava `opp.Tier` foi removido do laço.

**4. Eixo removido do Discovery** — `opponentTierOptions` saiu de
`combinations.go`. **540 → 135 combinações por liga.** As 405 que saíram não eram
hipóteses: eram a mesma hipótese contada quatro vezes, três delas garantidamente
vazias.

**5. Frontend** — seletor "Contra equipes" removido do Simulador; `(G12)` ao lado
do nome do adversário removido do Dashboard; o componente para de enviar
`opponent_tier` e ignora esse campo ao recarregar uma definição antiga.

**6. Documento vivo** (`aidocs`) — o termo "Tier de adversário" passa a explicar
que está desativado e por quê, com instrução explícita para outra IA ignorar
`opponent_tier` em definições antigas.

### Fase C — Testes

`internal/usecase/filter_tier_test.go` (novo, 4 testes) + 1 em `discovery_test.go`.

**Sobre o teste obrigatório da auditoria.** O enunciado era: *"filtro por tier deve
retornar apenas partidas cuja classificação vigente na data era a informada"*. Esse
teste **não é executável hoje**, e escrever algo que passasse no lugar dele seria
fabricar conformidade: ele pressupõe uma classificação com dimensão temporal
("vigente na data"), que o sistema não tem. O que está testado é o comportamento
seguro na ausência dela. Quando a classificação point-in-time existir, o teste
original passa a fazer sentido e deve substituir estes.

| Teste | Garante |
|-------|---------|
| `TestTierRecusadoNoBacktest` | backtest com `opponent_tier` falha, e a mensagem explica o motivo |
| `TestTodosOsTiersSaoRecusados` | G6, G12, Z4, "1" e valores arbitrários — todos recusados |
| `TestSemTierOBacktestRodaNormalmente` | a recusa não virou bloqueio geral |
| `TestTierDeEquipeNaoInfluenciaResultado` | mesmo repovoando `teams.tier`, o resultado não muda — guarda contra a volta silenciosa do filtro |
| `TestGenerateCombosNaoUsaTier` | nenhuma combinação nasce com tier preenchido |

```
go build ./...                            OK
go vet ./...                              OK
gofmt -l internal cmd pkg                 OK
go test ./...                             ok  todos os pacotes
npx ng build --configuration production   OK
```

`TestGenerateCombosCoversFullLeagueGrid` foi ajustado (540 → 135) porque a grade
mudou de propósito — não para fazer o teste passar.

### Fase D — Validação

| Item | Antes | Depois |
|------|-------|--------|
| Combinações por liga | 540 | **135** |
| Combinações que nunca podiam render nada (G6, Z4) | 270 | 0 |
| Combinações filtrando por constante (G12) | 135 | 0 |
| Equipes com classificação real | 0 | 0 *(inalterado — nunca houve)* |
| Estratégias publicadas afetadas | 0 de 5 | 0 |
| Filtros salvos afetados | 0 de 0 | 0 |

**Interação favorável com o AUD-003:** a correção de múltiplos testes penaliza pelo
número de hipóteses testadas. Com 5.940 combinações no ciclo, o limiar de
Benjamini–Yekutieli era 0,0108; com o eixo removido o espaço cai para ~1.485 e o
limiar afrouxa para ~0,0134 — **sem relaxar critério nenhum**. O ciclo estava sendo
punido por hipóteses que não existiam.

**O que esta correção NÃO faz.** Não devolve a capacidade de analisar força do
adversário. Essa análise é legítima e valiosa — foi desligada porque o que estava no
lugar dela era falso, não porque a ideia seja ruim.

**Trabalho futuro especificado (não é AUD, é backlog de produto):**
`team_season_tier(team_id, season_id, as_of_date, tier)` apurada **incrementalmente**
a partir dos jogos anteriores a cada data. Três obstáculos que precisam de decisão
antes de implementar:

1. **Custo e complexidade:** exige recomputar a tabela de classificação a cada
   rodada de cada temporada, e o sistema não importa classificação — teria que
   derivá-la dos resultados (`home_goals`/`away_goals`), que existem.
2. **Mata-mata não tem tabela.** Copa do Brasil, Libertadores, Sul-Americana — e a
   Champions/Europa League que estão para entrar — não têm classificação. O eixo só
   é definível em competições de pontos corridos, e isso precisa estar no desenho
   desde o começo.
3. **Rodadas iniciais são ruído.** Na 3ª rodada a "classificação vigente" é quase
   aleatória. Provavelmente é preciso um mínimo de rodadas antes de o tier valer.

### Fase E — Registro

- `AUDITORIA_COMPLETA_SISTEMA.md` — AUD-004 marcado
- Arquivos alterados:
  - `backend/migrations/014_tier_nao_classificado.sql` *(novo, não aplicado)*
  - `backend/internal/repository/postgres/sync_repo.go`
  - `backend/internal/repository/postgres/statsync_repo.go`
  - `backend/internal/usecase/filter_usecase.go`
  - `backend/internal/usecase/discovery/combinations.go`
  - `backend/internal/usecase/discovery/discovery_test.go`
  - `backend/internal/usecase/aidocs/aidocs.go`
  - `backend/internal/usecase/filter_tier_test.go` *(novo)*
  - `frontend/src/app/features/filters/filters.component.html` e `.ts`
  - `frontend/src/app/features/dashboard/dashboard.component.html`

**Ações pendentes do usuário:**
1. Aprovar e aplicar a migration `014_tier_nao_classificado.sql` (não destrutiva,
   reversível). Sem ela o banco continua afirmando "G12" para 228 equipes, mas
   nenhum código lê esse valor.
2. Decidir se `team_season_tier` entra no backlog.

---

## REV-P0 — sétima passagem (14/09/2026, ~11:50 UTC) — verificação independente da execução agendada

**Nota de ordem.** Esta passagem foi executada pela tarefa agendada de validação, em
paralelo com a sexta passagem acima, e **sem conhecimento dela** até o momento de gravar.
As duas apuraram o mesmo incidente por caminhos independentes e chegaram ao mesmo log. O
que segue é a medição feita no painel e no banco, mais dois achados que a sexta passagem
não registra: o estado dos contadores e **o fato de a correção ainda não estar publicada**.

Esta passagem tinha um único objetivo: colher a primeira execução agendada válida do cron
`cornerlab-worker` (14/09 11:00 UTC) e, com ela, fechar os quatro itens da Definition of
Done que dependiam do relógio.

**A execução ocorreu, foi de fato agendada, e falhou. Nenhum item foi fechado.
REV-P0 permanece PARCIAL.** Nenhum disparo manual foi usado como prova e nada foi
modificado no banco ou no serviço — esta passagem é só medição.

### Render — a execução de hoje

Painel do Cron Job `crn-dair0e95efls73ek3080`, aba Runs. Build corrente **`80a494c`**,
branch `main`, marcado `Latest` — é a reconstrução feita após a rotação da credencial.

| execução | gatilho | duração | resultado |
|---|---|---|---|
| **14/09 11:00 UTC** | **Scheduled** | **24,7s** | ❌ **falha** |
| 13/09 11:00 UTC | Scheduled | 23,0s | ❌ falha |

Total de execuções registradas no serviço: **2**. O painel exibe, no cabeçalho, a frase
**"No successful runs yet."** — ou seja, o cron **nunca teve uma execução bem-sucedida**,
nem antes nem depois do rebuild.

O gatilho da execução de hoje é **"Scheduled"**, não manual. A prova exigida pelo REV-P0
era essa; ela foi produzida, e o que ela prova é que o cron ainda não funciona.

### O erro exato, como o log diz

Log do serviço, 14/09 (o painel exibe em GMT-3; o carimbo dentro da linha JSON é UTC):

```
08:00:44 AM  ==> Cron job run started
08:00:49 AM  {"time":"2026-09-14T11:00:49.006369054Z","level":"ERROR",
              "msg":"worker não vai rodar",
              "error":"configuração inválida para ENVIRONMENT=production: JWT_SECRET (vazio ou ainda no valor de exemplo)"}
08:01:08 AM  ❌ Your cronjob failed because of an error: Exited with status 1
```

Três observações, e nenhuma além disso:

1. **A causa nomeada pelo log é `JWT_SECRET`**, não `DATABASE_URL`. A mensagem diz
   "vazio ou ainda no valor de exemplo" e não distingue entre os dois casos.
2. **O log não menciona `DATABASE_URL`.** Conferi o motivo no código antes de tirar
   conclusão: `Validate` acumula todas as pendências numa lista (`faltando`) e devolve
   **uma mensagem só**. Como `DATABASE_URL` vazia ou apontando para `localhost`/`127.0.0.1`
   entraria nessa mesma lista e não entrou, o silêncio é informação: a variável está
   presente e não aponta para máquina local. **Isso não é conexão efetiva** — o processo
   morreu antes de tentar abrir o Postgres. É a mesma leitura da sexta passagem, e a
   sustento.
3. O processo morreu **5 segundos** após o início, na validação de configuração. Os 24,7s
   de duração são do ciclo de vida do container, não de trabalho realizado.

Não investiguei o ambiente do serviço nem examinei valores de variáveis — nenhuma
credencial foi lida, exibida ou registrada.

### Banco — nenhum vestígio da execução

`sync_runs`, três últimas linhas (consulta às ~11:45 UTC de 14/09):

| id | triggered_by | status | erro | quando | targets | achadas | gravadas | checadas | finalizadas | erros | duration_ms |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 18 | `manual` | success | — | 13/09 18:56:28 | 12 | 3.712 | 3.712 | 300 | 294 | 0 | 900.210 |
| 17 | `manual` | success | — | 13/09 10:02:52 | 12 | 3.712 | 3.712 | 300 | 299 | 0 | 917.502 |
| 16 | `manual` | success | — | 12/09 01:40:03 | 12 | 0 | 0 | 50 | 0 | 62 | 402.069 |

**Não existe nenhuma linha com `triggered_by='cron'`.** A execução de hoje não gravou
nada: nem sucesso, nem falha. Coerente com o log — o processo saiu antes de tocar o banco.

A linha mais recente continua sendo o disparo manual de 13/09 18:56. Logo, **os dados de
produção não são atualizados automaticamente desde sempre**, e a última atualização de
qualquer origem tem ~17 horas no momento desta consulta.

### Contadores

| medida | valor |
|---|---|
| `worker_runs` | **0** |
| `team_metrics` | **0** |
| `matches` AGENDADO com `match_date` vencido há +3h | **54** |

`worker_runs` e `team_metrics` **continuam zerados** — não saíram de zero. Registro o fato
e não investigo: é pendência de outra prioridade.

As 54 partidas agendadas vencidas são consequência direta de não haver ciclo automático
finalizando resultados.

### Efeito sobre o item 10 (persistência de falhas)

Uma falha real ocorreu hoje e **não gerou linha em `sync_runs`**. Pelo que o log mostra, o
processo encerrou na validação de configuração, antes de abrir conexão com o banco — um
mecanismo que registra falhas *no banco* não tem como capturar uma falha que acontece
antes do banco existir para o processo. O item continua **implementado, sem evidência**;
esta execução não serviu como evidência natural e não deve ser contada como tal.

### Definition of Done — estado real

| # | Item | Estado |
|---|---|---|
| 1 | `DATABASE_URL` funcional no cron | ⏳ **evidência indireta** — passou na validação; conexão não demonstrada |
| 2 | cron conecta ao banco | ⚠️ **SEM EVIDÊNCIA** — processo abortou antes de tentar |
| 3 | migration 016 aplicada | ✅ |
| 4 | backend compila | ✅ |
| 5 | `go vet` passa | ⚠️ **NÃO VERIFICADO** |
| 6 | `go test` passa | ⚠️ **NÃO VERIFICADO** |
| 7 | frontend compila | ✅ |
| 8 | execução automática real tem sucesso | ❌ **FALHOU** — 14/09 11:00 UTC, Scheduled, exit 1 |
| 9 | `sync_runs` registra `triggered_by='cron'` | ❌ **NÃO OCORREU** — zero linhas |
| 10 | mecanismo de falha implementado | ✅ implementado, **sem evidência** |
| 11 | UI diferencia automático/manual | ✅ |
| 12 | alerta de desatualização funciona | ✅ |

**REV-P0 = PARCIAL.** 5 verdes, 2 não verificados, 1 com evidência indireta, 1 sem
evidência, 2 falhados, 1 implementado sem evidência.

Os itens 8 e 9 mudam de categoria: deixam de ser "aguardando o relógio" e passam a ser
**falha observada**. O relógio chegou. A resposta foi negativa.

### A correção da regressão NÃO está publicada

Este é o achado que me parece mais consequente, e não o encontrei registrado na sexta
passagem. Aba **Builds** do Cron Job, no momento desta consulta:

| build | gatilho | commit | quando |
|---|---|---|---|
| 4 | Manual | **`80a494c`** | 17h atrás — **é o build corrente, marcado `Latest`** |
| 3 | Auto-Deploy | `dbb922a` | 23h atrás |
| 2 | Manual | `1a3f3ab` | 23h atrás |
| 1 | First Build | `1a3f3ab` | 1d atrás |

**Nenhum build depois de `80a494c`.** A separação `Validate` / `ValidateAPI` existe no
repositório local — conferi `pkg/config/config.go`, e `Validate` hoje só checa
`DATABASE_URL` —, mas **a imagem que o cron executa ainda é a de 17 horas atrás**, com a
validação antiga.

Consequência direta, e é melhor dizê-la agora do que descobri-la amanhã: **se nada for
publicado, a execução de 15/09 11:00 UTC falha exatamente igual.** A correção só passa a
valer depois de um novo build.

### O que mudou desde a quinta passagem

- A hipótese de que bastava esperar a execução agendada está **derrubada**. O cron roda,
  é agendado, e falha na largada.
- A causa de falha mudou de identidade: era "imagem construída antes das variáveis
  existirem"; agora é **regressão da própria validação de configuração** introduzida na
  quarta passagem. Sintoma novo, build novo, causa nova.
- `go vet` e `go test` permanecem **NÃO VERIFICADOS**. Nada nesta passagem os toca, e o
  build do Docker continua não servindo de substituto. Vale registrar que os quatro testes
  citados na sexta passagem (`TestWorkerSemJWTSecretEhValido`,
  `TestAPISemJWTSecretEhInvalida` e os dois renomeados) **também não foram executados** —
  existem no arquivo, não há saída de `go test` para nenhum deles.

### Pendências, sem ordem de prioridade atribuída aqui

1. **Publicar** a correção de `config.go`. Sem novo build, 15/09 repete 14/09.
2. Só depois disso a próxima execução agendada volta a ser prova útil para os itens 1, 2,
   8 e 9 — e nem essa fecha o P0 sozinha, porque 5 e 6 continuam dependendo da saída de
   `go vet` e `go test` colada por alguém com Go na máquina.
3. Não iniciei o P1 (Dashboard) nem qualquer outra prioridade.

---

## REV-P1 — Dashboard / Visão Geral

**Data:** 2026-09-15 · **Status:** ⚠️ PARCIAL — correções implementadas e validadas em
build/teste, aguardando deploy para validação ponta a ponta em produção.

---

### A — Investigação

Reproduzido no sistema atual, já com o pipeline do REV-P0 funcionando. **O problema
relatado NÃO desapareceu** — mudou de forma.

#### Caso reproduzido: clicar numa equipe e chegar em outra

Navegando para o mesmo endereço que o calendário gera (`/dashboard?league_id=8&team_id=442`,
Athletic Club na La Liga), a tela abriu assim:

```
Campeonato: La Liga   Temporada: 2026   Equipe: Alaves
"Escolha campeonato, temporada e equipe acima e clique em Analisar..."
URL reescrita para: ?league_id=8&team_id=454&season_id=33&limit=10
```

Pedi o time 442 e recebi o 454. A própria URL foi reescrita, apagando o rastro do que fora
pedido. Se o usuário clicasse em "Analisar" ali, analisaria o Alavés acreditando estar
vendo o Athletic Club.

**Rastreamento camada a camada:**

| camada | o que faz | resultado |
|---|---|---|
| UI (calendário) | `openTeamDashboard()` navega com `league_id` + `team_id` | **sem `season_id`** |
| modelo | `UpcomingMatch` | **não tem `season_id`** |
| backend (overview) | `ListUpcoming` não seleciona `m.season_id` | temporada nunca sai do banco |
| UI (dashboard) | sem temporada, adivinha `MAX(year)` | escolhe 2026 |
| `GET /teams?league_id=8&season_id=33` | `JOIN matches ... WHERE league_id AND season_id` | 20 equipes, sem o Athletic |
| UI (dashboard) | equipe pedida ausente → `teams[0]` | **troca silenciosa** |

Confirmação no banco: o Athletic Club tem 38 partidas finalizadas na La Liga **2025** e
**nenhuma linha em 2026** — foi rebaixado. A troca não era um bug aleatório: era a
consequência inevitável de adivinhar a temporada.

#### Caso reproduzido: a tela anuncia mais evidência do que tem

`GET /dashboard?team_id=455&league_id=8&season_id=33&limit=20` (Celta Vigo, La Liga 2026):

```json
{ "sample_size": 6, "period": "Últimos 20 jogos" }
```

E na tela: `Últimos 10 jogos · amostra de 6 jogos` — contradição na mesma linha.
`Period` vinha de `fmt.Sprintf("Últimos %d jogos", limit)`, a janela PEDIDA.

#### Caso reproduzido: ausência virando zero

`GET /dashboard?team_id=442&league_id=8&season_id=33&limit=20`:

```json
{ "sample_size": 0, "period": "Últimos 20 jogos",
  "total_corners": { "count": 0, "mean": 0, "max": 0, "min": 0 },
  "balance": 0, "frequencies": [{ "threshold": 4, "count": 0, "total": 0, "pct": 0 }] }
```

HTTP 200, zeros por toda parte. No frontend, `noData` era `b.sample === 0`, e escanteios e
gols usam `sample: null` — logo `null === 0` é falso e **a tela renderizaria média 0,
mediana 0, moda 0 e "0/0 = 0%"**. Números com cara de observação onde não houve observação.

#### O que NÃO estava quebrado

- **Carregamento automático funciona** quando a equipe existe na temporada resolvida:
  abrir `?league_id=8&team_id=455` já mostrou o Celta sem nenhum clique.
- **`sample_size` sempre foi o número real** de partidas — o defeito estava em `Period` e
  na renderização, não na contagem.
- **Nenhum número inventado** foi encontrado: médias, frequências e denominadores batem
  com a amostra.
- Os leilões de IDs interno × externo estão corretos: o Dashboard trafega apenas IDs
  internos; `external_id` não aparece em nenhuma rota dessa tela.

#### Botão "Analisar" — evidência

`(click)="runDashboard()"`, e `runDashboard()` faz um único `GET /api/v1/dashboard`.
Nenhum processamento, nenhum efeito colateral, nenhuma chamada a IA. **É um botão que pede
um clique para ler dado que o worker já calculou e persistiu.**

Pior: trocar a equipe ou a janela (5/10/15/20) chamava só `syncQueryParams()`. O usuário
mudava de 10 para 20 jogos e a tela continuava mostrando o resultado anterior — o "clico e
nada acontece" relatado, ao contrário: mudava a seleção e nada acontecia.

---

### B — Causa raiz

Três defeitos independentes, todos na fronteira entre camadas:

1. **A temporada não trafega.** O calendário conhece a temporada da partida clicada e não a
   transmite; o backend do calendário nem a devolve. O Dashboard preenche a lacuna com um
   palpite (`MAX(year)`) e, quando erra, **substitui a equipe em silêncio**.
   *Camada: domínio + repositório (overview) + navegação Angular.*

2. **`Period` descrevia o pedido, não o dado.** *Camada: usecase (backend).*

3. **`noData` não cobria amostra zero** para escanteios e gols. *Camada: componente Angular.*

---

### C — Implementação

| arquivo | alteração | por quê |
|---|---|---|
| `internal/domain/overview.go` | `UpcomingMatch.SeasonID` | a temporada é um fato da partida |
| `internal/repository/postgres/match_repo.go` | `ListUpcoming` passa a selecionar `m.season_id` | idem |
| `internal/usecase/dashboard_usecase.go` | `describePeriod()`; campo `RequestedLimit` | separar amostra real de janela pedida |
| `frontend/core/models.ts` | `UpcomingMatch.season_id`, `DashboardResult.requested_limit` | contrato |
| `features/overview/overview.component.ts` | `openTeamDashboard` envia `season_id` | elimina o palpite na origem |
| `features/dashboard/dashboard.component.ts` | sem troca silenciosa; `equipePedidaAusente`; `noData` cobre `sample_size === 0`; `avisoAmostraCurta`; `onTeamChange`/`onLimitChange` recarregam | honestidade + carregamento automático |
| `features/dashboard/dashboard.component.html` | **botão "Analisar" removido**; estados de ausência distintos | ler dado persistido não precisa de clique |

**Decisão sobre o "Analisar": REMOVIDO.** A evidência é inequívoca — ele só chamava
`runDashboard()`. Qualquer mudança de campeonato, temporada, equipe ou janela agora
recarrega sozinha. Se um dia existir explicação narrativa por IA, será outro botão, com
outro nome, separado das estatísticas.

**Três ausências, três frases diferentes — nenhuma delas "0":**

- equipe pedida não joga nesta liga/temporada → aviso âmbar nomeando a liga, **sem trocar
  de equipe**;
- nenhuma partida na combinação → *"Nenhuma partida encontrada... Zero seria um resultado
  observado; aqui não houve observação."*;
- métrica não publicada pelo provedor (impedimentos/chutes) → mensagem própria, já existente.

**Zero real continua zero:** um time que não fez escanteio em três jogos mantém média 0 com
amostra 3. Há teste para isso.

---

### D — Testes e validação de código

`backend/internal/usecase/dashboard_sample_test.go` (7 casos), todos derivados de números
medidos em produção, não de hipótese:

```
--- PASS: TestPeriodoDescreveAAmostraRealENaoAPedida
--- PASS: TestPeriodoSemPartidasNaoDizUltimosNJogos
--- PASS: TestPeriodoComAmostraCompletaNaoPoluiComRessalva
--- PASS: TestPeriodoNuncaAnunciaMenosDoQueAnalisou
--- PASS: TestResumoDeAmostraVaziaNaoInventaObservacao
--- PASS: TestZeroObservadoContinuaSendoZero
--- PASS: TestFrequenciaSemAmostraNaoInventaDenominador
```

Comandos executados nesta sessão:

```
go build ./...                          → exit 0
go vet ./...                            → exit 0
go test ./...                           → exit 0  (9 pacotes, 0 falhas)
gofmt -l internal cmd pkg               → vazio
npx ng build --configuration production → exit 0
```

O projeto **não tem suíte de testes de frontend** configurada (sem `ng test` utilizável);
registrado como ausência, não como aprovação.

---

### E — Validação com dados reais (BANCO × BACKEND)

| CASO | Banco | Backend | Resultado |
|---|---|---|---|
| **1. Celta Vigo · La Liga · 2025** (`limit=20`) | 38 finalizadas, 38 com escanteios | `sample_size` 20, `recent_matches` 20 | **CORRETO** (janela de 20 sobre 38 disponíveis) |
| **2. Celta Vigo · La Liga · 2026** (`limit=20`) | 6 finalizadas, 6 com escanteios | `sample_size` 6, `recent_matches` 6 | **CORRETO** na contagem; `period` era a mentira, agora corrigido |
| **3. Flamengo · Brasileirão Série A · 2026** (`limit=20` e `limit=5`) | 27 finalizadas, 27 com escanteios | 20 e 5, `count` 20 | **CORRETO** — a janela é respeitada nos dois sentidos |
| **4. Athletic Club · La Liga · 2026** | **nenhuma partida** (rebaixado; 38 em 2025) | `sample_size` 0 com médias 0 | **ERA INCORRETO** na apresentação; agora bloqueado por `noData` |

**Antes → depois (comportamento):**

| situação | antes | depois |
|---|---|---|
| clicar em equipe rebaixada no calendário | abre outra equipe, sem aviso | abre a temporada da partida; se ainda faltar, avisa sem trocar |
| Celta 2026 com janela 20 | "Últimos 20 jogos · amostra de 6" | "Últimos 6 jogos (de 20 pedidos)" + aviso explicativo |
| combinação sem partidas | médias 0, moda 0, "0/0 = 0%" | "Nenhuma partida encontrada" |
| trocar equipe ou janela | nada acontece até clicar em "Analisar" | recarrega sozinho |

---

### Definition of Done — REV-P1

| # | Item | Estado |
|---|---|---|
| 1 | fluxo Calendário → Dashboard validado | ✅ reproduzido e corrigido |
| 2 | league/season/team corretos | ✅ `season_id` trafega ponta a ponta |
| 3 | dados existentes carregam automaticamente | ✅ botão removido |
| 4 | banco → backend → frontend rastreável | ✅ 4 casos em tabela |
| 5 | ausência de dados não aparece como zero | ✅ `noData` cobre `sample_size === 0` |
| 6 | tamanho real da amostra exibido/respeitado | ✅ `describePeriod` + `requested_limit` |
| 7 | filtros não vazam estado | ✅ sem troca silenciosa de equipe |
| 8 | botão "Analisar" removido ou legítimo | ✅ removido, com evidência |
| 9 | nenhuma estatística inventada | ✅ nada hardcoded, nenhum fallback |
| 10 | backend build passa | ✅ exit 0 |
| 11 | backend vet passa | ✅ exit 0 |
| 12 | backend tests passam | ✅ exit 0 |
| 13 | frontend production build passa | ✅ exit 0 |
| 14 | ≥3 casos reais ponta a ponta | ⚠️ **PARCIAL** — BANCO × BACKEND validado em 4 casos; **FRONTEND não**, porque o código corrigido ainda não está publicado |
| 15 | documentação atualizada | ✅ esta seção |

**REV-P1 = PARCIAL.**

Falta um único item, e ele depende de deploy: a coluna "Frontend" das tabelas acima foi
medida contra o código ANTIGO, que é o que está no ar. Depois de publicar, é preciso
repetir os quatro casos na interface e confirmar que a tela mostra o que o backend devolve.

---

### Pendências descobertas, fora do escopo do P1 (não corrigidas)

- **`GET /teams?league_id&season_id` inclui partidas AGENDADO.** Por isso o Celta aparece
  na lista de 2026 mesmo com só 6 jogos disputados. É provavelmente o comportamento
  desejado (o time joga a temporada), mas significa que "estar na lista" não garante
  amostra — quem garante é `sample_size`.
- **Ligas sem nenhuma partida** (Liga Portugal, Segunda División, Ligue 2, Serie B, EFL
  Championship, Liga Profesional, Primera Nacional, USL, 2. Bundesliga) têm temporada
  cadastrada e zero jogos. Não aparecem em `/leagues`, então não afetam o Dashboard hoje.
- **UEFA Champions League tem a temporada 2025 vazia** (0 partidas) além da 2026 com 234.
- **Sem suíte de testes no frontend.** As correções de apresentação só têm cobertura
  indireta, via o contrato do backend.
- Avisos `NG8107` pré-existentes em `dashboard.component.html` e `integrations.component.html`.


---

## REV-P1 — validação do item 14 em produção (22/09/2026)

Commit `fad19be` publicado. Backend e frontend confirmados como código novo:
`requested_limit` presente na resposta, textos novos na tela, botão "Analisar" ausente.

> **Nota de método:** as primeiras leituras da API devolveram payload ANTIGO (sem
> `requested_limit`). Era cache HTTP do navegador — repetido com `cache:'no-store'` e
> cache-buster, veio o payload novo. Todas as medições abaixo usam cache desligado.
> O console do Neon não está autenticado nesta sessão, então a coluna BANCO foi medida
> pelo backend com janela ampla (`limit=500`), que lê `matches` direto. **Não é consulta
> SQL direta** — está registrado como tal.

### Casos validados — BANCO → BACKEND → FRONTEND

| CASO | Banco (janela ampla) | Backend | Frontend (tela publicada) | Resultado |
|---|---|---|---|---|
| **1. Celta Vigo · La Liga · 2025 · limit 20** | 38 finalizadas, 38 com escanteios | `sample_size` 20, `requested_limit` 20, `period` "Últimos 20 jogos" | — (ver divergência) | **NÃO VALIDADO NA TELA** |
| **2. Celta Vigo · La Liga · 2026 · limit 20** | 7 finalizadas, 7 com escanteios | `sample_size` 7, `requested_limit` 20, `period` "Últimos 7 jogos (de 20 pedidos)" | "Últimos 7 jogos (de 20 pedidos) · amostra de 7 jogos" + aviso "Você pediu os últimos 20 jogos, mas só existem 7…"; frequências com denominador 7 (`6/7`, `4/7`, `2/7`); 7 partidas listadas | **CORRETO** |
| **3. Flamengo · Série A · 2026 · limit 20** | 28 finalizadas, 28 com escanteios | `sample_size` 20, `period` "Últimos 20 jogos" | "Últimos 20 jogos · amostra de 20 jogos" | **CORRETO** |
| **3b. Flamengo · limit 5 (clique no seletor)** | idem | `sample_size` 5 | amostra passou de 20 → 5 sozinha, `period` "Últimos 5 jogos", URL virou `limit=5`, nova requisição disparada | **CORRETO** |
| **4. Athletic Club (442) · La Liga · 2026** | 0 partidas | `sample_size` 0, `period` "Nenhuma partida encontrada" | campo Equipe **vazio** + "A equipe que você abriu não tem partidas registradas em La Liga nesta temporada." | **CORRETO — sem substituição** |

### Confirmações pedidas

| verificação | resultado |
|---|---|
| `requested_limit` presente | ✅ |
| `period` representa a amostra real | ✅ |
| Celta 2026 não anuncia 20 jogos | ✅ "Últimos 7 jogos (de 20 pedidos)" |
| Athletic Club não é substituído silenciosamente | ✅ equipe some da URL e a tela explica; **antes** virava `team_id=454` (Alavés) |
| ausência de partidas não aparece como zero | ⚠️ parcial — ver abaixo |
| zero real continua zero | ⚠️ não reproduzido na tela — ver abaixo |
| 5/10/15/20 recarregam automaticamente | ✅ medido com clique real |
| botão "Analisar" removido | ✅ inexistente no DOM |
| calendário envia `season_id` | ✅ payload do `/overview/upcoming` traz `season_id`; clique real gerou `?league_id=19&season_id=27&team_id=398&limit=10` |

### DIVERGÊNCIA — `season_id` da URL é ignorado quando não é a temporada mais recente

Reproduzido três vezes, com evidência de rede:

```
Navegação:  /dashboard?league_id=8&season_id=12&team_id=455&limit=15
Requisição: /api/v1/teams?league_id=8&season_id=33
            /api/v1/dashboard?team_id=455&limit=15&league_id=8&season_id=33
Tela:       Temporada 2026
URL final:  ?league_id=8&season_id=33&...        (reescrita)
```

`limit=15` foi respeitado na mesma navegação, então o problema é específico de
`season_id`. O Dashboard continua resolvendo para `MAX(year)`.

**Consequência para o REV-P1:** o lado do calendário foi corrigido — ele agora ENVIA a
temporada — mas o Dashboard a DESCARTA. Para partida da temporada corrente os dois
coincidem e nada aparenta estar errado (foi o que ocorreu no teste do calendário:
Criciúma, Série B, season 27, que também é a mais recente). Para partida de temporada
anterior — exatamente o cenário que motivou a correção — o comportamento antigo persiste.

**Hipótese não comprovada** (registrada como hipótese, não como causa): o handler
`(selectionChange)="onLeagueChange()"` do seletor de campeonato dispara sem argumentos
depois da chamada com presets, sobrescrevendo a temporada. A contagem de requisições
`/teams` repetidas em algumas cargas é compatível com isso. **Não investiguei a fundo nem
corrigi** — fora do escopo desta passagem, que era só validar.

### Itens não reproduzíveis na interface hoje

- **Ausência virando zero:** o único caminho alcançável é o de equipe ausente, e esse foi
  validado. Uma combinação com equipe válida e zero partidas não é produzível pela tela,
  porque o Dashboard descarta a equipe antes. Coberto pelo backend
  (`period: "Nenhuma partida encontrada"`, `sample_size: 0`) e por teste unitário.
- **Zero real continua zero:** não encontrei em produção equipe com escanteios observados
  iguais a zero. Coberto por `TestZeroObservadoContinuaSendoZero`.

Registro como cobertura indireta, não como validação de tela.

### Correção de um erro meu na passagem anterior

Afirmei que o Athletic Club "foi rebaixado". **Estava errado.** Existem DOIS registros com
esse nome:

| id | nome | país | temporada |
|---|---|---|---|
| 442 | Athletic Club | **Brazil** | La Liga 2025 |
| 1198 | Athletic Club | Espanha | La Liga 2026 |

É duplicidade/rotulagem incorreta de equipe, não rebaixamento. O caso de teste continua
válido (442 não tem partidas em 2026), mas a explicação que dei era inferência apresentada
como fato. Fica registrado como pendência nova, **fora do escopo do P1**.

### Definition of Done — REV-P1

Item 14: ⚠️ **PARCIAL** — quatro casos medidos ponta a ponta, três corretos na tela, um
(Celta 2025) não validável enquanto `season_id` for ignorado. Demais itens seguem ✅.

## REV-P1 = PARCIAL

Falta uma coisa só, e é concreta: **o Dashboard precisa respeitar `season_id` da URL**.
Enquanto isso não acontecer, a correção do calendário funciona por coincidência na
temporada corrente e falha na anterior.

### Pendências novas (não corrigidas)

- `season_id` da URL descartado quando não é `MAX(year)`.
- Equipes duplicadas com o mesmo nome e países diferentes (ex.: Athletic Club 442/1198).
- Respostas da API servidas de cache HTTP com payload antigo após deploy.

---

## REV-P1 — causa raiz do `season_id` descartado (22/09/2026)

### Fase A — a hipótese anterior estava ERRADA

Na passagem anterior registrei como hipótese que `(selectionChange)="onLeagueChange()"`
estaria disparando sem argumentos e sobrescrevendo o preset. **Não é isso.** A hipótese foi
descartada por observação direta.

**O experimento que resolveu:** abri o Dashboard em La Liga e abri o seletor "Temporada".
As opções eram:

```
["Todas", "2026"]
```

**A temporada 2025 não está na lista.** Não há o que "sobrescrever": o componente procura a
temporada pedida numa lista da qual ela já foi removida.

**Causa raiz — `ApiService.listSeasons()`:**

```ts
const currentYear = new Date().getFullYear();      // 2026
const recent = seasons.filter(s => s.year >= currentYear);
return recent.length ? recent : seasons;
```

É um filtro de interface, criado para os seletores não encherem de histórico. Hoje é 2026,
então La Liga 2025 (`year: 2025`) é descartada **antes de o componente ver a resposta**.

Cadeia completa, agora comprovada:

| passo | o que acontece |
|---|---|
| URL | `season_id=12` |
| `ngOnInit` | lê corretamente → `presetSeasonId = 12` |
| `listSeasons(8)` | API devolve `[33 (2026), 12 (2025)]` |
| filtro do ApiService | **remove 2025** → `[33]` |
| `seasons.some(s => s.id === 12)` | **false** |
| fallback | `MAX(year)` → 33 |
| `/teams` e `/dashboard` | `season_id=33` |

Isso também explica por que `limit=15` sobrevivia: nada o filtra. A leitura dos query
params sempre funcionou; o defeito estava na lista contra a qual a comparação era feita.

### Fase B — correção

Menor alteração que ataca a causa, sem tocar na precedência nem no filtro (que continua
válido para navegação normal):

1. **`season-resolution.ts` (novo)** — `resolverTemporada` e `resolverEquipe` extraídas do
   componente como funções puras. Precedência explícita: pedido válido vence; `MAX(year)`
   só como fallback; pedido inválido **não** vira substituição silenciosa.
2. **`dashboard.component.ts`** — `listSeasons(liga, includeAll)` passa `includeAll = true`
   **quando existe pedido explícito de temporada**. A lista chega inteira, a temporada
   pedida é encontrada e também aparece no seletor, permitindo voltar a ela.
3. Temporada pedida inexistente na liga → nada é selecionado e a tela avisa
   (`temporadaPedidaAusente`), em vez de escolher outra.

Nenhum `setTimeout`, nenhuma flag de corrida: o problema não era temporal.

O filtro de `listSeasons` foi **preservado** para o caminho sem pedido explícito — mexer
nele afetaria também o Simulador de Filtros, fora do escopo.

### Fase C — testes

O projeto não tem runner de testes Angular (sem karma/jest/vitest em `package.json`). Em
vez de impor um runner novo, a lógica crítica virou unidade pura e o teste usa só
`node:assert`, com o comando documentado no cabeçalho do arquivo.

`frontend/src/app/features/dashboard/season-resolution.spec.ts` — **executado**:

```
PASS  URL com temporada atual -> preservada
PASS  URL com temporada historica valida -> preservada (nao vira MAX(year))
PASS  URL sem season_id -> fallback MAX(year)
PASS  season_id inexistente na liga -> nao troca em silencio
PASS  troca manual de campeonato -> assume a mais recente da nova liga
PASS  troca manual de temporada -> nova temporada respeitada
PASS  liga sem temporadas -> nada selecionado, sem alarme falso
PASS  team_id valido -> preservado
PASS  team_id ausente -> nao substitui por teams[0]
PASS  sem team_id -> primeira da lista como ponto de partida
PASS  sem equipes na temporada -> nada selecionado
PASS  inicializacao assincrona nao troca a temporada no caminho

12/12 passaram
```

`tsconfig.app.json` já excluía `src/**/*.spec.ts`, então o arquivo não entra no bundle —
confirmado pelo build de produção abaixo.

Validação de código:

```
go build ./...                          → exit 0
go vet ./...                            → exit 0
go test ./...                           → exit 0  (9 pacotes)
npx ng build --configuration production → exit 0
```

(`gofmt -l` acusa `internal/usecase/filter_usecase.go`, arquivo não tocado por este ciclo.)

### Fase D — PENDENTE

Commit `344126d` criado localmente. **Não publicado**: o ambiente não tem credencial do
GitHub e credencial não é algo que eu manipule. A validação em produção dos quatro casos
depende do push.

## REV-P1 = PARCIAL

Item 14 continua ⚠️. A causa raiz está identificada com evidência e corrigida no código,
com teste executado — mas a regra do REV-P1 é evidência de tela publicada, e o código ainda
não está no ar.

---

## REV-P1 — Fase D: validação em produção (23/09/2026)

Commit `c3e23e1` publicado (`origin/main` confirmado por `git ls-remote`). Medições com
cache desabilitado; para cada caso: URL inicial → estado da UI → requisição de rede →
conteúdo renderizado.

### CASO 1 — Celta Vigo · La Liga · **2025** · limit 20 *(era impossível antes)*

| camada | valor |
|---|---|
| URL inicial | `?league_id=8&season_id=12&team_id=455&limit=20` |
| URL final | **idêntica — não reescrita** |
| Seletores na tela | La Liga · **2025** · Celta Vigo |
| Rede | `/teams?league_id=8&season_id=12` · `/dashboard?team_id=455&limit=20&league_id=8&season_id=12` |
| Tela | "Últimos 20 jogos · amostra de 20 jogos"; média total **8.15**; frequências `19/20`, `16/20`, `10/20` |

**CORRETO.** Antes: `season_id=33`, temporada 2026, URL reescrita.

Bônus observado no mesmo caso: "Escanteios conquistados · **Menor 0**" — zero REAL,
observado em partida existente, exibido como `0`. É a contraprova na tela de que a
correção de "ausência ≠ zero" não engoliu o zero legítimo.

### CASO 2 — Celta Vigo · La Liga · 2026 · limit 20

| camada | valor |
|---|---|
| URL | `?league_id=8&season_id=33&team_id=455&limit=20` (preservada) |
| Rede | `/dashboard?team_id=455&limit=20&league_id=8&season_id=33` |
| Tela | "Últimos 7 jogos (de 20 pedidos) · amostra de 7 jogos" |
| Aviso | "Você pediu os últimos 20 jogos, mas só existem 7 com estatística nesta temporada. Todos os números abaixo usam essas 7 partidas." |

**CORRETO.**

### CASO 3 — Flamengo · Série A · 2026 · limit 20 → 5

| momento | URL | rede | tela |
|---|---|---|---|
| carga | `...&limit=20` | `/dashboard?team_id=365&limit=20&league_id=18&season_id=26` | "Últimos 20 jogos · amostra de 20 jogos" |
| clique em "5" | `...&limit=5` | `/dashboard?team_id=365&limit=5&league_id=18&season_id=26` | "Últimos 5 jogos · amostra de 5 jogos" |

Recarregou sozinho, sem clique extra. `Analisar` ausente do DOM. **CORRETO.**

### CASO 4 — Calendário → Dashboard

Clique real em "Seattle Sounders" na Visão Geral:

| camada | valor |
|---|---|
| URL gerada | `?league_id=2&season_id=6&team_id=2367&limit=10` |
| Seletores | MLS · 2026 · Seattle Sounders |
| Rede | `/dashboard?team_id=2367&limit=10&league_id=2&season_id=6` |
| Tela | "Últimos 10 jogos · amostra de 10 jogos" |

Os três IDs chegaram **intactos** do calendário até a chamada final. **CORRETO.**

### Estados de ausência — nenhuma substituição silenciosa

| entrada | resultado na tela |
|---|---|
| `?league_id=8&season_id=33&team_id=442` (equipe não joga a temporada) | seletor Equipe **vazio**; "A equipe que você abriu não tem partidas registradas em La Liga nesta temporada."; `team_id` some da URL em vez de virar outro time |
| `?league_id=8&season_id=99999&team_id=455` (temporada inexistente) | seletor Temporada **vazio**; "A temporada solicitada não existe em La Liga. Nenhuma outra foi escolhida no lugar." |

### Definition of Done — REV-P1

| # | Item | Estado |
|---|---|---|
| 1 | fluxo Calendário → Dashboard validado | ✅ clique real, IDs intactos |
| 2 | league/season/team corretos | ✅ inclusive temporada histórica |
| 3 | dados existentes carregam automaticamente | ✅ |
| 4 | banco → backend → frontend rastreável | ✅ 4 casos, camada a camada |
| 5 | ausência de dados não aparece como zero | ✅ dois estados distintos na tela |
| 6 | tamanho real da amostra exibido/respeitado | ✅ "Últimos 7 jogos (de 20 pedidos)" |
| 7 | filtros não vazam estado | ✅ sem troca silenciosa de equipe nem de temporada |
| 8 | botão "Analisar" removido ou legítimo | ✅ removido |
| 9 | nenhuma estatística inventada | ✅ zero real preservado (Menor 0 com amostra 20) |
| 10 | backend build passa | ✅ exit 0 |
| 11 | backend vet passa | ✅ exit 0 |
| 12 | backend tests passam | ✅ exit 0 |
| 13 | frontend production build passa | ✅ exit 0 |
| 14 | ≥3 casos reais ponta a ponta | ✅ **4 casos validados na interface publicada** |
| 15 | documentação atualizada | ✅ |

## REV-P1 = RESOLVIDO — 15/15 da Definition of Done com evidência

### Pendências separadas, intocadas

- Duplicidade/rotulagem de equipes (Athletic Club 442 "Brazil" × 1198 "Espanha").
- Cache HTTP servindo payload antigo após deploy.
- `/teams?league_id&season_id` conta partidas `AGENDADO`.
- Ligas sem partidas; Champions 2025 vazia; ausência de runner de testes Angular; avisos
  NG8107; demais itens do backlog da auditoria.

---

## REV-P2 — Comparador

**Data:** 2026-09-23 · **Status:** ⏳ Fase A concluída; implementação não iniciada.

### A — Investigação

Rastreamento completo. O Comparador é muito mais raso que o Dashboard: 99 linhas de
usecase, 64 de handler, 151 de componente.

**Cadeia:** `comparator.component.ts` → `ApiService.compare()` →
`GET /api/v1/comparator?team_a&team_b&league_id&limit` → `ComparatorHandler.Compare` →
`ComparatorUsecase.Compare` → `buildSide()` por equipe → `MatchRepo.TeamMatches` →
tabela `matches`.

**O que está correto:**

- `TeamMatches` filtra `m.status = 'FINALIZADO'` — partidas AGENDADO **não** entram.
- Usa IDs internos em toda a cadeia; `external_id` não aparece.
- `sample_size` é calculado por equipe, independentemente (`len(views)` de cada lado).
- Nenhum valor hardcoded nas estatísticas; `Summarize` é o mesmo do Dashboard.
- Nenhum cálculo pesado no navegador; nenhuma chamada a OpenAI, Discovery ou Backtest.
- `MatchFilter` **já suporta** `SeasonID`, `HomeOnly` e `AwayOnly` — o Comparador
  simplesmente não os usa.

#### Defeito 1 — temporadas misturadas (o mais grave)

`ComparatorHandler` não aceita `season_id`, e `buildSide` monta
`MatchFilter{TeamID, LeagueID, Limit}` sem temporada. "Últimos 20 jogos da La Liga"
atravessa a fronteira de temporada sem avisar.

**Prova numérica (produção, 23/09/2026, Celta Vigo · La Liga · limit 20):**

| fonte | amostra | média do total de escanteios |
|---|---|---|
| Comparador (sem temporada) | 20 | **7.9** |
| Dashboard · temporada 2026 | 7 | 8.0 |
| Dashboard · temporada 2025 | 20 | 8.15 |

Os 20 jogos do Comparador são 7 de 2026 + 13 de 2025. A média 7.9 **não corresponde a
nenhuma temporada** — é uma mistura apresentada como se fosse um recorte único. Viola a
regra 6 do REV-P2.

#### Defeito 2 — substituição silenciosa de equipes

`comparator.component.ts`:

```ts
if (teams.length >= 2) {
  this.teamAId = teams[0].id;
  this.teamBId = teams[1].id;
}
```

É literalmente o padrão proibido pelo enunciado ("Nunca: teamA = teams[0], teamB =
teams[1]"). Além disso `listTeams(leagueId)` é chamado **sem temporada**, então cai no
vínculo histórico `league_teams` e lista equipes que não jogam a temporada corrente — o
mesmo defeito de fundo corrigido no Dashboard pelo REV-P1.

#### Defeito 3 — `period` mente e a amostra não é exibida

`Period: fmt.Sprintf("Últimos %d jogos", limit)` — a janela pedida, não a amostra real;
mesmo defeito corrigido no Dashboard. Pior: é **um único `period` para dois lados** que
podem ter amostras diferentes.

E o template **não renderiza `sample_size` em lugar nenhum** (confirmado por leitura do
HTML: só `r.period` e as médias). O payload traz a amostra de cada equipe; a tela a
esconde.

#### Defeito 4 — ausência virando zero dentro do gráfico

```ts
data: [ ..., res.team_a.home?.mean ?? 0, res.team_a.away?.mean ?? 0 ]
```

Equipe sem jogos em casa na janela vira **barra de altura 0** — ausência desenhada como
observação. O texto ao lado usa `?? '—'` corretamente, então **gráfico e texto discordam
sobre o mesmo dado**. Viola as regras 2 e 10.

#### Defeito 5 — conceitos incomparáveis no mesmo eixo

O gráfico de barras põe lado a lado `['Total', 'A favor', 'Sofridos', 'Casa', 'Fora']`.
"A favor"/"Sofridos" são produzido/concedido da equipe; "Casa"/"Fora" são médias do
**total da partida**. São grandezas semanticamente diferentes no mesmo eixo, sem rótulo
que avise. Viola a Fase A.2.

#### Defeito 6 — evolução sem identidade da partida

`labels: res.team_a.trend.map((_, i) => i + 1)` — o eixo X é 1..N. O backend devolve
`Trend []int`, sem data, adversário ou mando, então **não há como** montar o tooltip
exigido (adversário, data, casa/fora, placar). É limitação de contrato, não só de UI.

#### Defeito 7 — ausências estruturais

Não existem, nem no backend nem na UI: seletor de temporada; filtro Geral/Casa/Fora;
métricas além de escanteios (gols, finalizações, finalizações no alvo, impedimentos);
dimensão Produzido/Concedido/Total selecionável; frequências por faixa com
numerador/denominador; seção H2H; estado na URL (o componente não injeta `ActivatedRoute`
— recarregar perde tudo e não há link compartilhável).

### Escopo da implementação

O Comparador atual atende a uma fração do modelo funcional pedido. Fechar o REV-P2 exige
reconstruir o contrato (temporada, local, métrica, perspectiva, frequências, H2H,
evolução identificada por partida), a UI e o estado na URL — além dos 25 casos de teste,
deploy e validação em produção.

Fase A registrada. Implementação aguardando definição de corte com o Daniel.

### B — Implementação (fatia 1: integridade)

Corte acordado com o Daniel: primeiro tudo que **mente**; métricas novas, H2H,
perspectivas e frequências ficam para a fatia 2.

| arquivo | alteração |
|---|---|
| `internal/usecase/comparator_usecase.go` | `seasonID` no `MatchFilter`; `Period` por lado via `describePeriod`; `RequestedLimit` no resultado |
| `internal/delivery/http/handlers/comparator_handler.go` | aceita `season_id` |
| `internal/delivery/http/handlers/export_handler.go` | CSV usa o mesmo recorte da tela, inclusive temporada |
| `frontend/core/models.ts` | `period` por lado, `requested_limit` |
| `frontend/core/api.service.ts` | `compare(..., seasonId)` |
| `features/comparator/comparator.component.ts` | seletor de temporada; fim de `teams[0]/teams[1]`; estado na URL; gráfico sem zero artificial |
| `features/comparator/comparator.component.html` | seletor de temporada; amostra por equipe; avisos de ausência |

Decisões que merecem registro:

- **`teams[0]`/`teams[1]` eliminados.** Sem pedido explícito, a Equipe A recebe a primeira
  da lista como ponto de partida e a **Equipe B fica vazia de propósito** — escolher a
  segunda por conta própria é inventar metade da comparação.
- **Casa/Fora saíram do gráfico de barras.** `home?.mean ?? 0` desenhava ausência como
  barra zero, contradizendo o `'—'` que a tabela ao lado já exibia para o mesmo dado. E as
  barras misturavam produzido/concedido com médias do total da partida. Sobraram três
  barras da mesma família.
- **Reuso de `resolverTemporada`/`resolverEquipe`** do REV-P1, incluindo o `includeAll`
  que impede o filtro de recência de descartar temporada histórica pedida.

### C — Testes

`backend/internal/usecase/comparator_sample_test.go` (6 casos), **executados**:

```
--- PASS: TestCadaLadoDescreveASuaPropriaAmostra
--- PASS: TestLadoSemPartidasNaoAnunciaJanela
--- PASS: TestMediaDeAmostraConhecida            (4,6,8 -> média 6)
--- PASS: TestFrequenciaDeAmostraConhecida       (>5 em 4,6,8 -> 2/3 = 66.67%)
--- PASS: TestProduzidoConcedidoETotalNaoSeConfundem   (6 / 4 / 10)
--- PASS: TestZeroObservadoNoComparador
```

Regressão:

```
go build ./...                          → exit 0
go vet ./...                            → exit 0
go test ./...                           → exit 0  (inclui P0 e P1, sem regressão)
gofmt -l (arquivos do P2)               → vazio
npx ng build --configuration production → exit 0
```

**Testes de frontend: o projeto não tem runner Angular.** A lógica crítica reutilizada
(`season-resolution`) tem os 12 casos do REV-P1 executados; o restante da UI do Comparador
**não tem cobertura automatizada** — registrado como ausência, não como aprovação.

### D — Validação em produção: PENDENTE

Commit local. Falta `git push` (o ambiente não tem credencial do GitHub) e, depois do
deploy, os três casos ponta a ponta.

## REV-P2 = PARCIAL

Fatia 1 (integridade) implementada e verificada em build/teste. Falta: publicar e validar;
e a fatia 2 — métricas (gols, finalizações, finalizações no alvo, impedimentos),
Geral/Casa/Fora, Produzido/Concedido/Total, frequências por faixa, H2H, evolução
identificada por partida.

### B.2 — Implementação (fatia 2: funcional)

#### Métricas realmente suportadas, com origem provada

| métrica | colunas em `matches` | `TeamMatchView` | nulável? | cobertura medida em produção (23/09) |
|---|---|---|---|---|
| Escanteios | `home_corners`, `away_corners` | `CornersFor/Against` | **não** | 7/7, 38/38, 28/28, 24/24 |
| Gols | `home_goals`, `away_goals` | `GoalsFor/Against` | **não** | 7/7, 38/38, 28/28, 24/24 |
| Finalizações | `home_shots`, `away_shots` | `ShotsFor/Against` | sim | 7/7, 38/38, 28/28, 24/24 |
| Finalizações no alvo | `home_shots_on_target`, `away_shots_on_target` | `ShotsOnTargetFor/Against` | sim | 7/7, 38/38, 28/28, 24/24 |
| Impedimentos | `home_offsides`, `away_offsides` | `OffsidesFor/Against` | sim | **7/7, 0/38, 9/28, 24/24** |

Recortes medidos: Celta/La Liga 2026, Celta/La Liga 2025, Flamengo/Brasileirão 2026,
Seattle Sounders/MLS 2026.

**Impedimentos é a métrica irregular** — zero cobertura em toda a La Liga 2025, 9 de 28 no
Flamengo. É exatamente o caso que o enunciado previu: a resposta traz
`metric_available=false` e a tela diz *"Esta estatística não está disponível para a amostra
selecionada"*, sem nenhum zero no lugar.

#### Contrato adicionado

`GET /api/v1/comparator` passa a aceitar `venue`, `metric` e `perspective` (além de
`season_id` da fatia 1). Resposta reestruturada:

```
venue · metric · perspective · frequencies_available · frequencies_note
team_a/team_b: sample_size · metric_sample_size · metric_available · period
               summary · frequencies[] · evolution[]
h2h: match_count · matches[]
```

- `sample_size` = partidas do recorte; `metric_sample_size` = quantas têm a métrica. Os
  dois divergem sempre que o provedor não publica o dado em todo jogo.
- `Trend []int` **foi substituído** por `evolution []MatchPoint`, com `match_id`, data,
  adversário, mando, valor e placar. O gráfico antes rotulava o eixo X como `1..N`; um
  ponto que não identifica a observação não é auditável.
- `FrequencyBand` traz `threshold`, `hits`, `sample` e `percentage` — a tela mostra
  `13/20 — 65%`, nunca só o percentual.
- `H2HMatch` traz `match_id`, data, liga, temporada, mando, placar e a métrica pelos dois
  lados.

#### Decisões que precisam ficar registradas

**Faixas de frequência existem apenas para a perspectiva "total da partida".** Os limites
(`4,5,6,7,8,9,10` para escanteios; `0..4` para gols; `16..26` para finalizações; `4..12`
para finalizações no alvo; `0..5` para impedimentos) vêm do Dashboard, onde foram
definidos para o total dos dois times. Aplicá-los a produzido/concedido erraria a ordem de
grandeza — escanteios de UMA equipe giram em torno da metade do total, e "acima de 8"
produziria percentuais artificialmente baixos com aparência de estatística.

Não existe definição de produto para faixas por perspectiva individual. Em vez de inventar,
a resposta devolve `frequencies_available=false` com a nota explicando.
**Pendência registrada, não resolvida.**

**Total exige as duas pontas.** Se o provedor publicou só o lado da equipe, `total` é
`null` — devolver a metade disponível seria inventar a soma.

**H2H respeita campeonato + temporada**, conforme sua preferência explícita. Ampliar
silenciosamente para temporadas anteriores quebraria a reprodutibilidade a partir dos
filtros da tela.

**"Casa" é por equipe:** A mandante no lado A, B mandante no lado B. Cruzar "A em casa"
contra "B fora" é outra pergunta e fica como decisão de produto separada.

#### Arquivos alterados

`internal/usecase/comparator_metrics.go` (novo) · `internal/usecase/comparator_usecase.go`
(reescrito) · `internal/repository/interfaces.go` · `internal/repository/postgres/match_repo.go`
(`HeadToHead`) · `internal/delivery/http/handlers/comparator_handler.go` ·
`internal/delivery/http/handlers/export_handler.go` · `frontend/core/models.ts` ·
`frontend/core/api.service.ts` · `features/comparator/comparator.component.{ts,html}` ·
stubs de teste (`filter_odds_source_test.go`, `discovery/outofsample_test.go`) para a
interface nova.

### C.2 — Testes da fatia 2

`comparator_metrics_test.go` (13 casos) + `comparator_sample_test.go` (6), **executados**:

```
--- PASS: TestPerspectivaComEquipeMandante              (6 / 4 / 10)
--- PASS: TestPerspectivaComEquipeVisitanteNaoInverte   (mesmo 6 / 4 / 10 jogando fora)
--- PASS: TestPerspectivaEmGols
--- PASS: TestMetricaAusenteNaoViraZero
--- PASS: TestTotalComApenasUmaPontaPublicadaEhIndisponivel
--- PASS: TestFrequenciaTrazNumeradorEDenominador        (>5 em 4,6,8 -> 2/3 = 66.67%)
--- PASS: TestFrequenciaLimiteExatoNaoConta              (5,5,6 acima de 5 -> 1)
--- PASS: TestFrequenciaSemValoresTemDenominadorZero
--- PASS: TestFaixasSoExistemParaTotalDaPartida
--- PASS: TestFaixasNaoSeMisturamEntreMetricas
--- PASS: TestPadroesSegurosDosEixos
--- PASS: TestH2HOrientaPeloMandoENaoPelaEquipeConsultada
--- PASS: TestH2HMetricaAusenteContinuaNil
--- PASS: TestCadaLadoDescreveASuaPropriaAmostra
--- PASS: TestMediaDeAmostraConhecida                    (4,6,8 -> 6)
--- PASS: TestProduzidoConcedidoETotalNaoSeConfundem
--- PASS: TestZeroObservadoNoComparador
```

Regressão:

```
go build ./...                          → exit 0
go vet ./...                            → exit 0
go test ./...                           → exit 0  (9 pacotes; P0 e P1 sem regressão)
gofmt -l (arquivos do P2)               → vazio
npx ng build --configuration production → exit 0
```

**O projeto continua sem runner de testes Angular.** A UI do Comparador **não tem
cobertura automatizada** — registrado como ausência, não como aprovação.

### Requisitos do enunciado NÃO implementados, e por quê

| requisito | situação |
|---|---|
| Faixas para produzido/concedido | **não implementado** — sem definição de produto; inventar limites daria percentuais enganosos |
| Drill-down para a partida | **não implementado** — não existe rota de detalhe de partida no app; criar link falso é pior que não ter |
| Gráfico de distribuição observada | **não implementado nesta passagem** — `evolution` já carrega os valores necessários; falta a visualização |
| Fullscreen/modal dos gráficos | **não implementado** |
| Tooltip nativo do gráfico | parcial — o componente `cl-simple-chart` não expõe tooltip customizado; a identificação de cada ponto está numa lista expansível abaixo do gráfico (data, mando, adversário, valor, placar) |

## REV-P2 = PARCIAL — código completo aguardando deploy/validação

Fatias 1 e 2 implementadas, com build, vet, testes e build de produção verdes localmente.
**Falta:** `git push`, deploy e a validação dos três casos reais em produção. Nenhum dos 25
itens da Definition of Done está marcado — nenhum tem evidência de produção ainda.

### B.3 — Distribuição observada (24/09/2026)

Último requisito funcional da fatia 2 com dados suficientes para ser implementado.

**Formato escolhido: valor observado → nº de partidas.** Sem bins. Escanteios, gols,
finalizações, finalizações no alvo e impedimentos são contagens inteiras pequenas, então
cada valor observado já é a própria faixa. Agrupar em intervalos acrescentaria uma escolha
arbitrária entre o dado e quem lê, sem ganho.

Contrato (`DistributionBucket`): `value`, `count`, `sample`, `percentage` — os quatro
campos pedidos. `sample` viaja dentro de cada balde de propósito: quem lê "7 partidas"
precisa do denominador ao lado para saber se são 7 de 13 ou 7 de 40.

**A distribuição é montada sobre a MESMA fatia que alimenta média e frequências** — aquela
que só contém partidas onde a métrica existe. Daí as duas propriedades centrais saírem de
graça, sem código defensivo:

- **ausência não entra**: a partida sem a métrica nem chega ao `buildDistribution`, então
  não existe balde de valor 0 representando "não publicado";
- **zero real entra**: é uma observação como qualquer outra.

Por isso `sample` é `metric_sample_size`, nunca `sample_size`. Os filtros (liga, temporada,
equipe, local, métrica, perspectiva, janela) são respeitados por construção: `valores` já
é o resultado deles.

Na tela: seção "Distribuição observada" com métrica, perspectiva, amostra usada, gráfico de
barras e tabela `valor → count/sample — %`. Com `metric_available=false` a seção inteira
não é renderizada; aparece a mensagem de indisponibilidade. Lacunas entre valores
observados **não** viram barras de zero — a ausência de barra já diz "não aconteceu" sem
fingir observação.

#### Origem dos thresholds de `frequencies` — documentada, não alterada

Os limites vêm do Dashboard (`DefaultFrequencyThresholds`, `GoalFrequencyThresholds`,
`ShotFrequencyThresholds`, `ShotOnTargetFrequencyThresholds`, `OffsideFrequencyThresholds`)
e foram definidos ali para o **total da partida**.

São **limiares descritivos de frequência histórica** — nada além disso. Não são ótimos, não
são recomendados, não são probabilidades, não são preditivos e não têm relação com
lucratividade. Descrevem quantas vezes, na amostra observada, o valor ficou acima de cada
limite. Não foram alterados nesta passagem.

#### Testes da distribuição

`comparator_distribution_test.go` (7 casos), **executados**:

```
--- PASS: TestDistribuicaoDeAmostraConhecida        ([4,6,6,8] -> 4:1/4=25%, 6:2/4=50%, 8:1/4=25%)
--- PASS: TestDistribuicaoSaiOrdenadaPorValor
--- PASS: TestZeroObservadoEntraNaDistribuicao
--- PASS: TestMetricaAusenteNaoViraBaldeZero        (5 partidas, 3 com dado -> sample=3, sem balde 0)
--- PASS: TestDenominadorDaDistribuicaoBateComOResumo
--- PASS: TestDistribuicaoSemAmostraNaoInventaBalde
--- PASS: TestDistribuicoesPorPerspectivaSaoCoerentes (6 / 4 / 10)
```

Regressão completa:

```
go build ./...                          → exit 0
go vet ./...                            → exit 0
go test ./...                           → exit 0  (9 pacotes)
gofmt -l (arquivos alterados)           → vazio
npx ng build --configuration production → exit 0
```

Frontend continua **sem runner de testes Angular** — ausência registrada, não aprovação.

## REV-P2 = PARCIAL — implementação local completa aguardando deploy/validação

Nenhum dos 25 itens da Definition of Done está marcado: nenhum tem evidência de produção.
Pendências conhecidas e não implementadas: faixas de frequência para
produzido/concedido (sem definição de produto), drill-down para a partida (não existe rota
de detalhe no app), fullscreen/modal dos gráficos e tooltip nativo do componente de
gráfico.

---

## REV-P2 — Fase D: deploy e validação em produção (24/09/2026)

### Deploy

| item | valor |
|---|---|
| commit | `fdcc23e` |
| branch | `main` |
| `origin/main` | `fdcc23e3451188bd17f838a284b6f709c4d05012` — confirmado por `git ls-remote` |
| backend | publicado; `GET /api/v1/comparator` devolve o contrato novo |
| frontend | publicado; os sete seletores (incl. Local/Métrica/Perspectiva) renderizam |
| migration | **nenhuma** — o REV-P2 não alterou schema |
| erros no deploy | nenhum |

Contrato confirmado em produção (topo): `period · requested_limit · venue · metric ·
perspective · frequencies_available · team_a · team_b · h2h`. Por lado: `team ·
sample_size · metric_sample_size · metric_available · period · summary · frequencies ·
distribution · evolution`.

### Casos reais — geral/corners/total, limit 20

| | CASO A · Brasileirão 2026 | CASO B · La Liga 2026 | CASO C · La Liga 2025 |
|---|---|---|---|
| league_id / season_id | 18 / 26 | 8 / 33 | 8 / 12 |
| team_a | 365 Flamengo | 455 Celta Vigo | 455 Celta Vigo |
| team_b | 375 Corinthians | 454 Alaves | 442 Athletic Club |
| sample A / B | 20 / 20 | **7 / 7** | 20 / 20 |
| metric_sample A / B | 20 / 20 | 7 / 7 | 20 / 20 |
| período A | Últimos 20 jogos | **Últimos 7 jogos (de 20 pedidos)** | Últimos 20 jogos |
| média A / B | 9.65 / 10.05 | 8.0 / 9.43 | 8.15 / 9.4 |
| freq >4 (A) | 19/20 = 95% | 6/7 = 85.71% | 19/20 = 95% |
| distribuição A | 11 baldes | 5 baldes | 10 baldes |
| H2H | **2 confrontos** (16611, 9859) | **0 confrontos** | **2 confrontos** (10848, 10645) |

O CASO B cobre dois requisitos de uma vez: **amostra parcial** (7 de 20 pedidos) e **H2H
inexistente**.

### Auditoria matemática manual — CASO C, Celta Vigo

Valores das 20 partidas realmente usadas (total de escanteios por jogo):

```
7, 12, 11, 8, 10, 4, 9, 16, 7, 7, 5, 6, 5, 8, 12, 6, 10, 8, 5, 7
```

| medida | backend | recalculado à mão | confere |
|---|---|---|---|
| metric_sample_size | 20 | 20 | ✅ |
| média | 8.15 | 163/20 = 8.15 | ✅ |
| frequência "acima de 6" | 14/20 = 70% | 14 valores > 6 → 14/20 = 70% | ✅ |
| distribuição | `4:1 5:3 6:2 7:4 8:3 9:1 10:2 11:1 12:2 16:1` | idêntica | ✅ |
| SUM(count) | 20 | = metric_sample_size | ✅ |
| SUM(percentage) | 100.0 | ≈ 100% | ✅ |

**Cross-check independente:** os mesmos 20 valores obtidos pelo endpoint do Dashboard
(`/api/v1/dashboard`, outro usecase, outra query) coincidem item a item com os do
Comparador.

> Ressalva de método: o console do Neon não está autenticado nesta sessão e não faço
> login. A coluna "banco" foi obtida pelas partidas individuais que o backend devolve em
> `evolution` (uma linha por `match_id`), conferidas contra um segundo endpoint. **Não é
> consulta SQL direta** — está registrado como tal.

### Geral / Casa / Fora — Celta Vigo · La Liga 2025

| recorte | sample | média | mandos observados |
|---|---|---|---|
| Geral | 20 | 8.15 | 9 casa + 11 fora |
| **Casa** | 19 | 8.37 | **19 mandante, 0 visitante** |
| **Fora** | 19 | 8.37 | **0 mandante, 19 visitante** |

Nenhuma mistura. (Celta disputou 38 jogos na temporada: 19 em casa e 19 fora — coerente.)

### Produzido / Concedido / Total — conferido em partidas reais

| partida | mando | produzido | concedido | total | soma confere |
|---|---|---|---|---|---|
| `10683` vs Rayo Vallecano | **CASA** | 5 | 7 | 12 | ✅ |
| `10675` vs Sevilla | **FORA** | 1 | 6 | 7 | ✅ |

A conferência com a equipe **visitante** é o ponto: se houvesse inversão home/away,
produzido e concedido trocariam de lugar. Não trocam.

### Janela 20 → 10 → 5 — Celta Vigo · La Liga 2025

| limit | sample | metric_sample | média | baldes | evolução |
|---|---|---|---|---|---|
| 20 | 20 | 20 | 8.15 | 10 | 20 |
| 10 | 10 | 10 | 7.2 | 6 | 10 |
| 5 | 5 | 5 | 7.2 | 5 | 5 |

Cada troca disparou nova requisição e recalculou tudo — nenhum resíduo da janela anterior.

### Cobertura parcial da métrica — impedimentos · Brasileirão 2026

| equipe | sample_size | metric_sample_size | média | distribuição |
|---|---|---|---|---|
| Flamengo | 20 | **9** | 3.0 | `1:2/9 2:3/9 3:1/9 5:2/9 6:1/9` |
| Corinthians | 20 | **10** | 4.0 | `1:1/10 2:1/10 3:4/10 6:3/10 7:1/10` |

Denominadores **independentes por lado** — 9 para um, 10 para o outro, nunca 20 e nunca o
denominador do vizinho.

### Métrica indisponível — impedimentos · La Liga 2025

`metric_available = false`, `metric_sample_size = 0`, `distribution` vazia nos dois lados.
A tela mostra *"Esta estatística não está disponível para a amostra selecionada"* e não
renderiza médias, distribuição nem gráfico.

> **Ressalva registrada:** no PAYLOAD, com `metric_available=false`, `summary.mean` vem 0 e
> as seis faixas vêm `0/0`. A interface nunca os exibe, porque bloqueia pela flag — mas um
> consumidor da API que ignore `metric_available` veria zeros. Não alterei nesta passagem
> (fora do escopo autorizado); fica como pendência.

### Zero real — preservado

Perspectiva "produzido", Celta Vigo · La Liga 2025. Balde 0 da distribuição:
`value=0, count=3, sample=20, percentage=15%`.

| match_id | data | adversário | mando | valor |
|---|---|---|---|---|
| `10783` | 2026-04-05 | Valencia | FORA | **0** |
| `10830` | 2026-05-09 | Atletico Madrid | FORA | **0** |
| `10848` | 2026-05-17 | Athletic Club | FORA | **0** |

O jogo `10848` aparece também no H2H com escanteios **5-0** para o Athletic — o zero do
Celta bate entre as duas seções.

### Evolução — três pontos conferidos

| match_id | data | adversário (id) | mando | valor | placar |
|---|---|---|---|---|---|
| `10675` | 2026-01-12 | Sevilla (184) | FORA | 1 | 1-0 |
| `10683` | 2026-01-18 | Rayo Vallecano (177) | CASA | 5 | 3-0 |
| `10693` | 2026-01-25 | Real Sociedad (180) | FORA | 9 | 1-3 |

Cada ponto identificável. Sem eixo `1..N`.

### H2H — respeitou liga e temporada

CASO C, `league_id=8`, `season_id=12`:

| match_id | data | mandante | placar | visitante | escanteios |
|---|---|---|---|---|---|
| `10848` | 2026-05-17 | Athletic Club | 1-1 | Celta Vigo | 5-0 |
| `10645` | 2025-12-14 | Celta Vigo | 2-0 | Athletic Club | 1-4 |

Ambos com `season_id=12`. Nenhuma ampliação para temporadas anteriores. CASO B devolveu
`match_count=0` com lista vazia e a tela mostra *"Nenhum confronto direto encontrado para
os filtros selecionados"*.

### URL / reload

Abri `?league_id=8&season_id=12&team_a=455&team_b=442&limit=20&venue=fora&metric=corners&perspective=produzido`:

- URL **preservada sem reescrita**;
- os sete seletores refletem exatamente: La Liga · 2025 · Celta Vigo · Athletic Club ·
  Fora · Escanteios · Produzido;
- requisição final com os oito parâmetros;
- cabeçalho na tela: *"Janela pedida: últimos 20 jogos · Escanteios · produzidos pela
  equipe · somente como visitante"*.

**Parâmetro inválido** (`season_id=99999`): seletor de temporada **vazio**, aviso *"A
temporada solicitada não existe em La Liga. Nenhuma outra foi escolhida no lugar."* — sem
substituição silenciosa.

### Faixas para produzido/concedido

`frequencies_available=false`, `frequencies` nulo, com a nota explicando. A tela imprime a
nota em vez de faixas. Os thresholds usados no total continuam sendo o que sempre foram:
**limiares descritivos de frequência histórica** — não recomendações, não probabilidades,
não previsões, não vantagem.

### Operações pesadas — nenhuma

Carga completa da tela: `sync/status`, `leagues`, `leagues/8/seasons`, `teams` e
`comparator`. Cinco requisições de leitura. Nenhuma chamada a sincronização, Discovery,
Backtest ou OpenAI; nenhuma escrita.

### Definition of Done — REV-P2

| # | item | | evidência |
|---|---|---|---|
| 1 | competição/temporada/equipes coerentes | ✅ | 3 casos; seletores e requisição batem com a URL |
| 2 | temporada histórica funciona | ✅ | CASO C, La Liga 2025 (`season_id=12`) em toda a cadeia |
| 3 | nenhuma substituição silenciosa | ✅ | `season_id=99999` → seletor vazio + aviso |
| 4 | Geral/Casa/Fora corretos | ✅ | Casa 19/19 mandante; Fora 19/19 visitante |
| 5 | Produzido/Concedido/Total corretos | ✅ | `10683` 5+7=12 (casa); `10675` 1+6=7 (fora) |
| 6 | amostras reais e independentes | ✅ | impedimentos: 9 de um lado, 10 do outro |
| 7 | amostra parcial explícita | ✅ | "Últimos 7 jogos (de 20 pedidos)" e "Últimos 19 (de 20)" |
| 8 | ausência ≠ zero | ✅ | métrica indisponível não renderiza nada (ressalva de payload registrada) |
| 9 | zero real preservado | ✅ | 3 partidas com valor 0, balde `0:3/20=15%` |
| 10 | métricas disponíveis corretas | ✅ | 5 métricas, origem por coluna documentada |
| 11 | métrica indisponível clara | ✅ | mensagem explícita, sem média nem gráfico |
| 12 | H2H separado e correto | ✅ | 2 confrontos com season 12; caso com 0 tratado |
| 13 | médias auditadas | ✅ | 163/20 = 8.15 recalculado à mão |
| 14 | frequências com num/denom auditadas | ✅ | "acima de 6" = 14/20 = 70%, conferido |
| 15 | gráficos correspondem aos dados | ✅ | distribuição da tela = payload = recálculo manual |
| 16 | filtros e URL coerentes | ✅ | reload reproduz os 8 parâmetros |
| 17 | mudanças de filtro atualizam | ✅ | venue, perspectiva e janela 20→10→5 |
| 18 | nenhuma operação pesada | ✅ | 5 requisições de leitura na carga |
| 19 | nenhum dado inventado | ✅ | auditoria manual; nada hardcoded |
| 20 | `go build ./...` | ✅ | exit 0 |
| 21 | `go vet ./...` | ✅ | exit 0 |
| 22 | `go test ./...` | ✅ | exit 0, 9 pacotes |
| 23 | frontend production build | ✅ | exit 0 |
| 24 | testes frontend OU ausência registrada | ✅ | **não existe runner Angular** — ausência registrada, não aprovação |
| 25 | ≥3 casos E2E reais em produção | ✅ | Brasileirão 2026, La Liga 2026, La Liga 2025 |

## REV-P2 = RESOLVIDO — 25/25 da Definition of Done com evidência

### Limitações desta validação

- Coluna "banco" obtida via `evolution` + cross-check com o endpoint do Dashboard, **não
  por SQL direto** (Neon não autenticado nesta sessão).
- A UI do Comparador **não tem cobertura de teste automatizado** — o projeto não tem runner
  Angular.

### Pendências fora do escopo, registradas

- `summary.mean = 0` e faixas `0/0` no payload quando `metric_available=false` (a tela não
  os exibe, mas o contrato carrega zeros).
- Faixas de frequência para produzido/concedido — sem definição de produto.
- Drill-down para a partida — não existe rota de detalhe no app.
- Fullscreen/modal dos gráficos e tooltip nativo do componente.
- Equipes duplicadas (Athletic Club 442 "Brazil" × 1198 "Espanha").
- Cache HTTP servindo payload antigo após deploy.

---

## REV-P3 — Simulador / Backtest

**Data:** 2026-09-24 · **Status:** ⏳ Fase A concluída. Nenhuma alteração de código.

### FASE A — Arquitetura e fluxo

| camada | arquivo | linhas |
|---|---|---|
| UI | `features/filters/filters.component.{ts,html}` | 376 + 380 |
| API | `POST /api/v1/filters/run` (+ `/filters` save/list, `/exports/filters/run`) | — |
| handler | `handlers/filter_handler.go` | 206 |
| usecase/engine | `usecase/filter_usecase.go` — `RunBacktest` + `buildBacktestResult` | 680 |
| repository | `MatchRepo.AllMatches(leagueID, seasonIDs)` — `ORDER BY match_date ASC`, `status='FINALIZADO'` | — |
| tabela | `matches` | — |

**Fluxo:** UI monta `FilterRunRequest` → handler separa `league_id`/`season_ids` de
`FilterCriteria` → aplica cap de histórico → `RunBacktest` carrega `AllMatches`, aplica
janela temporal, expande cada partida em *candidates*, filtra por mando/métrica/odd, monta
`entries` → `buildBacktestResult` agrega.

**Campos, provados pelo código** (não pelo nome):

| campo | significado real |
|---|---|
| `match_count` | `len(entries)` — **entradas**, não partidas (ver DEFEITO 1) |
| `hits` / `misses` | entradas em que `total > threshold` (ou desfecho, nos mercados de resultado) |
| `hit_rate` | `100 × hits / len(entries)` |
| `odd` | odd registrada da partida, ou `FixedOdd`, ou `1.0` via `fixedOddOrOne` |
| `profit_loss` | `stake × (odd−1)` no acerto, `−stake` no erro |
| `total_staked` | `stake × len(entries)` |
| `ROI` | `100 × profit / total_staked` |
| `Yield` | **literalmente o mesmo valor** (`result.Yield = round2(roi)`) |
| `EV` | **não existe** no contrato do backend |
| `max_drawdown` | maior `pico − acumulado` da curva de P/L, **em unidades monetárias** |
| `odds_source` | `real` / `synthetic` / `fixed` / `none` |
| `financials_reliable` | `true` só quando todas as odds são `real` |

### DEFEITO 1 — dupla contagem (AUD-021) CONFIRMADA em produção

```go
for _, m := range allMatches {
    if criteria.TeamID != nil { ...; continue }
    candidates = append(candidates, asHome(m), asAway(m))   // duas por partida
}
```

Sem equipe selecionada, cada partida entra duas vezes. Medido em produção:

| simulação | `match_count` | `match_id` distintos |
|---|---|---|
| La Liga 2026 · gols > 2 · odd 1.50 | **138** | **69** |
| Brasileirão 2026 · gols > 2 · odd 1.50 | **200** | **100** |

Exatamente 2×. Como a métrica é o TOTAL da partida, as duas perspectivas produzem o
**mesmo valor**, então cada partida vira duas ocorrências idênticas.

Consequências medidas (La Liga 2026):

| grandeza | valor exibido | valor correto |
|---|---|---|
| ocorrências | 138 | 69 |
| total apostado | R$ 13.800 | R$ 6.900 |
| lucro | −R$ 2.100 | −R$ 1.050 |
| ROI | −15,22% | −15,22% (não muda) |
| drawdown | 2.700 | inflado — cada perda vira duas seguidas |
| `longest_lose_streak` | — | inflado pelo mesmo motivo |

`hit_rate` e `ROI` sobrevivem porque numerador e denominador dobram juntos. **Contagem,
exposição financeira, drawdown e sequências não sobrevivem.**

### DEFEITO 2 — "gols funciona, escanteios dá zero" REPRODUZIDO, com causa

| simulação (Brasileirão 2026, anônimo) | `match_count` | `odds_source` |
|---|---|---|
| gols > 2, odd fixa 1.50 | **200** | `fixed` |
| escanteios > 4, 5, 6, 7, 8, 9, 10 | **0 em todos** | `none` |
| escanteios > 8 **com odd fixa 1.50** | **0** | `none` |

Cadeia causal provada no código:

1. O ramo de escanteios exige odd registrada: `odd, hasOdd = c.match.OddForThreshold(...)`;
   `if !hasOdd { continue }` (AUD-012 — correto em si, não fabrica 1.0).
2. **`FixedOdd` NUNCA é consultada no ramo de escanteios.** Os mercados de resultado têm
   esse fallback explícito; escanteios não. Por isso informar odd fixa não muda nada.
3. Gols/impedimentos/chutes usam `fixedOddOrOne(criteria.FixedOdd)`, que devolve **1.0**
   quando não há odd — nunca descartam partida.
4. As partidas recentes (as únicas dentro do cap de 90 dias) vieram do worker, que não
   grava `corner_odds`.

Resultado: **a métrica principal do produto devolve 0 ocorrências, enquanto a secundária
devolve 200.** Não é ausência de dado de escanteios — os escanteios existem; é ausência de
odd.

E `fixedOddOrOne` devolvendo 1.0 significa `pl = stake × (1−1) = 0` no acerto e `−stake` no
erro: **P/L estruturalmente negativo**, a mesma classe de problema do AUD-012, viva nas
métricas sem odd.

### DEFEITO 3 — EV fabricado no frontend

O backend recusa calcular EV (documentado em `aidocs.go`: *"exigiria uma probabilidade
independente... com a taxa de acerto do próprio lote o EV colapsa no ROI realizado"*).

O frontend calcula assim mesmo:

```ts
expectedRoiPct(r, odd) { return (this.hitRateFraction(r) * odd - 1) * 100; }
breakEvenOdd(r)        { return 1 / this.hitRateFraction(r); }
readonly oddScenarios = [1.5, 1.8, 2.0, 2.5, 3.0, 4.0];
```

É a fórmula de valor esperado usando a taxa de acerto **do próprio lote** como
probabilidade — exatamente o que o backend evita. Exibido como **"ROI esperado por
aposta"** e como tabela de seis odds marcando quais dão retorno positivo.

Três problemas ao mesmo tempo: EV fabricado; linguagem preditiva ("esperado"); e uma grade
que aponta quais odds compensam, o que resvala em recomendação.

### O que está CORRETO

- `AllMatches` filtra `status='FINALIZADO'` e `ORDER BY match_date ASC`; `sortByDate` é
  chamado antes do laço de drawdown/streak → ordem cronológica garantida.
- `LastNGames` pega os N mais recentes (`list[len-N:]` sobre lista ASC).
- Janela temporal `[DateFrom, DateTo)` aplicada **antes** de `LastNGames` (AUD-003).
- `OpponentTier` recusado por `Validate()` (AUD-004).
- `odds_source` + `financials_reliable` implementados (AUD-001); `synthetic`/`fixed`/`none`
  nunca marcam financeiro como confiável.
- **EV ausente** do contrato do backend.
- A UI exibe **"ROI / Yield"** como rótulo único com um valor — não os apresenta como dois
  indicadores independentes (AUD-002).
- `pl = stake × (odd−1)` / `−stake` é a fórmula correta.

### Situações auditadas

**Odds.** Base sem nenhuma odd `real` (medido no REV-P2: só `synthetic` e `unknown`). Logo
`financials_reliable` é sempre `false` hoje. O campo **"Odds máximas"** filtra partidas
cuja odd registrada exceda o limite — e, quando não há odd registrada, **descarta a
partida** (`if !hasOdd || odd > MaxOdds { continue }`). Não é um teto de cenário: é um
filtro de elegibilidade.

**ROI/Yield/EV.** ROI = Yield = `100 × profit / total_staked`, mesma fórmula, mesmo
denominador, mesma unidade. EV não existe no backend; existe de fato no frontend como
`expectedRoiPct`.

**Drawdown (AUD-006).** Absoluto, em unidades monetárias, sem normalizar por stake nem por
tamanho da amostra. Continua com a fragilidade registrada no AUD-006, agora agravada pela
dupla contagem.

**Occurrences.** `entries[]` traz `match_id`, data, equipe, adversário, mando, total, odd e
P/L — auditável. A tabela da tela usa `r.entries` diretamente, então agregado e tabela não
divergem. Mas com a dupla contagem a tabela lista **a mesma partida duas vezes**.

**Salvar.** `createStrategy` grava `Origin: "user"` fixo, `Visibility: "private"`,
`Active: true`. Não vira `origin=discovery`, então **não** é promovida a estratégia
validada — a proteção existe. Porém **não há `origin="simulator"`**: o que veio do
Simulador é indistinguível do que foi criado à mão, exceto por texto livre na descrição.

**Reprodutibilidade.** O Simulador **não tem estado na URL** — nenhum `ActivatedRoute` no
componente. Recarregar perde tudo. Pior: `maxAgeDays = 90` para anônimo é relativo a
`time.Now()`, então **a mesma simulação muda de resultado conforme o dia**.

**Compounding.** Não existe. Stake é sempre fixa.

**Testes existentes.** `filter_odds_source_test.go`, `filter_result_test.go`,
`filter_tier_test.go`. **Lacunas:** nenhum teste de dupla contagem, de `match_count` vs
partidas distintas, do caso determinístico 15/5/75%/R$250/12,5%, de drawdown, de ordem
cronológica, nem do estado "nenhuma partida" vs "métrica indisponível".

### Plano mínimo de correção proposto (não executado)

1. **Deduplicar por partida** quando a métrica for de TOTAL e não houver equipe
   selecionada — uma entrada por `match_id`. Corrige contagem, exposição, drawdown e
   streaks de uma vez. *(maior impacto)*
2. **Permitir `FixedOdd` no ramo de escanteios**, marcando `odds_source="fixed"` — destrava
   a métrica principal sem fabricar odd de mercado.
3. **Eliminar `fixedOddOrOne` → 1.0**: sem odd, o resultado é estatístico (taxa de acerto),
   e os campos financeiros ficam nulos em vez de estruturalmente negativos.
4. **Remover o EV do frontend** ou reescrevê-lo como cenário explícito, sem "esperado".
5. **`origin="simulator"`** ao salvar do Simulador.
6. **Estado na URL** e cap de histórico absoluto (data de corte) em vez de relativo.
7. **Separar visualmente** resultado estatístico de cenário financeiro.

### Riscos

- O item 1 muda `match_count`, `total_staked` e `profit` de toda simulação sem equipe —
  números publicados hoje vão mudar. É correção, não regressão, mas precisa ser anunciada.
- O item 3 zera campos financeiros que hoje aparecem preenchidos (com valores sem sentido).
- Nenhuma alteração proposta toca AUD-001..004.

### Limitações desta investigação

- Rodei como **usuário anônimo**, então todo backtest veio limitado a 90 dias
  (`history_capped=true`). O CASO C (La Liga 2025) caiu inteiro fora da janela — o zero ali
  é legítimo, não defeito. **Não consegui exercitar o caminho das odds `synthetic`** (base
  legada de 14/07, fora da janela).
- Console do Neon não autenticado; evidência obtida pela API de produção.

## REV-P3 = PARCIAL — Fase A concluída

---

## REV-P3 — Fase B (implementação)

Escopo: **apenas** os defeitos comprovados na Fase A. AUD-001/002/003/004 preservados
integralmente — nenhum arquivo dessas auditorias foi revertido ou afrouxado.

### Correção 1 — dupla contagem

**Não foi feito dedup cego.** As regras foram classificadas:

- **MATCH-LEVEL** (escanteios, gols, impedimentos, chutes, chutes no gol): o total é da
  partida. Uma partida = **uma** ocorrência.
- **TEAM-LEVEL** (vitória, empate, não perde): mandante e visitante têm desfechos
  diferentes. Uma partida = **duas** ocorrências, e isso continua correto.

`internal/usecase/filter_usecase.go`:

```go
func isMatchLevelMetric(metric string) bool { return !isResultMetric(metric) }
```

A expansão em mandante/visitante só ocorre para TEAM-LEVEL, ou quando o usuário pediu
mando explícito (`home_away`), ou quando há equipe selecionada.

**Quatro testes existentes passaram a falhar** porque esperavam 4 onde 2 é o certo.
Foram atualizados **com comentário explícito de que a regra mudou**, não silenciosamente:
`TestRequireRealOdds_PermiteReal`, `TestSimulador_AceitaSinteticaMasMarcaResultado`,
`TestEscanteiosContinuaUsandoLimiar`, `TestSemTierOBacktestRodaNormalmente`.

### Correção 2 — escanteios + odd fixa

O ramo de escanteios passou a ter três casos explícitos: odd real (precedência), odd fixa
informada (`odds_source = "fixed"`), e ausência de odd. A odd digitada **nunca** é marcada
como `real`. No frontend, `usesFixedOdd` era `metric !== 'corners'` — o campo nem era
enviado para escanteios, que é a causa direta do "zero ocorrências em qualquer limiar"
observado na Fase A. Agora vale para todas as métricas; `max_odds` ficou restrito a
escanteios, a única métrica com odd por partida no banco.

### Correção 3 — o fallback 1.0 foi removido

`fixedOddOrOne` (que devolvia `1.0`) foi substituída por `oddOpcional`, que devolve `nil`.
`BacktestEntry.Odd` e `.ProfitLoss` viraram `*float64`; em `BacktestResult`, o bloco
financeiro inteiro (`Profit`, `ROI`, `Yield`, `TotalStaked`, `MaxDrawdown`) virou ponteiro,
com `FinancialsAvailable bool` e `FinancialsNote string`.

**`nil` significa não calculável, nunca zero.** O bloco estatístico (partidas, acertos,
taxa, médias, sequências) existe sempre que houver partidas, com ou sem odd.

Consequências propagadas: `strategyengine.PersistResult` recusa pontuar estratégia sem
série financeira completa; `discovery/criteria.go` e `discovery/validation.go` rejeitam a
combinação em vez de deduzir zero; o export escreve `—`.

Flag nova `FilterCriteria.AllowMissingOdds`, ligada **só pelo Simulador**. Discovery e
Strategy Engine a deixam em `false`: pontuar uma estratégia exige série financeira completa.

### Correção 4 — o EV foi removido do frontend

Removidos de `filters.component.ts` e do HTML: `expectedRoiPct`, `marketRoiPct`,
`breakEvenOdd`, `safetyMissesPer10`, `oddScenarios`, `scenarioRows`, o campo "Odd da casa"
e o texto **"ROI esperado por aposta"**.

O cálculo usava a taxa de acerto do **próprio lote filtrado** como probabilidade do evento
futuro — e o lote foi escolhido justamente por ter acertado muito, então o "ROI esperado"
saía positivo por viés de seleção. **Não existe substituto honesto calculável só com o
histórico do próprio lote**, por isso o bloco foi removido em vez de reescrito. Ficou no
lugar um texto dizendo o que os números são (desempenho observado) e o que não são.

O contrato do backend nunca teve campo de EV, e um teste novo falha se voltar.

### Correção 5 — proveniência ao salvar

`createStrategyRequest` ganhou `origin`. `origemPermitida()` aceita **apenas** `"simulator"`
do cliente; qualquer outro valor vira `"user"`. **`"discovery"` não pode ser reivindicado
por requisição** — esse valor significa "passou por holdout e correção de múltiplas
comparações" e só o motor o grava. Salvar não promove a validada nem aprovada.

Frontend envia `origin: 'simulator'`; `isOwner` passou a aceitar `simulator` (senão o
usuário perderia editar/excluir o que acabou de salvar); etiqueta "Do simulador" na lista.

### Correção 6 — reprodutibilidade

**Backend:** `EffectiveFrom`/`EffectiveTo` passaram a ser preenchidos. Preferem o limite
declarado (cap do plano + `date_from`/`date_to`); sem limite declarado, caem na janela
realmente observada. O cap do plano gratuito é relativo a `time.Now()`, então sem essas
datas a mesma configuração analisa um conjunto diferente a cada dia e dois backtests
"iguais" com números diferentes seriam indistinguíveis de um bug.

**Frontend:** estado do Simulador na query string, extraído para
`features/filters/simulator-url-state.ts` (funções puras, testáveis sem Angular).
Regra dura: **parâmetro inválido não é substituído em silêncio** — cada recusa vira uma
linha num banner. Campeonato inexistente não é trocado por outro: a tela diz que nada foi
substituído e pede escolha.

### Arquivos alterados

Backend: `internal/usecase/filter_usecase.go`, `internal/domain/analytics.go`,
`internal/delivery/http/handlers/filter_handler.go`,
`internal/delivery/http/handlers/strategy_handler.go`,
`internal/delivery/http/handlers/export_handler.go`,
`internal/usecase/strategyengine/engine.go`, `internal/usecase/discovery/{criteria,validation,engine}.go`.

Frontend: `core/models.ts`, `core/api.service.ts`,
`features/filters/{filters.component.ts,filters.component.html,simulator-url-state.ts}`,
`features/strategies/{strategies.component.ts,strategies.component.html}`.

---

## REV-P3 — Fase C (testes locais)

### Executado

| Comando | Resultado |
|---|---|
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test ./...` | todos os pacotes `ok` |
| `gofmt -l internal/ pkg/ cmd/` | vazio |
| `npx ng build --configuration production` | OK (641,96 kB inicial; só warnings pré-existentes) |
| `npx tsc … && node` (spec de URL) | 11/11 |

### Teste determinístico obrigatório

`TestDeterministico_20Ocorrencias_15Win_5Loss_Stake100_Odd150` — **PASS**

20 ocorrências · 15 WIN · 5 LOSS · stake R$100 · odd 1,50 →
`hit_rate` 75% · lucro R$250 · total apostado R$2.000 · ROI/Yield 12,5% ·
`odds_source = "fixed"` · `financials_reliable = false` · EV ausente do contrato.

Aritmética: 15 × 100 × (1,50 − 1) = +750; 5 × (−100) = −500; lucro 250.
Apostado 20 × 100 = 2.000. ROI 250 ÷ 2.000 = 12,5%.

### Novos testes (`internal/usecase/filter_revp3_test.go`, 18 no total)

- **Dedup:** 2 partidas MATCH-LEVEL → 2 (não 4); mesma partida nunca duplicada;
  TEAM-LEVEL continua com 2 por partida; MATCH-LEVEL com mando continua separando.
- **Escanteios + odd fixa:** aceita e marca `fixed`; odd real tem precedência sobre a fixa.
- **Ausência de odd:** estatística existe, financeiro é `nil`; odd 1,00 nunca é fabricada.
- **Três tipos de ausência, que não podem se confundir:** (A) nenhuma partida no recorte;
  (B) partidas sem a métrica (ficam de fora, não entram como zero); (C) **zero observado
  continua sendo zero** e conta como erro da linha.
- **EV:** falha se `ev`/`expected_value`/`expected_roi` voltar ao JSON do backtest.
- **Recorte efetivo:** com cap de 90 dias devolve hoje−90; sem cap usa a janela observada.

### Validação local dos números de produção

`TestFaseA_ParesDobradosNaoOcorremMais` — **PASS**. Reconstrói a forma do caso observado
(100 partidas, 69 acertando) e falha explicitamente se reaparecerem **200/100** ou
**138/69**, os pares dobrados vistos em produção na Fase A.

### Proveniência

`internal/delivery/http/handlers/strategy_origin_test.go` — `origin` vindo como
`discovery`, `validated` ou `approved` é rebaixado para `user`; só `simulator` é honrado.

### Estado de URL (`features/filters/simulator-url-state.spec.ts`, 11 testes)

Round-trip preserva os critérios; campeonato inválido recusa o estado inteiro;
métrica/mando/temporada inválidos avisam em vez de substituir calados; número inválido
vira ausência reportada e **não zero**.

### Ausências registradas (não são aprovações)

- **Não existe runner de testes de componente Angular** neste repositório (sem karma,
  jest ou vitest). Por isso a lógica de URL foi extraída para módulo puro e testada com
  `node:assert`, mesmo padrão de `season-resolution.spec.ts`. **O template do Simulador em
  si não foi exercitado por teste automatizado** — NA, não aprovado.
- **`@types/node` não está instalado**; o `tsc` do spec emite `TS2688` e mesmo assim gera o
  JS, que roda. Registrado como está, sem alterar as dependências do projeto.
- Nada foi validado em produção nesta fase, por instrução explícita.

---

## REV-P3 = PARCIAL — Fases A/B/C concluídas, aguardando deploy e validação em produção

---

## REV-P3 — Fase D (deploy + validação em produção)

Data da validação: **25/09/2026**. Toda evidência abaixo foi obtida contra
`https://dsfrcornerlab.com.br` (frontend) e `https://dsfrcornerlab.com.br/api/v1`
(backend), como **usuário anônimo**. Código local não foi usado como prova de produção.

### 1. Deploy

| Item | Valor |
|---|---|
| Branch | `main` |
| Commit | `848899258202ff90bdd54ce2097e1d149997423c` (`8488992`) |
| Autor / data | Daniel Ramos · 24/09/2026 23:53:51 −03 |
| Mensagem | `+---++++++++` |
| Push | Confirmado — `origin/main` aponta para `8488992`; árvore local limpa |
| Arquivos | 34 (19 backend, 15 frontend) |
| Migrations | **Nenhuma** neste commit — as correções do REV-P3 não tocam schema |
| Backend deploy | **No ar** — `POST /api/v1/filters/run` devolve os campos novos `financials_available`, `effective_from`, `effective_to` e os financeiros nuláveis |
| Frontend deploy | **No ar** — `/filtros` mostra "Odd fixa (simular)" para escanteios, o rótulo único "ROI / Yield", a linha "Recorte efetivamente analisado" e a URL com query string |
| Erros | Nenhum |

⚠️ **Warning de processo:** a mensagem de commit (`+---++++++++`) não descreve a mudança.
Um commit que altera o contrato financeiro de toda a plataforma merece mensagem
rastreável. Não bloqueia, mas fica registrado.

### 2. Casos reais em produção

Parâmetros comuns: usuário anônimo ⇒ `history_capped = true`, `history_cap_days = 90`.

#### CASO A — Brasileirão Série A · 2026

```
league_id 18 · season_id 26 · team_id (nenhum) · metric corners · rule "acima de"
threshold 8 · venue qualquer · perspective total da partida · last_n_games 0 (todos)
FixedOdd 1.50 · stake 100
URL https://dsfrcornerlab.com.br/filtros?league=18&metric=corners&threshold=8
    &last_n=0&stake=100&seasons=26&fixed_odd=1.5
```

Response: `match_count 100 · entries 100 · match_ids distintos 100 · hits 67 · misses 33 ·
hit_rate 67 · odds_source "fixed" · financials_reliable false · financials_available true ·
profit 50 · total_staked 10000 · roi 0.5 · yield 0.5 · max_drawdown 650 ·
effective_from 2026-06-27 · effective_to 2026-09-20`

Frontend: "Partidas encontradas 100 · Taxa de acerto 67% (67 acertos / 33 erros) ·
ROI / Yield 0.5% · Lucro 50 · Total apostado: 10000 · Drawdown máximo 650 ·
Recorte efetivamente analisado: 2026-06-27 a 2026-09-20 · janela limitada a 90 dias".

#### CASO B — La Liga · 2026

```
league_id 8 · season_id 33 · metric corners · threshold 8 · FixedOdd 1.50 · stake 100
```

Response: `match_count 69 · entries 69 · match_ids distintos 69 · hits 35 · misses 34 ·
hit_rate 50.72 · odds_source "fixed" · financials_reliable false · profit −1650 ·
total_staked 6900 · roi −23.91 · effective_from 2026-06-27 · effective_to 2026-09-20`

Amostra de match_ids: `50427, 50428, 50429, 50430, 50431, 50432`.

#### CASO C — La Liga · 2025

```
league_id 8 · season_id 12 · metric corners · threshold 8 · FixedOdd 1.50 · stake 100
URL .../filtros?league=8&metric=corners&threshold=8&last_n=0&stake=100&seasons=12&fixed_odd=1.5
```

Response: `match_count 0 · entries 0 · financials_available false · profit null ·
roi null · yield null · total_staked null · max_drawdown null · effective_from 2026-06-27 ·
effective_to (vazio)`

A temporada 2025 está **inteiramente fora da janela de 90 dias** do plano gratuito. O zero
aqui é legítimo — mas a forma como a tela o apresenta tem defeito (ver §13A abaixo).

### 3. A dupla contagem sumiu — prova em produção

| Caso | ANTES (Fase A) | DEPOIS (produção, 25/09/2026) |
|---|---|---|
| Brasileirão 2026 · escanteios | 200 entries / 100 match_ids | **100 entries / 100 match_ids** |
| La Liga 2026 · escanteios | 138 entries / 69 match_ids | **69 entries / 69 match_ids** |

`COUNT(entries) == COUNT(DISTINCT match_id)` verificado em JS na origem:
`new Set(entries.map(e => e.match_id)).size === entries.length` → **true** nos dois casos.

### 4. TEAM-LEVEL não foi destruído

```
league_id 18 · season_id 26 · metric win (vitória) · FixedOdd 1.80 · stake 100
```

`match_count 200 · entries 200 · match_ids distintos 100 · hits 70 · misses 130`

A duplicidade aqui é **semântica**, não acidental. Mesma partida, `match_id 16636`,
2026-09-20:

| Perspectiva | team | opponent | is_home | goals_for | goals_against | hit | odd | P/L |
|---|---|---|---|---|---|---|---|---|
| Mandante | Atletico Paranaense | Bahia | true | 2 | 1 | true | 1.8 | +80 |
| Visitante | Bahia | Atletico Paranaense | false | 1 | 2 | false | 1.8 | −100 |

São duas apostas diferentes com respostas opostas. 70 + 130 = 200 ✓.

### 5. Escanteios com odd fixa

CASO A acima. Confirmado: há 100 ocorrências (a métrica **não** volta vazia, que era o
defeito da Fase A); `odds_source = "fixed"`; `financials_reliable = false`; cada entrada
traz `odds_source: "fixed"` e `odd: 1.5`; a tabela da UI mostra "**1.5 fixa**" em todas as
linhas; e o banner diz *"Cenário com odd fixa que você informou. Nenhuma odd de mercado foi
usada… Lucro, ROI e yield abaixo descrevem esse cenário, não o que as casas pagavam em cada
jogo."* Em nenhum ponto a odd digitada é classificada como histórica real.

### 6. Escanteios sem odd — mesmo contexto, sem FixedOdd

```
league_id 18 · season_id 26 · metric corners · threshold 8 · stake 100 · (sem fixed_odd)
```

**Estatística idêntica ao CASO A**, o que prova que a ausência de odd não encolhe a amostra:
`match_count 100 · hits 67 · misses 33 · hit_rate 67 · average_corners 10.32 ·
longest_win_streak 10 · longest_lose_streak 3`.

**Financeiro indisponível:** `odds_source "none" · financials_available false ·
profit null · total_staked null · roi null · yield null · max_drawdown null`.
Todas as 100 entradas: `odd: null`, `profit_loss: null`, sem `odds_source`.

`financials_note` = *"Sem odd para estas partidas: o resultado abaixo é estatístico.
Informe uma odd fixa para simular um cenário financeiro."*

UI: "ROI / Yield **N/A** — Sem odd — não calculável", "Lucro **N/A**",
"Drawdown máximo **N/A**", e a tabela com "—" nas colunas Odd e Resultado (stake).

**Nenhuma odd 1.00 fabricada. Nenhum zero financeiro representando ausência.**

### 7. Auditoria manual — CASO A

```
N = 100 · wins = 67 · losses = 33 · stake = 100 · odd = 1.50

profit_win   = 100 × (1,50 − 1)      = +50
profit_loss  = −100
total_profit = 67 × 50 + 33 × (−100) = 3350 − 3300 = 50
total_staked = 100 × 100             = 10 000
ROI / Yield  = 50 ÷ 10 000           = 0,5 %
```

| | lucro | total apostado | ROI |
|---|---|---|---|
| MANUAL | 50 | 10 000 | 0,5 % |
| BACKEND | 50 | 10 000 | 0,5 % |
| FRONTEND | 50 | 10 000 | 0,5 % |

**Batem.** Auditoria cruzada no CASO B: `35 × 80 + 34 × (−100) = 2800 − 3400 = −600`… —
atenção, aqui a odd é 1,50, não 1,80: `35 × 50 + 34 × (−100) = 1750 − 3400 = −1650` ✓,
`total_staked = 69 × 100 = 6900` ✓, `ROI = −1650 ÷ 6900 = −23,913…% → −23.91` ✓.
E no TEAM-LEVEL (odd 1,80): `70 × 80 + 130 × (−100) = 5600 − 13000 = −7400` ✓,
`total_staked = 200 × 100 = 20 000` ✓, `ROI = −37%` ✓.

### 8. Caso determinístico — contrato local

Não usado como prova de produção. Registrado que continua coberto:
`TestDeterministico_20Ocorrencias_15Win_5Loss_Stake100_Odd150` — **PASS** no commit
`8488992` (20 ocorrências · 15 WIN · 5 LOSS · stake 100 · odd 1,50 → 75% · 250 · 2000 ·
12,5% · `fixed` · `financials_reliable=false` · EV ausente do JSON).

`TestFaseA_ParesDobradosNaoOcorremMais` — **PASS** (falha se 200/100 ou 138/69 voltarem).

### 9. Odds source — o que existe de fato em produção

| Origem | Observada? | Contrato conferido |
|---|---|---|
| `fixed` | **Sim** | `financials_reliable = false` + banner de cenário hipotético ✓ |
| `none` | **Sim** | financeiro inteiro `null`, UI em N/A ✓ |
| `real` | **NÃO** | — |
| `synthetic` | **NÃO** | — |

**NA — não existe amostra com `odds_source = "real"` em produção.** Requisição de escanteios
com `max_odds: 5.0` e sem odd fixa (Brasileirão 2026 e 2024) devolveu `odds_source: "none"`
e nenhuma entrada com procedência. A base legada de odds sintéticas (14/07) está fora da
janela de 90 dias do usuário anônimo, então `synthetic` também não foi exercitada. Nenhum
caso "real" foi inventado.

⚠️ **Achado novo (§A3 abaixo):** `max_odds: 5.0` não filtrou nada e as 100 partidas sem odd
entraram mesmo assim.

### 10. EV — confirmado ausente na UI real

Texto completo da página do Simulador em produção conferido. **Não aparece**:
"ROI esperado", "valor esperado", "EV", "odd de equilíbrio", "break-even",
"margem de segurança", "pode errar até N em 10", nem a tabela de cenários de odd.
O campo "Odd da casa" não existe mais. No JSON, `Object.keys(response)` não contém
`ev`, `expected_value` nem `expected_roi`.

No lugar, o bloco "Sobre o que estes números são" declara: *"O CornerLab não estima
probabilidade futura nem retorno esperado, e não compara a odd de uma casa com a taxa de
acerto histórica — essa conta usaria uma amostra escolhida justamente por ter acertado muito
e apontaria vantagem onde não há."*

### 11. ROI / Yield — rótulo único

A UI mostra **um único card** rotulado `ROI / Yield`. Não são apresentados como duas
evidências independentes. ✓

### 12. FinancialsAvailable — os dois casos

| | Configuração | `financials_available` | UI |
|---|---|---|---|
| CASO 1 | Brasileirão 2026 · escanteios · **odd fixa 1,50** | `true` | ROI 0.5%, lucro 50 |
| CASO 2 | Brasileirão 2026 · escanteios · **sem odd** | `false` | ROI N/A, lucro N/A, drawdown N/A |

### 13. No-data / metric-unavailable / zero real

**A. Nenhuma partida elegível** — CASO C (La Liga 2025). `match_count 0`, financeiro `null`.
❌ **DEFEITO NOVO NA UI** — ver §A1.

**B. Partidas existem, métrica indisponível** — Brasileirão 2026 · impedimentos:
`match_count 87` de 100 partidas do mesmo recorte. **13 partidas sem o dado ficaram de fora
em vez de entrar como zero** ✓. A UI exibe o aviso *"Essa métrica nem sempre é publicada
pelo provedor: jogos sem o dado ficam de fora do backtest…"*.
⚠️ O backend não informa **quantas** ficaram de fora — a tela não distingue "87 de 100" de
"87 partidas no recorte". Ver §A4.

**C. Zero real preservado** — Brasileirão 2026 · gols · limiar 0 ("acima de 0 gols"):
`match_count 100 · hits 91 · misses 9`. As **9 partidas 0×0** entraram com
`total_goals: 0` observado e `hit: false`. Exemplo: `match_id 16462`, 2026-07-23,
Botafogo × Vitória, `total_goals 0`, `hit false`, `profit_loss −100`.
Distribuição observada: 0, 1, 2, 3, 4, 5, 6. **Zero observado continua sendo zero** ✓.

### 14. Occurrences / tabela auditável

CASO A: `occurrences = 100` e a tabela renderiza **100 linhas**. Colunas presentes:
Data · Equipe · Adversário · Mando · Escanteios · Resultado · Odd · Resultado (stake).
Primeiras linhas conferidas contra o JSON:

```
2026-07-16  Vitoria    Vasco DA Gama    Casa   5   Erro     1.5 fixa   -100
2026-07-16  Botafogo   Santos           Casa  10   Acerto   1.5 fixa    +50
2026-07-17  Bahia      Chapecoense-sc   Casa  16   Acerto   1.5 fixa    +50
```

`match_id` está no payload de cada entrada (16442, 16443, …) mas **não é exibido como
coluna**. Agregado e lista correspondem: 67 "Acerto" + 33 "Erro" = 100.
⚠️ Ver §A2 sobre a coluna "Mando".

### 15. EffectiveFrom / EffectiveTo

Backend: `effective_from 2026-06-27`, `effective_to 2026-09-20`, `history_capped true`,
`history_cap_days 90`. Frontend: linha *"Recorte efetivamente analisado: 2026-06-27 a
2026-09-20 · janela limitada a 90 dias (plano gratuito)"* + o banner
*"Resultado limitado aos últimos 90 dias (plano gratuito)"*.

**Em nenhum momento a tela diz "histórico completo".** ✓

### 16. URL / reload

**Ida.** Execução pela UI produziu:
`/filtros?league=21&metric=corners&threshold=8&last_n=0&stake=100&seasons=29&fixed_odd=1.5`

**Volta.** Navegação direta para
`/filtros?league=18&seasons=26&metric=corners&threshold=8&last_n=0&stake=100&fixed_odd=1.5`
restaurou: Campeonato "Brasileirão Série A", Temporada "2026", métrica "Escanteios",
"Acima de (escanteios)" = 8, "Odd fixa (simular)" = 1.5, Stake = 100 — e reexecutou o
backtest automaticamente.

**Parâmetro inválido — antes/depois.** URL enviada:
`?league=18&seasons=26&metric=placar_exato&threshold=8&last_n=xyz&stake=100&home_away=neutro&fixed_odd=1.5`

Banner exibido, nominalmente, sem substituição silenciosa:

> A URL não pôde ser reproduzida integralmente: métrica desconhecida na URL
> ("placar_exato") — usando escanteios; mando inválido na URL ("neutro"); últimos jogos
> inválido na URL ("xyz"). Confira os critérios abaixo antes de ler o resultado.

### 17 e 18. Salvar simulação e reivindicação de provenance

**NA — não validável em produção nesta sessão.** `POST /api/v1/strategies` exige
autenticação (resposta conferida: `401 {"error":"token ausente"}`), e **não manipulo
credenciais**. Nenhum login foi feito.

Cobertura local no mesmo commit (`strategy_origin_test.go`, PASS):
`origin="simulator"` → `simulator`; `""`/`"user"` → `user`;
**`"discovery"`, `"validated"` e `"approved"` → rebaixados para `user`**.
`Visibility` é fixado em `"private"` e `Active` em `true` no servidor, ignorando o cliente.

Significado de `Active = true`: a estratégia **aparece na lista do usuário**. Não significa
validada nem aprovada — `stage`, health e scores são do Strategy Engine, e `PersistResult`
recusa pontuar backtest sem série financeira completa.

⚠️ Por ser cobertura apenas local, os itens 29 e 30 dos 40 ficam ⚠️, não ✅.

### 19. Ordem cronológica

CASO A, entradas na ordem em que vêm do backend:
primeira `2026-07-16` (Vitória × Vasco), última `2026-09-20` (Atlético-PR × Bahia).
No TEAM-LEVEL: primeira `2026-07-16`, última `2026-09-20`. **ASC confirmado** — e o
drawdown de 650 e as sequências 10/3 são calculados sobre essa série.

### 20. Drawdown

- Com financeiro válido (CASO A): `max_drawdown 650`, sobre a série ordenada.
- Sem financeiro válido (sem odd): `max_drawdown null`, UI "N/A" — **não zero** ✓.
- Com `fixed`: o banner declara cenário, não performance financeira real ✓.

AUD-006 não foi reaberto.

### 21. Regressão (commit `8488992`, árvore limpa)

| Comando | Resultado |
|---|---|
| `gofmt -l internal/ pkg/ cmd/` | vazio |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test ./...` | 9 pacotes `ok`, 0 FAIL |
| `npx ng build --configuration production` | OK — 641,96 kB inicial (160,86 kB comprimido) |
| Runner de teste de componente Angular | **NA — inexistente neste repositório** (sem karma/jest/vitest em `package.json`). Nenhuma execução inventada. |

---

## REV-P3 — Fase E (registro final)

### Achados novos desta fase

#### §A1 — ❌ Tela de resultado vazio apresenta ausência como zero observado

**Onde:** CASO C (La Liga 2025, `match_count = 0`).

A tela **não** mostra "Nenhuma partida encontrada para os filtros selecionados". Em vez
disso renderiza o painel completo com:

```
Partidas encontradas 0 · Taxa de acerto 0% (0 acertos / 0 erros)
Média de escanteios 0 · Maior sequência (acertos) 0 · Maior sequência (erros) 0
```

"Taxa de acerto 0%" e "Média de escanteios 0" **com amostra vazia são afirmações falsas** —
é exatamente a conversão *no-data → zero* que o REV-P3 proíbe. O bloco financeiro está
correto (N/A), o estatístico não.

Além disso a tela exibe **dois avisos contraditórios ao mesmo tempo**: o banner de "Cenário
com odd fixa" (porque o backend devolve `odds_source: "fixed"` mesmo com zero ocorrências —
declaração de intenção, não observação) e "Sem resultado financeiro. Nem todas as partidas
do recorte têm odd". E "Recorte efetivamente analisado: 2026-06-27 a **—**".

**Não corrigido nesta fase** (fora do escopo autorizado de D/E, que é validar e registrar).

#### §A2 — ⚠️ Coluna "Mando" diz "Casa" em 100/100 linhas de regra MATCH-LEVEL

Consequência direta da correção 1: para regra de partida inteira sem mando pedido, o motor
mantém `asHome(m)` como representante único da partida. A tabela então exibe
`Mando = Casa` e `Equipe / Adversário` em todas as linhas, **sugerindo perspectiva de
mandante onde o número é da partida**. Verificado: `is_home = true` em 100/100.

Não é dupla contagem — é rotulagem. O correto seria "—" ou "Partida" nessa coluna quando a
métrica é MATCH-LEVEL e nenhum mando foi pedido.

#### §A3 — ⚠️ `max_odds` sem efeito quando não há odd na base

`max_odds: 5.0` em escanteios (Brasileirão 2026) devolveu as mesmas 100 partidas, todas sem
odd, `odds_source: "none"`. O filtro declarado pelo usuário **não foi aplicado nem
anunciado como não aplicado**. É consequência de `AllowMissingOdds` no Simulador: partidas
sem odd são mantidas para preservar a estatística — decisão correta, comunicação ausente.

#### §A4 — ⚠️ Nº de partidas excluídas por falta da métrica não é exposto

Impedimentos: 87 de 100. O contrato não traz `metric_sample_size` nem o total do recorte,
então a tela não consegue dizer "87 de 100 partidas tinham o dado".

#### §A5 — ⚠️ Escanteios 0 e 1 em partidas reais (fora do escopo — pipeline)

`match_id 16451` (2026-07-21, Atlético-MG × Bahia) com **0 escanteios totais** e
`match_id 16476` (2026-07-26, Flamengo × São Paulo) com **1**. Valores implausíveis para
partidas reais. `domain.Match.HomeCorners/AwayCorners` são `int` (não ponteiro), então
`NULL` no banco vira `0` no scan e fica indistinguível de zero observado.

**Origem é o pipeline de sincronização, não o Simulador.** Não corrigido: está fora do
escopo declarado desta fase. Mas por causa dele o item 7 ("ausência ≠ zero") **não pode ser
marcado ✅ de ponta a ponta** — só no ramo do Simulador.

### Auditoria dos 40 itens

| # | Item | Status | Evidência |
|---|---|---|---|
| 1 | Cadeia UI → banco → engine documentada | ✅ | Fases A/B + §2 desta fase; request/response reais registrados |
| 2 | Campeonato correto | ✅ | `league_id 18` → "Brasileirão Série A"; `8` → "La Liga" na UI após reload |
| 3 | Temporada correta | ✅ | `seasons=26` → "2026"; `seasons=12` → La Liga 2025 |
| 4 | Nenhuma mistura silenciosa | ✅ | CASO B (69) ≠ CASO C (0); temporadas separadas |
| 5 | FINALIZADO apenas | ✅ | Última partida 2026-09-20 < hoje (25/09); nenhum jogo futuro no calendário entrou |
| 6 | Nenhuma duplicação indevida | ✅ | 100/100 e 69/69 em produção (antes 200/100 e 138/69) |
| 7 | Ausência ≠ zero | ⚠️ | No Simulador sim (odd `null`, financeiro `null`, impedimentos excluídos). **Mas §A5**: escanteios `NULL` viram `0` no scan — falha de pipeline |
| 8 | Zero real preservado | ✅ | §13C: 9 partidas 0×0, `total_goals 0`, `hit false`, exemplo `16462` |
| 9 | Métricas rastreáveis | ✅ | Cada entrada traz `match_id`, data, times, valor observado, `hit`, `odd`, `odds_source`, `profit_loss` |
| 10 | `sample_size` correto | ✅ | `match_count 100` = 100 partidas distintas do recorte |
| 11 | `metric_sample_size` quando necessário | ❌ | §A4: impedimentos 87 de 100 sem o total ser exposto |
| 12 | Occurrences auditáveis | ✅ | §14: 100 linhas para `occurrences = 100` |
| 13 | Tabela corresponde ao agregado | ✅ | 67 "Acerto" + 33 "Erro" = 100 = `match_count` |
| 14 | Hit rate correto | ✅ | 67 ÷ 100 = 67% ✓; 35 ÷ 69 = 50,72% ✓; 70 ÷ 200 = 35% ✓ |
| 15 | Produzido/concedido/total corretos | NA | O Simulador só opera "total da partida"; produzido/concedido é tela do Comparador (REV-P2) |
| 16 | Geral/Casa/Fora corretos | ⚠️ | O filtro de mando funciona (TEAM-LEVEL separa mandante/visitante corretamente), mas **§A2**: a coluna "Mando" mente em regra MATCH-LEVEL |
| 17 | `odds_source` respeitado | ✅ | `fixed` e `none` observados e coerentes com as entradas |
| 18 | Synthetic explicitamente cenário | NA | **Não existe amostra `synthetic` na janela de 90 dias** (§9). Banner existe no código, não exercitado em produção |
| 19 | Unknown não tratado como real | ✅ | `none` → financeiro inteiro `null`, `financials_reliable false` |
| 20 | Odd manual explicitamente cenário | ✅ | Banner "Cenário com odd fixa que você informou… não o que as casas pagavam" |
| 21 | ROI matematicamente correto | ✅ | §7: manual = backend = frontend em 3 casos independentes |
| 22 | Yield semanticamente correto | ✅ | Rótulo único "ROI / Yield"; sob stake fixa são a mesma divisão (AUD-002) |
| 23 | EV não fabricado | ✅ | §10: ausente da UI e do JSON |
| 24 | Drawdown auditado | ✅ | 650 com financeiro válido; `null`/N/A sem odd |
| 25 | Ordem cronológica correta | ✅ | §19: ASC, 2026-07-16 → 2026-09-20 |
| 26 | Scores auditados | NA | Scores são do Strategy Engine; fora do escopo do REV-P3 |
| 27 | Fixed stake separado de compounding | ✅ | `total_staked = N × stake` exato (10 000 = 100 × 100; 6 900 = 69 × 100; 20 000 = 200 × 100) — sem reinvestimento |
| 28 | Linguagem não preditiva | ✅ | §10: "desempenho observado", "não estima probabilidade futura nem retorno esperado" |
| 29 | Save possui provenance correto | ⚠️ | Coberto por teste local (PASS), **não validado em produção** — rota exige auth (§17) |
| 30 | Save não vira estratégia validada | ⚠️ | Idem: `discovery`/`validated`/`approved` rebaixados para `user` em teste local |
| 31 | Estado reproduzível | ✅ | §16: ida, volta e recusa nominal de parâmetro inválido |
| 32 | No-data explícito | ❌ | §A1: amostra vazia renderiza "Taxa de acerto 0%" e "Média 0" |
| 33 | Metric-unavailable explícito | ⚠️ | Aviso genérico existe; contagem de excluídas não (§A4) |
| 34 | Testes determinísticos passam | ✅ | §8: ambos PASS no commit em produção |
| 35 | `go build` passa | ✅ | §21 |
| 36 | `go vet` passa | ✅ | §21 |
| 37 | `go test` passa | ✅ | §21 — 9 pacotes ok |
| 38 | Frontend production build passa | ✅ | §21 — 641,96 kB |
| 39 | Runner frontend executado OU ausência registrada | ✅ | **NA registrado**: não existe runner de componente; 11 testes de URL via `node:assert` |
| 40 | ≥ 3 casos E2E reais em produção | ✅ | CASOS A, B, C + team-level + sem-odd + zero-real + métrica indisponível |

**Contagem:** ✅ 27 · ⚠️ 6 · ❌ 3 · NA 4.

### Pendências que impedem o fechamento

1. **§A1 / item 32** — amostra vazia apresenta "Taxa de acerto 0%" e "Média 0" como se
   fossem observações. Conversão no-data → zero na própria tela.
2. **§A4 / item 11 e 33** — o contrato não expõe quantas partidas do recorte tinham a
   métrica, então "87" não é auditável contra "100".
3. **§A2 / item 16** — coluna "Mando" = "Casa" em 100/100 linhas de regra MATCH-LEVEL.
4. **§A3** — `max_odds` silenciosamente sem efeito quando não há odd na base.
5. **Itens 29 e 30** — provenance do save não validada em produção (rota autenticada;
   não manipulo credenciais).
6. **Itens 18 e 9 (`real`/`synthetic`)** — não há amostra dessas origens na janela de 90
   dias do usuário anônimo. Nenhum caso foi inventado.
7. **§A5** — escanteios `NULL` gravados como `0` pelo pipeline. Fora do escopo do REV-P3,
   mas impede marcar o item 7 como ✅ de ponta a ponta.

### Conclusão

Os **seis defeitos que o REV-P3 se propôs a corrigir estão corrigidos e comprovados em
produção**: dupla contagem (200/100 → 100/100 e 138/69 → 69/69), escanteios com odd fixa,
fim do fallback 1.00, EV removido, provenance implementada e estado reproduzível na URL.
A auditoria aritmética bate nos três eixos (manual, backend, frontend) em três casos
independentes.

Mas a Definition of Done tem 40 itens, e 3 estão ❌ e 6 ⚠️ — a maioria em estados de
ausência na apresentação, que é justamente o princípio que este trabalho defende.

## REV-P3 = PARCIAL — 27 ✅ / 6 ⚠️ / 3 ❌ / 4 NA. Faltam as 7 pendências listadas acima.

---

## REV-P3 — Correções finais das pendências da Fase D

Data: **25/09/2026**. Escopo: **somente** as pendências comprovadas do próprio
Simulador. Nada de P0/P1/P2, Discovery, Estratégias, Banca, Projeções, AUD-006 ou AUD-007.

### §5 — Escanteios NULL → 0 · INVESTIGAÇÃO (correção NÃO implementada)

**Não corrigi por hipótese.** A cadeia foi percorrida inteira:

| Elo | Constatação |
|---|---|
| **DB** | `migrations/001_init.sql:45-46` — `home_corners INT **NOT NULL DEFAULT 0**` |
| **Ingestão** | `internal/usecase/sync_usecase.go:119-124` — **aqui está a conversão** |
| **Repository** | `match_repo.go` faz `&m.HomeCorners` — recebe int, nunca vê NULL |
| **Domain** | `entities.go:46` — `HomeCorners int` (não ponteiro) |
| **Engine** | `filter_usecase.go` recebe 0; indistinguível de zero observado |

O código exato da ingestão:

```go
homeCorners, awayCorners := 0, 0
if f.HomeCorners != nil { homeCorners = *f.HomeCorners }
if f.AwayCorners != nil { awayCorners = *f.AwayCorners }
```

O provedor **preserva** a nulabilidade (`sportsdata.Fixture.HomeCorners *int`, e
`SyncResult.CornersMissing` até conta os casos). A perda acontece na linha acima, no
momento de gravar — reforçada pelo `NOT NULL DEFAULT 0` do schema.

**A precondição do §5 do prompt ("se o banco usa colunas nullable") é FALSA.** Corrigir
de verdade exige três coisas encadeadas:

1. migration tornando `home_corners`/`away_corners` nullable;
2. `domain.Match.HomeCorners` virar `*int`;
3. propagar o ponteiro por `comparator_metrics.go` (REV-P2), Dashboard (REV-P1),
   `entities.go:165 TotalCorners()` e o engine.

Os passos 2 e 3 **reabrem P1 e P2**, explicitamente proibido neste escopo. E há um limite
que nenhuma correção remove: **a informação histórica já está perdida** — os zeros
gravados não registram se eram ausência, e tornar a coluna nullable hoje não recupera
quais dos zeros de 2026 eram estatística não publicada.

Comparação que fecha o diagnóstico: impedimentos e chutes entraram pelas migrations
004/010 **sem** `NOT NULL`, são `*int` no domínio, e por isso a distinção funciona
corretamente para eles — provado em `TestNullVsZero_MetricaNullable_DistinguiCorretamente`.

**Registrado como item próprio, fora do REV-P3.** O item 7 da DoD permanece ⚠️.

### §3 — max_odds · INVESTIGAÇÃO (não é legado, não estava quebrado)

Cadeia: `filters.component.ts` → `dto.FilterRunRequest.MaxOdds` →
`filter_handler.go:57` → `FilterCriteria.MaxOdds` → `filter_usecase.go:544` e `:608`.

A intenção original está declarada no próprio código, sem ambiguidade:

> "Odds máximas" é filtro de ELEGIBILIDADE sobre odd de mercado: descarta a partida cuja
> odd registrada passe do teto.

**Está implementado corretamente.** O filtro só não age quando não há odd para comparar —
o `switch` cai nos ramos de odd fixa / `AllowMissingOdds` antes de chegar ao teto, o que é
o comportamento certo: não se pode comparar um teto com uma odd inexistente.

Como **nenhuma** partida da janela de 90 dias tem `corner_odds` em produção, o controle
nunca tem o que filtrar. **O defeito não é o filtro: é o silêncio.** O usuário digitava
"odds máximas 5,00", recebia 100 partidas e não tinha como saber que o controle não agiu.

Decisão: **não removi da UI** (o filtro é legítimo e volta a funcionar assim que houver
odds reais) e **não inventei semântica nova**. Implementei a comunicação, junto do §4.

### §4 + §3 — Rastreabilidade da amostra (implementado)

Novo bloco `accounting` em `BacktestResult`, contando **observações** (não partidas —
em team-level uma partida gera duas):

```
matches_in_window · observations_in_window · eligible_entries · excluded_entries
excluded_no_metric · excluded_by_max_odds · excluded_no_odd · excluded_by_venue
excluded_other · max_odds_requested · max_odds_applicable
```

Identidade garantida por teste em 7 configurações diferentes:

```
observations_in_window = eligible_entries
                       + excluded_no_metric + excluded_by_max_odds
                       + excluded_no_odd + excluded_by_venue + excluded_other
```

Os **7 pontos de `continue`** do laço foram instrumentados, um contador cada.
`excluded_other` recebe a diferença: se alguém acrescentar um `continue` sem contador, o
resto aparece ali em vez de ser absorvido por uma categoria errada. **Nenhuma categoria é
estimada** — só o que o motor determina com certeza.

`max_odds_applicable` é o que responde ao §3: quantas observações tinham odd de mercado
para comparar com o teto. Sendo 0 com teto pedido, a UI declara que o controle não agiu.

### §2 — Coluna "Mando" em regra MATCH-LEVEL (implementado)

Novo campo `metric_scope` no contrato: `"match"` ou `"team"`.

- **match** (escanteios, gols, impedimentos, chutes): a coluna "Mando" **desaparece**, e
  os cabeçalhos viram "Mandante"/"Visitante" — os dois participantes do jogo, não uma
  equipe analisada e seu adversário.
- **team** (vitória, empate, não perde): "Mando" **permanece**, porque mandante e
  visitante são observações distintas e a perspectiva é real.

Antes: 100/100 linhas de escanteios diziam "Mando: Casa", porque a correção da dupla
contagem elegeu a perspectiva do mandante como representante canônico da partida. O número
era da partida; o rótulo afirmava que era do mandante. Nada foi mascarado — a coluna sai
onde não tem significado e fica onde tem.

### §1 — Estado vazio (implementado)

Dois ajustes:

**Backend.** `oddsSourceSummary` passou a receber a contagem de ocorrências e devolve
`none` quando ela é zero. Antes devolvia `"fixed"` só porque o usuário tinha digitado uma
odd, e a tela de amostra vazia exibia "Cenário com odd fixa que você informou" ao lado de
"0 partidas" — intenção apresentada como observação.

**Frontend.** Três estados mutuamente exclusivos, em `features/filters/sample-state.ts`
(funções puras):

| Estado | Condição | Mensagem |
|---|---|---|
| `vazio` | `match_count = 0` e `matches_in_window = 0` | "Nenhuma partida encontrada para os filtros selecionados." |
| `metrica-ausente` | `match_count = 0`, `matches_in_window > 0`, `excluded_no_metric > 0` | "Esta estatística não está disponível para a amostra selecionada." |
| `com-amostra` | `match_count > 0` | painel normal; zeros observados são zeros |

Nos dois primeiros o painel de cards **não é renderizado**: somem "Taxa de acerto 0%",
"Média 0", sequências 0 e drawdown 0. Exclusão por mando (por exemplo) **não** vira
`metrica-ausente` — trocar um estado de ausência por outro seria o mesmo erro.

### Antes / depois

| Cenário | ANTES (produção 25/09) | DEPOIS (local) |
|---|---|---|
| La Liga 2025, 0 partidas | "Taxa de acerto 0%", "Média 0", sequências 0 | "Nenhuma partida encontrada…"; sem cards |
| La Liga 2025, odd fixa digitada | `odds_source: "fixed"` + banner de cenário com 0 jogos | `odds_source: "none"`, sem banner |
| Impedimentos, 87 de 100 | "87 partidas", sem explicação | "100 partidas no recorte · 87 analisadas · 13 fora: 13 sem a estatística publicada pelo provedor." |
| Escanteios, `max_odds` 5,00 | 100 partidas, silêncio | aviso: teto não aplicado a nenhuma partida; `max_odds_applicable: 0` |
| Escanteios match-level | "Mando: Casa" em 100/100 | coluna ausente; "Mandante"/"Visitante" |
| Vitória team-level | "Mando: Casa/Fora" | inalterado — a perspectiva é real |

### Validação local — 8 cenários (§9)

Todos com a identidade contábil conferida (`-> true` em todos):

| # | Cenário | scope | match_count | odds_source | accounting |
|---|---|---|---|---|---|
| 1 | Sem partidas | match | 0 | **none** | tudo 0 |
| 2 | Métrica indisponível | match | 0 | none | in_window 1, `excluded_no_metric 1` |
| 3 | Zero real (impedimentos = 0) | match | 1 | fixed | `excluded_no_metric 0` |
| 4 | Match-level (escanteios) | **match** | 2 | fixed | 2 obs / 2 partidas |
| 5 | Team-level (vitória) | **team** | 4 | fixed | **4 obs / 2 partidas** |
| 6a | max_odds com odd na base | match | 1 | real | `excluded_by_max_odds 1`, `applicable 2` |
| 6b | max_odds sem odd na base | match | 2 | none | `requested 5`, **`applicable 0`** |
| 7 | Escanteios 0 (limitação NULL=0) | match | 1 | fixed | `excluded_no_metric 0` |
| 8 | NULL e zero real lado a lado | match | 1 | fixed | `excluded_no_metric 1`, o zero real entrou |

O cenário 8 é o que separa os conceitos: duas partidas, uma com impedimentos `NULL` e
outra com `0` observado. A primeira sai (`excluded_no_metric`), a segunda entra e conta
como erro da linha. **NULL permanece indisponível; 0 permanece zero real.**

### Testes

**Backend** — `internal/usecase/filter_revp3_pendencias_test.go`, 13 testes, todos PASS:
estados A/B/C; `MetricScope` match/team/match-com-mando; `max_odds` com e sem odd na base;
identidade contábil em 7 configurações; team-level contando observações; exclusão por
mando; NULL vs zero em métrica nullable; e a limitação conhecida dos escanteios.

`TestNullVsZero_Escanteios_LimitacaoConhecida` **falha de propósito** se alguém tornar a
coluna nullable, obrigando a revisitar esta decisão em vez de deixá-la desatualizada em
silêncio.

**Frontend** — `features/filters/sample-state.spec.ts`, 13 testes, todos PASS (node:assert).

| Comando | Resultado |
|---|---|
| `gofmt -l internal/ pkg/ cmd/` | vazio |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test ./...` | 9 pacotes `ok`, 0 FAIL |
| `npx ng build --configuration production` | OK — 642,16 kB (160,79 kB comprimido) |
| `sample-state.spec.ts` via tsc + node | 13/13 |
| `simulator-url-state.spec.ts` via tsc + node | 11/11 |
| Runner de componente Angular | **NA — inexistente neste repositório.** Nenhuma execução inventada. |

### Itens da DoD afetados (a confirmar só após deploy e validação em produção)

| # | Item | Antes | Agora (local) |
|---|---|---|---|
| 11 | `metric_sample_size` quando necessário | ❌ | corrigido localmente via `accounting` |
| 16 | Geral/Casa/Fora corretos | ⚠️ | corrigido localmente via `metric_scope` |
| 32 | No-data explícito | ❌ | corrigido localmente (3 estados) |
| 33 | Metric-unavailable explícito | ⚠️ | corrigido localmente (estado próprio + contagem) |

**Nenhum deles é marcado ✅ ainda.** Código local não é prova de produção — foi a regra da
Fase D e continua valendo.

### O que continua NA / ⚠️ e por quê

| Item | Status | Motivo |
|---|---|---|
| 7 — ausência ≠ zero (escanteios) | ⚠️ | §5: origem provada na ingestão + schema `NOT NULL`. Correção exige migration + mudança de tipo no domínio, reabrindo P1/P2. Dado histórico já perdido. |
| 18 — synthetic explicitamente cenário | NA | Não existe amostra `synthetic` na janela de 90 dias. Nada foi criado. |
| 9 (parcial) — `odds_source = real` | NA | Não existe amostra `real` em produção. Nada foi criado. |
| 29 — save com provenance correto | ⚠️ | Rota exige autenticação (`401 token ausente`). Não manipulo credenciais. Coberto só por teste local. |
| 30 — save não vira validada | ⚠️ | Idem. |

Conforme §6 e §7 do prompt: **nenhum destes virou ✅ por teste local**, e nenhum dado foi
criado para forjar amostra.

## REV-P3 = PARCIAL — correções finais implementadas localmente, aguardando novo deploy/validação

---

## REV-P3 — Deploy e validação focada das pendências

> **Nota de restauração (25/09/2026).** Esta seção e a seguinte foram escritas nas
> passagens anteriores, mas **desapareceram do arquivo antes de serem commitadas**: o
> `CORRECOES_AUDITORIA.md` no disco foi substituído pela versão do HEAD (convertida para
> CRLF), provavelmente por editor ou `git checkout` no Windows. Nenhuma delas chegou a
> nenhum commit. Foram restauradas com o texto original, sem alteração de conteúdo.

Data: **25/09/2026**. Escopo: somente os itens 11, 16, 32 e 33 e os pontos associados.
Os demais itens mantêm a evidência da Fase D. Toda evidência abaixo é de produção
(`https://dsfrcornerlab.com.br`), usuário anônimo.

### Correção de registro anterior

O resumo final da Fase E registrou **"27 ✅ / 6 ⚠️ / 3 ❌ / 4 NA"**. Está **errado**. A
tabela dos 40 itens logo acima dele, conferida linha a linha, soma:

- ✅ 30 — 1, 2, 3, 4, 5, 6, 8, 9, 10, 12, 13, 14, 17, 19, 20, 21, 22, 23, 24, 25, 27, 28, 31, 34–40
- ⚠️ 5 — 7, 16, 29, 30, 33
- ❌ 2 — 11, 32
- NA 3 — 15, 18, 26

A tabela estava certa; a linha de resumo, não. O texto anterior não foi alterado — esta
nota é a correção.

### Deploy

| Item | Valor |
|---|---|
| Branch | `main` |
| Commit | `25931e5` — "REV-P3 pendencias: estado vazio, metric_scope, accounting, max_odds_applicable" |
| origin/main | `25931e5` (push feito pelo Daniel; eu não manipulo credenciais) |
| Backend | **no ar** — resposta de `/filters/run` traz `accounting` e `metric_scope` |
| Frontend | **no ar** — mensagem "Nenhuma partida encontrada…", aviso de `max_odds` e cabeçalhos "Mandante/Visitante" renderizados |
| Migration | nenhuma |
| Erros | nenhum |

### Estado vazio — La Liga 2025

`/filtros?league=8&seasons=12&metric=corners&threshold=8&last_n=0&stake=100&fixed_odd=1.5`

Backend: `match_count 0 · odds_source "none"` (antes deste deploy: `"fixed"`) ·
financeiro `null` · `accounting` todo zero.

UI exibe *"Nenhuma partida encontrada para os filtros selecionados. Não há amostra para
analisar, então não existem taxa de acerto, média nem resultado financeiro para este
recorte — nem como zero."* Verificado ausente: "Taxa de acerto", "Média de", "Maior
sequência", "ROI / Yield", "Drawdown máximo" e o banner "Cenário com odd fixa".

### Métrica indisponível — Champions 2026 · Egnatia Rrogozhinë

Nenhuma liga/temporada inteira está 100 % sem métrica (varredura de 20 recortes ×
impedimentos/chutes/chutes no gol). O caso real foi obtido por filtro de equipe legítimo:

`/filtros?league=23&seasons=37&team=4748&metric=offsides&threshold=1&last_n=0&stake=100`

Backend: `match_count 0 · observations_in_window 4 · excluded_no_metric 4 ·
matches_in_window 108 · odds_source "none"` · financeiro `null`.

UI: estado próprio, *"Esta estatística não está disponível para a amostra selecionada"*,
sem nenhum card estatístico ou financeiro.

❌ **Defeito introduzido por mim, achado nesta validação.** O texto seguinte diz:

> O recorte tem **108** partida(s), mas o provedor não publicou esta métrica em
> **nenhuma** delas.

É **falso**: 53 das 108 partidas da Champions têm impedimentos. O recorte da equipe são
**4** observações. `sample-state` usa `matches_in_window` (contagem antes da regra,
incluindo o filtro de equipe) onde deveria usar `observations_in_window`. Os testes locais
não pegaram porque em nenhum deles havia filtro de equipe. Correção de uma linha,
**não aplicada** — esta passagem era só deploy, validação e registro.

### Zero real — Brasileirão 2026 · gols · "acima de 0"

`match_count 100 · 91 acertos · 9 erros`. As 9 partidas 0×0 aparecem na tabela com
**0**, "Erro", `−100` — estado `com-amostra`, nem vazio nem indisponível. Exemplo:
`match_id 16462`, 2026-07-23, Botafogo × Vitória, `total_goals 0`.

Financeiro com odd 1,20: `91 × 20 − 9 × 100 = 1.820 − 900 = 920`; `920 ÷ 10.000 = 9,2 %`.
UI: lucro 920, ROI 9,2 %. Bate.

### Coluna Mando

**Match-level** (escanteios, Brasileirão 2026): cabeçalhos
`Data · Mandante · Visitante · Escanteios · Resultado · Odd · Resultado (stake)`.
**Sem** coluna Mando. Primeira linha: `2026-07-16 | Vitoria | Vasco DA Gama | 5 | Erro | — | —`.

**Team-level** (vitória, Brasileirão 2026): cabeçalhos
`Data · Equipe · Adversário · Mando · Vitória · Resultado · Odd · Resultado (stake)`.
200 linhas: **100 Casa + 100 Fora**. Mesma partida 16442:

```
2026-07-16 | Vitoria       | Vasco DA Gama | Casa | Acerto | 1.8 fixa |  +80
2026-07-16 | Vasco DA Gama | Vitoria       | Fora | Erro   | 1.8 fixa | −100
```

A perspectiva é real e a coluna corresponde a ela.

### Accounting — identidade em 5 configurações de produção

| Config | obs. | eleg. | sem métrica | teto odd | sem odd | mando | outras | fecha |
|---|---|---|---|---|---|---|---|---|
| C1 Brasileirão · impedimentos > 2 | 100 | 87 | 13 | 0 | 0 | 0 | 0 | ✅ |
| C2 Brasileirão · vitória · só fora | 200 | 100 | 0 | 0 | 0 | 100 | 0 | ✅ |
| C3 Champions · chutes > 20 | 108 | 59 | 49 | 0 | 0 | 0 | 0 | ✅ |
| C4 Brasileirão · escanteios · max_odds 5 | 100 | 100 | 0 | 0 | 0 | 0 | 0 | ✅ |
| C5 Egnatia · impedimentos | 4 | 0 | 4 | 0 | 0 | 0 | 0 | ✅ |

C2 prova que o team-level conta **observações** (200) e não partidas (100).
UI de C1: *"Composição da amostra: 100 partidas no recorte · 87 analisadas · 13 fora:
13 sem a estatística publicada pelo provedor."*

### max_odds_applicable

**Caso A — sem odd aplicável.** C4: `max_odds_requested 5 · max_odds_applicable 0`. UI:
*"'Odds máximas' não teve efeito. O teto de 'odds máximas' (5) não foi aplicado a nenhuma
partida: nenhuma delas tem odd de mercado registrada no banco para comparar."* ✅

**Caso B — com odd aplicável.** **NA.** Varredura de todas as 20 liga/temporada ×
escanteios e vitória (40 consultas, 2.586 observações): `max_odds_applicable = 0` em
todas. Não existe amostra em produção. Nenhum dado foi criado. Coberto apenas por
`TestMaxOdds_TemEfeitoQuandoHaOddDeMercado` (local).

### Achado novo, pré-existente (não é regressão desta passagem)

Nos mercados de **resultado** (vitória, empate, não perde), a tabela mostra a coluna
"Vitória" com **0** em todas as linhas, e o card exibe **"Média de vitória 0"**.

Causa: `entryTotal()` e `averageValue()` em `filters.component.ts` não têm ramo para
esses mercados e caem no `default`, que devolve `total_corners`/`average_corners` — campos
que o backend **não preenche** para resultado. É ausência exibida como zero, no próprio
Simulador. Existe desde `fcd2854` (27/07/2026), quando os mercados de resultado entraram.
Não foi introduzido pelo REV-P3, mas afeta o item 7.

### Escanteios NULL → 0 (reafirmado, não corrigido)

- Origem provada: `internal/usecase/sync_usecase.go:119-124` converte `nil` em `0` antes de gravar.
- Schema: `home_corners/away_corners INT NOT NULL DEFAULT 0` (`migrations/001_init.sql`).
- Correção exige migration + `domain.Match.HomeCorners` → `*int` + propagação por Comparador (P2) e Dashboard (P1).
- Os zeros já gravados não guardam se eram ausência — não há como recuperar.

Pendência fora do fechamento imediato do P3.

### Regressão (commit `25931e5`)

| Comando | Resultado |
|---|---|
| `gofmt -l internal/ pkg/ cmd/` | vazio |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test -count=1 ./...` | 9 pacotes `ok` |
| `npx ng build --configuration production` | OK — 642,16 kB |
| `sample-state.spec.ts` (node:assert) | 13/13 |
| `simulator-url-state.spec.ts` (node:assert) | 11/11 |
| Runner de componente Angular | **NA — inexistente** |

Nenhuma regressão nos itens já aprovados: os números de CASO A (100 / 67 / 50 / 0,5 %) e
CASO B (69 / 35 / −1.650 / −23,91 %) seguem idênticos.

### Itens afetados

| # | Item | Antes | Agora | Evidência de produção |
|---|---|---|---|---|
| 11 | `metric_sample_size` | ❌ | ✅ | `accounting` em 5 configurações, identidade fecha em todas; UI "100 no recorte · 87 analisadas · 13 fora" |
| 16 | Geral/Casa/Fora | ⚠️ | ✅ | match-level sem coluna Mando; team-level 100 Casa + 100 Fora, par da partida 16442 |
| 32 | No-data explícito | ❌ | ✅ | La Liga 2025: mensagem explícita, nenhum card de 0 %, média 0, sequência, ROI ou drawdown; `odds_source none` |
| 33 | Metric-unavailable explícito | ⚠️ | ⚠️ | Estado e mensagem corretos, sem painel enganoso — **mas a frase de contagem é falsa com filtro de equipe** ("108 partidas… nenhuma delas") |
| 7 | Ausência ≠ zero | ⚠️ | ⚠️ | Continua: NULL→0 do pipeline **e** o novo achado dos mercados de resultado ("Média de vitória 0") |

### Status total dos 40 itens

- ✅ **33** — os 30 anteriores + 11, 16, 32
- ⚠️ **4** — 7, 29, 30, 33
- ❌ **0**
- NA **3** — 15, 18, 26 (justificativas na Fase E, inalteradas)

### O que impede o fechamento

| # | Bloqueio | Natureza |
|---|---|---|
| 33 | Frase de contagem usa `matches_in_window` em vez de `observations_in_window` | defeito meu, correção de 1 linha, dentro do escopo |
| 7 | (a) "Média de vitória 0" e coluna "Vitória 0" nos mercados de resultado | defeito pré-existente do Simulador, dentro do escopo |
| 7 | (b) escanteios NULL gravados como 0 na ingestão | fora do escopo — migration + P1/P2 |
| 29, 30 | provenance do save não validada em produção | exige sessão autenticada; só teste local |

## REV-P3 = PARCIAL — 33 ✅ / 4 ⚠️ / 0 ❌ / 3 NA. Bloqueiam: itens 7, 29, 30 e 33.

---

## REV-P3 — Correção dos itens 33 e 7 (Simulador)

> **Restaurada** — ver nota de restauração da seção anterior. Texto original, sem
> alteração. O código descrito aqui **foi** commitado: `28dc417` ("fix"), pelo Daniel.

Data: **25/09/2026**. Escopo: somente os dois defeitos funcionais restantes do próprio
Simulador. Escanteios NULL → 0 **não** foi tocado. P1/P2 não foram reabertos. Sem deploy.
**Nenhum arquivo de backend foi alterado** — as duas correções são de apresentação.

### Item 33 — contagem errada no estado metric-unavailable

**Causa.** `sample-state.ts` e o template usavam `accounting.matches_in_window` — partidas
da competição **antes** do filtro de equipe. Medido em produção (Champions 2026, Egnatia
Rrogozhinë, impedimentos): `matches_in_window 108`, `observations_in_window 4`,
`excluded_no_metric 4`. A tela dizia "O recorte tem 108 partida(s), mas o provedor não
publicou esta métrica em nenhuma delas" — falso: 53 das 108 têm o dado.

**Correção.**
- `estadoAmostra()` decide `metrica-ausente` por `observations_in_window > 0`
  (antes `matches_in_window > 0`). Equipe sem nenhuma observação no recorte passa a ser
  estado vazio, que é o que ela é.
- Nova função pura `mensagemMetricaAusente(r)`: usa `observations_in_window` e
  `excluded_no_metric`, ambos do recorte efetivo. Unidade: "partidas" em regra
  match-level (onde observação = partida, inclusive com filtro de equipe);
  "observações" em team-level.
- Se nem todas as observações saíram por falta de métrica (parte por mando, por exemplo),
  a frase **não** afirma "nenhuma delas" — separa os números.

### Item 7 — mercados de resultado exibindo zero artificial

**Causa.** `entryTotal()` e `averageValue()` em `filters.component.ts` resolviam o valor
por `switch (this.metric)` com `default` em escanteios. Vitória, empate e não perde
caíam no `default` e liam `total_corners`/`average_corners`, que o backend não preenche
nesses mercados. Medido em produção: coluna "Vitória" = 0 em 200/200 linhas e card
"Média de vitória 0". Existe desde `fcd2854` (27/07/2026).

Segundo defeito na mesma função, achado na investigação: ela lia `this.metric` — o
**formulário**, não a métrica com que o resultado foi gerado. Trocar a métrica depois de
rodar fazia a tabela antiga ler outro campo.

**Correção — classificação centralizada** em `sample-state.ts`:

```
tipoMercado(metric)  → 'numerico'   : corners, goals, offsides, shots, shots_on_target
                     → 'categorico' : win, draw, win_or_draw ("Não perde")
valorObservado(metric, e) → número, ou null em categórico
mediaObservada(r)         → número, ou null em categórico
rotuloMetrica(metric)     → rótulo; métrica desconhecida devolve o próprio código, nunca "Escanteios"
```

Todas recebem a métrica **do resultado** (`r.metric`). No componente, `entryTotal`,
`entryLabel`, `averageValue` e `averageLabel` foram removidos.

- **Tabela:** `colunasDaTabela()` só inclui a coluna numérica em mercado numérico. Em
  categórico fica a coluna "Resultado" (Acerto/Erro), que já é a observação.
- **Card de média:** renderizado só quando `mediaObservada(r) !== null`. A comparação é
  explícita com `null` porque `@if (x; as y)` trataria média 0 como ausente — e média
  observada 0 é zero real.
- **Não** se converteu booleano em 0/1 para fabricar uma "média de vitória". A proporção
  de acertos já existe e se chama taxa de acerto; não foi renomeada para probabilidade.

### Arquivos

`frontend/src/app/features/filters/sample-state.ts` · `sample-state.spec.ts` ·
`filters.component.ts` · `filters.component.html`.

### Testes — `sample-state.spec.ts` (node:assert), 28/28

Novos (15): 33.1 filtro de equipe diz 4 e não contém 108 · 33.2 sem filtro de equipe
continua correto · 33.3 team-level fala em observações · 33.4 exclusão mista não afirma
"nenhuma delas" · 33.5 equipe sem observação é estado vazio · 7.1 classificação
centralizada · 7.2–7.4 vitória/empate/não perde com `valorObservado` null · 7.5
categóricos com `mediaObservada` null mesmo com `average_corners: 0` · 7.6 taxa de acerto
preservada · 7.7 zero real numérico continua 0 · 7.8 numéricas leem o campo certo · 7.9
métrica desconhecida não vira "Escanteios" · 7.10 match/team-level seguem corretos.

### Validação local com payloads REAIS de produção

| Caso | Antes (produção) | Depois (lógica nova, dados reais) |
|---|---|---|
| **A** Egnatia · impedimentos | "O recorte tem **108** partida(s)… nenhuma delas" | "O recorte selecionado tem **4** partidas, mas o provedor não publicou esta métrica em nenhuma delas." |
| **B** Brasileirão · vitória | coluna "Vitória = 0" em 200/200; card "Média de vitória 0" | coluna numérica fora; card não renderizado; taxa 35 % (70/130) preservada; Mando presente |
| **C** Brasileirão · empate | mesmo caminho de código que B (*derivado, não observado na UI*) | mesma garantia; taxa 30 % (60/140) preservada |
| **D** Brasileirão · gols > 0 | 9 partidas 0×0 exibidas como 0 | inalterado: `valorObservado` = **0**, média 2,68, estado `com-amostra` |

Checagem cruzada: vitória 70 acertos + 30 empates (60 ÷ 2) = 100 partidas.

### Regressão

`gofmt` vazio · `go build`/`vet` OK · `go test -count=1` 9 pacotes ok · `ng build`
642,16 kB · `sample-state.spec.ts` 28/28 · `simulator-url-state.spec.ts` 11/11 ·
runner Angular **NA — inexistente**.

### Observações registradas, não corrigidas

1. **`metric` vazio em resultado sem ocorrências** — com `match_count 0` o backend devolve
   `"metric": ""`; `buildBacktestResult` retorna antes de preencher. Sem efeito visível hoje.
2. **`average_corners: 0` no JSON dos mercados de resultado** — a tela não exibe mais, mas o
   contrato envia 0 onde não há medida. Corrigir exige médias nuláveis no backend, que
   também alimentam Engine e Discovery.
3. **Escanteios NULL → 0** — backlog estrutural separado, inalterado.

### Impacto na DoD

| # | Antes | Agora |
|---|---|---|
| 33 | ⚠️ | ⚠️ — corrigido localmente, aguardando produção |
| 7 | ⚠️ | ⚠️ — parte do Simulador corrigida localmente; parte do pipeline segue como backlog |

## REV-P3 = PARCIAL — itens 7 e 33 corrigidos localmente, aguardando deploy final e validação de provenance

---

## REV-P3 — Validação final: itens 7, 33, 29 e 30

Data: **25/09/2026**. Escopo: validar em produção as correções dos itens 7 e 33 e,
com sessão autenticada, os itens 29 e 30. Os demais itens mantêm a evidência anterior.

### Deploy

| Item | Valor |
|---|---|
| Branch | `main` |
| Código dos itens 7 e 33 | `28dc417` ("fix") — commit e push feitos pelo Daniel |
| Restauração do registro | `84f6a8e` — commit meu, push feito pelo Daniel |
| origin/main | `84f6a8e` |
| Backend | sem alteração nesta passagem (último deploy de backend: `25931e5`) |
| Frontend | **no ar** — a mensagem nova do item 33 aparece em produção |
| Migration | nenhuma |
| Erros | nenhum |

⚠️ Mensagem do commit `28dc417` ("fix") não descreve a mudança.

### Item 33 — produção

**Com filtro de equipe, métrica totalmente ausente.**
`/filtros?league=23&seasons=37&team=4748&metric=offsides&threshold=1&last_n=0&stake=100`
(Champions 2026, Egnatia Rrogozhinë, impedimentos). Backend: `matches_in_window 108 ·
observations_in_window 4 · excluded_no_metric 4`. UI:

> O recorte selecionado tem **4** partidas, mas o provedor não publicou esta métrica em
> nenhuma delas.

"108" não aparece na página.

**Exclusão mista.** Mesmo caso com `home_away=home`: `observations 4 · excluded_by_venue 2
· excluded_no_metric 2`. UI:

> O recorte selecionado tem 4 partidas; em 2 delas o provedor não publicou esta métrica,
> e as demais ficaram de fora por outros critérios.

Não afirma "nenhuma delas".

**Sem filtro de equipe — sem regressão.** Brasileirão 2026, impedimentos > 2: "100
partidas no recorte · 87 analisadas · 13 fora: 13 sem a estatística publicada pelo
provedor"; tabela com 87 linhas, média de impedimentos 3,02.

### Item 7 — produção (parte do Simulador)

Brasileirão 2026, tabela renderizada no navegador (200 linhas por mercado):

| Mercado | Cabeçalhos | Card de média | Taxa de acerto | Mando |
|---|---|---|---|---|
| vitória (odd 1,80) | Data · Equipe · Adversário · Mando · Resultado · Odd · Resultado (stake) | ausente | 35 % (70/130) | 100 Casa + 100 Fora |
| empate (odd 3,20) | idem | ausente | 30 % (60/140) | 100 + 100 |
| não perde (odd 1,30) | idem | ausente | 65 % (130/70) | 100 + 100 |

Não aparecem coluna numérica "Vitória/Empate/Não perde" nem "Média de vitória/empate/não
perde". Acerto/Erro na partida 16442 (vitória do mandante): vitória Casa Acerto / Fora
Erro; empate Erro / Erro; não perde Casa Acerto / Fora Erro. Consistência:
não perde 130 = vitória 70 + empate 60.

Financeiro conferido: empate `60 × 220 − 140 × 100 = −800` → −4 %; não perde
`130 × 30 − 70 × 100 = −3.100` → −15,5 %. UI idem.

**Zero real numérico preservado.** Gols > 0: 100 linhas, **9** com "0" (ex.:
`2026-07-23 | Botafogo | Vitoria | 0 | Erro | 1.2 fixa | −100`), média de gols 2,68.

### Itens 29 e 30 — provenance autenticada

**Como foi obtida a evidência.** Eu não faço login com senha. O Daniel executou os
passos na **sessão dele, no Chrome**, por meio da extensão Claude in Chrome, com um roteiro
que eu escrevi: salvar pela tela do Simulador e depois ler a API. As respostas JSON abaixo
foram coletadas lá e coladas aqui. São evidência de produção, mas **não observadas
diretamente por mim**. O token não foi exibido em nenhum momento.

**A — salva pela tela do Simulador** (Brasileirão 2026, escanteios > 8, odd 1,50, stake 100):

```json
{"id":43,"owner_id":3,"name":"REV-P3 teste provenance A — pode excluir",
 "description":"Criada a partir do Simulador de Filtros (Escanteios)",
 "origin":"simulator","visibility":"private","active":true,"favorite":false,
 "created_at":"2026-09-25T21:43:13.582924Z"}
```
Bundle: `{strategy, backtests: []}` — sem `health`, sem `scores`.

**B — tentativa de forjar `origin:"discovery"` via `POST /api/v1/strategies`:**

```json
{"status_http":201,
 "resposta":{"id":44,"owner_id":3,"name":"REV-P3 teste provenance B — pode excluir",
  "origin":"user","visibility":"private","active":true,"favorite":false,
  "created_at":"2026-09-25T21:43:34.391814Z"}}
```

O servidor **aceitou a requisição e ignorou a origem forjada**: gravou `user`, não
`discovery`. Bundle também só com `{strategy, backtests: []}`.

**Na tela `/estrategias`:** A com etiqueta **"Do simulador"**, B sem etiqueta. No
detalhe de B, DSFR Score, Health, Ciclo de vida, Confiança, Robustez, Volatilidade, Risco
e Ranking aparecem "—", e o histórico de execuções está em 0. As 5 estratégias
anteriores continuam como "Pausada"/"Descoberta".

**Significado de `active: true`, provado.** O contrato de `Strategy` não tem nenhum campo
de etapa, validação ou aprovação (nenhum `stage`, `validated` ou `approved`). O que
indicaria avaliação quantitativa — `backtests`, `health`, `scores` — está vazio ou
ausente nas duas. Portanto `active` só quer dizer "aparece na lista do usuário"; salvar
não roda backtest, não gera score e não promove nada.

As estratégias 43 e 44 continuam na conta do Daniel, com "pode excluir" no nome. Eu não
as excluí.

**Observação.** No Passo 1 a extensão registrou 277 partidas encontradas, contra 100 nas
validações anônimas. É o esperado: usuário com assinatura não tem o limite de 90 dias.

### Achado de usabilidade (fora dos 40 itens)

**O Simulador não tem formulário de login.** O formulário só existe em Gestão de Banca,
Assinatura e Projeções. Um usuário anônimo que queira salvar uma estratégia pelo Simulador
não encontra onde entrar — foi o que travou esta validação. Registrado como backlog, não
corrigido.

### Regressão (commit `84f6a8e` = origin/main, árvore limpa)

| Comando | Resultado |
|---|---|
| `gofmt -l internal/ pkg/ cmd/` | vazio |
| `go build ./...` · `go vet ./...` | OK · OK |
| `go test -count=1 ./...` | 9 pacotes `ok` |
| `npx ng build --configuration production` | OK — 642,16 kB |
| `sample-state.spec.ts` | 28/28 |
| `simulator-url-state.spec.ts` | 11/11 |
| Runner de componente Angular | **NA — inexistente** |

### Reavaliação dos itens 7, 29, 30 e 33

| # | Item | Antes | Agora | Evidência |
|---|---|---|---|---|
| 33 | Metric-unavailable explícito | ⚠️ | ✅ | Egnatia: "tem 4 partidas", sem 108; exclusão mista sem "nenhuma delas"; sem filtro de equipe inalterado |
| 29 | Save com provenance correto | ⚠️ | ✅ | id 43: `origin "simulator"`, `private`, `owner_id 3`; etiqueta "Do simulador" |
| 30 | Save não vira validada | ⚠️ | ✅ | id 44: `origin "discovery"` forjado gravado como `user`; nenhum campo de aprovação; `backtests []`, sem health/scores; scores "—" na UI |
| 7 | Ausência ≠ zero | ⚠️ | ⚠️ | Parte do Simulador **resolvida** (categóricos sem zero artificial; estados vazio/indisponível; odd e financeiro nulos). **Resta** escanteios NULL → 0 (abaixo) |

### Por que o item 7 continua ⚠️

A correção que dependia do Simulador foi feita e validada. O que resta nasce na
**ingestão**: `sync_usecase.go:119-124` grava 0 quando o provedor não publica
escanteios, e o schema é `NOT NULL DEFAULT 0`. O Simulador **continua exibindo** esses
valores como zero observado — em produção, a partida 16451 (2026-07-21, Atlético-MG ×
Bahia) aparece com 0 escanteios totais, e a 16476 com 1.

Isso é backlog estrutural separado: migration, `*int` no domínio, propagação por P1/P2, e
histórico irrecuperável. Não foi reaberto, conforme instruído. Mas o item diz "ausência ≠
zero", e para escanteios isso **ainda não é verdade** na tela. Marcá-lo ✅ seria forçar o
fechamento.

### Novo total dos 40 itens

- ✅ **36** — os 33 anteriores + 29, 30, 33
- ⚠️ **1** — 7
- ❌ **0**
- NA **3** — 15 (produzido/concedido é do Comparador), 18 (não há amostra `synthetic` em
  produção), 26 (scores são do Strategy Engine)

### Status final

**Único bloqueio:** item 7, na forma do backlog estrutural "escanteios NULL → 0". Não é
defeito do Simulador corrigível dentro do escopo autorizado, mas afeta o que o Simulador
mostra.

Se o Daniel decidir **aceitar formalmente** essa limitação como fora do REV-P3 e movê-la
para um item próprio, os 39 itens restantes estão sustentados por evidência de produção
(36 ✅ + 3 NA justificados). Essa é uma decisão de escopo que cabe a ele — não a mim.

## REV-P3 = PARCIAL — 36 ✅ / 1 ⚠️ / 0 ❌ / 3 NA. Único bloqueio: item 7 (escanteios NULL → 0, backlog estrutural fora do escopo autorizado).

---

# REV-P4 — Discovery

## REV-P4 — Fase A (investigação)

Data: **25/09/2026**. Commit auditado: `374dbc3` (= origin/main, árvore limpa).
Nenhum código foi alterado. Três experimentos temporários rodaram localmente e foram
removidos (`git status` limpo ao final).

### 1. Arquitetura atual

| Camada | Arquivo | Papel |
|---|---|---|
| Grade | `internal/usecase/discovery/combinations.go` | `generateCombos` — espaço de busca |
| Motor | `internal/usecase/discovery/engine.go` | `RunAll` → `RunLeague` → `mine` → `applyFDR` → `validate` → `publish` → `DeactivateDiscoveredExcept` |
| Critérios | `internal/usecase/discovery/criteria.go` | limiares do doc 08 + parâmetros AUD-003 |
| Estatística | `internal/usecase/discovery/validation.go` | `splitHistory`, `pValue`, `checkHoldout` |
| Fórmulas | `internal/formulas/significance.go`, `scores.go` | binomial, BH/BY, DSFR v1.1 |
| Backtest | `usecase.FilterUsecase` (o mesmo do Simulador), com repositórios memoizados (`cache.go`) |
| Persistência | `repository/postgres/strategy_repo.go` | `UpsertDiscovered`, `DeactivateDiscoveredExcept`, `ListDiscovered` |
| Scores | `usecase/strategyengine/engine.go` | `PersistResult`, `scoresRow` (DSFR v1.1), `healthRow` |
| Migration | `012_discovery_engine.sql` | índice único parcial por `name` onde `origin='discovery'` |
| Worker | `cmd/worker/main.go` | `runStrategyDiscovery` (IncludeTeams=**true**); `runStrategies` (reavaliação diária) |
| API | `handlers/discovery_handler.go` | `GET /discovery/strategies` (público), `GET /discovery/progress` (público), `POST /discovery/run` (autenticado) |
| UI | `frontend/.../features/discovery/` | ranking, botão "Procurar agora", resumo da última varredura |

### 2–3. Grade atual e número real de combinações

```
Escanteios : linhas {6,7,8,9,10} × mando {"",home,away} × janela {0,10,20} × teto de odd {1.70,2.20,3.50}
           = 5 × 3 × 3 × 3 = 135
Resultado  : mercados {win,draw,win_or_draw} × mando {3} × janela {3} = 27
Total/liga = 162
```

**O "162" da UI vem daqui**: 135 de escanteios + 27 de mercados de resultado, que entraram
depois do corte 540 → 135. A UI exibe `LeagueResult.Combinations`, ou seja, as combinações
**geradas** — não as testadas.

**O worker usa uma grade maior que a API.** `cmd/worker` liga `IncludeTeams: true`
(soma 5 × 3 × 3 = **45 combinações por equipe**), e a API ("Procurar agora") usa `false`.
Uma liga com 20 equipes tem 162 combinações pela API e **1.062** pelo cron.

**Opponent tier:** eixo removido da grade (AUD-004) ✅. Resta código morto: `combo.tier`,
`Definition.OpponentTier` e `FilterCriteria.OpponentTier` são preenchidos com string vazia
e nunca variam.

### 4. Fluxo estatístico

1. `AllMatches` — só `status = 'FINALIZADO'`, ordenado por `match_date ASC`.
2. `splitHistory(TrainFraction = 0,70)` — corte pelo **quantil da contagem** de partidas;
   **todas** as partidas da data do corte vão para a validação; janelas meio-abertas
   `treino = [primeira, corte)` e `validação = [corte, ∞)`; `inWindow` implementa
   `[From, To)` ✅. Sem partidas futuras no holdout, porque só entra `FINALIZADO` ✅.
3. `mine` — backtest de cada combinação **só na janela de treino**, com
   `RequireRealOdds = true` ✅. Em seguida, **nesta ordem**: `crit.validate` (amostra ≥ 100,
   win rate ≥ 75 %, ROI ≥ 10 %, Yield ≥ 5 %, lucro > 0, drawdown ≤ 20 % do apostado), DSFR ≥ 40,
   e p-valor calculável.
4. `applyFDR` — Benjamini–Yekutieli, q = 0,10, **sobre os que sobraram do passo 3**.
5. `validate` — reexecuta na janela de validação: amostra ≥ 30, lucro > 0, p ≤ 0,05
   (sem correção de multiplicidade).
6. `publish` — ordena por DSFR, teto de 40 por liga, grava `origin='discovery'`,
   `visibility='public'`, `active=true` e persiste o backtest **da janela de treino**.
7. `DeactivateDiscoveredExcept` — as demais descobertas da liga viram `active = FALSE`
   (**não** são apagadas) ✅.

Fórmulas conferidas: `BinomialAtLeast` é a cauda exata em log ✅; `FDRThreshold` faz o
step-up de BH, com fator `1/H(m)` no BY ✅; hipótese nula `p0 = média(1/odd)` do próprio
backtest.

### 5. Odds / procedência (AUD-001)

`RequireRealOdds = true` na mineração ✅, na validação ✅ e na reavaliação
(`strategyengine.Definition.criteria()`) ✅. Odd `synthetic` e odd ausente são rejeitadas:
provado localmente com as duas situações — **162/162 combinações rejeitadas, 0 publicadas**.

Produção: `GET /discovery/strategies` → `count: 0`. Na varredura do REV-P3 (20
liga/temporada, 2.586 observações, janela de 90 dias), `max_odds_applicable = 0` em todas.
**Discovery financeiro publicando zero é o resultado correto hoje, e nada o contorna.**

### 6. FDR / múltiplas comparações

#### ❌ Bug comprovado B1 — o FDR recebe um m selecionado pelo próprio resultado

`applyFDR` roda sobre `passed`, que só contém combinações que **já** passaram por win
rate, ROI, Yield, drawdown e DSFR — todos funções do mesmo resultado que gera o p-valor.
Isso é inferência seletiva: o `m` fica artificialmente pequeno e o procedimento deixa de
controlar o FDR. E `LeagueResult.Tested` é documentado como "combinações com p-valor
calculável", mas conta só esses sobreviventes.

**Medido** (experimento local, ruído puro com odds justas, **critérios de produção**,
200 seeds × 700 partidas):

| | m médio no FDR | "significativas" falsas | seeds com ≥ 1 | validadas | publicadas |
|---|---|---|---|---|---|
| Atual | **0,75** | 132 | **17/200 (8,5 %)** | 0 | 0 |
| m honesto (todas as testáveis que só passaram no filtro de amostra) | 70 | 27 | 2/200 (1 %) | — | — |

A camada de FDR deixa passar cerca de **8× mais falsos positivos** que o procedimento
correto. **Hoje a proteção contra publicar ruído depende inteiramente do holdout** (0
validadas em 200 seeds). Não tem impacto atual em produção, porque sem odd real nada chega
lá, mas é **latente**: vale no dia em que entrarem odds reais.

O filtro de amostra mínima pode ficar antes do FDR, porque não depende do resultado
("independent filtering"). Win rate, ROI, Yield, drawdown e DSFR não podem.

### 7. Teste de ruído

O teste existente `TestCicloSobreRuidoPuroNaoPublicaNada` (5 seeds × 700 partidas) **passa**,
mas usa `permissiveDocCriteria()` — o que é intencional para exercitar o FDR, mas **não
exercita o B1**, que só aparece com os critérios de produção. O experimento acima (200 seeds,
critérios de produção) publicou **0**, mas também mostrou o FDR falhando no meio do caminho.

### 8. Dupla contagem e janela

Discovery usa o mesmo motor do Simulador: a regra de partida gera uma observação por
partida ✅ (sem 200/100 nem 138/69).

#### ❌ Bug comprovado B2 — regressão com origem no REV-P3: `LastNGames` em regra de partida usa só jogos EM CASA

Com a deduplicação da correção 1 do REV-P3, a regra de partida gera só o candidato do
mandante (`asHome`). O bloco `LastNGames` agrupa candidatos por `teamID` — que passou a ser
sempre o mandante. Então "últimos N jogos de cada equipe" virou **"últimos N jogos em casa
de cada equipe"**.

Prova (experimento local): duas equipes, 8 jogos alternando o mando, `last_n = 2`.

| Regra | Esperado | Obtido |
|---|---|---|
| Escanteios (partida) | partidas **7, 8** | partidas **5, 6, 7, 8** — as 5 e 6 não estão entre os 2 últimos de ninguém |
| Vitória (equipe) | 7, 8 nas duas perspectivas | 7, 8 nas duas perspectivas ✅ |

Afeta **o eixo de janela 10/20 do Discovery** (2/3 da grade de escanteios) e **o Simulador**,
cujo padrão na tela é `last_n = 10`. O REV-P3 foi declarado RESOLVIDO sem teste de janela
em regra de partida; é uma regressão, e corrigi-la mexe em código do P3.

### 9. Amostra

| Onde | Unidade | Limite |
|---|---|---|
| treino | observações (= partidas em escanteios; 2 por partida em resultado sem equipe) | `MinGames` 100 (trava absoluta 50) |
| validação | observações | `HoldoutMinGames` 30 |
| filtro de amostra | `MatchCount` do backtest | — |

Com `RequireRealOdds` e nenhuma odd real, a amostra é 0 em toda combinação.

### 10–12. Financeiro, EV, ROI/Yield

Todos os filtros de ROI, Yield, lucro e drawdown exigem `FinancialsAvailable` e ponteiros
não nulos ✅. EV não existe no contrato nem no score ✅ (AUD-002). Os gates de ROI e Yield
continuam medindo a mesma quantidade: o código diz isso em voz alta, e só o de ROI
efetivamente barra.

### 13. Drawdown (AUD-006 — registrado, não corrigido)

Duas normalizações **opostas** no mesmo pipeline:

- filtro do Discovery: `MaxDrawdown / TotalStaked` — **encolhe** com o tamanho da amostra;
- score DSFR: `1 − MaxDrawdown / drawdownCapStk` (stakes absolutos) — **piora** com o tamanho
  da amostra, enquanto o componente de amostra melhora.

Não bloqueia a validade da publicação hoje. Continua como AUD-006.

### 14. DSFR Score

`formulas.DSFRScoreV11`: ROI **30**, Win Rate **20**, Drawdown **15**, Amostra **15**,
Consistência **15**, Variância **5** ✅. Sem EV, sem Yield ✅. É o único chamado
(`strategyengine/engine.go:263`).

**AUD-007 continua aberto:** `InvVariance = 1 − 4p(1−p)` é função pura do win rate, e
`ConsistencyIndex` é composto de win rate, InvVariance, InvDrawdown e amostra. O peso
efetivo do win rate passa de 20 para cerca de 34 pontos.

### 15. Persistência

Por estratégia descoberta ficam: `origin='discovery'`, `visibility='public'`, `active`,
`created_at`, o backtest **da janela de treino**, health e scores. **Não existem colunas**
para janela de treino, janela de validação, p-valor, limiar FDR, número de testes ou
procedência da odd: essa evidência só aparece **como texto** na `description`.
Desativadas continuam no banco (`active = FALSE`) ✅.

### 16. Publicação

Exige, em sequência: split possível → treino com odd real → critérios do doc 08 →
DSFR ≥ 40 → p-valor calculável → sobreviver ao BY → holdout (≥ 30, lucro > 0, p ≤ 0,05) →
top 40 por DSFR. Win rate alto sozinho não publica ✅. O B1 enfraquece o passo do BY.

### 17. Reavaliação

`runStrategies` roda **todo dia** sobre todas as estratégias ativas, usando
`Definition.criteria()`, **sem `DateFrom`/`DateTo`** — ou seja, o histórico inteiro,
**incluindo a janela de treino onde o padrão foi garimpado**. A limitação histórica
continua. Além disso, a reavaliação grava novo backtest, health e scores por cima, e a tela
passa a mostrar a mistura; a evidência de descoberta (holdout) sobrevive só no texto.

Efeito colateral: as estratégias de usuário e do Simulador também são reavaliadas com
`RequireRealOdds`. Sem odd real, `PersistResult` recusa ("sem série financeira completa") e
cada uma vira **1 erro por dia** no `worker_runs` do strategy engine. Isso é do domínio do
P5; fica registrado.

### 18–19. Zero descobertas e funil de rejeição

A tela trata zero como resultado legítimo ✅ ("Nenhuma estratégia aprovada por aqui").

#### ❌ Bug comprovado B3 — o funil atribui a causa errada

Sem odd real (situação de produção), **162/162 combinações são rejeitadas como
`amostra_insuficiente`**, porque `RequireRealOdds` zera a amostra antes de qualquer teste.
A causa real — ausência de odd de mercado — não aparece. A mensagem de vazio da tela
("nenhuma combinação atingiu amostra suficiente, retorno, consistência e drawdown")
reforça a leitura errada. E o comentário de `combinations.go`, que diz que os mercados de
resultado caem em `sem_pvalor_calculavel`, é **falso**: caem antes, também como
`amostra_insuficiente`.

O `accounting` do REV-P3 (`excluded_no_odd`) já tem a informação para corrigir isso sem
inventar contagem.

**Persistência do funil:** `worker_runs.details` do Discovery guarda só ligas, combinações,
publicadas e desativadas — **sem** testadas, significativas, validadas nem motivos. O
resultado da varredura manual fica só em memória (`progress.Tracker`) e some a cada deploy.
**Nenhum endpoint expõe `worker_runs`.**

### 20. UI

- Linguagem: "padrão identificado", disclaimer "não constituem recomendação de aposta" ✅.
- ⚠️ "combinações testadas" mostra as **geradas** (162), não o `m` do teste.
- ⚠️ O texto "Cada combinação roda um backtest sobre o histórico completo" é **falso**: a
  mineração usa só os 70 % mais antigos.
- A tela não mostra o funil (testadas → significativas → validadas).

### 21. "Procurar agora"

| Pergunta | Resposta |
|---|---|
| Dispara processamento pesado? | Sim — todas as ligas (ou uma, se escolhida), 162 combinações por liga, timeout de 30 min |
| Síncrono? | Não — goroutine **dentro do processo da API** (web service), resposta 202 imediata |
| Mesmo pipeline? | Sim — `engine.RunLeague`/`RunAll`, mas com `IncludeTeams=false` (o cron usa `true`) |
| Publica? | **Sim** — publica e desativa estratégias **públicas** |
| É admin? | **Não** — basta estar autenticado; o botão aparece para qualquer usuário logado |
| Pode sobrecarregar produção? | Sim — roda no mesmo processo que atende o site; a única proteção é a trava de execução única (409) |

#### ⚠️ B4 — qualquer usuário autenticado altera o catálogo público

Não há papel de admin na rota. Um usuário comum pode disparar a varredura e, com isso,
publicar ou desativar descobertas que todos veem.

### 22. Endpoints

| Rota | Auth | Paginação | Observação |
|---|---|---|---|
| `GET /discovery/strategies` | pública | só `limit` (padrão 50), sem offset | filtro `league_id` |
| `GET /discovery/progress` | pública | — | estado em memória |
| `POST /discovery/run` | autenticada, sem papel | — | ver B4 |

### 24. Casos reais — o que foi possível provar sem acesso ao banco

- O Discovery roda **por liga, somando todas as temporadas** (`seasonIDs` vazio = todas).
  "La Liga 2026" e "La Liga 2025" **não são casos separados** para o motor: entram juntas
  e o corte de 70 % é feito sobre o histórico combinado.
- Odds reais na janela de 90 dias: **0** em todas as ligas (2.586 observações).
- Descobertas publicadas em produção: **0**.
- Contagem de partidas por temporada, data de corte, tamanho de treino/validação e odds
  reais no **histórico completo**: **NA — exige leitura do banco.** O usuário anônimo tem
  o limite de 90 dias, e eu não faço login no console do Neon. Consultas prontas, só de
  leitura, abaixo.
- Se o Discovery agendado **roda de fato** em produção (`DISCOVERY_RUN=true` está no
  `render.yaml`, mas as variáveis do cron foram configuradas pelo painel no REV-P0):
  **NA — só `worker_runs` responde**, e nenhum endpoint o expõe.

```sql
-- (1) Partidas e procedência das odds por liga/temporada (histórico completo)
SELECT league_id, season_id,
       count(*)                                              AS partidas,
       count(*) FILTER (WHERE odds_source = 'real')          AS odds_real,
       count(*) FILTER (WHERE odds_source = 'synthetic')     AS odds_sinteticas,
       count(*) FILTER (WHERE corner_odds::text <> '{}')     AS com_corner_odds,
       count(*) FILTER (WHERE result_odds::text <> '{}')     AS com_result_odds,
       min(match_date) AS primeira, max(match_date) AS ultima
FROM matches
WHERE status = 'FINALIZADO' AND league_id IN (8, 18)
GROUP BY league_id, season_id ORDER BY league_id, season_id;

-- (2) Data de corte 70/30 por liga, como o motor calcula (todas as temporadas juntas)
SELECT league_id, count(*) AS partidas,
       (array_agg(match_date ORDER BY match_date))[floor(count(*) * 0.7)::int + 1] AS corte_aprox
FROM matches WHERE status = 'FINALIZADO' AND league_id IN (8, 18)
GROUP BY league_id;

-- (3) O Discovery agendado roda?
SELECT id, worker, status, started_at, duration_ms, processed, errors, details
FROM worker_runs WHERE worker IN ('discovery', 'strategy')
ORDER BY id DESC LIMIT 10;

-- (4) Estado do catálogo de descobertas
SELECT active, visibility, count(*) FROM strategies
WHERE origin = 'discovery' GROUP BY active, visibility;
```

(A consulta 2 aproxima o corte; o motor ainda recua até a primeira partida da mesma data.)

### 12. Bugs comprovados — resumo

| # | Gravidade | Defeito | Impacto hoje |
|---|---|---|---|
| **B1** | alta (latente) | FDR aplicado depois de filtros dependentes do resultado; `Tested` mal rotulado | 0 — nada chega ao FDR sem odd real. Vale quando entrarem odds reais |
| **B2** | alta | `LastNGames` em regra de partida usa só jogos em casa — **regressão do REV-P3** | eixo de janela do Discovery e **Simulador em produção** (padrão `last_n = 10`) |
| **B3** | média | funil atribui tudo a `amostra_insuficiente` quando a causa é falta de odd real; comentário falso | usuário lê a causa errada |
| **B4** | média | "Procurar agora" sem papel de admin, publica/desativa o catálogo público, roda no processo da API | risco operacional e de integridade |
| B5 | média | reavaliação usa o histórico inteiro (inclui treino) e sobrescreve o backtest de descoberta | mistura evidência com monitoramento |
| B6 | baixa | texto "backtest sobre o histórico completo" falso | UI |
| B7 | baixa | "combinações testadas" = geradas | UI |
| B8 | baixa | funil não persistido; `worker_runs` sem endpoint | auditoria só com SQL |
| B9 | baixa | código morto de `tier`; comentário de `maxOddsOptions` fala em "odd 1.0" (pré-REV-P3) | manutenção |
| B10 | baixa | `threshold > 0` rejeitaria p-valor 0 por underflow | teórico |

### 13. Limitações (não são bugs desta fase)

- AUD-006 (drawdown com normalizações opostas) e AUD-007 (win rate contado ~34 %) — abertos.
- `p0 = média(1/odd)` aproxima a Poisson-binomial por uma binomial; com o overround da casa
  tende a ser conservador, mas não é exato.
- O holdout testa cada candidato a α = 0,05 sem correção; os candidatos costumam ser muito
  correlacionados (no experimento, ~8 por seed quando algum passou).
- Evidência de descoberta não estruturada (só texto).
- Casos reais do histórico completo pendentes das consultas acima.

### 14. Plano mínimo de correção (proposto, não executado)

Em ordem, uma correção de cada vez:

1. **B2** — em regra de partida, escolher a janela por equipe usando **as duas
   perspectivas** e só então deduplicar por `match_id`. Teste: o caso das 8 partidas
   alternadas precisa devolver {7, 8}. **Mexe no motor do REV-P3 e precisa de autorização
   explícita.**
2. **B1** — calcular o p-valor de **toda** combinação testável que passar só no filtro de
   amostra; aplicar o BY sobre esse `m`; aplicar os critérios do doc 08 **depois**, aos
   sobreviventes. `Tested = m` real. Teste: o experimento de 200 seeds com critérios de
   produção vira teste permanente.
3. **B3 + B8** — motivo `sem_odd_real` quando `accounting.excluded_no_odd` responde por toda
   a amostra; gravar o funil completo em `worker_runs.details`; corrigir o comentário.
4. **B4** — decisão de produto: restringir `POST /discovery/run` a admin, ou tirar a
   publicação do disparo manual.
5. **B6/B7** — textos da UI.
6. **B5** — separar evidência de descoberta de monitoramento; provavelmente escopo do P5.

Fora do plano: AUD-006 e AUD-007 globais, e integração de odds reais.

## REV-P4 = PARCIAL — Fase A concluída

---

## REV-P4 — Fase B (implementação)

Data: **25/09/2026**. Base: `f3adffe`. Ordem cumprida: **B2 → B1 → B3 → B4 → textos/UI**,
uma causa por vez, cada uma com teste antes de passar à seguinte.

Autorizações usadas: (1) reabrir **somente** o ponto do REV-P3 necessário ao B2;
(2) "Procurar agora" restrito a administrador.

### REV-P3 — regressão pontual descoberta no REV-P4 (B2 e B2b)

Único trecho do P3 reaberto: o bloco de `LastNGames` em `usecase/filter_usecase.go`.
Nenhum outro item do P3 foi tocado.

**B2 — janela em regra de partida.** Causa: a correção 1 do REV-P3 passou a gerar só
o candidato do mandante em regra de partida; o agrupamento por equipe da janela
enxergava então só jogos em casa. Correção: com janela, a regra de partida gera as
duas perspectivas **só para escolher a janela**; em seguida volta a uma observação
por partida (representante canônico: mandante), e só quando não há mando pedido nem
equipe selecionada. Não é dedup cego.

Semântica fixada:

- **Regra de partida:** janela por equipe, **independente de mando**; a partida entra
  se estiver entre as últimas N de qualquer das duas equipes, e entra uma vez.
- **Regra de equipe:** inalterada, perspectiva de cada equipe.
- **Mando pedido:** filtra **depois** da janela (como antes do REV-P3).

| Caso (8 jogos alternando mando, `last_n = 2`) | Antes | Depois |
|---|---|---|
| Escanteios, qualquer mando | **5, 6, 7, 8** | **7, 8** |
| Escanteios, só em casa | — | 7, 8 (ambas em casa) |
| Escanteios, só fora | — | 7, 8 (ambas fora) |
| Vitória (equipe) | 7, 7, 8, 8 | 7, 7, 8, 8 (inalterado) |

**B2b — ordem não determinística (achado durante o B2, mesmo bloco).** A janela
reconstruía os candidatos iterando um `map` do Go, cuja ordem é aleatória por
especificação. As datas saíam em ordem (`sortByDate` é estável), mas partidas do
**mesmo dia** trocavam de posição a cada execução. Medido antes da correção:
**100 execuções idênticas → 26 resultados diferentes** (maior sequência de erros entre
3 e 11, drawdown entre 16 e 19; nº de partidas e de acertos estáveis). Afetava o
Simulador em produção (padrão `last_n = 10`), o componente de drawdown do DSFR e o
filtro de drawdown do Discovery. Correção: equipes percorridas em ordem fixa e
ordenação explícita por (data, id da partida, mandante antes de visitante).

### B1 — FDR sobre o conjunto completo de hipóteses

`discovery/engine.go` (`mine`, `applyFDR`) e `discovery/criteria.go`.

Ordem nova, documentada no código:

1. **Elegibilidade estrutural** (não depende do resultado): erro de backtest, falta de
   odd real, métrica não publicada, amostra < mínimo, p-valor não calculável.
2. p-valor de **todas** as hipóteses elegíveis; **m = quantas são**.
3. BY (q = 0,10) sobre esse conjunto.
4. **Só então** os critérios do doc 08 que dependem do resultado — win rate, ROI,
   yield, lucro, drawdown, DSFR ≥ 40 — sobre os sobreviventes.
5. Holdout.

**O que entra antes do FDR:** só amostra mínima e testabilidade. A amostra pode reduzir
m porque não depende do resultado ("independent filtering"). **Nada que olha acerto,
retorno ou score reduz m.**

`crit.validate` foi dividido em `structuralSample` (antes do FDR) e `secondary` (depois);
`validate` continua existindo e fazendo os dois, para os testes antigos.

**Correção da minha redação na Fase A.** Escrevi que o procedimento antigo "deixa de
controlar o FDR". Mais preciso: com hipótese nula completa, o BY com m correto garante
no máximo q = 10 % de ciclos com falso positivo. O antigo deu 8,5 %, **ainda abaixo do
nominal**. O defeito é que ele deixou de ser o BY: aplicado a um conjunto pré-selecionado
pelo resultado, não tem garantia teórica e, na prática, deixou passar 8× mais que o
procedimento correto.

**B1c — ciclo interrompido desativava as descobertas vigentes (achado durante o B1).**
Com timeout/shutdown no meio da varredura, `mine` devolvia vazio e o fluxo seguia até
`DeactivateDiscoveredExcept(liga, [])` — **retirando do ar todas as descobertas da liga**
por causa de um ciclo que não terminou. O comentário do código dizia "ciclo interrompido
não publica nada", mas ele desativava. Correção: ciclo interrompido (na varredura ou no
holdout) retorna erro antes de publicar ou desativar, e o funil marca `interrupted`.

### B3 — funil de rejeição com a causa real

- Motivos novos: `sem_odd_real` e `sem_metrica`. A regra é mensurável, não um palpite:
  se as observações excluídas por falta de odd real, somadas às elegíveis, bastariam
  para o mínimo, a causa é a odd. O mesmo vale para métrica.
- `LeagueResult.Funnel` com 18 contadores e identidades verificadas por `Funnel.Check`:

```
geradas            = erros + sem_odd_real + sem_métrica + amostra + não_testável + testadas
testadas (= m)     = rejeitadas_FDR + sobreviventes_FDR
sobreviventes_FDR  = rejeitadas_secundários + entrada_holdout
entrada_holdout    = erros + reprovadas + validadas + interrompidas
validadas          = teto_por_liga + erros_publicação + publicadas
```

- **Achado:** o resultado agregado (`discovery.Result`) não tinha o campo `rejections`,
  mas a tela lia `run.rejections` no topo. **A lista de motivos nunca foi renderizada**,
  em nenhuma varredura. Agora `Result` traz `rejections` e `funnel` agregados
  (`FromLeague`, `absorb`).
- `worker_runs.details` do Discovery passa a gravar `funnel` e `rejections`.
- Comentário falso de `combinations.go` sobre os mercados de resultado corrigido.

### B4 — "Procurar agora" restrito a administrador

- `pkg/adminaccess`: lista de e-mails em `ADMIN_EMAILS`. **Lista vazia = ninguém é
  admin** (falha fechada), com aviso no log de inicialização.
- `middleware.RequireAdmin(users)`: carrega o usuário **do banco** a partir do
  `user_id` do token (mesmo padrão do `RequirePremium`). Respostas: 401 sem
  token/usuário, **403** para não administrador.
- Rota: `authGroup.POST("/discovery/run", middleware.RequireAdmin(users), …)`.
- `domain.User.IsAdmin()` e `MarshalJSON` com `is_admin` — toda resposta que já devolve
  o usuário (login, cadastro) informa o frontend. O hash de senha continua fora do JSON
  (testado).
- Frontend: `AuthService.isAdmin()`; o botão só aparece para admin; 403 mostra mensagem
  própria. **Esconder o botão não é a barreira** — a barreira é o backend.
- `render.yaml`: `ADMIN_EMAILS` declarada com `sync: false` (sem valor no repositório).

**Execução manual — auditoria:**

| Pergunta | Resposta |
|---|---|
| Síncrona / bloqueia o request? | Não: goroutine na API, resposta 202 |
| Publica / desativa? | Sim, publica e desativa descobertas públicas |
| Concorre com o worker? | ⚠️ **Pode.** A trava (`progress.Tracker`, 409) só vale dentro do processo da API. O Cron Job roda em outro container. Um disparo manual durante o cron diário pode fazer os dois ciclos publicarem/desativarem a mesma liga ao mesmo tempo. Com admin-only o risco cai, mas não some. Não redesenhei a infraestrutura; fica como pendência (ex.: advisory lock no Postgres por liga). |

### Textos e UI

| Onde | Antes | Depois |
|---|---|---|
| Tooltip do DSFR | "pondera ROI, **EV**, taxa de acerto, **yield**…" | pesos reais da v1.1 (retorno 30, acerto 20, drawdown 15, amostra 15, consistência 15, variância 5) |
| Barra de progresso | "backtest sobre o **histórico completo**" | "na parte mais antiga (70 %); sobreviventes reexecutados na mais recente" |
| Resumo da varredura | "**162 combinações testadas**" (eram as geradas) | "162 geradas · N testadas estatisticamente · N significativas · N validadas no holdout · N publicadas" |
| Última varredura | lista de motivos (nunca renderizava) | funil por estágio + frase com a causa dominante; se não fechar ou foi interrompido, avisa em vez de exibir |
| Estado vazio | culpa "amostra, retorno, consistência e drawdown" | explica as quatro exigências (odd real, amostra, significância, holdout) sem afirmar o estado atual da base |
| Cabeçalho | "descarta tudo que não passa nos critérios" | descreve busca, teste com correção e holdout; "padrão histórico, não recomendação" |
| Rótulos de motivo | faltavam os novos; tinha `ev_nao_positivo` | todos os motivos atuais; a chave antiga continua legível |
| `aidocs` (contexto para IA) | "135 combinações"; critérios antes do teste | 162 (+45 por equipe no cron); ordem estrutural → teste → critérios → holdout |

Termos proibidos ("oportunidade", "vencedora", "garantida") verificados ausentes da tela.

### Grade e código morto

Grade inalterada: **135 + 27 = 162** (API) e **+45 por equipe** (cron). Campo
`combo.tier` removido (era sempre vazio); a garantia "tier não volta" passou a ser
estrutural, e o teste agora verifica que **nenhuma definição persistida** carrega
`opponent_tier`. `FilterCriteria.OpponentTier` continua existindo porque é a trava do
AUD-004 que recusa definições antigas.

### Preservado

- `RequireRealOdds = true` na mineração, no holdout e na reavaliação; nenhum fallback
  sintético, nenhuma odd fixa no Discovery.
- Reavaliação **não** foi alterada. Registro de semântica:
  - **EVIDÊNCIA DE DESCOBERTA** = treino + holdout originais (texto da descrição);
  - **MONITORAMENTO** = reavaliação diária posterior, sobre o histórico inteiro
    (inclui o treino). **Não é uma nova validação fora da amostra.**

---

## REV-P4 — Fase C (testes locais)

### Experimento de ruído — antes e depois (critérios de produção, 200 seeds × 700 partidas)

| | geradas | m médio no FDR | ciclos com falso significativo | publicadas |
|---|---|---|---|---|
| Antes (Fase A) | 162 | **0,75** | 17/200 (8,5 %) | 0 |
| Controle correto (Fase A) | 162 | 70 | 2/200 (1 %) | — |
| **Depois** | 162 | **60,00** | **2/200 (1,0 %)** | **0** |

O procedimento corrigido converge para o controle. (O m do depois é 60, e não 70,
porque o B2 corrigiu a janela: as combinações de janela têm agora a amostra certa, que
é menor.) **Nenhuma publicação em ruído puro.** Esse experimento virou teste permanente
(`TestB1_RuidoPuro_CriteriosDeProducao_200Seeds`, pulado só com `-short`).

### Funil em produção simulada (sem odd real, lógica do motor)

`162 geradas · 111 sem odd real · 51 amostra insuficiente · 0 testadas · 0 publicadas`.
As 51 não teriam amostra nem com todas as odds (sobretudo as de janela curta), então
"amostra insuficiente" é verdade para elas. Antes: 162/162 "amostra insuficiente".

### Testes obrigatórios

| # | Exigência | Teste |
|---|---|---|
| 1–3 | grade 135 / 27 / 162 | `TestGrade_162_Por_Liga`, `TestGrade_PorEquipeSoma45` |
| 4 | opponent tier ausente | `TestGenerateCombosNaoUsaTier` (reescrito: nenhuma definição com `opponent_tier`) |
| 5 | janela em regra de partida | `TestB2_MatchLevel_*`, `TestB2_SoEmCasa_LastN`, `TestB2_SoFora_LastN`, `TestB2_ComEquipeSelecionada`, `TestB2_SemMisturaDeTemporada`, `TestB2b_OrdemCronologicaEDeterministica` |
| 6 | regra de equipe preservada | `TestB2_TeamLevel_Preservado` |
| 7 | FDR recebe todas as elegíveis | `TestB1_FDRRecebeTodasAsHipotesesElegiveis` (recálculo independente) |
| 8 | critérios de resultado não reduzem m | `TestB1_FiltrosDeResultadoNaoReduzemM`, `TestB1_CriteriosDeResultadoAgemDepoisDoFDR` |
| 9–10 | BY e BH | `TestFDR_BH_E_BY_CalculadosAMao` (m = 5, valores à mão) + testes existentes de `formulas` |
| 11 | q = 0 publica zero | `TestB1_QZeroPublicaZero` (e prova que a configuração não produz q = 0) |
| 12 | ruído | `TestB1_RuidoPuro_CriteriosDeProducao_200Seeds` + `TestCicloSobreRuidoPuroNaoPublicaNada` |
| 13 | sem odd real → `sem_odd_real` | `TestB3_SemOddReal_CausaCorreta` (synthetic, unknown e sem odd) |
| 14 | funil fecha | `Funnel.Check` em todos os testes do motor; `TestB3_FunilFecha_ComPublicacao`, `TestB3_ResultadoAgregadoTrazFunilEMotivos` |
| 15 | não-admin → 403 | `TestRequireAdmin_UsuarioComumRecebe403` + `TestRotaDiscoveryRun_UsuarioComumRecebe403` (**roteador real**) |
| 16 | admin permitido | `TestRequireAdmin_AdminPassa` |
| 17 | frontend esconde para não-admin | `discovery-funnel.spec.ts` (regra de `isAdmin`, sessão antiga = não-admin) |
| 18 | zero descobertas é válido | `TestZeroDescobertasEEstadoValido` + spec do frontend |
| extra | ciclo interrompido não desativa | `TestB1c_CicloInterrompidoNaoDesativa`, `TestB1c_CicloCompletoDesativaNormalmente` |
| extra | `is_admin` no JSON sem vazar hash | `TestUserJSON_TrazIsAdmin` |

**Três expectativas minhas estavam erradas e foram corrigidas nos testes, não no
código:** esperei 162/162 "sem odd real" (são 111 + 51 que não teriam amostra de
qualquer jeito); esperei 0 "sem odd real" com odd de escanteio presente (os mercados de
resultado continuam sem odd real, corretamente); e esperei os 27 de resultado como "sem
odd real" (6 não têm amostra nem com odd). Os testes agora recalculam de forma
independente em vez de usar números fixos.

### Regressão

| Comando | Resultado |
|---|---|
| `gofmt -l internal pkg cmd` | vazio |
| `go build ./...` · `go vet ./...` | OK · OK |
| `go test -count=1 ./...` | 11 pacotes `ok` (Discovery 51,9 s, pelo teste de 200 seeds) |
| Testes do REV-P3 afetados pelo B2 | todos passam (pacote `usecase` inteiro) |
| `npx ng build --configuration production` | OK — 642,44 kB |
| `sample-state.spec.ts` · `simulator-url-state.spec.ts` · `discovery-funnel.spec.ts` | 28/28 · 11/11 · 13/13 |
| Runner de componente Angular | **NA — inexistente** |

### Riscos e pendências

1. **`ADMIN_EMAILS` precisa ser configurado no web service do Render ANTES ou junto do
   deploy.** Sem ele ninguém dispara o Discovery manual (falha fechada, intencional).
2. **Sessões abertas antes do deploy** não têm `is_admin` no usuário salvo: o botão só
   aparece depois de sair e entrar de novo. O backend recusa corretamente enquanto isso.
3. **Concorrência cron × disparo manual** (ver B4) — não resolvida.
4. **Impacto em produção do B2/B2b:** números do Simulador com `last_n > 0` em regra de
   partida **vão mudar** (menos partidas, agora as certas) e passam a ser estáveis entre
   recargas. É correção, mas precisa ser anunciada.
5. Reavaliação sobre histórico inteiro — documentada, não alterada.
6. Casos reais com histórico completo e prova de que o cron roda o Discovery — seguem
   dependendo das consultas SQL da Fase A.
7. AUD-006 e AUD-007 — continuam abertos.

## REV-P4 = PARCIAL — Fases A/B/C concluídas, aguardando deploy e validação em produção

---

## REV-P4 — Fase D (deploy e validação em produção) — BLOQUEADA

Data: **25/09/2026**.

### Pré-requisitos

| # | Pré-requisito | Resultado | Evidência |
|---|---|---|---|
| 2 | `5fe6fba` em origin/main | ✅ | `git merge-base --is-ancestor 5fe6fba origin/main` |
| 3 | Frontend publicado no commit certo | ✅ | `/descobertas` exibe os textos novos ("162 por campeonato", "Nenhum padrão histórico publicado"), sem os antigos |
| 3 | Backend publicado no commit certo | ✅ | comportamento novo do B2 em produção (abaixo); `POST /discovery/run` sem token → `401 {"error":"token ausente"}` |
| 1 | `ADMIN_EMAILS` configurado no web service | ❌ **FALHOU** | ver abaixo |
| 4 | Sessão de admin renovada | ❌ bloqueado pelo 1 | — |

**Por que o pré-requisito 1 é dado como falho.** Na sessão do Daniel no Chrome, lida
pela extensão, `cornerlab_user.is_admin` = **`false`**. O valor **presente e falso**
(não ausente) indica login feito **depois** do deploy, porque sessões anteriores nem
têm o campo. Então o backend avaliou o e-mail e respondeu "não é admin". A causa mais
provável é que `ADMIN_EMAILS` não esteja definido no `cornerlab-backend`, ou não tenha
entrado em vigor; o log de inicialização da API diria "ADMIN_EMAILS vazio". O botão
"Procurar agora" não aparece, coerente com isso. **Nenhuma varredura foi disparada.**

**Neon.** A tentativa de rodar as consultas de `worker_runs` e de contagem de partidas
pelo navegador integrado caiu na tela de login do Neon (sem sessão). Não foi feito
login com senha; **as consultas não foram executadas**.

### Validado em produção antes do bloqueio (anônimo, limite de 90 dias)

**B2 — janela em regra de partida.** Brasileirão 2026, escanteios, `last_n = 2`:

| Recorte | Partidas | Duplicadas | Observação |
|---|---|---|---|
| Qualquer mando | **21** | 0 | todas representadas pelo mandante; datas **12/09 a 20/09** |
| Só casa | 19 | 0 | todas em casa |
| Só fora | 21 | 0 | todas fora |
| Vitória (equipe) | 40 observações em **21 partidas** | — | 19 casa + 21 fora |

As regras de partida e de equipe selecionaram **as mesmas 21 partidas** — checagem
cruzada independente. A regra antiga (últimos 2 jogos **em casa** de cada equipe)
voltaria cerca de 4 rodadas, até o fim de agosto. Uma equipe pode aparecer em até 3
partidas: um jogo mais antigo seu entra quando está entre os 2 últimos do adversário, o
que é a semântica definida ("entra se estiver na janela de qualquer das duas equipes").
Troca de temporada: não testável anonimamente (o limite de 90 dias corta 2024 inteira);
coberta por `TestB2_SemMisturaDeTemporada`.

**B2b — determinismo.** `last_n = 10`, **20 execuções idênticas → 1 resultado único**
nos dois casos:

| Caso | n | acertos/erros | seq. acertos/erros | drawdown | lucro | ROI |
|---|---|---|---|---|---|---|
| Escanteios (odd 1,50) | 100 | 67/33 | 10/3 | 650 | 50 | 0,5 % |
| Vitória (odd 1,80) | 195 | 65/130 | 2/9 | 7.840 | −7.800 | −40 % |

Ids e ordem idênticos em todas as execuções (primeiras: 16440, 16442, 16444, 16446…).

### O que falta para destravar

1. Definir `ADMIN_EMAILS` no web service `cornerlab-backend` do Render com o e-mail do
   administrador; salvar (o Render reinicia o serviço); confirmar no log que **não**
   aparece "ADMIN_EMAILS vazio".
2. Sair e entrar de novo no CornerLab; `is_admin` precisa passar a `true`.
3. Refazer a validação: botão, execução manual única, funil.
4. Consultas de `worker_runs` e de partidas: rodadas por quem tiver sessão no Neon.

## REV-P4 = PARCIAL — bloqueado por ADMIN_EMAILS (is_admin = false após login pós-deploy)

### Fase D — evidências do banco (Neon, projeto `cornerlab`, branch `production`)

Obtidas pelo Chrome do Daniel, **na sessão já aberta dele no Neon** (sem digitar
credencial), só com `SELECT`. Observação: o primeiro link recebido abria o projeto
**`finance-dsfr`**, que não é o CornerLab; nada foi executado lá — o editor daquele
projeto tinha um `CREATE EXTENSION` preenchido, que foi deixado intacto.

#### O Discovery agendado roda de verdade

```
SELECT id, worker, status, started_at, duration_ms, processed, errors, details
FROM worker_runs WHERE worker IN ('discovery','strategy') ORDER BY id DESC LIMIT 6;
```

| id | worker | status | started_at (UTC) | duração | details |
|---|---|---|---|---|---|
| 36 | discovery | ok | 2026-09-25 11:17:00 | 20,2 s | 12 ligas · 24.804 combinações · 0 publicadas · 0 desativadas |
| 35 | strategy | ok | 2026-09-25 11:17:00 | 0,2 s | 0 estratégias ativas · 0 avaliadas |
| 33 | discovery | ok | 2026-09-24 11:17:27 | 18,1 s | idem |
| 32 | strategy | ok | 2026-09-24 11:17:26 | 0,2 s | idem |
| 30 | discovery | ok | 2026-09-23 11:16:39 | 18,9 s | idem |
| 29 | strategy | ok | 2026-09-23 11:16:39 | 0,2 s | idem |

- **Item 9 do prompt respondido:** o cron (`DISCOVERY_RUN=true` efetivo) executa o
  Discovery todo dia por volta de 08:17 BRT, com status `ok`. Prova operacional, não
  `render.yaml`.
- 24.804 = 12 × 162 (grade das ligas) + 22.860 (grade por equipe, 45 × 508 vínculos
  equipe–liga): é o cron com `IncludeTeams = true`, como documentado na Fase A.
- As três execuções são **anteriores ao deploy do `5fe6fba`** (commit da noite de
  25/09), por isso `details` ainda não traz `funnel`/`rejections`. A primeira execução
  com o funil será a de **26/09, ~11:17 UTC**.
- Às 11:17 de 25/09 havia **0 estratégias ativas**. As de teste 43 e 44 (criadas às
  21:43) passarão a ser reavaliadas a partir de 26/09; sem odd real, `PersistResult`
  as recusa — a Fase A já previa 1 erro por estratégia por dia no `strategy`.

#### Odd real: zero em todo o histórico, em todas as ligas

```
SELECT m.league_id, l.name, count(*) partidas,
       count(*) FILTER (WHERE m.odds_source='real') odds_real, ...
FROM matches m JOIN leagues l ON l.id=m.league_id
WHERE m.status='FINALIZADO' GROUP BY 1,2 ORDER BY 1;
```

| liga | nome | partidas | odd real | odd sintética | com result_odds | primeira | última | corte 70 % (aprox.) |
|---|---|---|---|---|---|---|---|---|
| 2 | MLS | 388 | **0** | 0 | 0 | 2026-02-21 | 2026-09-24 | 2026-08-15 |
| 6 | Premier League | 430 | **0** | 380 | 0 | 2025-08-15 | 2026-09-20 | 2026-03-20 |
| 8 | La Liga | 449 | **0** | 380 | 0 | 2025-08-15 | 2026-09-20 | 2026-04-22 |
| 10 | Serie A | 430 | **0** | 380 | 0 | 2025-08-23 | 2026-09-20 | 2026-04-04 |
| 12 | Bundesliga | 344 | **0** | 308 | 0 | 2025-08-22 | 2026-09-20 | 2026-03-22 |
| 16 | Ligue 1 | 354 | **0** | 309 | 0 | 2025-08-15 | 2026-09-20 | 2026-04-05 |
| 18 | Brasileirão Série A | 348 | **0** | 248 | 0 | 2024-10-26 | 2026-09-20 | 2026-05-31 |
| 19 | Brasileirão Série B | 310 | **0** | 190 | 0 | 2024-11-15 | 2026-09-22 | 2026-07-28 |
| 20 | Copa do Brasil | 150 | **0** | 126 | 0 | 2026-02-17 | 2026-09-03 | 2026-04-23 |
| 21 | Libertadores | 149 | **0** | 125 | 0 | 2026-02-04 | 2026-09-18 | 2026-05-21 |
| 22 | Sul-Americana | 152 | **0** | 112 | 0 | 2026-03-03 | 2026-09-18 | 2026-05-27 |
| 23 | Champions League | 108 | **0** | 0 | 0 | 2026-07-07 | 2026-09-10 | 2026-08-11 |

Consequências verificáveis:

- **Discovery financeiro publicando 0 é o resultado correto em todas as ligas** — nenhuma
  tem uma única partida com odd de mercado, no histórico inteiro. Nada contorna isso
  (AUD-001 preservado).
- **O motor agrupa as temporadas de cada liga.** La Liga tem 449 partidas = 2025 inteira
  (380) + 2026 até agora; o corte de 70 % cai em 22/04/2026, então a janela de treino é
  quase toda 2025 e a de validação é o fim de 2025 mais 2026. Brasileirão idem: 2024
  parcial + 2026.
- As 1.968 partidas com odd sintética seguem descartadas pelo `RequireRealOdds`.

### Status após as evidências do banco

Resolvidos pela consulta: cron do Discovery (item 9), odds reais por liga (itens 10 e
24) e agrupamento de temporadas. **Continuam bloqueados por `ADMIN_EMAILS`:** B4 com
admin em produção, execução manual única, funil real por liga na tela e em
`/discovery/progress`.

## REV-P4 = PARCIAL — bloqueado por ADMIN_EMAILS (is_admin = false após login pós-deploy)
