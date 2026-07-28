import { Component, OnInit, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatIconModule } from '@angular/material/icon';

import { ApiService } from '../../core/api.service';
import { DashboardResult, League, Season, Team, TeamMatchView } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';
import { AdSlotComponent } from '../../shared/ad-slot.component';

const VALID_LIMITS = [5, 10, 15, 20];

type MatchSortColumn = 'match_date' | 'opponent' | 'is_home' | 'corners_for' | 'corners_against' | 'total_corners';

// Rótulo/valor de um gráfico já "congelado": só é recalculado quando um novo
// resultado chega do backend, nunca a cada ciclo de change detection. Isso
// evita recriar o gráfico (e, com ele, o bug de canvas em branco) a cada
// digest do Angular — ver comentário em shared/simple-chart.component.ts.
interface ChartData {
  labels: number[];
  datasets: { label: string; data: number[] }[];
}

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatSelectModule,
    MatButtonModule,
    MatButtonToggleModule,
    MatProgressSpinnerModule,
    MatTableModule,
    MatTooltipModule,
    MatIconModule,
    SimpleChartComponent,
    AdSlotComponent,
  ],
  templateUrl: './dashboard.component.html',
})
export class DashboardComponent implements OnInit {
  leagues = signal<League[]>([]);
  seasons = signal<Season[]>([]);
  teams = signal<Team[]>([]);

  selectedLeagueId?: number;
  selectedSeasonId?: number;
  selectedTeamId?: number;
  limit = 10;

  loading = signal(false);
  // Loading dedicado para os combos "Campeonato"/"Equipe" — sem isso, o combo fica
  // com a aparência de travado (vazio, sem feedback) durante o fetch inicial.
  leaguesLoading = signal(true);
  teamsLoading = signal(false);
  error = signal<string | null>(null);
  result = signal<DashboardResult | null>(null);

  // Métrica ativa — escanteios, gols ou impedimentos. O backend manda as três no mesmo
  // response, então a troca de aba é instantânea (sem refetch).
  metric = signal<'corners' | 'goals' | 'offsides'>('corners');

  setMetric(m: 'corners' | 'goals' | 'offsides'): void {
    this.metric.set(m);
  }

  // Gráfico de tendência derivado da métrica ativa (total por jogo).
  trendChart = computed<ChartData>(() => {
    const r = this.result();
    if (!r) return { labels: [], datasets: [] };
    const m = this.metric();
    const data = m === 'goals' ? r.goal_trend : m === 'offsides' ? r.offside_trend : r.trend;
    const label = m === 'goals' ? 'Gols' : m === 'offsides' ? 'Impedimentos' : 'Escanteios';
    return {
      labels: data.map((_, i) => i + 1),
      datasets: [{ label, data }],
    };
  });

  matchColumns = ['match_date', 'opponent', 'is_home', 'corners_for', 'corners_against', 'total_corners', 'expand'];

  // Linha expansível "Últimos jogos" — mostra posse, chutes, cartões etc. sem
  // poluir a tabela principal (dado complementar, ver conversa sobre estatísticas
  // extras da API-Football). Guardado por match_id, não por índice, pra sobreviver
  // à reordenação da tabela.
  private expandedMatchIds = signal<Set<number>>(new Set());

  toggleMatchDetails(matchId: number): void {
    const current = new Set(this.expandedMatchIds());
    if (current.has(matchId)) {
      current.delete(matchId);
    } else {
      current.add(matchId);
    }
    this.expandedMatchIds.set(current);
  }

  isMatchExpanded(matchId: number): boolean {
    return this.expandedMatchIds().has(matchId);
  }

  // Ordenação manual da tabela "Últimos jogos" (ver setSort/sortedMatches) —
  // clique no cabeçalho da coluna para alternar asc/desc.
  sortColumn = signal<MatchSortColumn>('match_date');
  sortDir = signal<'asc' | 'desc'>('desc');

  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo. Valores baixos indicam resultados mais imprevisíveis.';
  readonly stdDevTooltip =
    'Desvio padrão: mede o quanto os valores de escanteios costumam se afastar da média. Quanto maior, mais irregulares foram os jogos dessa amostra.';
  readonly modeTooltip =
    'Moda: o(s) valor(es) de escanteios que mais se repetiram na amostra — pode haver mais de um em caso de empate.';

  constructor(private api: ApiService, private router: Router, private route: ActivatedRoute) {}

