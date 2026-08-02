import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { BacktestEntry, BacktestResult, FilterRunRequest, League, Season, Team } from '../../core/models';
import { AdSlotComponent } from '../../shared/ad-slot.component';

@Component({
  selector: 'app-filters',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    RouterLink,
    MatCardModule,
    MatFormFieldModule,
    MatSelectModule,
    MatInputModule,
    MatButtonModule,
    MatButtonToggleModule,
    MatProgressSpinnerModule,
    MatTableModule,
    MatChipsModule,
    MatTooltipModule,
    AdSlotComponent,
  ],
  templateUrl: './filters.component.html',
})
export class FiltersComponent implements OnInit {
  leagues = signal<League[]>([]);
  seasons = signal<Season[]>([]);
  teams = signal<Team[]>([]);

  // critérios do filtro (espelham FilterRunRequest)
  selectedLeagueId?: number;
  selectedSeasonIds: number[] = [];
  selectedTeamId?: number; // opcional: restringe a uma equipe
  // 0 = "Todos" (nenhum limite de últimos jogos) — mesmo controle segmentado
  // usado no Dashboard e no Comparador, para padronizar a interação entre telas.
  lastNGames = 10;
  homeAway = ''; // '', 'home', 'away'
  cornersThreshold = 5;
  opponentTier = '';
  maxOdds?: number;
  stake = 10;

  // Métrica: 'corners' (com odds históricas) ou as demais (odd fixa simulada). Cada
  // uma tem seu threshold (linha over/under: 2 = "acima de 2.5"; chutes usam faixas
  // inteiras maiores).
  metric: 'corners' | 'goals' | 'offsides' | 'shots' | 'shots_on_target' = 'corners';
  goalsThreshold = 2;
  offsidesThreshold = 2;
  shotsThreshold = 20;
  shotsOnTargetThreshold = 6;
  fixedOdd?: number;

  loading = signal(false);
  error = signal<string | null>(null);
  result = signal<BacktestResult | null>(null);

  entryColumns = ['match_date', 'team', 'opponent', 'is_home', 'total', 'hit', 'odd', 'profit_loss'];

  get isGoals(): boolean {
    return this.metric === 'goals';
  }
  get isOffsides(): boolean {
    return this.metric === 'offsides';
  }
  get isShots(): boolean {
    return this.metric === 'shots';
  }
  get isShotsOnTarget(): boolean {
    return this.metric === 'shots_on_target';
  }
  // Métricas sem odds históricas (usam odd fixa simulada) e nullable (jogos sem o dado
  // ficam de fora do backtest).
  get usesFixedOdd(): boolean {
    return this.metric !== 'corners';
  }
  get isNullableMetric(): boolean {
    return this.metric === 'offsides' || this.metric === 'shots' || this.metric === 'shots_on_target';
  }
  private label(): string {
    return { corners: 'Escanteios', goals: 'Gols', offsides: 'Impedimentos', shots: 'Chutes', shots_on_target: 'Chutes no gol' }[this.metric];
  }
  // Total da métrica ativa por entrada.
  entryTotal(e: BacktestEntry): number {
    switch (this.metric) {
      case 'goals': return e.total_goals;
      case 'offsides': return e.total_offsides;
      case 'shots': return e.total_shots;
      case 'shots_on_target': return e.total_shots_on_target;
      default: return e.total_corners;
    }
  }
  entryLabel(): string {
    return this.label();
  }
  averageValue(r: BacktestResult): number {
    switch (this.metric) {
      case 'goals': return r.average_goals;
      case 'offsides': return r.average_offsides;
      case 'shots': return r.average_shots;
      case 'shots_on_target': return r.average_shots_on_target;
      default: return r.average_corners;
    }
  }
  averageLabel(): string {
    return `Média de ${this.label().toLowerCase()}`;
  }

  // ---- Análise de valor / odd -------------------------------------------------
  // A odd que a casa oferece para o evento — usada só na calculadora de valor, não
  // altera o backtest. hit_rate e roi já vêm em % do backend.
  marketOdd?: number;

  private hitRateFraction(r: BacktestResult): number {
    return (r.hit_rate ?? 0) / 100;
  }

  // Odd mínima para a estratégia empatar no longo prazo = 1 / taxa de acerto. Acima
  // disso é lucro esperado; abaixo, prejuízo esperado.
  breakEvenOdd(r: BacktestResult): number | null {
    const p = this.hitRateFraction(r);
    return p > 0 ? 1 / p : null;
  }

