// REV-P3 §1/§2/§3/§4 — testes dos estados da amostra do Simulador.
//
// O projeto NÃO tem runner de teste de componente Angular (sem karma, jest ou
// vitest em package.json). Registrado como ausência, nunca como aprovação.
// Estes testes usam node:assert e rodam com:
//
//   cd frontend
//   npx tsc src/app/features/filters/sample-state.ts \
//           src/app/features/filters/sample-state.spec.ts \
//           --outDir /tmp/ss --module commonjs --target es2022 --skipLibCheck --esModuleInterop
//   node /tmp/ss/features/filters/sample-state.spec.js

import assert from 'node:assert';
import { BacktestResult, SampleAccounting } from '../../core/models';
import {
  avisoMaxOddsSemEfeito,
  estadoAmostra,
  explicacaoExclusoes,
  mostraColunaMando,
} from './sample-state';

let executados = 0;
function teste(nome: string, fn: () => void): void {
  fn();
  executados++;
  console.log('  ok —', nome);
}

function conta(p: Partial<SampleAccounting> = {}): SampleAccounting {
  return {
    matches_in_window: 0, observations_in_window: 0, eligible_entries: 0,
    excluded_entries: 0, excluded_no_metric: 0, excluded_by_max_odds: 0,
    excluded_no_odd: 0, excluded_by_venue: 0, excluded_other: 0,
    max_odds_applicable: 0, ...p,
  };
}

function resultado(p: Partial<BacktestResult> = {}): BacktestResult {
  return {
    match_count: 0, metric_scope: 'match', accounting: conta(),
    ...p,
  } as BacktestResult;
}

// --- §1: os três estados não se confundem -----------------------------------

teste('ESTADO A — recorte sem nenhuma partida', () => {
  const r = resultado({ match_count: 0, accounting: conta({ matches_in_window: 0 }) });
  assert.strictEqual(estadoAmostra(r), 'vazio');
});

teste('ESTADO B — há partidas, mas o provedor não publicou a métrica', () => {
  const r = resultado({
    match_count: 0,
    accounting: conta({
      matches_in_window: 100, observations_in_window: 100,
      eligible_entries: 0, excluded_entries: 100, excluded_no_metric: 100,
    }),
  });
  assert.strictEqual(estadoAmostra(r), 'metrica-ausente',
    'ter achado 100 partidas sem o dado é diferente de não ter achado partida nenhuma');
});

teste('ESTADO C — há amostra: zeros observados são observações', () => {
  const r = resultado({ match_count: 100, accounting: conta({ matches_in_window: 100, eligible_entries: 100 }) });
  assert.strictEqual(estadoAmostra(r), 'com-amostra');
});

teste('sem accounting (backend antigo) cai em vazio, sem inventar motivo', () => {
  const r = { match_count: 0 } as BacktestResult;
  assert.strictEqual(estadoAmostra(r), 'vazio');
});

teste('partidas no recorte mas exclusão por OUTRO motivo não vira metrica-ausente', () => {
  // Exclusão por mando não é "estatística indisponível". Dizer que é seria
  // trocar um estado de ausência por outro — o erro que a §13 proíbe.
  const r = resultado({
    match_count: 0,
    accounting: conta({
      matches_in_window: 10, observations_in_window: 10,
      excluded_entries: 10, excluded_by_venue: 10,
    }),
  });
  assert.strictEqual(estadoAmostra(r), 'vazio');
});

// --- §2: coluna Mando --------------------------------------------------------

teste('match-level NÃO mostra coluna Mando', () => {
  assert.strictEqual(mostraColunaMando(resultado({ metric_scope: 'match' })), false);
});

teste('team-level MOSTRA coluna Mando', () => {
  assert.strictEqual(mostraColunaMando(resultado({ metric_scope: 'team' })), true);
});

// --- §3: max_odds sem efeito -------------------------------------------------

teste('teto de odd pedido, nenhuma odd na base → avisa', () => {
  const r = resultado({
    match_count: 100,
    accounting: conta({ max_odds_requested: 5, max_odds_applicable: 0, eligible_entries: 100 }),
  });
  const aviso = avisoMaxOddsSemEfeito(r);
  assert.ok(aviso, 'o usuário precisa saber que o controle não agiu');
  assert.ok(aviso!.includes('5'));
});

teste('teto de odd pedido e aplicável → não avisa', () => {
  const r = resultado({
    match_count: 8,
    accounting: conta({ max_odds_requested: 2, max_odds_applicable: 10, excluded_by_max_odds: 2, eligible_entries: 8 }),
  });
  assert.strictEqual(avisoMaxOddsSemEfeito(r), null);
});

teste('sem teto pedido → não avisa nada', () => {
  assert.strictEqual(avisoMaxOddsSemEfeito(resultado({ match_count: 5 })), null);
});

// --- §4: explicação das exclusões fecha --------------------------------------

teste('explicação lista cada motivo com seu número', () => {
  const r = resultado({
    match_count: 80, metric_scope: 'match',
    accounting: conta({
      matches_in_window: 100, observations_in_window: 100, eligible_entries: 80,
      excluded_entries: 20, excluded_no_metric: 13, excluded_by_max_odds: 5, excluded_other: 2,
    }),
  });
  const exp = explicacaoExclusoes(r)!;
  assert.ok(exp.includes('100 partidas no recorte'));
  assert.ok(exp.includes('80 analisadas'));
  assert.ok(exp.includes('20 fora'));
  assert.ok(exp.includes('13 sem a estatística publicada'));
  assert.ok(exp.includes('5 com odd acima do teto'));
  assert.ok(exp.includes('2 por outros critérios'));
});

teste('team-level diz "observações", não "partidas"', () => {
  const r = resultado({
    match_count: 190, metric_scope: 'team',
    accounting: conta({
      matches_in_window: 100, observations_in_window: 200, eligible_entries: 190,
      excluded_entries: 10, excluded_no_odd: 10,
    }),
  });
  const exp = explicacaoExclusoes(r)!;
  assert.ok(exp.includes('200 observações no recorte'),
    'em team-level uma partida gera duas observações; chamá-las de partidas seria errado');
});

teste('sem exclusão nenhuma → sem texto (ruído não ajuda)', () => {
  const r = resultado({
    match_count: 100,
    accounting: conta({ matches_in_window: 100, observations_in_window: 100, eligible_entries: 100, excluded_entries: 0 }),
  });
  assert.strictEqual(explicacaoExclusoes(r), null);
});

console.log(`\n${executados} testes de estado de amostra do Simulador — todos passaram.`);