  ngOnInit(): void {
    // Restaura a última seleção (e reexecuta a análise) a partir da URL, para o
    // usuário não perder o resultado ao trocar de aba e voltar — ver
    // syncQueryParams(). Também permite compartilhar/favoritar o link de uma
    // análise específica.
    const qp = this.route.snapshot.queryParamMap;
    const qpLeagueId = qp.get('league_id') ? Number(qp.get('league_id')) : undefined;
    const qpSeasonId = qp.get('season_id') ? Number(qp.get('season_id')) : undefined;
    const qpTeamId = qp.get('team_id') ? Number(qp.get('team_id')) : undefined;
    const qpLimit = Number(qp.get('limit'));
    if (VALID_LIMITS.includes(qpLimit)) this.limit = qpLimit;

    this.leaguesLoading.set(true);
    this.api.listLeagues().subscribe({
      next: leagues => {
        this.leagues.set(leagues);
        this.leaguesLoading.set(false);
        if (leagues.length) {
          this.selectedLeagueId = qpLeagueId && leagues.some(l => l.id === qpLeagueId) ? qpLeagueId : leagues[0].id;
          this.onLeagueChange(qpSeasonId, qpTeamId);
        }
      },
      error: err => {
        this.leaguesLoading.set(false);
        this.error.set(err?.error?.error ?? 'Erro ao carregar campeonatos');
      },
    });
  }

  onLeagueChange(presetSeasonId?: number, presetTeamId?: number): void {
    if (!this.selectedLeagueId) return;
    this.selectedSeasonId = undefined;
    this.teamsLoading.set(true);
    // Temporada é resolvida ANTES de buscar as equipes (não mais em paralelo):
    // a lista de equipes depende de qual temporada fica selecionada por padrão,
    // senão equipes de temporadas passadas (ex: rebaixadas) aparecem como se
    // ainda estivessem na liga atual — ver loadTeams().
    this.api.listSeasons(this.selectedLeagueId).subscribe({
      next: seasons => {
        this.seasons.set(seasons);
        // Evita o campo "Temporada" ficar vazio (tela morta ao clicar em
        // Analisar sem nenhuma seleção visível): assume a mais recente por
        // padrão. "Todas" continua disponível como opção explícita. Se veio
        // um valor restaurado da URL (voltando de outra aba), prevalece.
        if (presetSeasonId !== undefined && seasons.some(s => s.id === presetSeasonId)) {
          this.selectedSeasonId = presetSeasonId;
        } else if (seasons.length) {
          this.selectedSeasonId = seasons.reduce((a, b) => (a.year > b.year ? a : b)).id;
        }
        this.loadTeams(presetTeamId);
      },
      error: () => this.teamsLoading.set(false),
    });
  }

  /** Recarrega a lista de equipes para a liga+temporada atualmente selecionadas.
   * preferredTeamId é mantido se ainda existir na nova lista; senão, cai para a
   * primeira equipe — evita a análise rodar com uma equipe de outra liga/temporada
   * (amostra de 0 jogos). */
  private loadTeams(preferredTeamId?: number): void {
    this.teamsLoading.set(true);
    this.api.listTeams(this.selectedLeagueId, undefined, this.selectedSeasonId).subscribe({
      next: teams => {
        this.teams.set(teams);
        if (preferredTeamId !== undefined && teams.some(t => t.id === preferredTeamId)) {
          this.selectedTeamId = preferredTeamId;
        } else {
          this.selectedTeamId = teams.length ? teams[0].id : undefined;
        }
        this.teamsLoading.set(false);

        if (preferredTeamId !== undefined && this.selectedTeamId === preferredTeamId) {
          this.runDashboard();
        } else {
          this.result.set(null);
          this.syncQueryParams();
        }
      },
      error: () => this.teamsLoading.set(false),
    });
  }

  /** Chamado ao trocar a Temporada sem trocar o Campeonato — a lista de equipes
   * precisa ser recarregada para a nova temporada (ver loadTeams()). */
  onSeasonChange(): void {
    this.loadTeams(this.selectedTeamId);
  }

  runDashboard(): void {
    if (!this.selectedTeamId) return;
    this.loading.set(true);
    this.error.set(null);
    this.api.getDashboard(this.selectedTeamId, this.selectedLeagueId, this.selectedSeasonId, this.limit).subscribe({
      next: res => {
        this.result.set(res);
        // trendChart é computed a partir de result() + metric() — nada a setar aqui.
        this.loading.set(false);
        this.syncQueryParams();
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar dashboard');
        this.loading.set(false);
      },
    });
  }

