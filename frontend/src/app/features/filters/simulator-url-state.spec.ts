// REV-P3 / correção 6 — testes do estado do Simulador na URL.
//
// O projeto NÃO tem runner de testes de componente Angular (registrado como
// ausência em CORRECOES_AUDITORIA.md, nunca como aprovação). Estes testes usam
// node:assert e rodam com:
//
//   cd frontend
//   npx tsc src/app/features/filters/simulator-url-state.ts \
//           src/app/features/filters/simulator-url-state.spec.ts \
//           --outDir /tmp/su --module commonjs --target es2022 --skipLibCheck --esModuleInterop
//   node /tmp/su/simulator-url-state.spec.js
//
// Mesmo padrão de features/dashboard/season-resolution.spec.ts.

import assert from 'node:assert';
import { desserializarEstado, serializarEstado } from './simulator-url-state';

const padroes = { threshold: 5, stake: 10 };
let executados = 0;

// assert.ok não estreita o tipo; esta função sim — e falha alto em vez de
// deixar o teste seguir com null.
function naoNulo<T>(v: T | null, msg: string): T {
  if (v === null) throw new Error(msg);
  return v;
}

function teste(nome: string, fn: () => void): void {
  fn();
  executados++;
  console.log('  ok —', nome);
}

// --- ida e volta -----------------------------------------------------------

teste('URL válida restaura exatamente os mesmos critérios', () => {
  const problemas: string[] = [];
  const d = desserializarEstado(
    {
      league: '71',
      seasons: '3,4',
      team: '10',
      last_n: '20',
      home_away: 'home',
      metric: 'corners',
      threshold: '8',
      max_odds: '1.6',
      stake: '100',
    },
    problemas,
    padroes,
  );
  assert.deepStrictEqual(problemas, [], 'URL válida não deve gerar problemas');
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.strictEqual(dd.league_id, 71);
  assert.deepStrictEqual(dd.season_ids, [3, 4]);
  assert.strictEqual(dd.team_id, 10);
  assert.strictEqual(dd.last_n_games, 20);
  assert.strictEqual(dd.home_away, 'home');
  assert.strictEqual(dd.metric, 'corners');
  assert.strictEqual(dd.corners_threshold, 8);
  assert.strictEqual(dd.max_odds, 1.6);
  assert.strictEqual(dd.stake, 100);
});

teste('serializar → desserializar preserva os critérios (round-trip)', () => {
  const q = serializarEstado({
    leagueId: 71,
    seasonIds: [3, 4],
    teamId: 10,
    lastNGames: 20,
    homeAway: 'away',
    metric: 'goals',
    threshold: 2,
    fixedOdd: 1.5,
    stake: 100,
  });
  const problemas: string[] = [];
  const d = desserializarEstado(q, problemas, padroes);
  assert.deepStrictEqual(problemas, []);
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.strictEqual(dd.league_id, 71);
  assert.strictEqual(dd.metric, 'goals');
  assert.strictEqual(dd.goals_threshold, 2);
  assert.strictEqual(dd.home_away, 'away');
  assert.strictEqual(dd.fixed_odd, 1.5);
  assert.strictEqual(dd.stake, 100);
});

teste('escanteios podem levar odd fixa na URL (correção 2)', () => {
  const q = serializarEstado({
    leagueId: 71, seasonIds: [], lastNGames: 0, homeAway: '',
    metric: 'corners', threshold: 8, fixedOdd: 1.5, stake: 100,
  });
  assert.strictEqual(q['fixed_odd'], '1.5');
  const d = desserializarEstado(q, [], padroes);
  assert.strictEqual(d?.fixed_odd, 1.5);
});

// --- inválido NÃO é substituído em silêncio --------------------------------

teste('campeonato inválido recusa o estado inteiro e reporta', () => {
  const problemas: string[] = [];
  const d = desserializarEstado({ league: 'abc', metric: 'corners' }, problemas, padroes);
  assert.strictEqual(d, null, 'não pode escolher outro campeonato por conta própria');
  assert.strictEqual(problemas.length, 1);
  assert.ok(problemas[0].includes('campeonato inválido'));
});

teste('métrica desconhecida cai em escanteios MAS avisa', () => {
  const problemas: string[] = [];
  const d = desserializarEstado({ league: '71', metric: 'placar_exato' }, problemas, padroes);
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.strictEqual(dd.metric, 'corners');
  assert.strictEqual(problemas.length, 1, 'a substituição precisa ser dita em voz alta');
  assert.ok(problemas[0].includes('placar_exato'));
});

teste('mando inválido é ignorado MAS avisa', () => {
  const problemas: string[] = [];
  const d = desserializarEstado({ league: '71', home_away: 'neutro' }, problemas, padroes);
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.strictEqual(dd.home_away, undefined);
  assert.ok(problemas.some(p => p.includes('mando inválido')));
});

teste('número inválido não vira zero — vira ausência reportada', () => {
  const problemas: string[] = [];
  const d = desserializarEstado(
    { league: '71', stake: 'muito', last_n: 'x', fixed_odd: 'grátis' },
    problemas,
    padroes,
  );
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.strictEqual(dd.last_n_games, undefined, 'ausência de dado não é zero');
  assert.strictEqual(dd.fixed_odd, undefined);
  assert.strictEqual(dd.stake, 10, 'stake cai no padrão do formulário, não em zero');
  assert.strictEqual(problemas.length, 3);
});

teste('temporada não numérica é descartada em voz alta', () => {
  const problemas: string[] = [];
  const d = desserializarEstado({ league: '71', seasons: '3,lixo,4' }, problemas, padroes);
  const dd = naoNulo(d, 'estado deveria ter sido restaurado');
  assert.deepStrictEqual(dd.season_ids, [3, 4]);
  assert.strictEqual(problemas.length, 1);
  assert.ok(problemas[0].includes('temporada inválida'));
});

teste('URL sem estado devolve null sem reclamar', () => {
  const problemas: string[] = [];
  assert.strictEqual(desserializarEstado({}, problemas, padroes), null);
  assert.deepStrictEqual(problemas, [], 'ausência de estado não é erro');
});

// --- serialização: só o que tem valor --------------------------------------

teste('serializar omite campos vazios em vez de escrever undefined', () => {
  const q = serializarEstado({
    leagueId: 71, seasonIds: [], lastNGames: 0, homeAway: '',
    metric: 'corners', threshold: 8, stake: 10,
  });
  assert.strictEqual(q['seasons'], undefined);
  assert.strictEqual(q['team'], undefined);
  assert.strictEqual(q['home_away'], undefined);
  assert.strictEqual(q['fixed_odd'], undefined);
  assert.strictEqual(q['league'], '71');
});

teste('max_odds não vai na URL fora de escanteios (não há odd real a limitar)', () => {
  const q = serializarEstado({
    leagueId: 71, seasonIds: [], lastNGames: 0, homeAway: '',
    metric: 'goals', threshold: 2, maxOdds: 1.6, stake: 10,
  });
  assert.strictEqual(q['max_odds'], undefined);
});

console.log(`\n${executados} testes de estado de URL do Simulador — todos passaram.`);
