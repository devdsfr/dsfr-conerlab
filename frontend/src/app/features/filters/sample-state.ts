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

import { BacktestEntry, BacktestResult } from '../../core/models';

export type EstadoAmostra = 'vazio' | 'metrica-ausente' | 'com-amostra';

export function estadoAmostra(r: BacktestResult): EstadoAmostra {
  if (r.match_count > 0) return 'com-amostra';
  // Sem `accounting` (resposta de um backend antigo) não dá para afirmar o
  // motivo. Cair em 'vazio' é a leitura mais conservadora: diz "não há amostra"
  // sem inventar uma explicação que o dado não sustenta.
  const a = r.accounting;
  if (!a) return 'vazio';
  // REV-P3 item 33: o critério usa o recorte EFETIVO (observações depois do
  // filtro de equipe), não a competição inteira. Com equipe filtrada,
  // matches_in_window pode ser 108 enquanto a equipe tem 4 observações.
  if (a.observations_in_window > 0 && a.excluded_no_metric > 0) return 'metrica-ausente';
  return 'vazio';
}

/**
 * Unidade da contagem do recorte efetivo.
 *
 * Em regra match-level cada observação É uma partida (inclusive com filtro de
 * equipe: são as partidas daquela equipe). Em regra team-level uma partida gera
 * até duas observações, e chamá-las de "partidas" dobraria a contagem.
 */
function unidadeDoRecorte(r: BacktestResult, n: number): string {
  if (r.metric_scope === 'team') return n === 1 ? 'observação' : 'observações';
  return n === 1 ? 'partida' : 'partidas';
}

/**
 * Mensagem do estado 'metrica-ausente'.
 *
 * REV-P3 item 33 — defeito medido em produção (25/09/2026, Champions 2026,
 * Egnatia Rrogozhinë, impedimentos): a tela dizia "O recorte tem 108 partida(s),
 * mas o provedor não publicou esta métrica em nenhuma delas". Falso: 53 das 108
 * partidas da competição têm o dado. As 108 eram `matches_in_window`, contadas
 * ANTES do filtro de equipe; o recorte da simulação eram 4 observações.
 *
 * A contagem certa é `excluded_no_metric` sobre `observations_in_window`: ambas
 * do recorte efetivo, e é delas que a frase "nenhuma delas" fala.
 */
export function mensagemMetricaAusente(r: BacktestResult): string {
  const a = r.accounting;
  const n = a?.observations_in_window ?? 0;
  const sem = a?.excluded_no_metric ?? 0;
  const u = unidadeDoRecorte(r, n);
  if (sem === n) {
    return `O recorte selecionado tem ${n} ${u}, mas o provedor não publicou esta ` +
      `métrica em nenhuma delas.`;
  }
  // Nem todas saíram por falta de métrica (ex.: parte saiu por mando). Dizer
  // "nenhuma delas" aqui voltaria a ser falso — então a frase separa os números.
  return `O recorte selecionado tem ${n} ${u}; em ${sem} delas o provedor não ` +
    `publicou esta métrica, e as demais ficaram de fora por outros critérios.`;
}

// =============================================================================
// REV-P3 item 7 — mercados CATEGÓRICOS não têm valor numérico
//
// Vitória, empate e "não perde" são desfechos: a observação é acerto/erro, não
// uma contagem como escanteios, gols, chutes ou impedimentos. O componente
// resolvia o valor da linha e a média com um `switch` cujo `default` era
// escanteios — e o backend não preenche escanteios nesses mercados. Resultado
// medido em produção: coluna "Vitória = 0" em 200/200 linhas e card
// "Média de vitória 0". Ausência exibida como zero.
//
// A classificação fica AQUI, num lugar só, e tudo o que depende dela a consulta.
// Não se converte booleano em 0/1 para fabricar uma "média de vitória": a
// proporção de acertos já existe, e se chama taxa de acerto.
// =============================================================================

export type TipoMercado = 'numerico' | 'categorico';

// Valores de `metric` do contrato. "Não perde" é `win_or_draw` no código.
const MERCADOS_CATEGORICOS: ReadonlySet<string> = new Set(['win', 'draw', 'win_or_draw']);

export function tipoMercado(metric: string | undefined | null): TipoMercado {
  return metric && MERCADOS_CATEGORICOS.has(metric) ? 'categorico' : 'numerico';
}

/**
 * Valor observado da linha para a métrica COM QUE O RESULTADO FOI GERADO.
 *
 * Recebe a métrica do resultado (`r.metric`), não a do formulário: se o usuário
 * troca a métrica depois de rodar, a tabela antiga não pode passar a ler outro
 * campo. Devolve null em mercado categórico — não existe número a mostrar.
 */
export function valorObservado(
  metric: string | undefined | null,
  e: Pick<BacktestEntry, 'total_corners' | 'total_goals' | 'total_offsides' | 'total_shots' | 'total_shots_on_target'>,
): number | null {
  if (tipoMercado(metric) === 'categorico') return null;
  switch (metric) {
    case 'goals': return e.total_goals;
    case 'offsides': return e.total_offsides;
    case 'shots': return e.total_shots;
    case 'shots_on_target': return e.total_shots_on_target;
    default: return e.total_corners;
  }
}

/** Média observada da métrica numérica; null (N/A) em mercado categórico. */
export function mediaObservada(r: BacktestResult): number | null {
  if (tipoMercado(r.metric) === 'categorico') return null;
  switch (r.metric) {
    case 'goals': return r.average_goals;
    case 'offsides': return r.average_offsides;
    case 'shots': return r.average_shots;
    case 'shots_on_target': return r.average_shots_on_target;
    default: return r.average_corners;
  }
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

const ROTULOS: Readonly<Record<string, string>> = {
  corners: 'Escanteios',
  goals: 'Gols',
  offsides: 'Impedimentos',
  shots: 'Chutes',
  shots_on_target: 'Chutes no gol',
  win: 'Vitória',
  draw: 'Empate',
  win_or_draw: 'Não perde',
};

/** Rótulo humano da métrica. Desconhecida → o próprio código, nunca "Escanteios". */
export function rotuloMetrica(metric: string | undefined | null): string {
  return (metric && ROTULOS[metric]) || String(metric ?? '');
}
