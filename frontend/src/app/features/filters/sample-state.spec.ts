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
  mediaObservada,
  mensagemMetricaAusente,
  mostraColunaMando,
  rotuloMetrica,
  tipoMercado,
  valorObservado,
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


// =============================================================================
// REV-P3 item 33 — metric-unavailable usa o recorte EFETIVO
// Caso real de produção (25/09/2026): Champions 2026 · Egnatia · impedimentos.
// =============================================================================

teste('33.1 — com filtro de equipe a mensagem diz 4, não 108', () => {
  const r = resultado({
    match_count: 0, metric_scope: 'match',
    accounting: conta({
      matches_in_window: 108, observations_in_window: 4,
      excluded_entries: 4, excluded_no_metric: 4, eligible_entries: 0,
    }),
  });
  assert.strictEqual(estadoAmostra(r), 'metrica-ausente');
  const msg = mensagemMetricaAusente(r);
  assert.ok(msg.includes('4 partidas'), `mensagem deveria citar 4: "${msg}"`);
  assert.ok(!msg.includes('108'),
    'a contagem da competição inteira não pode aparecer: 53 das 108 TÊM o dado');
  assert.ok(msg.includes('nenhuma delas'));
});

teste('33.2 — sem filtro de equipe (observações = partidas) continua correto', () => {
  const r = resultado({
    match_count: 0, metric_scope: 'match',
    accounting: conta({
      matches_in_window: 12, observations_in_window: 12,
      excluded_entries: 12, excluded_no_metric: 12,
    }),
  });
  const msg = mensagemMetricaAusente(r);
  assert.ok(msg.includes('12 partidas'), msg);
});

teste('33.3 — team-level fala em observações, não partidas', () => {
  const r = resultado({
    match_count: 0, metric_scope: 'team',
    accounting: conta({
      matches_in_window: 50, observations_in_window: 6,
      excluded_entries: 6, excluded_no_metric: 6,
    }),
  });
  assert.ok(mensagemMetricaAusente(r).includes('6 observações'));
});

teste('33.4 — exclusão mista não afirma "nenhuma delas"', () => {
  const r = resultado({
    match_count: 0, metric_scope: 'team',
    accounting: conta({
      matches_in_window: 4, observations_in_window: 8,
      excluded_entries: 8, excluded_no_metric: 3, excluded_by_venue: 5,
    }),
  });
  const msg = mensagemMetricaAusente(r);
  assert.ok(!msg.includes('nenhuma delas'), 'só 3 de 8 saíram por falta de métrica');
  assert.ok(msg.includes('8 observações') && msg.includes('em 3 delas'), msg);
});

teste('33.5 — equipe sem nenhuma observação no recorte é estado vazio, não métrica-ausente', () => {
  const r = resultado({
    match_count: 0,
    accounting: conta({ matches_in_window: 108, observations_in_window: 0 }),
  });
  assert.strictEqual(estadoAmostra(r), 'vazio',
    'equipe sem nenhum jogo no recorte: não há métrica ausente, há ausência de partida');
});

// =============================================================================
// REV-P3 item 7 — mercados categóricos não têm valor numérico
// =============================================================================

const linha = { total_corners: 0, total_goals: 0, total_offsides: 0, total_shots: 0, total_shots_on_target: 0 };

teste('7.1 — classificação centralizada', () => {
  for (const m of ['win', 'draw', 'win_or_draw']) assert.strictEqual(tipoMercado(m), 'categorico', m);
  for (const m of ['corners', 'goals', 'offsides', 'shots', 'shots_on_target']) {
    assert.strictEqual(tipoMercado(m), 'numerico', m);
  }
});

teste('7.2 — vitória NÃO produz valor 0 artificial na linha', () => {
  assert.strictEqual(valorObservado('win', linha), null);
});

teste('7.3 — empate NÃO produz valor 0 artificial na linha', () => {
  assert.strictEqual(valorObservado('draw', linha), null);
});

teste('7.4 — não perde NÃO produz valor 0 artificial na linha', () => {
  assert.strictEqual(valorObservado('win_or_draw', linha), null);
});

teste('7.5 — mercados categóricos não têm média numérica (N/A, não 0)', () => {
  for (const m of ['win', 'draw', 'win_or_draw']) {
    const r = resultado({ match_count: 200, metric: m, average_corners: 0 } as Partial<BacktestResult>);
    assert.strictEqual(mediaObservada(r), null, `${m}: "Média de ${rotuloMetrica(m).toLowerCase()} 0" era o defeito`);
  }
});

teste('7.6 — a taxa de acerto continua disponível nos categóricos', () => {
  // O conserto remove a MÉDIA, não a taxa. A proporção de acertos já é o número
  // honesto desses mercados — sem renomeá-la para probabilidade.
  const r = resultado({ match_count: 200, metric: 'win', hit_rate: 35, hits: 70, misses: 130 } as Partial<BacktestResult>);
  assert.strictEqual(tipoMercado(r.metric), 'categorico');
  assert.strictEqual(r.hit_rate, 35);
  assert.strictEqual(estadoAmostra(r), 'com-amostra');
});

teste('7.7 — ZERO REAL numérico continua zero (0 gols, 0 escanteios, 0 impedimentos)', () => {
  assert.strictEqual(valorObservado('goals', { ...linha, total_goals: 0 }), 0);
  assert.strictEqual(valorObservado('corners', { ...linha, total_corners: 0 }), 0);
  assert.strictEqual(valorObservado('offsides', { ...linha, total_offsides: 0 }), 0);
  // e a média observada 0 é média, não ausência
  const r = resultado({ match_count: 5, metric: 'goals', average_goals: 0 } as Partial<BacktestResult>);
  assert.strictEqual(mediaObservada(r), 0);
});

teste('7.8 — métricas numéricas continuam lendo o campo certo', () => {
  const e = { total_corners: 11, total_goals: 3, total_offsides: 4, total_shots: 25, total_shots_on_target: 9 };
  assert.strictEqual(valorObservado('corners', e), 11);
  assert.strictEqual(valorObservado('goals', e), 3);
  assert.strictEqual(valorObservado('offsides', e), 4);
  assert.strictEqual(valorObservado('shots', e), 25);
  assert.strictEqual(valorObservado('shots_on_target', e), 9);
  const r = resultado({ match_count: 5, metric: 'shots', average_shots: 22.4 } as Partial<BacktestResult>);
  assert.strictEqual(mediaObservada(r), 22.4);
});

teste('7.9 — métrica desconhecida não vira "Escanteios"', () => {
  assert.strictEqual(rotuloMetrica('placar_exato'), 'placar_exato');
  assert.strictEqual(rotuloMetrica('win_or_draw'), 'Não perde');
});

teste('7.10 — match-level/team-level seguem corretos junto com o tipo de mercado', () => {
  // escanteios: numérico + partida → sem Mando, com valor
  const esc = resultado({ metric_scope: 'match', metric: 'corners' } as Partial<BacktestResult>);
  assert.strictEqual(mostraColunaMando(esc), false);
  assert.strictEqual(tipoMercado(esc.metric), 'numerico');
  // vitória: categórico + equipe → com Mando, sem valor numérico
  const vit = resultado({ metric_scope: 'team', metric: 'win' } as Partial<BacktestResult>);
  assert.strictEqual(mostraColunaMando(vit), true);
  assert.strictEqual(tipoMercado(vit.metric), 'categorico');
});

console.log(`\n${executados} testes de estado de amostra do Simulador — todos passaram.`);
