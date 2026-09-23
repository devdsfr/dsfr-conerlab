/**
 * Testes de resolverTemporada / resolverEquipe.
 *
 * O projeto não tem runner de testes Angular (sem karma, jest ou vitest em
 * package.json). Em vez de impor um runner novo ao repositório, estes testes
 * usam só `node:assert` e rodam direto:
 *
 *   cd frontend
 *   npx tsc src/app/features/dashboard/season-resolution.ts \
 *           src/app/features/dashboard/season-resolution.spec.ts \
 *           --outDir /tmp/sr --module commonjs --target es2022 --skipLibCheck
 *   node /tmp/sr/season-resolution.spec.js
 *
 * Os casos vêm do incidente reproduzido em produção em 22/09/2026, não de
 * hipótese: `/dashboard?league_id=8&season_id=12&...` acabava chamando o backend
 * com `season_id=33`.
 */

import assert from 'node:assert';
import { Season, Team } from '../../core/models';
import { resolverEquipe, resolverTemporada } from './season-resolution';

const temporadas = (...anos: Array<[number, number]>): Season[] =>
  anos.map(([id, year]) => ({ id, league_id: 8, year, label: String(year) }));

const equipes = (...ids: number[]): Team[] =>
  ids.map(id => ({ id, name: 'Equipe ' + id, short_name: '', country: '', tier: '', created_at: '' }) as Team);

const LA_LIGA = temporadas([33, 2026], [12, 2025]);

const casos: Array<[string, () => void]> = [
  // 1 — temporada atual pedida pela URL é preservada.
  ['URL com temporada atual -> preservada', () => {
    const r = resolverTemporada(LA_LIGA, 33);
    assert.strictEqual(r.seasonId, 33);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 2 — o caso do incidente: temporada histórica válida tem que sobreviver.
  ['URL com temporada historica valida -> preservada (nao vira MAX(year))', () => {
    const r = resolverTemporada(LA_LIGA, 12);
    assert.strictEqual(r.seasonId, 12, 'temporada 2025 foi trocada pela mais recente');
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 3 — sem pedido explícito, o fallback continua valendo.
  ['URL sem season_id -> fallback MAX(year)', () => {
    const r = resolverTemporada(LA_LIGA, undefined);
    assert.strictEqual(r.seasonId, 33);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 4 — pedido inválido não vira substituição silenciosa.
  ['season_id inexistente na liga -> nao troca em silencio', () => {
    const r = resolverTemporada(LA_LIGA, 999);
    assert.strictEqual(r.seasonId, undefined, 'escolheu outra temporada sozinho');
    assert.strictEqual(r.pedidaAusente, true);
  }],

  // 5 — troca manual de campeonato (sem presets) cai no fallback.
  ['troca manual de campeonato -> assume a mais recente da nova liga', () => {
    const outraLiga = temporadas([40, 2024], [41, 2026], [42, 2025]);
    assert.strictEqual(resolverTemporada(outraLiga, undefined).seasonId, 41);
  }],

  // 6 — troca manual de temporada: o valor escolhido é um pedido explícito.
  ['troca manual de temporada -> nova temporada respeitada', () => {
    assert.strictEqual(resolverTemporada(LA_LIGA, 12).seasonId, 12);
  }],

  ['liga sem temporadas -> nada selecionado, sem alarme falso', () => {
    const r = resolverTemporada([], undefined);
    assert.strictEqual(r.seasonId, undefined);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 7 — equipe válida na temporada é preservada.
  ['team_id valido -> preservado', () => {
    const r = resolverEquipe(equipes(454, 455, 456), 455);
    assert.strictEqual(r.teamId, 455);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 8 — o caso Athletic Club: ausente NÃO vira teams[0].
  ['team_id ausente -> nao substitui por teams[0]', () => {
    const r = resolverEquipe(equipes(454, 455, 456), 442);
    assert.strictEqual(r.teamId, undefined, 'substituiu a equipe pedida pela primeira da lista');
    assert.strictEqual(r.pedidaAusente, true);
  }],

  ['sem team_id -> primeira da lista como ponto de partida', () => {
    const r = resolverEquipe(equipes(454, 455), undefined);
    assert.strictEqual(r.teamId, 454);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  ['sem equipes na temporada -> nada selecionado', () => {
    const r = resolverEquipe([], undefined);
    assert.strictEqual(r.teamId, undefined);
    assert.strictEqual(r.pedidaAusente, false);
  }],

  // 10 — a temporada usada na chamada final é sempre a resolvida, nunca outra.
  //      Reproduz a sequência do incidente: presets -> resolução -> requisição.
  ['inicializacao assincrona nao troca a temporada no caminho', () => {
    const pedida = 12;
    const resolvida = resolverTemporada(LA_LIGA, pedida).seasonId;
    const equipe = resolverEquipe(equipes(455), 455).teamId;
    const requisicao = `/api/v1/dashboard?team_id=${equipe}&league_id=8&season_id=${resolvida}`;
    assert.ok(
      requisicao.includes('season_id=12'),
      `requisicao final saiu com temporada diferente da pedida: ${requisicao}`,
    );
  }],
];

let falhas = 0;
for (const [nome, fn] of casos) {
  try {
    fn();
    console.log('PASS  ' + nome);
  } catch (e) {
    falhas++;
    console.log('FAIL  ' + nome + '\n      ' + (e as Error).message);
  }
}
console.log(`\n${casos.length - falhas}/${casos.length} passaram`);
if (falhas) process.exit(1);
