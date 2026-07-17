-- Diagnóstico: times duplicados na Brasileirão Série B (Brasil)
-- Rode no Neon Console e cole o resultado de volta para gerar o script de correção.

SELECT
  t.id,
  t.name,
  t.short_name,
  t.external_id,
  (SELECT count(*) FROM matches m WHERE m.home_team_id = t.id OR m.away_team_id = t.id) AS total_partidas
FROM teams t
JOIN league_teams lt ON lt.team_id = t.id
JOIN leagues l ON l.id = lt.league_id
WHERE l.name ILIKE '%Série B%'
ORDER BY t.name;