  // Reflete a seleção atual na URL (sem poluir o histórico do navegador — ver
  // replaceUrl) para sobreviver à navegação entre abas do SPA e permitir
  // compartilhar/favoritar o link de uma análise específica.
  syncQueryParams(): void {
    this.router.navigate([], {
      relativeTo: this.route,
      queryParams: {
        league_id: this.selectedLeagueId ?? null,
        season_id: this.selectedSeasonId ?? null,
        team_id: this.selectedTeamId ?? null,
        limit: this.limit,
      },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  selectedLeagueName(): string {
    return this.leagues().find(l => l.id === this.selectedLeagueId)?.name ?? '';
  }

  selectedTeamName(): string {
    return this.teams().find(t => t.id === this.selectedTeamId)?.name ?? '';
  }

  setSort(column: MatchSortColumn): void {
    if (this.sortColumn() === column) {
      this.sortDir.set(this.sortDir() === 'asc' ? 'desc' : 'asc');
    } else {
      this.sortColumn.set(column);
      this.sortDir.set('desc');
    }
  }

  sortArrow(column: MatchSortColumn): string {
    if (this.sortColumn() !== column) return '';
    return this.sortDir() === 'asc' ? '▲' : '▼';
  }

  sortedMatches(matches: TeamMatchView[]): TeamMatchView[] {
    const col = this.sortColumn();
    const dir = this.sortDir() === 'asc' ? 1 : -1;
    const value = (m: TeamMatchView): number | string => {
      switch (col) {
        case 'match_date': return m.match_date;
        case 'opponent': return m.opponent.name;
        case 'is_home': return m.is_home ? 1 : 0;
        // As colunas "A favor/Sofridos/Total" mostram a métrica ativa; a ordenação
        // segue a mesma métrica. Impedimentos ausentes (null) vão pro fim (-1).
        case 'corners_for': return this.matchFor(m) ?? -1;
        case 'corners_against': return this.matchAgainst(m) ?? -1;
        case 'total_corners': return this.matchTotal(m) ?? -1;
      }
    };
    return [...matches].sort((a, b) => {
      const va = value(a);
      const vb = value(b);
      if (va < vb) return -1 * dir;
      if (va > vb) return 1 * dir;
      return 0;
    });
  }

  // --- Acessores da métrica ativa — evitam duplicar o template. Impedimentos podem
  // vir nulos por jogo (nem todo provedor publica), daí o retorno number | null.
  matchFor(m: TeamMatchView): number | null {
    const mt = this.metric();
    if (mt === 'goals') return m.goals_for;
    if (mt === 'offsides') return m.offsides_for ?? null;
    return m.corners_for;
  }
  matchAgainst(m: TeamMatchView): number | null {
    const mt = this.metric();
    if (mt === 'goals') return m.goals_against;
    if (mt === 'offsides') return m.offsides_against ?? null;
    return m.corners_against;
  }
  matchTotal(m: TeamMatchView): number | null {
    const mt = this.metric();
    if (mt === 'goals') return m.total_goals;
    if (mt === 'offsides') {
      if (m.offsides_for == null || m.offsides_against == null) return null;
      return m.offsides_for + m.offsides_against;
    }
    return m.total_corners;
  }

  // Bloco consolidado da métrica ativa (summaries, saldo, frequências, casa/fora e
  // rótulos), recalculado só quando result() ou metric() mudam.
  readonly view = computed(() => {
    const r = this.result();
    if (!r) return null;
    const m = this.metric();
    const goals = m === 'goals';
    const offs = m === 'offsides';
    return {
      metric: m,
      forSummary: offs ? r.offsides_for : goals ? r.goals_for : r.corners_for,
      againstSummary: offs ? r.offsides_against : goals ? r.goals_against : r.corners_against,
      totalSummary: offs ? r.total_offsides : goals ? r.total_goals : r.total_corners,
      balance: offs ? r.offside_balance : goals ? r.goal_balance : r.balance,
      frequencies: offs ? r.offside_frequencies : goals ? r.goal_frequencies : r.frequencies,
      homeStats: offs ? r.offside_home_stats : goals ? r.goal_home_stats : r.home_stats,
      awayStats: offs ? r.offside_away_stats : goals ? r.goal_away_stats : r.away_stats,
      forLabel: offs ? 'Impedimentos cometidos' : goals ? 'Gols marcados' : 'Escanteios conquistados',
      againstLabel: offs ? 'Impedimentos do adversário' : goals ? 'Gols sofridos' : 'Escanteios sofridos',
      totalLabel: offs ? 'Total de impedimentos' : goals ? 'Total de gols' : 'Total de escanteios',
      freqTitle: offs ? 'Frequências (total de impedimentos acima de N)' : goals ? 'Frequências (total de gols acima de N)' : 'Frequências (total de escanteios acima de N)',
      trendTitle: offs ? 'Tendência (total de impedimentos por jogo)' : goals ? 'Tendência (total de gols por jogo)' : 'Tendência (total de escanteios por jogo)',
      // Impedimentos têm amostra própria (só jogos com o dado). Vazia => aviso.
      noData: offs && r.offside_sample_size === 0,
      sampleNote: offs && r.offside_sample_size > 0 && r.offside_sample_size < r.sample_size
        ? `Impedimentos disponíveis em ${r.offside_sample_size} de ${r.sample_size} jogos.`
        : '',
    };
  });

  // Rótulo de cada linha de frequência: gols e impedimentos usam linha over/under
  // (0.5, 1.5...), escanteios usam inteiro.
  freqLabel(threshold: number): string {
    return this.metric() === 'corners' ? `Acima de ${threshold}` : `Acima de ${threshold}.5`;
  }
}
