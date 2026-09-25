// REV-P3 / correção 6 — estado do Simulador na URL.
//
// Estas funções são PURAS de propósito: sem Angular, sem Router, sem signals.
// O projeto não tem runner de testes de componente (ver CORRECOES_AUDITORIA.md),
// então lógica que precisa ser testada mora fora do componente e é exercitada
// por simulator-url-state.spec.ts com node:assert — mesmo padrão já usado em
// features/dashboard/season-resolution.ts.
//
// Princípio que estas funções implementam: parâmetro inválido NÃO é substituído
// em silêncio. Cada recusa vira uma linha em `problemas`, e a tela precisa dizer
// o que não honrou — senão o usuário lê um backtest diferente do que a URL
// prometia, achando que é o mesmo. Seria exatamente o modo de falha que a
// reprodutibilidade deveria impedir.

import { FilterRunRequest } from '../../core/models';

export const METRICAS_VALIDAS = [
  'corners',
  'goals',
  'offsides',
  'shots',
  'shots_on_target',
  'win',
  'draw',
  'win_or_draw',
] as const;

export type MetricaSimulador = (typeof METRICAS_VALIDAS)[number];

/** Critérios do Simulador no formato em que viajam pela query string. */
export interface EstadoSimulador {
  leagueId?: number;
  seasonIds: number[];
  teamId?: number;
  lastNGames: number;
  homeAway: string;
  metric: MetricaSimulador;
  threshold: number;
  maxOdds?: number;
  fixedOdd?: number;
  stake: number;
}

/**
 * Serializa os critérios na query string. Só inclui o que tem valor: uma URL
 * cheia de `undefined` é pior de ler e não acrescenta informação.
 */
export function serializarEstado(e: EstadoSimulador): Record<string, string> {
  const q: Record<string, string> = {
    league: String(e.leagueId),
    metric: e.metric,
    threshold: String(e.threshold),
    last_n: String(e.lastNGames || 0),
    stake: String(e.stake),
  };
  if (e.seasonIds.length) q['seasons'] = e.seasonIds.join(',');
  if (e.teamId) q['team'] = String(e.teamId);
  if (e.homeAway) q['home_away'] = e.homeAway;
  // max_odds só faz sentido em escanteios, a única métrica com odd por partida
  // no banco. Nas demais não há odd real para limitar.
  if (e.metric === 'corners' && e.maxOdds) q['max_odds'] = String(e.maxOdds);
  if (e.fixedOdd) q['fixed_odd'] = String(e.fixedOdd);
  return q;
}

/**
 * Lê a query string e devolve uma FilterRunRequest reproduzível.
 *
 * Devolve `null` quando não há estado na URL (nada a restaurar) OU quando o
 * campeonato é inválido — sem campeonato não há backtest, e escolher outro por
 * conta própria produziria um resultado que a URL não pediu.
 *
 * `problemas` acumula tudo que foi recusado, para a tela poder dizer em voz alta.
 */
export function desserializarEstado(
  p: Record<string, string | null>,
  problemas: string[],
  padroes: { threshold: number; stake: number },
): FilterRunRequest | null {
  const bruto = p['league'] ?? null;
  if (bruto === null || bruto === '') return null;

  const league = Number(bruto);
  if (!Number.isFinite(league) || league <= 0) {
    problemas.push(`campeonato inválido na URL ("${bruto}")`);
    return null;
  }

  const num = (chave: string, rotulo: string): number | undefined => {
    const raw = p[chave];
    if (raw === null || raw === undefined || raw === '') return undefined;
    const v = Number(raw);
    if (!Number.isFinite(v)) {
      problemas.push(`${rotulo} inválido na URL ("${raw}")`);
      return undefined;
    }
    return v;
  };

  let metric = (p['metric'] ?? 'corners') as string;
  if (!(METRICAS_VALIDAS as readonly string[]).includes(metric)) {
    problemas.push(`métrica desconhecida na URL ("${metric}") — usando escanteios`);
    metric = 'corners';
  }

  let homeAway = p['home_away'] ?? '';
  if (homeAway && homeAway !== 'home' && homeAway !== 'away') {
    problemas.push(`mando inválido na URL ("${homeAway}")`);
    homeAway = '';
  }

  // Ids de temporada não numéricos são descartados, mas em voz alta: descartar
  // em silêncio mudaria o recorte sem o usuário saber.
  const seasonsBruto = (p['seasons'] ?? '').split(',').filter(s => s.trim() !== '');
  const seasonIds: number[] = [];
  for (const s of seasonsBruto) {
    const v = Number(s.trim());
    if (Number.isFinite(v) && v > 0) seasonIds.push(v);
    else problemas.push(`temporada inválida na URL ("${s}")`);
  }

  const th = num('threshold', 'limiar') ?? padroes.threshold;

  return {
    league_id: league,
    season_ids: seasonIds,
    team_id: num('team', 'equipe') ?? null,
    last_n_games: num('last_n', 'últimos jogos'),
    home_away: homeAway || undefined,
    metric,
    // corners_threshold é sempre enviado (o backend o ignora nas outras
    // métricas); os demais limiares só quando a métrica é a deles.
    corners_threshold: metric === 'corners' ? th : padroes.threshold,
    goals_threshold: metric === 'goals' ? th : undefined,
    offsides_threshold: metric === 'offsides' ? th : undefined,
    shots_threshold: metric === 'shots' ? th : undefined,
    shots_on_target_threshold: metric === 'shots_on_target' ? th : undefined,
    max_odds: metric === 'corners' ? num('max_odds', 'odds máximas') : undefined,
    fixed_odd: num('fixed_odd', 'odd fixa'),
    stake: num('stake', 'stake') ?? padroes.stake,
  };
}