  // ROI esperado por aposta para uma dada odd = taxa de acerto × odd − 1 (em %).
  expectedRoiPct(r: BacktestResult, odd: number): number | null {
    if (!odd || odd <= 1) return null;
    return (this.hitRateFraction(r) * odd - 1) * 100;
  }

  marketRoiPct(r: BacktestResult): number | null {
    return this.marketOdd ? this.expectedRoiPct(r, this.marketOdd) : null;
  }

  // A cada 10 apostas, quantas dá pra errar e ainda sair no lucro, para uma odd.
  // Precisa vencer mais que 10/odd unidades; mínimo de vitórias = floor(10/odd)+1.
  safetyMissesPer10(odd?: number): number | null {
    if (!odd || odd <= 1) return null;
    const minWins = Math.floor(10 / odd) + 1;
    return Math.max(0, 10 - minWins);
  }

  // Cenários de odd para a mini-tabela comparativa (usa a taxa de acerto do filtro).
  readonly oddScenarios = [1.5, 1.8, 2.0, 2.5, 3.0, 4.0];
  scenarioRows(r: BacktestResult): { odd: number; roiPct: number; positive: boolean }[] {
    return this.oddScenarios.map(odd => {
      const roi = this.expectedRoiPct(r, odd) ?? 0;
      return { odd, roiPct: roi, positive: roi > 0 };
    });
  }

  readonly drawdownTooltip =
    'Drawdown máximo: a maior sequência de perdas acumuladas (em unidades de stake) observada durante o backtest — indica o pior momento de "prejuízo" pelo qual a estratégia passou.';
  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo.';

  constructor(private api: ApiService, public auth: AuthService) {}

  // ---- Salvar como estratégia (Strategy Workspace, Remodelagem F5) ----------
  strategyName = '';
  savingStrategy = signal(false);
  strategySaved = signal(false);
  strategyError = signal<string | null>(null);

  // Monta a definition persistida — mesmo payload do backtest atual.
  private buildDefinition(): string {
    return JSON.stringify({
      league_id: this.selectedLeagueId,
      season_ids: this.selectedSeasonIds,
      team_id: this.selectedTeamId ?? undefined,
      last_n_games: this.lastNGames || undefined,
      home_away: this.homeAway || undefined,
      corners_threshold: this.cornersThreshold,
      opponent_tier: this.opponentTier || undefined,
      max_odds: this.usesFixedOdd ? undefined : (this.maxOdds || undefined),
      stake: this.stake || undefined,
      metric: this.metric,
      goals_threshold: this.isGoals ? this.goalsThreshold : undefined,
      offsides_threshold: this.isOffsides ? this.offsidesThreshold : undefined,
      shots_threshold: this.isShots ? this.shotsThreshold : undefined,
      shots_on_target_threshold: this.isShotsOnTarget ? this.shotsOnTargetThreshold : undefined,
      fixed_odd: this.usesFixedOdd ? (this.fixedOdd || undefined) : undefined,
    });
  }

  saveAsStrategy(): void {
    if (!this.selectedLeagueId || !this.strategyName.trim()) return;
    this.savingStrategy.set(true);
    this.strategyError.set(null);
    this.api.createStrategy({
      name: this.strategyName.trim(),
      description: `Criada a partir do Simulador de Filtros (${this.label()})`,
      definition: this.buildDefinition(),
    }).subscribe({
      next: () => {
        this.savingStrategy.set(false);
        this.strategySaved.set(true);
        this.strategyName = '';
        setTimeout(() => this.strategySaved.set(false), 6000);
      },
      error: err => {
        this.strategyError.set(err?.error?.error ?? 'Erro ao salvar estratégia');
        this.savingStrategy.set(false);
      },
    });
  }

  // Sinaliza no template que o formulário foi pré-preenchido por uma descoberta,
  // e não montado pelo próprio usuário.
  loadedFromDiscovery = signal(false);

  ngOnInit(): void {
    // Definition vinda da página "Descobertas" (router state). O usuário clicou em
    // "Conferir no Simulador": reproduzibilidade é um princípio da plataforma, então
    // ele precisa poder reexecutar EXATAMENTE o backtest que gerou o número do
    // ranking e conferir jogo por jogo, em vez de confiar na lista.
    const incoming = (history.state?.definition ?? null) as FilterRunRequest | null;

    this.api.listLeagues().subscribe(leagues => {
      this.leagues.set(leagues);
      if (!leagues.length) return;

      if (incoming?.league_id) {
        this.selectedLeagueId = incoming.league_id;
        this.applyDefinition(incoming);
        return;
      }
      this.selectedLeagueId = leagues[0].id;
      this.onLeagueChange();
    });
  }

