-- CornerLab - log de uso das integrações externas (OpenAI, API-Football, SportMonks)
-- Usado pelo painel "Integrações" (diagnóstico de chaves de API e consumo).

CREATE TABLE IF NOT EXISTS api_usage_log (
    id                  BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(20) NOT NULL, -- 'openai' | 'api_football' | 'sportmonks'
    endpoint            VARCHAR(120) NOT NULL,
    success             BOOLEAN NOT NULL,
    status_code         INT,
    tokens_prompt       INT,
    tokens_completion   INT,
    tokens_total        INT,
    error_message       TEXT,
    duration_ms         INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_usage_provider_created ON api_usage_log(provider, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_usage_created ON api_usage_log(created_at DESC);
