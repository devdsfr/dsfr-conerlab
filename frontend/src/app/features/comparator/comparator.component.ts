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
import { ComparatorMetric, ComparatorPerspective, ComparatorVenue, ComparisonResult, League, MatchPoint, Season, Team } from '../../core/models';
import { resolverEquipe, resolverTemporada } from '../dashboard/season-resolution';
import { SimpleChartComponent } from '../../shared/simple-chart.component';
import { AdSlotComponent } from '../../shared/ad-slot.component';
import { PageLoaderComponent } from '../../shared/page-loader.component';

/** Mesmas janelas do Dashboard. */
const VALID_LIMITS = [5, 10, 15, 20];

/** Rótulos legíveis dos eixos — usados nos títulos de gráfico e nas mensagens. */
const METRIC_LABEL: Record<ComparatorMetric, string> = {
  corners: 'Escanteios',
  goals: 'Gols',
  shots: 'Finalizações',
  shots_on_target: 'Finalizações no alvo',
  offsides: 'Impedimentos',
};
const PERSPECTIVE_LABEL: Record<ComparatorPerspective, string> = {
  produzido: 'produzidos pela equipe',
  concedido: 'concedidos ao adversário',
  total: 'total da partida',
};
const VENUE_LABEL: Record<ComparatorVenue, string> = {
  geral: 'todos os jogos',
  casa: 'somente como mandante',
  fora: 'somente como visitante',
};

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

  // Os três eixos do REV-P2. Padrões iguais ao comportamento anterior do
  // Comparador (geral / escanteios / total da partida), para quem já usava a
  // tela não ver os números mudarem sem ter pedido.
  venue: ComparatorVenue = 'geral';
  metric: ComparatorMetric = 'corners';
  perspective: ComparatorPerspective = 'total';

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

    // Eixos restaurados da URL. Valor desconhecido cai no padrão — o backend
    // aplica a mesma regra, então tela e resposta nunca divergem.
    const v = qp.get('venue') as ComparatorVenue | null;
    if (v && v in VENUE_LABEL) this.venue = v;
    const m = qp.get('metric') as ComparatorMetric | null;
    if (m && m in METRIC_LABEL) this.metric = m;
    const pe = qp.get('perspective') as ComparatorPerspective | null;
    if (pe && pe in PERSPECTIVE_LABEL) this.perspective = pe;

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
        venue: this.venue,
        metric: this.metric,
        perspective: this.perspective,
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
    this.api
      .compare({
        teamA: this.teamAId,
        teamB: this.teamBId,
        leagueId: this.selectedLeagueId,
        seasonId: this.selectedSeasonId,
        limit: this.limit,
        venue: this.venue,
        metric: this.metric,
        perspective: this.perspective,
      })
      .subscribe({
      next: res => {
        this.result.set(res);

        // Evolução com IDENTIDADE: cada ponto é uma partida real, rotulada pela
        // data. Antes o eixo X era 1..N e não havia como saber contra quem.
        // Partida sem a métrica publicada é DESCARTADA do gráfico em vez de
        // virar zero — o ponto some, a mentira não entra.
        const serie = (lado: typeof res.team_a): ChartData => {
          const pontos = lado.evolution.filter(p => p.value !== null);
          return {
            labels: pontos.map(p => this.rotuloPonto(p)),
            datasets: [{ label: lado.team.short_name, data: pontos.map(p => p.value as number) }],
          };
        };
        this.chartA.set(serie(res.team_a));
        this.chartB.set(serie(res.team_b));

        // Barras: a MESMA métrica e a MESMA perspectiva para as duas equipes.
        // Nada de grandezas diferentes no mesmo eixo — foi o que o gráfico
        // anterior fazia ao pôr produzido/concedido ao lado de médias do total.
        // Lado sem a métrica não entra: não há barra de altura zero para
        // representar ausência.
        const barras: { label: string; data: number[] }[] = [];
        for (const lado of [res.team_a, res.team_b]) {
          if (lado.metric_available) barras.push({ label: lado.team.short_name, data: [lado.summary.mean] });
        }
        this.barChart.set({ labels: [this.tituloMetrica()], datasets: barras });

        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao comparar equipes');
        this.loading.set(false);
      },
    });
  }

  /** Trocar métrica, perspectiva ou local recarrega na hora — o dado já está
   * persistido, não há motivo para exigir um segundo clique em "Comparar". */
  onAxisChange(): void {
    this.syncQueryParams();
    if (this.teamAId !== undefined && this.teamBId !== undefined) this.runCompare();
  }

  /** Rótulo do ponto no eixo X: data curta da partida real. */
  rotuloPonto(p: MatchPoint): string {
    const d = new Date(p.date);
    return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}`;
  }

  /** Descrição completa de um ponto — é o conteúdo do tooltip exigido pelo
   * REV-P2: data, adversário, mando, valor e placar. */
  descricaoPonto(p: MatchPoint): string {
    const d = new Date(p.date).toLocaleDateString('pt-BR');
    const mando = p.is_home ? 'Casa' : 'Fora';
    const valor = p.value === null ? 'sem dado' : String(p.value);
    return `${d} · ${mando} · vs ${p.opponent_name} · ${this.tituloMetrica()}: ${valor} · placar ${p.goals_for}-${p.goals_against}`;
  }

  tituloMetrica(): string {
    return `${METRIC_LABEL[this.metric]} (${PERSPECTIVE_LABEL[this.perspective]})`;
  }

  descricaoRecorte(): string {
    return `${METRIC_LABEL[this.metric]} · ${PERSPECTIVE_LABEL[this.perspective]} · ${VENUE_LABEL[this.venue]}`;
  }

  metricLabel(): string {
    return METRIC_LABEL[this.metric];
  }

  /** Linha "13/20 — 65%": numerador, denominador e percentual, nunca só o
   * percentual. */
  faixaTexto(b: { hits: number; sample: number; percentage: number }): string {
    return `${b.hits}/${b.sample} — ${b.percentage}%`;
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
