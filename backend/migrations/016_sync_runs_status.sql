-- 016 — sync_runs passa a registrar TAMBÉM os ciclos que falharam.
--
-- POR QUE: até aqui, recordRun só era chamado depois que descoberta E atualização
-- terminavam com sucesso. Um ciclo que morresse no meio não deixava rastro nenhum.
--
-- Evidência concreta disso em produção: o ciclo manual de 12/09 ~17:00 bateu no
-- teto de tempo e morreu com "context deadline exceeded" — o api_usage_log tem as
-- 17 chamadas falhas daquele instante, mas sync_runs não tem nenhuma linha de
-- 12/09 17:00. A tela continuou anunciando "última sincronização: 11/09 22:40",
-- ou seja, informou um estado melhor do que o real.
--
-- Isso é exatamente a classe de problema que a auditoria já tinha encontrado em
-- outra forma (ciclo que roda, não traz dado e é contado como saudável). Aqui o
-- efeito é o inverso e igualmente enganoso: o ciclo roda, FALHA, e some.
--
-- DEFAULT 'success' nas linhas existentes é uma afirmação verdadeira: todas as 17
-- linhas atuais foram gravadas pelo caminho de sucesso, porque o caminho de falha
-- simplesmente não gravava. Não estamos rotulando nada que não saibamos.

ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'success';
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS error_message TEXT;

-- Valores possíveis:
--   success — o ciclo completou as duas fases (pode ter trazido zero dado)
--   failed  — o ciclo foi interrompido por erro; error_message diz qual
--
-- Restrição em vez de enum para não travar migrations futuras: acrescentar um
-- estado novo é um ALTER simples.
ALTER TABLE sync_runs DROP CONSTRAINT IF EXISTS sync_runs_status_check;
ALTER TABLE sync_runs ADD CONSTRAINT sync_runs_status_check
    CHECK (status IN ('success', 'failed'));

COMMENT ON COLUMN sync_runs.status IS
    'success = ciclo completou as duas fases; failed = interrompido por erro (ver error_message).';
COMMENT ON COLUMN sync_runs.error_message IS
    'Mensagem do erro que interrompeu o ciclo. NULL quando status = success.';

-- Índice para a consulta "último ciclo desta origem" e "último ciclo bem-sucedido",
-- que a tela de Integrações faz a cada carregamento.
CREATE INDEX IF NOT EXISTS idx_sync_runs_trigger_created
    ON sync_runs (triggered_by, created_at DESC);
