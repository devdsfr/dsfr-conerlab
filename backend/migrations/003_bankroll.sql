-- CornerLab - Módulo de Gestão Evolutiva de Banca
-- Fases configuráveis de evolução de banca, critérios objetivos de promoção/
-- rebaixamento e histórico completo (nunca apagado) de toda mudança de banca.

CREATE TABLE IF NOT EXISTS bankroll_phases (
    id       BIGSERIAL PRIMARY KEY,
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sequence INT NOT NULL,
    name     VARCHAR(80) NOT NULL,
    amount   NUMERIC(12,2) NOT NULL,
    UNIQUE (user_id, sequence)
);

-- Um único conjunto de critérios por usuário (aplicado a toda transição de fase).
CREATE TABLE IF NOT EXISTS bankroll_criteria (
    user_id                 BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    min_days                INT NOT NULL DEFAULT 90,
    min_bets                INT NOT NULL DEFAULT 100,
    min_win_rate            NUMERIC(5,2) NOT NULL DEFAULT 80,
    min_roi                 NUMERIC(5,2) NOT NULL DEFAULT 10,
    min_yield               NUMERIC(5,2) NOT NULL DEFAULT 5,
    require_positive_profit BOOLEAN NOT NULL DEFAULT true,
    min_completed_cycles    INT NOT NULL DEFAULT 20,
    cycle_win_streak        INT NOT NULL DEFAULT 3,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Estado atual do usuário no módulo. current_phase_sequence referencia
-- bankroll_phases.sequence (não um FK de id) para que reconfigurar as fases nunca
-- quebre o estado — o usecase reposiciona o ponteiro se necessário.
CREATE TABLE IF NOT EXISTS bankroll_state (
    user_id                BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_phase_sequence INT NOT NULL DEFAULT 1,
    phase_started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    highest_phase_sequence INT NOT NULL DEFAULT 1,
    promotions             INT NOT NULL DEFAULT 0,
    demotions              INT NOT NULL DEFAULT 0
);

-- Histórico completo de mudanças de banca. Nunca é apagado (ver regra do módulo).
CREATE TABLE IF NOT EXISTS bankroll_history (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_amount NUMERIC(12,2) NOT NULL,
    to_amount   NUMERIC(12,2) NOT NULL,
    direction   VARCHAR(12) NOT NULL, -- 'promotion' | 'demotion'
    reason      TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bankroll_history_user ON bankroll_history(user_id, created_at DESC);
