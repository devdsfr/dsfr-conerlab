-- Estatísticas complementares de partida (Prioridade Média — ver conversa sobre
-- "quais dados extras da API-Football fazem sentido pro CornerLab"): chutes de
-- dentro/fora da área, chutes bloqueados, faltas e impedimentos. Vêm do mesmo
-- endpoint /fixtures/statistics já chamado pelo Worker de Atualização — nenhuma
-- chamada extra à API, só passamos a capturar mais campos da mesma resposta.
-- Nullable porque nem todo fixture (principalmente ligas menores) tem 100% das
-- estatísticas publicadas pelo provedor.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_shots_insidebox  SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_shots_insidebox  SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_shots_outsidebox SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_shots_outsidebox SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_blocked_shots    SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_blocked_shots    SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_fouls            SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_fouls            SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS home_offsides         SMALLINT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS away_offsides         SMALLINT;
