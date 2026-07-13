import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';

import { ApiService } from '../../core/api.service';
import { DashboardResult, League, Season, Team } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';

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
    SimpleChartComponent,
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
  error = signal<string | null>(null);
  result = signal<DashboardResult | null>(null);

  matchColumns = ['match_date', 'opponent', 'is_home', 'corners_for', 'corners_against', 'total_corners'];

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
    this.selectedSeasonId = undefined;
    this.api.listSeasons(this.selectedLeagueId).subscribe(s => this.seasons.set(s));
    this.api.listTeams(this.selectedLeagueId).subscribe(teams => {
      this.teams.set(teams);
      if (teams.length && !this.selectedTeamId) {
        this.selectedTeamId = teams[0].id;
      }
    });
  }

  runDashboard(): void {
    if (!this.selectedTeamId) return;
    this.loading.set(true);
    this.error.set(null);
    this.api.getDashboard(this.selectedTeamId, this.selectedLeagueId, this.selectedSeasonId, this.limit).subscribe({
      next: res => {
        this.result.set(res);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar dashboard');
        this.loading.set(false);
      },
    });
  }

  trendLabels(): number[] {
    const r = this.result();
    return r ? r.trend.map((_, i) => i + 1) : [];
  }
}
