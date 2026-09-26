// REV-P4 — testes do funil do Discovery e do controle de administrador no
// frontend. O projeto NÃO tem runner de teste de componente Angular
// (registrado como ausência). Rodar com:
//
//   cd frontend
//   npx tsc src/app/features/discovery/discovery-funnel.ts \
//           src/app/features/discovery/discovery-funnel.spec.ts \
//           --outDir /tmp/df --module commonjs --target es2022 --skipLibCheck --esModuleInterop
//   node /tmp/df/features/discovery/discovery-funnel.spec.js

import assert from 'node:assert';
import { DiscoveryFunnel, DiscoveryLastRun, DiscoveryLastRunResponse } from '../../core/models';
import {
  causaDominante,
  estadoUltimoCiclo,
  etapasFunil,
  explicacaoZero,
  funilFecha,
  quandoTerminou,
  resumoFunil,
  rotuloMotivo,
  rotuloOrigem,
  rotuloStatus,
} from './discovery-funnel';

let executados = 0;
function teste(nome: string, fn: () => void): void {
  fn();
  executados++;
  console.log('  ok —', nome);
}

function funil(p: Partial<DiscoveryFunnel> = {}): DiscoveryFunnel {
  return {
    generated: 0, backtest_errors: 0, rejected_no_real_odds: 0, rejected_no_metric: 0,
    rejected_insufficient_sample: 0, rejected_not_testable: 0, tested_statistically: 0,
    rejected_fdr: 0, fdr_survivors: 0, rejected_secondary: 0, holdout_input: 0,
    holdout_errors: 0, rejected_holdout: 0, holdout_interrupted: 0, holdout_validated: 0,
    capped_per_league: 0, publish_errors: 0, published: 0, ...p,
  };
}

// Situação de PRODUÇÃO (medida localmente com a lógica do motor): sem odd real,
// 111 combinações pararam por falta de odd e 51 por amostra.
const producao = funil({ generated: 162, rejected_no_real_odds: 111, rejected_insufficient_sample: 51 });

teste('funil de produção fecha', () => {
  assert.strictEqual(funilFecha(producao), true);
});

teste('causa dominante sem odd real é "sem odd real", não "amostra insuficiente"', () => {
  const c = causaDominante(producao);
  assert.ok(c);
  assert.strictEqual(c.chave, 'rejected_no_real_odds');
  assert.strictEqual(c.quantidade, 111);
});

teste('zero publicadas é explicado como resultado legítimo, com a causa real', () => {
  const t = explicacaoZero(producao)!;
  assert.ok(t.includes('162 combinações'));
  assert.ok(t.includes('111'));
  assert.ok(t.includes('sem odd real de mercado'));
  assert.ok(t.includes('resultado legítimo'));
  assert.ok(!/erro no sistema|falha/i.test(t));
});

teste('resumo distingue geradas de testadas estatisticamente', () => {
  const r = resumoFunil(producao);
  assert.ok(r.startsWith('162 combinações geradas'));
  assert.ok(r.includes('0 testadas estatisticamente'),
    'antes a tela dizia "162 combinações testadas" — eram as geradas');
});

teste('estágios zerados são omitidos, exceto geradas/testadas/publicadas', () => {
  const chaves = etapasFunil(producao).map(e => e.chave);
  assert.deepStrictEqual(chaves, [
    'generated', 'rejected_no_real_odds', 'rejected_insufficient_sample',
    'tested_statistically', 'published',
  ]);
});

teste('funil que não fecha é detectado (a tela não o exibe)', () => {
  assert.strictEqual(funilFecha(funil({ generated: 162, rejected_no_real_odds: 100 })), false);
});

teste('ciclo interrompido nunca é exibido como funil confiável', () => {
  assert.strictEqual(funilFecha({ ...producao, interrupted: true }), false);
});

teste('funil com publicação fecha em todos os estágios', () => {
  // Caso medido localmente com vantagem real (TestB3_FunilFecha_ComPublicacao).
  const f = funil({
    generated: 162, rejected_no_real_odds: 27, rejected_insufficient_sample: 81,
    tested_statistically: 54, rejected_fdr: 9, fdr_survivors: 45,
    holdout_input: 45, holdout_validated: 45, capped_per_league: 5, published: 40,
  });
  assert.strictEqual(funilFecha(f), true);
  assert.strictEqual(causaDominante(f), null, 'houve publicação: não há "causa do zero"');
  assert.strictEqual(explicacaoZero(f), null);
});

teste('rótulos: chave nova, chave antiga do AUD-002 e desconhecida', () => {
  assert.strictEqual(rotuloMotivo('sem_odd_real'), 'sem odd real de mercado');
  assert.strictEqual(rotuloMotivo('ev_nao_positivo'), 'sem lucro', 'ciclos antigos continuam legíveis');
  assert.strictEqual(rotuloMotivo('xyz'), 'xyz');
});

