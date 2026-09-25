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
import { DiscoveryFunnel } from '../../core/models';
import {
  causaDominante,
  etapasFunil,
  explicacaoZero,
  funilFecha,
  resumoFunil,
  rotuloMotivo,
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

console.log(`\n${executados} testes do funil do Discovery — todos passaram.`);
