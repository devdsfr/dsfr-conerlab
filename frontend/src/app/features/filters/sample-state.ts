// REV-P3 §1 e §4 — estados da amostra do Simulador.
//
// Funções PURAS: sem Angular, sem signals. O projeto não tem runner de teste de
// componente (ver CORRECOES_AUDITORIA.md), então a lógica que precisa ser
// testada mora fora do template e é exercitada por sample-state.spec.ts com
// node:assert — mesmo padrão de simulator-url-state.ts.
//
// O defeito que isto corrige, medido em produção em 25/09/2026 com La Liga 2025
// (temporada inteira fora da janela de 90 dias): com ZERO partidas a tela ainda
// renderizava "Taxa de acerto 0%", "Média de escanteios 0" e sequências 0, como
// se fossem observações. Não eram: eram ausência pintada de zero.
//
// Regra: os três estados são DISTINTOS e nunca se convertem um no outro.
//
//   vazio           — não havia partida no recorte
//   metrica-ausente — havia partidas, nenhuma com o dado publicado
//   com-amostra     — há observações; zeros observados são zeros de verdade

import { BacktestResult } from '../../core/models';

export type EstadoAmostra = 'vazio' | 'metrica-ausente' | 'com-amostra';

export function estadoAmostra(r: BacktestResult): EstadoAmostra {
  if (r.match_count > 0) return 'com-amostra';
  // Sem `accounting` (resposta de um backend antigo) não dá para afirmar o
  // motivo. Cair em 'vazio' é a leitura mais conservadora: diz "não há amostra"
  // sem inventar uma explicação que o dado não sustenta.
  const a = r.accounting;
  if (!a) return 'vazio';
  if (a.matches_in_window > 0 && a.excluded_no_metric > 0) return 'metrica-ausente';
  return 'vazio';
}

/**
 * Explica em uma frase por que a amostra é menor que o recorte pedido.
 * Devolve null quando não houve exclusão — nesse caso não há o que explicar e
 * um texto genérico só faria ruído.
 */
export function explicacaoExclusoes(r: BacktestResult): string | null {
  const a = r.accounting;
  if (!a || a.excluded_entries <= 0) return null;

  const partes: string[] = [];
  if (a.excluded_no_metric > 0) {
    partes.push(`${a.excluded_no_metric} sem a estatística publicada pelo provedor`);
  }
  if (a.excluded_by_max_odds > 0) {
    partes.push(`${a.excluded_by_max_odds} com odd acima do teto informado`);
  }
  if (a.excluded_no_odd > 0) {
    partes.push(`${a.excluded_no_odd} sem odd disponível`);
  }
  if (a.excluded_by_venue > 0) {
    partes.push(`${a.excluded_by_venue} fora do mando escolhido`);
  }
  if (a.excluded_other > 0) {
    partes.push(`${a.excluded_other} por outros critérios`);
  }
  if (!partes.length) return null;

  const unidade = r.metric_scope === 'team' ? 'observações' : 'partidas';
  return `${a.observations_in_window} ${unidade} no recorte · ` +
    `${a.eligible_entries} analisadas · ${a.excluded_entries} fora: ${partes.join('; ')}.`;
}

/**
 * Aviso de "odds máximas" sem efeito.
 *
 * O usuário informou um teto de odd, mas NENHUMA observação tinha odd de mercado
 * para comparar — então o controle não agiu sobre nada. Um filtro que aparenta
 * influenciar o backtest sem influenciar é pior do que não existir, e o silêncio
 * é o que o torna enganoso.
 */
export function avisoMaxOddsSemEfeito(r: BacktestResult): string | null {
  const a = r.accounting;
  if (!a || !a.max_odds_requested || a.max_odds_requested <= 0) return null;
  if (a.max_odds_applicable > 0) return null;
  return `O teto de "odds máximas" (${a.max_odds_requested}) não foi aplicado a nenhuma ` +
    `partida: nenhuma delas tem odd de mercado registrada no banco para comparar. ` +
    `O resultado abaixo é o mesmo que você obteria sem informar esse teto.`;
}

/**
 * A coluna "Mando" só tem significado quando a ocorrência é de uma EQUIPE.
 *
 * Em regra MATCH-LEVEL (escanteios, gols, chutes, impedimentos) o valor é da
 * partida inteira: desde a correção da dupla contagem, a perspectiva do mandante
 * é apenas o representante canônico da partida. Exibir "Casa" ali — como
 * acontecia em 100/100 linhas em produção — afirma uma perspectiva que a
 * ocorrência não tem.
 */
export function mostraColunaMando(r: BacktestResult): boolean {
  return r.metric_scope === 'team';
}
