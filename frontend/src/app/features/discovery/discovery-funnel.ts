// REV-P4 (B3 e textos) — funil do Discovery em linguagem de produto.
//
// Funções PURAS, sem Angular: o projeto não tem runner de teste de componente
// (ver CORRECOES_AUDITORIA.md). Testadas em discovery-funnel.spec.ts com
// node:assert, no mesmo padrão de features/filters/sample-state.ts.
//
// Regras:
//   - nenhum número é calculado aqui além de somar o que o backend já contou;
//   - cada estágio usa o nome do que ele É ("candidato", "validado no holdout"),
//     nunca "oportunidade" ou "estratégia vencedora";
//   - zero publicadas é resultado legítimo, e a frase diz POR QUÊ com a causa
//     dominante real.

import { DiscoveryFunnel } from '../../core/models';

export interface EtapaFunil {
  chave: string;
  rotulo: string;
  quantidade: number;
  /** 'saida' = combinações que pararam aqui; 'passou' = seguiram adiante. */
  tipo: 'saida' | 'passou';
}

/**
 * Rótulos dos motivos de descarte. Inclui as chaves antigas para ciclos
 * gravados antes do REV-P4 continuarem legíveis.
 */
export const ROTULOS_MOTIVO: Readonly<Record<string, string>> = {
  sem_odd_real: 'sem odd real de mercado',
  sem_metrica: 'estatística não publicada',
  amostra_insuficiente: 'amostra insuficiente',
  sem_pvalor_calculavel: 'teste estatístico não calculável',
  nao_sobreviveu_correcao_fdr: 'não significativa após correção de múltiplos testes',
  win_rate_baixo: 'taxa de acerto abaixo do mínimo',
  roi_baixo: 'retorno abaixo do mínimo',
  yield_baixo: 'retorno abaixo do mínimo (yield)',
  lucro_nao_positivo: 'sem lucro',
  drawdown_alto: 'drawdown acima do máximo',
  score_baixo: 'score abaixo do mínimo',
  validacao_amostra_insuficiente: 'holdout com amostra insuficiente',
  validacao_sem_lucro: 'sem lucro no holdout',
  validacao_nao_significativa: 'não significativa no holdout',
  liga_sem_janela_de_validacao: 'histórico sem janela de validação',
  // Chave antiga do AUD-002 (renomeada para lucro_nao_positivo).
  ev_nao_positivo: 'sem lucro',
};

export function rotuloMotivo(chave: string): string {
  return ROTULOS_MOTIVO[chave] ?? chave;
}

/**
 * Estágios do funil na ordem em que o motor os percorre. Estágios com zero
 * são omitidos, exceto os três que o usuário sempre precisa ver: geradas,
 * testadas estatisticamente e publicadas.
 */
export function etapasFunil(f: DiscoveryFunnel): EtapaFunil[] {
  const todas: EtapaFunil[] = [
    { chave: 'generated', rotulo: 'combinações geradas', quantidade: f.generated, tipo: 'passou' },
    { chave: 'rejected_no_real_odds', rotulo: 'sem odd real de mercado', quantidade: f.rejected_no_real_odds, tipo: 'saida' },
    { chave: 'rejected_no_metric', rotulo: 'estatística não publicada', quantidade: f.rejected_no_metric, tipo: 'saida' },
    { chave: 'rejected_insufficient_sample', rotulo: 'amostra insuficiente', quantidade: f.rejected_insufficient_sample, tipo: 'saida' },
    { chave: 'rejected_not_testable', rotulo: 'teste não calculável', quantidade: f.rejected_not_testable, tipo: 'saida' },
    { chave: 'backtest_errors', rotulo: 'erro ao avaliar', quantidade: f.backtest_errors, tipo: 'saida' },
    { chave: 'tested_statistically', rotulo: 'testadas estatisticamente', quantidade: f.tested_statistically, tipo: 'passou' },
    { chave: 'rejected_fdr', rotulo: 'não significativas após correção de múltiplos testes', quantidade: f.rejected_fdr, tipo: 'saida' },
    { chave: 'fdr_survivors', rotulo: 'candidatos significativos', quantidade: f.fdr_survivors, tipo: 'passou' },
    { chave: 'rejected_secondary', rotulo: 'abaixo dos critérios mínimos (acerto, retorno, drawdown, score)', quantidade: f.rejected_secondary, tipo: 'saida' },
    { chave: 'rejected_holdout', rotulo: 'reprovados no holdout', quantidade: f.rejected_holdout + f.holdout_errors, tipo: 'saida' },
    { chave: 'holdout_validated', rotulo: 'validados no holdout', quantidade: f.holdout_validated, tipo: 'passou' },
    { chave: 'capped_per_league', rotulo: 'além do teto por campeonato', quantidade: f.capped_per_league + f.publish_errors, tipo: 'saida' },
    { chave: 'published', rotulo: 'padrões históricos publicados', quantidade: f.published, tipo: 'passou' },
  ];
  const sempre = new Set(['generated', 'tested_statistically', 'published']);
  return todas.filter(e => sempre.has(e.chave) || e.quantidade > 0);
}

/**
 * Causa dominante de "nada publicado": o estágio de SAÍDA com mais combinações.
 * Devolve null se houve publicação ou se o funil está vazio.
 */
export function causaDominante(f: DiscoveryFunnel): EtapaFunil | null {
  if (f.published > 0 || f.generated === 0) return null;
  const saidas = etapasFunil(f).filter(e => e.tipo === 'saida' && e.quantidade > 0);
  if (!saidas.length) return null;
  return saidas.reduce((a, b) => (b.quantidade > a.quantidade ? b : a));
}

/** Uma frase: quantas foram geradas, testadas, validadas e publicadas. */
export function resumoFunil(f: DiscoveryFunnel): string {
  return `${f.generated} combinações geradas · ${f.tested_statistically} testadas estatisticamente · ` +
    `${f.fdr_survivors} significativas · ${f.holdout_validated} validadas no holdout · ` +
    `${f.published} publicadas`;
}

/** Explicação do zero, sem sugerir erro. */
export function explicacaoZero(f: DiscoveryFunnel): string | null {
  const c = causaDominante(f);
  if (!c) return null;
  const pct = Math.round((100 * c.quantidade) / f.generated);
  const base = `Nenhum padrão foi publicado. Das ${f.generated} combinações, ${c.quantidade} (${pct}%) ` +
    `pararam em: ${c.rotulo}.`;
  if (c.chave === 'rejected_no_real_odds') {
    return base + ' Sem odd real de mercado não existe hipótese a testar — e o sistema não usa ' +
      'odd estimada nem odd digitada para validar padrões. É um resultado legítimo, não um erro.';
  }
  return base + ' É um resultado legítimo, não um erro.';
}

/**
 * Verifica as identidades do funil no cliente — as mesmas de Funnel.Check no
 * backend. Serve para a tela não exibir um funil que não fecha como se fosse
 * confiável.
 */
export function funilFecha(f: DiscoveryFunnel): boolean {
  if (f.interrupted) return false;
  const pre = f.backtest_errors + f.rejected_no_real_odds + f.rejected_no_metric +
    f.rejected_insufficient_sample + f.rejected_not_testable + f.tested_statistically;
  return f.generated === pre &&
    f.tested_statistically === f.rejected_fdr + f.fdr_survivors &&
    f.fdr_survivors === f.rejected_secondary + f.holdout_input &&
    f.holdout_input === f.holdout_errors + f.rejected_holdout + f.holdout_validated + f.holdout_interrupted &&
    f.holdout_validated === f.capped_per_league + f.publish_errors + f.published;
}
