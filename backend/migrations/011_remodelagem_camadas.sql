-- CornerLab — Remodelagem Fase 2: camadas de dados (Remodelagem/16-modelo-de-dados.md)
--
-- Filosofia do doc 16: separar o banco em três camadas e nunca misturar dados
-- importados com dados calculados.
--
--   RAW        -> exatamente o que a API devolveu (nunca apagar, nunca alterar)
--   NORMALIZED -> leagues/seasons/teams/matches (JÁ EXISTE — decisão da migration
--                 004 mantida: matches é a fonte única da verdade normalizada)
--   ANALYTICS  -> tudo que é calculado (reprocessável a partir de RAW+NORMALIZED)
--
-- Versionamento (doc 16): toda métrica calculada carrega algorithm_version,
-- permitindo recalcular resultados antigos quando uma fórmula mudar de versão no
-- Formula Catalog (Remodelagem/27 → backend/internal/formulas, Version).
--
-- Particionamento por temporada (doc 16) fica ADIADO de propósito: com ~3 mil
-- partidas o custo de manutenção supera o ganho. Quando fixtures/statistics
-- passarem de ~1M de linhas, particionar por season_id.

-- ============================================================================
-- CAMADA RAW — retenção: NUNCA APAGAR (doc 16)
-- ============================================================================

