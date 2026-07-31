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
import { BacktestEntry, BacktestResult, League, Season, Team } from '../../core/models';
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

  readonly drawdownTooltip =
    'Drawdown máximo: a maior sequência de perdas acumuladas (em unidades de stake) observada durante o backtest — indica o pior momento de "prejuízo" pelo qual a estratégia passou.';
  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo.';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.listLeagues().subscribe(leagues => {
      this.leagues.set(leagues);
      if (leagues.length) {
        this.selectedLeagueId = leagues[0].id;
        this.onLeagueChange();
      }
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
