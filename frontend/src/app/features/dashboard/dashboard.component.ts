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
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { DashboardResult, League, Season, Team } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';

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
  // Loading dedicado para os combos "Campeonato"/"Equipe" — sem isso, o combo fica
  // com a aparência de travado (vazio, sem feedback) durante o fetch inicial.
  leaguesLoading = signal(true);
  teamsLoading = signal(false);
  error = signal<string | null>(null);
  result = signal<DashboardResult | null>(null);
  trendChart = signal<ChartData>({ labels: [], datasets: [] });

  matchColumns = ['match_date', 'opponent', 'is_home', 'corners_for', 'corners_against', 'total_corners'];

  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo. Valores baixos indicam resultados mais imprevisíveis.';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.leaguesLoading.set(true);
    this.api.listLeagues().subscribe({
      next: leagues => {
        this.leagues.set(leagues);
        this.leaguesLoading.set(false);
        if (leagues.length) {
          this.selectedLeagueId = leagues[0].id;
          this.onLeagueChange();
        }
      },
      error: err => {
        this.leaguesLoading.set(false);
        this.error.set(err?.error?.error ?? 'Erro ao carregar campeonatos');
      },
    });
  }

  onLeagueChange(): void {
    if (!this.selectedLeagueId) return;
    this.selectedSeasonId = undefined;
    this.teamsLoading.set(true);
    this.api.listSeasons(this.selectedLeagueId).subscribe(seasons => {
      this.seasons.set(seasons);
      // Evita o campo "Temporada" ficar vazio (tela morta ao clicar em
      // Analisar sem nenhuma seleção visível): assume a mais recente por
      // padrão. "Todas" continua disponível como opção explícita.
      if (seasons.length) {
        this.selectedSeasonId = seasons.reduce((a, b) => (a.year > b.year ? a : b)).id;
      }
    });
    this.api.listTeams(this.selectedLeagueId).subscribe({
      next: teams => {
        this.teams.set(teams);
        // Sempre reposiciona para a primeira equipe do campeonato selecionado —
        // manter o id da equipe do campeonato anterior selecionado fazia a
        // análise rodar com uma equipe de outra liga (amostra de 0 jogos).
        this.selectedTeamId = teams.length ? teams[0].id : undefined;
        this.result.set(null);
        this.teamsLoading.set(false);
      },
      error: () => this.teamsLoading.set(false),
    });
  }

  runDashboard(): void {
    if (!this.selectedTeamId) return;
    this.loading.set(true);
    this.error.set(null);
    this.api.getDashboard(this.selectedTeamId, this.selectedLeagueId, this.selectedSeasonId, this.limit).subscribe({
      next: res => {
        this.result.set(res);
        this.trendChart.set({
          labels: res.trend.map((_, i) => i + 1),
          datasets: [{ label: 'Escanteios', data: res.trend }],
        });
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar dashboard');
        this.loading.set(false);
      },
    });
  }

  selectedLeagueName(): string {
    return this.leagues().find(l => l.id === this.selectedLeagueId)?.name ?? '';
  }

  selectedTeamName(): string {
    return this.teams().find(t => t.id === this.selectedTeamId)?.name ?? '';
  }
}
