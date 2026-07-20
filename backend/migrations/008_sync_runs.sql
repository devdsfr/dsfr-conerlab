-- Histórico de execuções do ciclo de sincronização (descoberta + atualização),
-- disparadas manualmente pelo botão "Sincronizar agora" ou pelo Render Cron Job.
-- Alimenta o "Última sincronização: DD/MM HH:mm" mostrado no painel Integrações,
-- pra o usuário saber se precisa clicar em sincronizar de novo sem ter que adivinhar.
CREATE TABLE IF NOT EXISTS sync_runs (
    id BIGSERIAL PRIMARY KEY,
    triggered_by VARCHAR(20) NOT NULL, -- 'manual' | 'cron'
    targets INT NOT NULL DEFAULT 0,
    fixtures_found INT NOT NULL DEFAULT 0,
    fixtures_upserted INT NOT NULL DEFAULT 0,
    matches_checked INT NOT NULL DEFAULT 0,
    matches_finalized INT NOT NULL DEFAULT 0,
    errors INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sync_runs_created ON sync_runs(created_at DESC);