-- Payload bruto de cada fixture como veio do provedor. O worker de importação
-- (Fase 3) grava aqui ANTES de normalizar para matches; reprocessar = reler raw.
CREATE TABLE IF NOT EXISTS raw_fixtures (
    id          BIGSERIAL PRIMARY KEY,
    provider    VARCHAR(40)  NOT NULL,          -- 'api-football', ...
    external_id BIGINT       NOT NULL,          -- id do fixture no provedor
    payload     JSONB        NOT NULL,          -- resposta bruta, sem alteração
    fetched_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

-- Payload bruto das estatísticas de cada fixture (endpoint separado no provedor).
CREATE TABLE IF NOT EXISTS raw_statistics (
    id          BIGSERIAL PRIMARY KEY,
    provider    VARCHAR(40)  NOT NULL,
    external_id BIGINT       NOT NULL,          -- id do fixture no provedor
    payload     JSONB        NOT NULL,
    fetched_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

CREATE INDEX IF NOT EXISTS idx_raw_fixtures_fetched   ON raw_fixtures(fetched_at);
CREATE INDEX IF NOT EXISTS idx_raw_statistics_fetched ON raw_statistics(fetched_at);

-- ============================================================================
-- CAMADA ANALYTICS — reprocessável (doc 16); calculada só por workers (doc 15)
-- ============================================================================

-- Métricas pré-calculadas por equipe/temporada/métrica. Substitui gradualmente o
-- cálculo on-the-fly do Dashboard (corners, goals, offsides, shots, sot).
CREATE TABLE IF NOT EXISTS team_metrics (
    team_id            BIGINT      NOT NULL REFERENCES teams(id)   ON DELETE CASCADE,
    season_id          BIGINT      NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    league_id          BIGINT      NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    metric             VARCHAR(30) NOT NULL,   -- 'corners' | 'goals' | 'offsides' | 'shots' | 'shots_on_target'
    sample_size        INT         NOT NULL DEFAULT 0,
    avg_total          NUMERIC(6,2),
    avg_for            NUMERIC(6,2),
    avg_against        NUMERIC(6,2),
    avg_home           NUMERIC(6,2),
    avg_away           NUMERIC(6,2),
    last5_avg          NUMERIC(6,2),
    last10_avg         NUMERIC(6,2),
    last20_avg         NUMERIC(6,2),
    variance           NUMERIC(8,3),
    std_dev            NUMERIC(7,3),
    consistency        NUMERIC(5,2),           -- ConsistencyIndex 0..100 (Catálogo 22)
    trend              NUMERIC(4,3),           -- TrendScore -1..1 (Catálogo 29)
    frequencies        JSONB       NOT NULL DEFAULT '{}',  -- {"limiar": pct, ...}
    algorithm_version  VARCHAR(10) NOT NULL DEFAULT '1.0',
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, season_id, metric)
);

-- Estratégias (doc 16 STRATEGIES + STRATEGY FILTERS). A definição dos filtros vai
-- em JSONB (mesmo padrão de saved_filters.definition) em vez de tabela filha —
-- o Discovery Engine gera/valida definições completas de uma vez.
CREATE TABLE IF NOT EXISTS strategies (
    id          BIGSERIAL   PRIMARY KEY,
    owner_id    BIGINT      REFERENCES users(id) ON DELETE CASCADE,  -- NULL = estratégia do sistema (Discovery)
    name        VARCHAR(120) NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    definition  JSONB       NOT NULL,           -- filtros: liga, métrica, limiar, mando, janela...
    origin      VARCHAR(20) NOT NULL DEFAULT 'user',      -- 'user' | 'discovery'
    visibility  VARCHAR(20) NOT NULL DEFAULT 'private',   -- 'private' | 'public'
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    favorite    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Resultado de backtests persistidos (doc 16 BACKTESTS) — hoje o backtest do
-- Simulador é efêmero; aqui vira histórico auditável por estratégia.
CREATE TABLE IF NOT EXISTS backtests (
    id                BIGSERIAL   PRIMARY KEY,
    strategy_id       BIGINT      NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    games             INT         NOT NULL,
    wins              INT         NOT NULL,
    losses            INT         NOT NULL,
    voids             INT         NOT NULL DEFAULT 0,
    roi               NUMERIC(8,3),
    yield             NUMERIC(8,3),
    ev                NUMERIC(10,4),
    drawdown          NUMERIC(8,3),
    profit            NUMERIC(12,2),
    confidence        NUMERIC(5,2),            -- ConfidenceScore 0..100 (Catálogo 23)
    period_start      DATE,
    period_end        DATE,
    algorithm_version VARCHAR(10) NOT NULL DEFAULT '1.0',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Simulações financeiras (doc 16 SIMULATIONS; Monte Carlo do Catálogo 19).
CREATE TABLE IF NOT EXISTS simulations (
    id                   BIGSERIAL   PRIMARY KEY,
    strategy_id          BIGINT      NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    stake                NUMERIC(12,2) NOT NULL,
    bankroll             NUMERIC(12,2) NOT NULL,
    win_rate             NUMERIC(5,4)  NOT NULL,  -- fração 0..1
    odd                  NUMERIC(6,2)  NOT NULL,
    runs                 INT           NOT NULL,
    expected_profit      NUMERIC(12,2),
    expected_capital     NUMERIC(12,2),
    drawdown             NUMERIC(5,4),            -- fração média
    probability_positive NUMERIC(5,4),
    ruin_probability     NUMERIC(5,4),
    algorithm_version    VARCHAR(10) NOT NULL DEFAULT '1.0',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Saúde da estratégia (doc 16 HEALTH; Catálogo 25) — 1 linha por estratégia,
-- recalculada pelo Health Worker.
CREATE TABLE IF NOT EXISTS strategy_health (
    strategy_id       BIGINT      PRIMARY KEY REFERENCES strategies(id) ON DELETE CASCADE,
    health_score      NUMERIC(5,2) NOT NULL,   -- 0..100 (50 = estável)
    trend             NUMERIC(4,3),            -- TrendScore -1..1
    variation         JSONB       NOT NULL DEFAULT '{}',  -- deltas: roi, ev, drawdown, consistency
    algorithm_version VARCHAR(10) NOT NULL DEFAULT '1.0',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Scores proprietários (doc 16 DSFR SCORE; Catálogo 24 e 26–32).
CREATE TABLE IF NOT EXISTS strategy_scores (
    strategy_id       BIGINT      PRIMARY KEY REFERENCES strategies(id) ON DELETE CASCADE,
    dsfr_score        NUMERIC(5,2) NOT NULL,
    components        JSONB       NOT NULL DEFAULT '{}',  -- roi/ev/winrate/yield/drawdown/jogos/consistência/variância normalizados
    confidence        NUMERIC(5,2),
    robustness        NUMERIC(5,2),
    volatility        NUMERIC(5,2),
    risk              NUMERIC(5,2),
    ranking           NUMERIC(5,2),
    lifecycle_stage   VARCHAR(20),             -- nascimento|crescimento|maturidade|declinio|obsoleta
    algorithm_version VARCHAR(10) NOT NULL DEFAULT '1.0',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Oportunidades detectadas (doc 16 OPPORTUNITIES; Opportunity Engine, doc 13).
CREATE TABLE IF NOT EXISTS opportunities (
    id                BIGSERIAL   PRIMARY KEY,
    team_id           BIGINT      REFERENCES teams(id)      ON DELETE CASCADE,
    strategy_id       BIGINT      REFERENCES strategies(id) ON DELETE CASCADE,
    priority          SMALLINT    NOT NULL DEFAULT 0,
    opportunity_score NUMERIC(5,2) NOT NULL,
    reason            TEXT        NOT NULL,    -- explicação legível (princípio: explicar > números)
    status            VARCHAR(20) NOT NULL DEFAULT 'open',  -- 'open' | 'seen' | 'expired'
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ
);

-- Insights gerados (doc 16 INSIGHTS; Insight Engine, doc 11).
CREATE TABLE IF NOT EXISTS insights (
    id          BIGSERIAL   PRIMARY KEY,
    type        VARCHAR(40) NOT NULL,           -- 'health_drop' | 'new_strategy' | 'trend_change' ...
    title       VARCHAR(160) NOT NULL,
    description TEXT        NOT NULL,
    priority    SMALLINT    NOT NULL DEFAULT 0,
    strategy_id BIGINT      REFERENCES strategies(id) ON DELETE CASCADE,
    team_id     BIGINT      REFERENCES teams(id)      ON DELETE CASCADE,
    status      VARCHAR(20) NOT NULL DEFAULT 'new',  -- 'new' | 'read' | 'archived'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Execuções de workers do novo pipeline (doc 16 WORKERS). sync_runs continua
-- para o sync legado; worker_runs cobre o pipeline analytics da Fase 3.
CREATE TABLE IF NOT EXISTS worker_runs (
    id          BIGSERIAL   PRIMARY KEY,
    worker      VARCHAR(40) NOT NULL,           -- 'import' | 'analytics' | 'score' | ...
    status      VARCHAR(20) NOT NULL DEFAULT 'running',  -- 'running' | 'ok' | 'error'
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    duration_ms BIGINT,
    processed   INT         NOT NULL DEFAULT 0,
    errors      INT         NOT NULL DEFAULT 0,
    details     JSONB       NOT NULL DEFAULT '{}'
);

-- Auditoria (doc 16 AUDIT) — retenção 180 dias (aplicada pelo worker de limpeza).
CREATE TABLE IF NOT EXISTS audit_log (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    entity     VARCHAR(60) NOT NULL,
    entity_id  BIGINT,
    operation  VARCHAR(20) NOT NULL,            -- 'create' | 'update' | 'delete'
    old_value  JSONB,
    new_value  JSONB,
    ip         VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- ÍNDICES (doc 16: fixture/team/league/season/strategy/score/health/updated/status/date)
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_team_metrics_league_season ON team_metrics(league_id, season_id, metric);
CREATE INDEX IF NOT EXISTS idx_team_metrics_updated       ON team_metrics(updated_at);
CREATE INDEX IF NOT EXISTS idx_strategies_owner           ON strategies(owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_strategies_origin_active   ON strategies(origin, active);
CREATE INDEX IF NOT EXISTS idx_backtests_strategy_created ON backtests(strategy_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_simulations_strategy       ON simulations(strategy_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scores_dsfr                ON strategy_scores(dsfr_score DESC);
CREATE INDEX IF NOT EXISTS idx_scores_ranking             ON strategy_scores(ranking DESC);
CREATE INDEX IF NOT EXISTS idx_health_score               ON strategy_health(health_score);
CREATE INDEX IF NOT EXISTS idx_opportunities_status       ON opportunities(status, priority DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_insights_status_created    ON insights(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_worker_runs_worker_started ON worker_runs(worker, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_entity               ON audit_log(entity, entity_id, created_at DESC);
