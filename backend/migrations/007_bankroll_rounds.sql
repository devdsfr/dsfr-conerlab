-- Registro de rodadas confirmadas manualmente pelo usuário no Módulo de Gestão
-- Evolutiva de Banca. Cada linha é a confirmação de que uma fase/rodada foi
-- executada na vida real, com o resultado (lucro/prejuízo) obtido e o saldo
-- acumulado resultante — nunca apagado, é a prova histórica de que a estratégia
-- está funcionando (ou não) ao longo do tempo.
CREATE TABLE IF NOT EXISTS bankroll_rounds (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phase_sequence INT NOT NULL,
    phase_name VARCHAR(100) NOT NULL,
    result NUMERIC(12,2) NOT NULL,
    balance_after NUMERIC(12,2) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bankroll_rounds_user ON bankroll_rounds(user_id, confirmed_at);
