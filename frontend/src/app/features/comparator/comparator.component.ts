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

import { ActivatedRoute, Router } from '@angular/router';

import { ApiService } from '../../core/api.service';
import { ComparisonResult, League, Season, Team } from '../../core/models';
import { resolverEquipe, resolverTemporada } from '../dashboard/season-resolution';

/** Mesmas janelas do Dashboard. */
const VALID_LIMITS = [5, 10, 15, 20];
import { SimpleChartComponent } from '../../shared/simple-chart.component';
import { AdSlotComponent } from '../../shared/ad-slot.component';
import { PageLoaderComponent } from '../../shared/page-loader.component';

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
    PageLoaderComponent,
  ],
  templateUrl: './comparator.component.html',
})
export class ComparatorComponent implements OnInit {
  leagues = signal<League[]>([]);
  seasons = signal<Season[]>([]);
  teams = signal<Team[]>([]);

  selectedLeagueId?: number;
  selectedSeasonId?: number;
  teamAId?: number;
  teamBId?: number;
  limit = 10;

  /** IDs vindos da URL que não existem no recorte resolvido. Guardados para a
   * tela dizer isso — a alternativa anterior era cair em teams[0]/teams[1]. */
  temporadaPedidaAusente = signal<number | null>(null);
  equipesPedidasAusentes = signal<number[]>([]);

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

  constructor(private api: ApiService, private router: Router, private route: ActivatedRoute) {}

  ngOnInit(): void {
    // Estado na URL: permite recarregar sem perder a análise, compartilhar a
    // comparação e reproduzir um caso relatado. Antes o componente não injetava
    // ActivatedRoute — qualquer refresh zerava tudo.
    const qp = this.route.snapshot.queryParamMap;
    const num = (k: string) => (qp.get(k) ? Number(qp.get(k)) : undefined);
    const qpLeague = num('league_id');
    const qpSeason = num('season_id');
    const qpA = num('team_a');
    const qpB = num('team_b');
    const qpLimit = num('limit');
    if (qpLimit && VALID_LIMITS.includes(qpLimit)) this.limit = qpLimit;

    this.api.listLeagues().subscribe(leagues => {
      this.leagues.set(leagues);
      if (!leagues.length) return;
      this.selectedLeagueId = qpLeague && leagues.some(l => l.id === qpLeague) ? qpLeague : leagues[0].id;
      this.onLeagueChange(qpSeason, qpA, qpB);
    });
  }

  onLeagueChange(presetSeasonId?: number, presetA?: number, presetB?: number): void {
    if (!this.selectedLeagueId) return;
    this.selectedSeasonId = undefined;
    this.temporadaPedidaAusente.set(null);
    this.equipesPedidasAusentes.set([]);
    this.result.set(null);

    // includeAll quando há pedido explícito — mesma razão do Dashboard
    // (REV-P1): listSeasons() esconde temporadas anteriores ao ano corrente, e
    // procurar a temporada pedida numa lista truncada faz o pedido ser
    // silenciosamente descartado.
    this.api.listSeasons(this.selectedLeagueId, presetSeasonId !== undefined).subscribe(seasons => {
      this.seasons.set(seasons);
      const r = resolverTemporada(seasons, presetSeasonId);
      this.selectedSeasonId = r.seasonId;

      if (r.pedidaAusente) {
        this.temporadaPedidaAusente.set(presetSeasonId ?? null);
        this.teams.set([]);
        this.teamAId = undefined;
        this.teamBId = undefined;
        return;
      }

      this.loadTeams(presetA, presetB);
    });
  }

  onSeasonChange(): void {
    this.loadTeams(this.teamAId, this.teamBId);
  }

  /** Carrega as equipes da liga+temporada e resolve A e B.
   *
   * O que NÃO se faz mais aqui: `teamA = teams[0]; teamB = teams[1]`. Isso
   * respondia qualquer pedido inválido com duas equipes arbitrárias, e o usuário
   * comparava times que nunca escolheu. */
  private loadTeams(preferredA?: number, preferredB?: number): void {
    this.api.listTeams(this.selectedLeagueId, undefined, this.selectedSeasonId).subscribe(teams => {
      this.teams.set(teams);

      const ra = resolverEquipe(teams, preferredA);
      const rb = resolverEquipe(teams, preferredB);
      const ausentes: number[] = [];
      if (ra.pedidaAusente && preferredA !== undefined) ausentes.push(preferredA);
      if (rb.pedidaAusente && preferredB !== undefined) ausentes.push(preferredB);
      this.equipesPedidasAusentes.set(ausentes);

      // Sem pedido explícito, A recebe a primeira da lista como ponto de partida
      // e B fica VAZIO de propósito: escolher a segunda equipe por conta própria
      // seria inventar metade da comparação.
      this.teamAId = preferredA !== undefined ? ra.teamId : teams.length ? teams[0].id : undefined;
      this.teamBId = preferredB !== undefined ? rb.teamId : undefined;

      this.syncQueryParams();
      if (this.teamAId !== undefined && this.teamBId !== undefined) this.runCompare();
    });
  }

  syncQueryParams(): void {
    this.router.navigate([], {
      relativeTo: this.route,
      queryParams: {
        league_id: this.selectedLeagueId ?? null,
        season_id: this.selectedSeasonId ?? null,
        team_a: this.teamAId ?? null,
        team_b: this.teamBId ?? null,
        limit: this.limit,
      },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  runCompare(): void {
    if (!this.teamAId || !this.teamBId) return;
    this.loading.set(true);
    this.error.set(null);
    this.syncQueryParams();
    this.api.compare(this.teamAId, this.teamBId, this.selectedLeagueId, this.limit, this.selectedSeasonId).subscribe({
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
        // Casa/Fora SAÍRAM deste gráfico, por dois motivos:
        //
        //  1. `home?.mean ?? 0` transformava "não jogou em casa nesta janela" em
        //     barra de altura zero — ausência desenhada como observação. O texto
        //     ao lado já usava '—' para o mesmo dado, então gráfico e tabela se
        //     contradiziam.
        //  2. "A favor"/"Sofridos" são escanteios DA EQUIPE; "Casa"/"Fora" eram
        //     médias do TOTAL da partida. Grandezas diferentes no mesmo eixo,
        //     sem nada avisando.
        //
        // Sobram três barras da mesma família (escanteios por partida, sob três
        // perspectivas), que é o que o eixo comporta honestamente.
        this.barChart.set({
          labels: ['Total da partida', 'A favor', 'Sofridos'],
          datasets: [
            {
              label: res.team_a.team.short_name,
              data: [
                res.team_a.total_corners.mean,
                res.team_a.corners_for.mean,
                res.team_a.corners_against.mean,
              ],
            },
            {
              label: res.team_b.team.short_name,
              data: [
                res.team_b.total_corners.mean,
                res.team_b.corners_for.mean,
                res.team_b.corners_against.mean,
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
