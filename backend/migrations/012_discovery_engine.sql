-- CornerLab — Remodelagem Fase 6: Strategy Discovery Engine
-- (Remodelagem/08-strategy-discovery-engine.md)
--
-- O Discovery Engine roda periodicamente, combina automaticamente dezenas de
-- filtros, executa backtest em cada combinação e persiste apenas as que passam
-- nos critérios mínimos — como estratégias do sistema (owner_id IS NULL,
-- origin='discovery', visibility='public'), reaproveitando toda a infraestrutura
-- de backtests/health/scores já criada na Fase 4 (migration 011).
--
-- Nenhuma tabela nova é necessária: a Fase 2 já modelou `strategies` prevendo
-- origin='discovery'. O que falta é garantir IDEMPOTÊNCIA — cada nova execução
-- precisa ATUALIZAR a estratégia equivalente já descoberta em vez de duplicá-la,
-- senão o ranking cresceria indefinidamente com cópias do mesmo padrão.
--
-- A chave de identidade é o `name`, que o engine gera de forma determinística a
-- partir da definição (liga + métrica + limiar + mando + janela + adversário).
-- Definições iguais produzem nomes iguais e, portanto, colidem no índice abaixo.

-- Índice único PARCIAL: só vale para estratégias descobertas pelo sistema. As
-- estratégias do usuário (origin='user') continuam podendo repetir nomes entre
-- usuários diferentes — a restrição não se aplica a elas.
CREATE UNIQUE INDEX IF NOT EXISTS idx_strategies_discovery_name
    ON strategies(name)
    WHERE origin = 'discovery';

-- Ranking de descobertas: a listagem pública ordena por strategy_scores.ranking
-- e filtra por liga (extraída do JSONB da definição). Sem este índice, filtrar
-- por liga exigiria varrer a tabela inteira desserializando o JSONB linha a linha.
CREATE INDEX IF NOT EXISTS idx_strategies_discovery_league
    ON strategies(((definition ->> 'league_id')::BIGINT))
    WHERE origin = 'discovery';