teste('linguagem: nenhum estágio promete oportunidade ou vitória', () => {
  const f = funil({ generated: 10, tested_statistically: 10, fdr_survivors: 10, holdout_input: 10, holdout_validated: 10, published: 10 });
  for (const e of etapasFunil(f)) {
    assert.ok(!/oportunidade|vencedora|garantid|lucro certo/i.test(e.rotulo), e.rotulo);
  }
});

// --- controle de admin no frontend (é conveniência; a barreira é o backend) ---

function isAdmin(token: string | null, user: { is_admin?: boolean } | null): boolean {
  // Mesma regra de AuthService.isAdmin().
  return !!token && user?.is_admin === true;
}

teste('não-admin não vê o botão', () => {
  assert.strictEqual(isAdmin('t', { is_admin: false }), false);
});

teste('sessão antiga sem is_admin é tratada como não-admin', () => {
  assert.strictEqual(isAdmin('t', {}), false);
});

teste('admin vê o botão; sem token ninguém vê', () => {
  assert.strictEqual(isAdmin('t', { is_admin: true }), true);
  assert.strictEqual(isAdmin(null, { is_admin: true }), false);
});


// --- REV-P4 (item 27): último ciclo persistido --------------------------------

// Resposta REAL de produção, reconstruída a partir da execução manual de
// 26/09/2026 01:46 UTC (mesmo contrato de GET /discovery/last-run).
const funilReal = funil({ generated: 1944, rejected_no_real_odds: 1671, rejected_insufficient_sample: 273 });
const respReal: DiscoveryLastRunResponse = {
  available: true,
  run: {
    id: 37, status: 'ok', trigger: 'manual',
    started_at: '2026-09-26T01:46:39Z', finished_at: '2026-09-26T01:47:06Z', duration_ms: 27524,
    leagues: 12, combinations: 1944, published: 0, deactivated: 0, errors: 0,
    funnel: funilReal, rejections: { sem_odd_real: 1671, amostra_insuficiente: 273 },
  } as DiscoveryLastRun,
};

teste('11. ao abrir, com ciclo registrado, o painel mostra o ciclo', () => {
  const e = estadoUltimoCiclo(true, respReal);
  assert.strictEqual(e.tipo, 'ciclo');
});

teste('12. refresh preserva: a mesma resposta produz o mesmo painel', () => {
  // A tela não guarda estado próprio: o painel é função da resposta do backend,
  // que vem de worker_runs. Duas cargas iguais → mesmo resultado.
  const a = estadoUltimoCiclo(true, respReal);
  const b = estadoUltimoCiclo(true, JSON.parse(JSON.stringify(respReal)));
  assert.deepStrictEqual(a, b);
});

teste('13. sem ciclo registrado: estado vazio, sem zeros inventados', () => {
  const e = estadoUltimoCiclo(true, { available: false });
  assert.strictEqual(e.tipo, 'vazio');
  assert.ok(!('run' in e));
  // Sem login a rota não é chamada: estado próprio, não "vazio".
  assert.strictEqual(estadoUltimoCiclo(false, null).tipo, 'sem-login');
});

teste('14. funil renderiza os valores reais do ciclo persistido', () => {
  const f = respReal.run!.funnel!;
  assert.strictEqual(funilFecha(f), true);
  const et = etapasFunil(f).map(x => `${x.rotulo}=${x.quantidade}`);
  assert.deepStrictEqual(et, [
    'combinações geradas=1944', 'sem odd real de mercado=1671', 'amostra insuficiente=273',
    'testadas estatisticamente=0', 'padrões históricos publicados=0',
  ]);
});

teste('15. geradas nunca são chamadas de testadas', () => {
  const r = resumoFunil(respReal.run!.funnel!);
  assert.ok(r.includes('1944 combinações geradas'));
  assert.ok(r.includes('0 testadas estatisticamente'));
  assert.ok(!/1944 combinações testadas/.test(r));
});

teste('16. origem cron/manual e ciclo antigo sem origem', () => {
  assert.strictEqual(rotuloOrigem('cron'), 'automática (diária)');
  assert.strictEqual(rotuloOrigem('manual'), 'manual (administrador)');
  assert.strictEqual(rotuloOrigem(null), 'origem não registrada');
});

teste('status de erro aparece como erro', () => {
  assert.strictEqual(rotuloStatus('ok'), 'concluída');
  assert.strictEqual(rotuloStatus('error'), 'com erro ou interrompida');
});

teste('horário exibido em Brasília (UTC−3)', () => {
  assert.ok(quandoTerminou(respReal.run!).includes('25/09/2026'), quandoTerminou(respReal.run!));
  assert.ok(quandoTerminou(respReal.run!).includes('22:47'), quandoTerminou(respReal.run!));
});

console.log(`\n${executados} testes do funil do Discovery — todos passaram.`);
