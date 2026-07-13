-- CornerLab - schema inicial (MVP)
-- Mercado suportado no MVP: Escanteios

CREATE TABLE IF NOT EXISTS leagues (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(120) NOT NULL,
    country     VARCHAR(80) NOT NULL,
    tier        VARCHAR(20) NOT NULL DEFAULT 'G6',
    external_id VARCHAR(40) UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS seasons (
    id          BIGSERIAL PRIMARY KEY,
    league_id   BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    year        INT NOT NULL,
    label       VARCHAR(20) NOT NULL,
    UNIQUE (league_id, year)
);

CREATE TABLE IF NOT EXISTS teams (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(120) NOT NULL,
    short_name  VARCHAR(20) NOT NULL,
    country     VARCHAR(80) NOT NULL,
    tier        VARCHAR(20) NOT NULL DEFAULT 'G6',
    external_id VARCHAR(40) UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS league_teams (
    league_id   BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id     BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    PRIMARY KEY (league_id, team_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id                      BIGSERIAL PRIMARY KEY,
    league_id               BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    season_id               BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    round                   INT NOT NULL DEFAULT 0,
    match_date              TIMESTAMPTZ NOT NULL,
    home_team_id            BIGINT NOT NULL REFERENCES teams(id),
    away_team_id            BIGINT NOT NULL REFERENCES teams(id),
    home_corners            INT NOT NULL DEFAULT 0,
    away_corners            INT NOT NULL DEFAULT 0,
    home_goals              INT NOT NULL DEFAULT 0,
    away_goals              INT NOT NULL DEFAULT 0,
    corner_odds             JSONB NOT NULL DEFAULT '{}',
    external_id             VARCHAR(40) UNIQUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_matches_home_team ON matches(home_team_id);
CREATE INDEX IF NOT EXISTS idx_matches_away_team ON matches(away_team_id);
CREATE INDEX IF NOT EXISTS idx_matches_league_season ON matches(league_id, season_id);
CREATE INDEX IF NOT EXISTS idx_matches_date ON matches(match_date);

CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(120) NOT NULL,
    email           VARCHAR(160) NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Filtros personalizados do Módulo 3 (Simulador de Filtros).
-- `definition` guarda o JSON serializado do critério (ver FilterCriteria no backend).
CREATE TABLE IF NOT EXISTS saved_filters (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    definition   JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Histórico de execuções de backtest (para comparação ao longo do tempo, regra geral do MVP)
CREATE TABLE IF NOT EXISTS filter_backtests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    saved_filter_id BIGINT REFERENCES saved_filters(id) ON DELETE SET NULL,
    definition      JSONB NOT NULL,
    league_id       BIGINT REFERENCES leagues(id),
    season_ids      BIGINT[] NOT NULL DEFAULT '{}',
    result          JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bets (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    match_label  VARCHAR(200) NOT NULL,
    league_id    BIGINT REFERENCES leagues(id),
    market       VARCHAR(80) NOT NULL,
    odd          NUMERIC(6,2) NOT NULL,
    stake        NUMERIC(12,2) NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    profit_loss  NUMERIC(12,2) NOT NULL DEFAULT 0,
    event_date   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bets_user ON bets(user_id);

-- Alertas inteligentes (Módulo 7) - estrutura básica preparada para evolução futura
CREATE TABLE IF NOT EXISTS alert_rules (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    definition   JSONB NOT NULL,
    active       BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
