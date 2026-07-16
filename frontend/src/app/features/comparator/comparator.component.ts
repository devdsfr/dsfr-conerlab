import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { ComparisonResult, League, Team } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';
import { AdSlotComponent } from '../../shared/ad-slot.component';

interface ChartData {
  labels: (string | number)[];
  datasets: { label: string; data: number[] }[];
}

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
    MatTooltipModule,
    SimpleChartComponent,
    AdSlotComponent,
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

  // Inputs dos gráficos calculados uma única vez por resultado (não a cada
  // change detection) — ver comentário em shared/simple-chart.component.ts.
  chartA = signal<ChartData>({ labels: [], datasets: [] });
  chartB = signal<ChartData>({ labels: [], datasets: [] });
  barChart = signal<ChartData>({ labels: [], datasets: [] });

  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo. Valores baixos indicam resultados mais imprevisíveis.';
  readonly stdDevTooltip =
    'Desvio padrão: mede o quanto os valores de escanteios costumam se afastar da média. Quanto maior, mais irregulares foram os jogos dessa amostra.';

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
        this.chartA.set({
          labels: res.team_a.trend.map((_, i) => i + 1),
          datasets: [{ label: res.team_a.team.short_name, data: res.team_a.trend }],
        });
        this.chartB.set({
          labels: res.team_b.trend.map((_, i) => i + 1),
          datasets: [{ label: res.team_b.team.short_name, data: res.team_b.trend }],
        });
        this.barChart.set({
          labels: ['Total', 'A favor', 'Sofridos', 'Casa', 'Fora'],
          datasets: [
            {
              label: res.team_a.team.short_name,
              data: [
                res.team_a.total_corners.mean,
                res.team_a.corners_for.mean,
                res.team_a.corners_against.mean,
                res.team_a.home?.mean ?? 0,
                res.team_a.away?.mean ?? 0,
              ],
            },
            {
              label: res.team_b.team.short_name,
              data: [
                res.team_b.total_corners.mean,
                res.team_b.corners_for.mean,
                res.team_b.corners_against.mean,
                res.team_b.home?.mean ?? 0,
                res.team_b.away?.mean ?? 0,
              ],
            },
          ],
        });
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao comparar equipes');
        this.loading.set(false);
      },
    });
  }

  selectedLeagueName(): string {
    return this.leagues().find(l => l.id === this.selectedLeagueId)?.name ?? '';
  }

  // Explica por que o botão "Comparar" está desabilitado — sem isso o botão só
  // aparece esmaecido, sem indicar o que falta preencher (ver comparator.component.html).
  missingSelectionMessage(): string | null {
    if (this.loading()) return null;
    if (!this.teamAId && !this.teamBId) return 'Selecione as duas equipes para comparar.';
    if (!this.teamAId) return 'Selecione a Equipe A para comparar.';
    if (!this.teamBId) return 'Selecione a Equipe B para comparar.';
    return null;
  }
}
