import { Season, Team } from '../../core/models';

/**
 * Resolução de temporada e equipe do Dashboard, extraída do componente para
 * poder ser testada sem Angular.
 *
 * POR QUE ISTO EXISTE — incidente de 22/09/2026, reproduzido em produção:
 *
 *   URL:        /dashboard?league_id=8&season_id=12&team_id=455&limit=15
 *   Requisição: /api/v1/dashboard?...&league_id=8&season_id=33
 *   Tela:       Temporada 2026
 *
 * `limit=15` era preservado, então a leitura dos query params funcionava. A
 * temporada 12 (La Liga 2025) sumia porque `ApiService.listSeasons()` esconde
 * temporadas anteriores ao ano corrente — filtro de interface, criado para não
 * poluir os seletores. O componente então procurava a temporada pedida numa
 * lista da qual ela já tinha sido removida, não encontrava, e caía no fallback
 * `MAX(year)`.
 *
 * Ou seja: a lógica de precedência estava correta; a LISTA é que chegava
 * truncada. Por isso a correção é pedir a lista completa quando existe pedido
 * explícito — e não mexer na precedência.
 */

export interface ResolucaoTemporada {
  /** Temporada que deve ficar selecionada. undefined = nenhuma. */
  seasonId?: number;
  /**
   * true quando veio um pedido explícito que NÃO existe nesta liga. Nesse caso
   * `seasonId` fica undefined de propósito: trocar por outra temporada em
   * silêncio é justamente o que não pode acontecer.
   */
  pedidaAusente: boolean;
}

/**
 * Regra de precedência:
 *
 *  1. temporada pedida explicitamente e existente na liga -> vence sempre;
 *  2. temporada pedida explicitamente e inexistente        -> nada é selecionado
 *     e quem chamou avisa o usuário;
 *  3. sem pedido explícito                                 -> a mais recente
 *     (MAX(year)) como fallback.
 *
 * O fallback é só isso: fallback. Nunca substitui um pedido válido.
 */
export function resolverTemporada(seasons: Season[], pedida?: number): ResolucaoTemporada {
  if (pedida !== undefined) {
    return seasons.some(s => s.id === pedida)
      ? { seasonId: pedida, pedidaAusente: false }
      : { seasonId: undefined, pedidaAusente: true };
  }
  if (!seasons.length) {
    return { seasonId: undefined, pedidaAusente: false };
  }
  return {
    seasonId: seasons.reduce((a, b) => (a.year > b.year ? a : b)).id,
    pedidaAusente: false,
  };
}

export interface ResolucaoEquipe {
  teamId?: number;
  /** true quando a equipe pedida não joga nesta liga+temporada. */
  pedidaAusente: boolean;
}

/**
 * Mesma regra para a equipe, com uma diferença deliberada: sem pedido
 * explícito, cair na primeira da lista é aceitável (é só um ponto de partida);
 * COM pedido explícito, cair na primeira seria trocar a equipe que o usuário
 * abriu — foi o que fazia clicar no Athletic Club abrir o Alavés.
 */
export function resolverEquipe(teams: Team[], pedida?: number): ResolucaoEquipe {
  if (pedida !== undefined) {
    return teams.some(t => t.id === pedida)
      ? { teamId: pedida, pedidaAusente: false }
      : { teamId: undefined, pedidaAusente: true };
  }
  return { teamId: teams.length ? teams[0].id : undefined, pedidaAusente: false };
}
