-- CornerLab — cadastro das competições UEFA
--
-- POR QUE ISTO EXISTE. A Champions League "não carregava" porque nunca foi
-- cadastrada. Não era falha de sincronização: o worker é dirigido por dados —
-- ListSyncTargets (statsync_repo.go:43) busca as ligas com external_id
-- preenchido, junta com a temporada mais recente de cada uma, e sincroniza o que
-- encontrar. Não existe lista fixa de campeonatos no código. Cadastrar a liga e
-- uma temporada é tudo que o worker precisa para adotá-la sozinha.
--
-- CONVENÇÃO seguida (extraída das ligas já cadastradas):
--   external_id : id da liga na API-Football (v3 — mesmo id em todas as temporadas)
--   country     : nome em português ("Espanha", "Inglaterra", "América do Sul")
--   tier        : número da divisão no provedor ('1' = principal)
--   seasons.year: ano de INÍCIO da temporada europeia (2025 = temporada 2025/26),
--                 igual ao que Premier League e La Liga usam
--
-- Temporada 2025 e não 2026 de propósito: 2025/26 está completa e serve de
-- histórico para backtest; 2026/27 mal começou. Para acompanhar a temporada
-- corrente, acrescente uma linha em seasons com year = 2026 — o worker passa a
-- seguir a mais recente automaticamente (ListSyncTargets usa MAX(year)).

-- ---------------------------------------------------------------------------
-- APLICADO EM PRODUÇÃO em 09/09/2026 (league_id = 23).
--
-- ID verificado na documentação do próprio fornecedor: o tutorial oficial
-- "HOW TO FIND IDS" da API-Football usa, como exemplo, "league: 2 (UEFA
-- Champions League)".
-- https://www.api-football.com/news/post/how-to-find-ids
-- ---------------------------------------------------------------------------

WITH nova AS (
    INSERT INTO leagues (external_id, name, country, tier)
    VALUES ('2', 'UEFA Champions League', 'Europa', '1')
    ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
    RETURNING id
)
INSERT INTO seasons (league_id, year, label)
SELECT id, 2025, '2025' FROM nova
ON CONFLICT (league_id, year) DO NOTHING;

-- ---------------------------------------------------------------------------
-- NÃO APLICADO — CONFIRME O ID ANTES DE RODAR.
--
-- A UEFA Europa League é, quase certamente, a liga 3 na API-Football. "Quase
-- certamente" não é suficiente: não consegui confirmar esse número numa fonte
-- primária, e cadastrar um id errado não deixa a liga vazia — carrega dados de
-- OUTRA competição com o nome de Europa League, que é pior do que não carregar
-- nada.
--
-- Como confirmar em 30 segundos, no painel da API-Football:
--   dashboard.api-football.com → Apis → API-FOOTBALL → Ids → Leagues
--   → buscar "Europa League" → ler o id da coluna V3
--
-- Confirmado o número, troque o '3' abaixo se for o caso e rode.
-- ---------------------------------------------------------------------------

-- WITH nova AS (
--     INSERT INTO leagues (external_id, name, country, tier)
--     VALUES ('3', 'UEFA Europa League', 'Europa', '1')
--     ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
--     RETURNING id
-- )
-- INSERT INTO seasons (league_id, year, label)
-- SELECT id, 2025, '2025' FROM nova
-- ON CONFLICT (league_id, year) DO NOTHING;

-- ---------------------------------------------------------------------------
-- COMO DESFAZER, se um id se revelar errado. Apaga a liga e tudo que pendurou
-- nela; equipes ficam (são compartilhadas entre campeonatos).
-- ---------------------------------------------------------------------------

-- DELETE FROM matches       WHERE league_id = (SELECT id FROM leagues WHERE external_id = '3');
-- DELETE FROM league_teams  WHERE league_id = (SELECT id FROM leagues WHERE external_id = '3');
-- DELETE FROM seasons       WHERE league_id = (SELECT id FROM leagues WHERE external_id = '3');
-- DELETE FROM leagues       WHERE external_id = '3';

-- ---------------------------------------------------------------------------
-- CONFERIR DEPOIS DO PRÓXIMO CICLO DO WORKER
-- ---------------------------------------------------------------------------

-- SELECT l.name, s.year, count(m.id) AS jogos,
--        count(*) FILTER (WHERE m.home_corners IS NOT NULL) AS com_escanteios
--   FROM leagues l
--   JOIN seasons s  ON s.league_id = l.id
--   LEFT JOIN matches m ON m.season_id = s.id
--  WHERE l.external_id IN ('2','3')
--  GROUP BY 1, 2;

-- Uma checagem que vale a pena na primeira vez: confirme que os NOMES DAS
-- EQUIPES que apareceram são mesmo de clubes daquela competição. É a prova
-- definitiva de que o external_id está certo.
--
-- SELECT t.name FROM teams t
--   JOIN league_teams lt ON lt.team_id = t.id
--   JOIN leagues l ON l.id = lt.league_id
--  WHERE l.external_id = '2' ORDER BY t.name;
