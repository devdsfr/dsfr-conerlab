-- CornerLab — AUD-004: teams.tier nunca foi uma classificação
--
-- PROBLEMA (auditoria, AUD-004). O relatório descreveu a coluna como "corrompida".
-- A investigação mostrou algo diferente e pior: ela NUNCA foi preenchida com
-- significado nenhum.
--
--   * 228 equipes têm 'G12' porque as duas rotas de sincronização gravam essa
--     string literal, fixa no código:
--         sync_repo.go:44      INSERT INTO teams (..., tier) VALUES (..., 'G12')
--         statsync_repo.go:74  INSERT INTO teams (..., tier) VALUES (..., 'G12')
--     Nenhum provedor devolve esse campo — não existe uma única referência a
--     "tier" em internal/integration/. É uma constante, não uma medida.
--
--   * 110 equipes têm '1', que é o NÚMERO DA DIVISÃO da liga (leagues.tier vale
--     '1' para 13 ligas e '2' para 8). Em algum momento esse valor foi copiado de
--     leagues para teams. Divisão não é grupo de classificação.
--
--   * G6 = 0 e Z4 = 0. Não porque o dado se perdeu: porque nada, em lugar nenhum
--     do sistema, jamais calculou classificação de equipe.
--
-- CONSEQUÊNCIA: filtrar "contra o G12" não seleciona os 12 primeiros colocados.
-- Seleciona as equipes que vieram do provedor — e exclui as 110 que carregam o
-- número da divisão. O filtro parecia uma variável de força do adversário e era,
-- na prática, um filtro de procedência do cadastro.
--
-- LOOK-AHEAD (o segundo defeito, que sobreviveria mesmo se o dado existisse):
-- teams.tier é um valor ÚNICO por equipe, sem dimensão temporal. Aplicar a
-- classificação de hoje a uma partida de 2023 é usar informação que não existia
-- na data do jogo. Uma classificação utilizável precisa ser por temporada E
-- apurada com os jogos ANTERIORES à data da partida — ver o item "trabalho
-- futuro" em CORRECOES_AUDITORIA.md § AUD-004.
--
-- ESTA MIGRATION NÃO APAGA NADA. Preserva o valor atual em tier_legacy e esvazia
-- tier, para que a coluna pare de afirmar uma classificação que não existe. O
-- bloqueio de uso fica no código (o motor de backtest passa a recusar filtro por
-- tier). É inteiramente reversível: UPDATE teams SET tier = tier_legacy.

ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS tier_legacy VARCHAR(20);

-- Preserva o que estava lá antes de esvaziar. Roda uma vez só: na segunda
-- execução tier já está vazio e o WHERE não pega nada.
UPDATE teams
   SET tier_legacy = tier
 WHERE tier_legacy IS NULL
   AND tier <> '';

UPDATE teams
   SET tier = ''
 WHERE tier <> '';

COMMENT ON COLUMN teams.tier IS
    'NÃO CLASSIFICADO (AUD-004). Vazio de propósito. Até 09/2026 esta coluna '
    'continha uma constante ("G12", gravada no código) ou o número da divisão da '
    'liga ("1"), nunca uma classificação real. Só volte a preenchê-la com uma '
    'classificação POR TEMPORADA e apurada com os jogos anteriores à data da '
    'partida — qualquer outra coisa gera look-ahead. Valor anterior preservado em '
    'tier_legacy.';

COMMENT ON COLUMN teams.tier_legacy IS
    'Valor que teams.tier tinha antes da correção do AUD-004. Guardado só para '
    'auditoria; não use em cálculo.';

-- leagues.tier NÃO é tocada: ali o valor ('1', '2') é o número da divisão do
-- provedor, que é uma informação correta e usada como tal.
COMMENT ON COLUMN leagues.tier IS
    'Número da divisão no provedor (1 = primeira divisão, 2 = segunda). Não '
    'confundir com teams.tier, que é outra coisa e está vazia (AUD-004).';
