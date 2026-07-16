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
import { BacktestResult, League, Season, Team } from '../../core/models';
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

  loading = signal(false);
  error = signal<string | null>(null);
  result = signal<BacktestResult | null>(null);

  entryColumns = ['match_date', 'team', 'opponent', 'is_home', 'total_corners', 'hit', 'odd', 'profit_loss'];

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
    this.api.listSeasons(this.selectedLeagueId).subscribe(s => {
      this.seasons.set(s);
      this.selectedSeasonIds = s.map(x => x.id); // por padrão, roda em todas as temporadas
    });
    this.api.listTeams(this.selectedLeagueId).subscribe(t => this.teams.set(t));
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
      corners_threshold: this.cornersThreshold,
      opponent_tier: this.opponentTier || undefined,
      max_odds: this.maxOdds || undefined,
      stake: this.stake || undefined,
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
