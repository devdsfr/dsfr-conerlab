-- CornerLab — AUD-001: distinguir odd real de mercado de odd sintética
--
-- PROBLEMA (auditoria, AUD-001): as odds gravadas em matches.corner_odds nunca
-- vieram de mercado. São geradas em internal/usecase/oddsgen.go a partir de uma
-- normal cuja média é a média de escanteios DO PRÓPRIO LOTE sincronizado — ou seja,
-- derivadas da mesma amostra que depois é backtestada. Filtrar por "odd <= 2.20"
-- passa a ser equivalente a selecionar lotes de média alta, o que garante acerto
-- alto por construção.
--
-- EVIDÊNCIA: em produção, Premier League, La Liga, Serie A, Ligue 1 e Bundesliga
-- têm UMA ÚNICA odd distinta para a linha 8.5 em ~380 jogos cada (um valor por
-- lote). No Brasileirão Série A, dos 51 jogos com odd <= 2.20 nessa linha, 51
-- acertaram (100%).
--
-- CORREÇÃO: esta migration só cria a marcação. O bloqueio de uso fica no código
-- (Discovery passa a exigir odds reais). NADA é apagado: as odds sintéticas
-- continuam no banco, agora identificadas, e seguem servindo ao Simulador como
-- cenário hipotético — desde que a interface diga isso claramente.

ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS odds_source TEXT NOT NULL DEFAULT 'unknown';

-- Vocabulário controlado:
--   real      — odd efetivamente ofertada por um mercado, capturada antes do jogo
--   synthetic — odd derivada dos próprios dados históricos (NÃO usar para validar
--               estratégia: contém informação da amostra que será testada)
--   unknown   — sem odd registrada, ou origem não rastreável
ALTER TABLE matches
    DROP CONSTRAINT IF EXISTS matches_odds_source_check;
ALTER TABLE matches
    ADD CONSTRAINT matches_odds_source_check
    CHECK (odds_source IN ('real', 'synthetic', 'unknown'));

-- Backfill. Todo corner_odds existente é sintético: as únicas rotas que gravam
-- essa coluna são cmd/sync (odds do lote, ver sync_usecase.go) e cmd/seed, ambas
-- chamando usecase.SyntheticCornerOdds. O worker de sincronização (statsync) nunca
-- escreve odds — ver o comentário de cabeçalho em statsync_repo.go.
UPDATE matches
   SET odds_source = 'synthetic'
 WHERE corner_odds IS NOT NULL
   AND corner_odds::text <> '{}'
   AND odds_source = 'unknown';

-- Índice parcial: o Discovery vai passar a filtrar por odds reais, e essa consulta
-- precisa ser barata mesmo quando (como hoje) o resultado é vazio.
CREATE INDEX IF NOT EXISTS idx_matches_real_odds
    ON matches(league_id, season_id)
    WHERE odds_source = 'real';

-- Invalidação das descobertas publicadas sobre odds sintéticas.
--
-- NÃO são apagadas: a linha, o histórico de backtests, os scores e a saúde
-- permanecem no banco para auditoria. Apenas saem do ar (active = false), porque
-- apresentá-las como oportunidade atual seria apresentar artefato como evidência.
-- O ciclo de descoberta as republicará automaticamente se um dia passarem nos
-- critérios com odds reais — o upsert é idempotente por nome.
UPDATE strategies
   SET active = false,
       updated_at = now()
 WHERE origin = 'discovery'
   AND active = true;