  // Aplica uma definition salva/descoberta no formulário. Diferente de
  // onLeagueChange(), NUNCA limpa a seleção — as listas dependentes (temporadas,
  // equipes) são carregadas em volta dos valores que acabaram de chegar.
  private applyDefinition(d: FilterRunRequest): void {
    this.metric = (d.metric as typeof this.metric) || 'corners';
    this.cornersThreshold = d.corners_threshold ?? this.cornersThreshold;
    this.goalsThreshold = d.goals_threshold ?? this.goalsThreshold;
    this.offsidesThreshold = d.offsides_threshold ?? this.offsidesThreshold;
    this.shotsThreshold = d.shots_threshold ?? this.shotsThreshold;
    this.shotsOnTargetThreshold = d.shots_on_target_threshold ?? this.shotsOnTargetThreshold;
    this.fixedOdd = d.fixed_odd ?? undefined;

    this.homeAway = d.home_away ?? '';
    this.lastNGames = d.last_n_games ?? 0;
    this.opponentTier = d.opponent_tier ?? '';
    this.maxOdds = d.max_odds ?? undefined;
    this.stake = d.stake ?? this.stake;
    this.selectedTeamId = d.team_id ?? undefined;
    this.loadedFromDiscovery.set(true);

    this.api.listSeasons(this.selectedLeagueId!).subscribe(s => {
      this.seasons.set(s);
      // Temporadas que não existem mais na liga são ignoradas; sem interseção,
      // cai no comportamento padrão (todas).
      const wanted = (d.season_ids ?? []).filter(id => s.some(x => x.id === id));
      this.selectedSeasonIds = wanted.length ? wanted : s.map(x => x.id);
      this.reloadTeams();
      this.runFilter();
    });
  }

  onLeagueChange(): void {
    if (!this.selectedLeagueId) return;
    this.selectedSeasonIds = [];
    this.selectedTeamId = undefined;
    this.api.listSeasons(this.selectedLeagueId).subscribe(s => {
      this.seasons.set(s);
      this.selectedSeasonIds = s.map(x => x.id); // por padrão, roda em todas as temporadas
      this.reloadTeams();
    });
  }

  // O dropdown de times reflete só quem jogou a liga nas temporadas selecionadas —
  // sem isso, um time rebaixado (ex: Atlético-GO na Série A) aparecia para sempre
  // via vínculo histórico. Recarrega ao trocar a seleção de temporadas.
  onSeasonChange(): void {
    this.reloadTeams();
  }

  private reloadTeams(): void {
    if (!this.selectedLeagueId) return;
    this.api.listTeams(this.selectedLeagueId, undefined, this.selectedSeasonIds).subscribe(t => {
      this.teams.set(t);
      // se o time escolhido não joga mais nas temporadas selecionadas, limpa.
      if (this.selectedTeamId && !t.some(x => x.id === this.selectedTeamId)) {
        this.selectedTeamId = undefined;
      }
    });
  }

  runFilter(): void {
    if (!this.selectedLeagueId) return;
    this.loading.set(true);
    this.error.set(null);
    this.api.runFilter({
      league_id: this.selectedLeagueId,
      season_ids: this.selectedSeasonIds,
      team_id: this.selectedTeamId ?? null,
      last_n_games: this.lastNGames || undefined,
      home_away: this.homeAway || undefined,
      // corners_threshold sempre vai (ignorado no backend quando metric=goals);
      // para gols enviamos metric/goals_threshold/fixed_odd.
      corners_threshold: this.cornersThreshold,
      opponent_tier: this.opponentTier || undefined,
      max_odds: this.usesFixedOdd ? undefined : (this.maxOdds || undefined),
      stake: this.stake || undefined,
      metric: this.metric,
      goals_threshold: this.isGoals ? this.goalsThreshold : undefined,
      offsides_threshold: this.isOffsides ? this.offsidesThreshold : undefined,
      shots_threshold: this.isShots ? this.shotsThreshold : undefined,
      shots_on_target_threshold: this.isShotsOnTarget ? this.shotsOnTargetThreshold : undefined,
      fixed_odd: this.usesFixedOdd ? (this.fixedOdd || undefined) : undefined,
    }).subscribe({
      next: res => {
        this.result.set(res);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao executar o filtro');
        this.loading.set(false);
      },
    });
  }

  selectedLeagueName(): string {
    return this.leagues().find(l => l.id === this.selectedLeagueId)?.name ?? '';
  }
}
