import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

import { ApiService } from '../../core/api.service';
import { ComparisonResult, League, Team } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';

@Component({
  selector: 'app-comparator',
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
    SimpleChartComponent,
  ],
  templateUrl: './comparator.component.html',
})
export class ComparatorComponent implements OnInit {
  leagues = signal<League[]>([]);
  teams = signal<Team[]>([]);

  selectedLeagueId?: number;
  teamAId?: number;
  teamBId?: number;
  limit = 10;

  loading = signal(false);
  error = signal<string | null>(null);
  result = signal<ComparisonResult | null>(null);

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
    this.api.listTeams(this.selectedLeagueId).subscribe(teams => {
      this.teams.set(teams);
      if (teams.length >= 2) {
        this.teamAId = teams[0].id;
        this.teamBId = teams[1].id;
      }
    });
  }

  runCompare(): void {
    if (!this.teamAId || !this.teamBId) return;
    this.loading.set(true);
    this.error.set(null);
    this.api.compare(this.teamAId, this.teamBId, this.selectedLeagueId, this.limit).subscribe({
      next: res => {
        this.result.set(res);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao comparar equipes');
        this.loading.set(false);
      },
    });
  }

  trendLabels(len: number): number[] {
    return Array.from({ length: len }, (_, i) => i + 1);
  }
}
