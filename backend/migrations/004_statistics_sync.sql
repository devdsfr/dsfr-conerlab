-- CornerLab - Módulo de Sincronização de Dados (Statistics Provider)
--
-- Decisão de arquitetura: o critério de aceite pedia uma tabela nova "fixtures".
-- Como o próprio critério exige "PostgreSQL como única fonte da verdade", criar uma
-- tabela paralela a `matches` (que já guarda partidas e já é usada por Dashboard,
-- Comparador, Filtros e Bankroll) duplicaria a fonte da verdade em vez de unificá-la.
-- Por isso `matches` é ESTENDIDA com as colunas novas (status + estatísticas
-- detalhadas), e as tabelas genuinamente novas ficam limitadas ao que não existia
-- antes: `team_statistics` (agregados por equipe) e `provider_incidents` (saúde do
-- provedor de dados, ver Worker de Health Check).

-- status do ciclo de vida da partida no pipeline de sincronização.
-- 'AGENDADO'   -> descoberta encontrou o jogo, ainda não aconteceu (ou não confirmado como encerrado)
-- 'FINALIZADO' -> resultado e estatísticas completas já sincronizados
-- Partidas já existentes (seed manual anterior) são todas históricas reais, por isso
-- o default é 'FINALIZADO'.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'FINALIZADO';

ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_possession   SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_possession   SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_shots        SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_shots        SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_shots_on_target SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_shots_on_target SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_yellow_cards SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_yellow_cards SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_red_cards    SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_red_cards    SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS referee           VARCHAR(120);
ALTER TABLE matches ADD COLUMN IF NOT EXISTS venue             VARCHAR(120);

-- updated_at: regra do critério "toda gravação/cálculo deve ter timestamp de última
-- atualização". stats_synced_at marca quando as estatísticas detalhadas (posse,
-- chutes, cartões) foram buscadas com sucesso pela última vez — o Worker de
-- Atualização usa essa coluna (junto de status) para saber o que ainda falta buscar.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS updated_at        TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE matches ADD COLUMN IF NOT EXISTS stats_synced_at   TIMESTAMPTZ;

-- Índice usado pelos workers: "quais partidas AGENDADAS já deveriam ter acontecido"
-- (Worker de Atualização) e "quantas partidas AGENDADAS existem por liga" (Worker de
-- Descoberta, para evitar duplicar).
CREATE INDEX IF NOT EXISTS idx_matches_status_date ON matches(status, match_date);

-- Estatísticas agregadas por equipe (last5/10/20, casa/fora, consistência etc.).
-- Calculadas pelo Worker de Recálculo (fase 2 deste módulo) sempre que uma partida da
-- equipe é finalizada — nunca no frontend/client. over_percentages guarda o percentual
-- de jogos acima de cada linha (4 a 10), no mesmo padrão JSONB já usado por
-- matches.corner_odds, em vez de 7 colunas separadas.
CREATE TABLE IF NOT EXISTS team_statistics (
    team_id               BIGINT PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
    last5_avg_corners     NUMERIC(5,2),
    last10_avg_corners    NUMERIC(5,2),
    last20_avg_corners    NUMERIC(5,2),
    home_avg_corners      NUMERIC(5,2),
    away_avg_corners      NUMERIC(5,2),
    corners_against_avg   NUMERIC(5,2),
    over_percentages      JSONB NOT NULL DEFAULT '{}', -- ex: {"4": 92.5, "5": 81.0, ...}
    std_dev               NUMERIC(5,2),
    variance              NUMERIC(6,2),
    consistency_score     NUMERIC(4,3),
    confidence_index      NUMERIC(4,3),
    last_update           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Saúde do provedor de dados (essencial por depender de uma API não-oficial, como o
-- Sofascore, sem contrato de estabilidade). O Worker de Health Check grava uma linha
-- aqui a cada falha; enquanto existir incidente aberto (resolved_at IS NULL) para um
-- provedor, os Workers de Descoberta/Atualização pulam o ciclo daquele provedor sem
-- derrubar a aplicação — o Postgres continua servindo os dados já sincronizados
-- normalmente.
CREATE TABLE IF NOT EXISTS provider_incidents (
    id          BIGSERIAL PRIMARY KEY,
    provider    VARCHAR(40) NOT NULL,
    check_type  VARCHAR(40) NOT NULL, -- 'endpoint' | 'schema' | 'latency' | 'rate_limit'
    message     TEXT NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_provider_incidents_open ON provider_incidents(provider) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_provider_incidents_provider_created ON provider_incidents(provider, created_at DESC);
