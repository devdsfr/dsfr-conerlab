-- Duração (em ms) de cada ciclo de sincronização registrado em sync_runs — pedido
-- do usuário para saber quanto tempo o botão "Sincronizar agora" demora, e decidir
-- o horário do Render Cron Job com base nisso.
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS duration_ms INT NOT NULL DEFAULT 0;
