-- CornerLab — mercados de resultado (vitória, empate, dupla chance)
--
-- CONTEXTO. O CornerLab nasceu em torno de escanteios, e todo o motor de backtest
-- é construído sobre uma pergunta só: "o total passou de N?". Vitória e empate não
-- são limiar sobre um total — são desfecho. Esta migration cria o lugar onde a odd
-- desses mercados vai morar.
--
-- O DADO DO JOGO JÁ EXISTE: home_goals e away_goals estão preenchidos desde sempre,
-- então saber quem venceu não custa nenhuma requisição nova ao provedor. O que não
-- existe é a ODD desses mercados, e sem odd não há hipótese nula — sem hipótese
-- nula o Discovery não valida nada (AUD-003).
--
-- POR ISSO ESTA MIGRATION É INERTE HOJE. Ela cria as colunas; nenhuma rota do
-- sistema grava nelas ainda. Até existir coleta de odds reais de 1X2, todo backtest
-- de resultado será corretamente rejeitado por "sem_pvalor_calculavel". Isso é o
-- sistema funcionando, não uma pendência esquecida.
--
-- POR QUE COLUNAS SEPARADAS de corner_odds/odds_source, e não reaproveitar:
-- são mercados distintos, coletados separadamente e com procedências que podem
-- divergir. É perfeitamente possível ter odd real de 1X2 sem ter de escanteios, e
-- misturar as duas procedências numa coluna só reintroduziria exatamente a
-- ambiguidade que o AUD-001 custou caro para eliminar.

ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS result_odds JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS result_odds_source TEXT NOT NULL DEFAULT 'unknown';

-- Mesmo vocabulário controlado de odds_source (migration 013). Repetido de
-- propósito: são duas procedências independentes.
ALTER TABLE matches
    DROP CONSTRAINT IF EXISTS matches_result_odds_source_check;
ALTER TABLE matches
    ADD CONSTRAINT matches_result_odds_source_check
    CHECK (result_odds_source IN ('real', 'synthetic', 'unknown'));

COMMENT ON COLUMN matches.result_odds IS
    'Odd de mercado por desfecho da partida. Chaves: home, draw, away, '
    'home_or_draw, away_or_draw. Vazio ate existir coleta de odds reais de 1X2. '
    'NUNCA preencher com valor derivado do proprio historico -- ver AUD-001.';

COMMENT ON COLUMN matches.result_odds_source IS
    'Procedencia de result_odds: real | synthetic | unknown. Independente de '
    'odds_source, que descreve corner_odds.';

-- Índice parcial pelo mesmo motivo do idx_matches_real_odds: o Discovery vai
-- filtrar por odd real de resultado, e essa consulta precisa ser barata mesmo
-- enquanto o resultado for vazio (que é o caso hoje, para todas as partidas).
CREATE INDEX IF NOT EXISTS idx_matches_real_result_odds
    ON matches(league_id, season_id)
    WHERE result_odds_source = 'real';
