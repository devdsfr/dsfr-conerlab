import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
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
import { desserializarEstado, serializarEstado } from './simulator-url-state';
import { AdSlotComponent } from '../../shared/ad-slot.component';
import { PageLoaderComponent } from '../../shared/page-loader.component';

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
    PageLoaderComponent,
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
  // AUD-004: opponentTier removido. O backend recusa opponent_tier enquanto não
  // houver classificação por temporada — mandá-lo faria o backtest falhar.
  maxOdds?: number;
  stake = 10;

  // Métrica: 'corners' (com odds históricas) ou as demais (odd fixa simulada). Cada
  // uma tem seu threshold (linha over/under: 2 = "acima de 2.5"; chutes usam faixas
  // inteiras maiores).
  metric:
    | 'corners'
    | 'goals'
    | 'offsides'
    | 'shots'
    | 'shots_on_target'
    // Mercados de resultado: não têm linha, o desfecho da partida já é a resposta.
    | 'win'
    | 'draw'
    | 'win_or_draw' = 'corners';
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
  // Escanteios é a ÚNICA métrica com odd por partida no banco, então é a única
  // que aceita o filtro "odds máximas". As demais não têm odd nenhuma.
  get usesRealOdds(): boolean {
    return this.metric === 'corners';
  }
  // REV-P3 / correção 2: a odd fixa agora vale para TODAS as métricas, escanteios
  // inclusive. Antes era `metric !== 'corners'`, e por isso escanteios sem odd real
  // no período devolvia zero ocorrências a qualquer limiar mesmo com odd informada:
  // o campo nem era enviado. Em escanteios ela é o fallback de quem não tem odd
  // real; nas outras é a única odd possível. Em ambos os casos o backend marca
  // odds_source = "fixed" — NUNCA "real".
  get usesFixedOdd(): boolean {
    return true;
  }
  get isNullableMetric(): boolean {
    return this.metric === 'offsides' || this.metric === 'shots' || this.metric === 'shots_on_target';
  }
  // Mercados de resultado (vitória / empate / não perde): sem limiar, e sem odd
  // coletada — o backend só aceita odd fixa informada aqui.
  get isResultMetric(): boolean {
    return this.metric === 'win' || this.metric === 'draw' || this.metric === 'win_or_draw';
  }
  private label(): string {
    return {
      corners: 'Escanteios',
      goals: 'Gols',
      offsides: 'Impedimentos',
      shots: 'Chutes',
      shots_on_target: 'Chutes no gol',
      win: 'Vitória',
      draw: 'Empate',
      win_or_draw: 'Não perde',
    }[this.metric];
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

  // ---- REV-P3 / correção 4: o "EV" foi REMOVIDO ------------------------------
  // Existia aqui um bloco de "análise de valor" que calculava
  //   ROI esperado por aposta = taxa de acerto do próprio lote × odd − 1
  // e derivava dele uma odd de equilíbrio (1 ÷ taxa de acerto), uma margem de
  // segurança ("pode errar até N em 10") e uma tabela de cenários de odd.
  //
  // Isso estava errado por construção: usava a taxa de acerto OBSERVADA no mesmo
  // lote filtrado como se fosse a PROBABILIDADE do evento futuro. É a definição de
  // transformar estatística histórica em previsão — e, pior, a taxa vinha de uma
  // amostra escolhida justamente por ter acertado muito, então o "ROI esperado"
  // era positivo quase sempre, por viés de seleção, não por vantagem.
  //
  // Não existe substituto honesto calculável só com o histórico do próprio lote.
  // Por isso o bloco foi removido em vez de reescrito. NÃO reintroduzir.
  //
  // O que o Simulador mostra agora é apenas desempenho OBSERVADO (taxa de acerto
  // e, quando há série financeira completa, lucro/ROI/yield daquele histórico).

  readonly drawdownTooltip =
    'Drawdown máximo: a maior sequência de perdas acumuladas (em unidades de stake) observada durante o backtest — indica o pior momento de "prejuízo" pelo qual a estratégia passou.';
  readonly consistencyTooltip =
    'Consistência (0 a 1): quanto mais perto de 1, menos os escanteios variam de jogo para jogo.';

  constructor(
    private api: ApiService,
    public auth: AuthService,
    private router: Router,
    private route: ActivatedRoute,
  ) {}

  // ---- REV-P3 / correção 6: estado do Simulador na URL -----------------------
  //
  // Sem isso o Simulador não era reproduzível: o resultado dependia inteiramente
  // do estado em memória do componente, e não havia como alguém (nem o próprio
  // usuário no dia seguinte) chegar ao MESMO backtest. Agora cada execução
  // reescreve a query string, e abrir a URL reconstrói os mesmos critérios.
  //
  // Regra dura: parâmetro inválido NÃO é substituído em silêncio. Se a URL pedir
  // algo que não dá para honrar, a tela diz o que ignorou — caso contrário o
  // usuário veria um backtest diferente do que a URL prometia, achando que é o
  // mesmo. É exatamente o modo de falha que a reprodutibilidade deveria impedir.
  urlStateError = signal<string | null>(null);

  // Serializa os critérios atuais na query string, sem empilhar histórico: cada
  // execução SUBSTITUI a anterior (replaceUrl), então o botão "voltar" continua
  // saindo da tela em vez de percorrer N backtests. A montagem em si mora em
  // simulator-url-state.ts, testável sem Angular.
  private writeUrlState(): void {
    this.router.navigate([], {
      relativeTo: this.route,
      queryParams: serializarEstado({
        leagueId: this.selectedLeagueId,
        seasonIds: this.selectedSeasonIds,
        teamId: this.selectedTeamId,
        lastNGames: this.lastNGames,
        homeAway: this.homeAway,
        metric: this.metric,
        threshold: this.currentThreshold(),
        maxOdds: this.maxOdds,
        fixedOdd: this.fixedOdd,
        stake: this.stake,
      }),
      replaceUrl: true,
    });
  }

  // Limiar da métrica corrente — um único número na URL em vez de cinco campos.
  private currentThreshold(): number {
    if (this.isGoals) return this.goalsThreshold;
    if (this.isOffsides) return this.offsidesThreshold;
    if (this.isShots) return this.shotsThreshold;
    if (this.isShotsOnTarget) return this.shotsOnTargetThreshold;
    return this.cornersThreshold;
  }

  // Ponte fina entre o ActivatedRoute e a função pura de desserialização.
  private readUrlState(problemas: string[]): FilterRunRequest | null {
    const map = this.route.snapshot.queryParamMap;
    const p: Record<string, string | null> = {};
    for (const k of map.keys) p[k] = map.get(k);
    return desserializarEstado(p, problemas, {
      threshold: this.cornersThreshold,
      stake: this.stake,
    });
  }

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
      max_odds: this.usesRealOdds ? (this.maxOdds || undefined) : undefined,
      stake: this.stake || undefined,
      metric: this.metric,
      goals_threshold: this.isGoals ? this.goalsThreshold : undefined,
      offsides_threshold: this.isOffsides ? this.offsidesThreshold : undefined,
      shots_threshold: this.isShots ? this.shotsThreshold : undefined,
      shots_on_target_threshold: this.isShotsOnTarget ? this.shotsOnTargetThreshold : undefined,
      fixed_odd: this.fixedOdd || undefined,
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
      // REV-P3 / correção 5: a procedência viaja junto. Salvar aqui NÃO valida nem
      // aprova a estratégia — ela não passou por holdout nem por correção de
      // múltiplas comparações, e a lista de Estratégias precisa poder dizer isso.
      origin: 'simulator',
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
    const fromState = (history.state?.definition ?? null) as FilterRunRequest | null;

    // REV-P3 / correção 6: precedência explícita. router state (clique em
    // "Conferir no Simulador") > query string > padrões. A URL só entra quando
    // não veio definition pelo state, para o clique não ser sobrescrito por uma
    // query string antiga que ainda esteja na barra de endereço.
    const problemas: string[] = [];
    const fromUrl = fromState?.league_id ? null : this.readUrlState(problemas);
    if (problemas.length) {
      this.urlStateError.set(
        'A URL não pôde ser reproduzida integralmente: ' + problemas.join('; ') +
        '. Confira os critérios abaixo antes de ler o resultado.',
      );
    }
    const incoming = fromState?.league_id ? fromState : fromUrl;

    this.api.listLeagues().subscribe(leagues => {
      this.leagues.set(leagues);
      if (!leagues.length) return;

      if (incoming?.league_id) {
        if (!leagues.some(l => l.id === incoming.league_id)) {
          // Campeonato que não existe (ou não é visível para este usuário). NÃO
          // trocamos por outro em silêncio: o resultado não seria o que a URL pediu.
          this.urlStateError.set(
            `O campeonato pedido (id ${incoming.league_id}) não existe ou não está disponível. ` +
            'Nada foi substituído automaticamente — escolha um campeonato abaixo.',
          );
          this.selectedLeagueId = undefined;
          return;
        }
        this.selectedLeagueId = incoming.league_id;
        this.applyDefinition(incoming, !!fromState?.league_id);
        return;
      }
      this.selectedLeagueId = leagues[0].id;
      this.onLeagueChange();
    });
  }

  // Aplica uma definition salva/descoberta no formulário. Diferente de
  // onLeagueChange(), NUNCA limpa a seleção — as listas dependentes (temporadas,
  // equipes) são carregadas em volta dos valores que acabaram de chegar.
  private applyDefinition(d: FilterRunRequest, vindaDeDescoberta = true): void {
    this.metric = (d.metric as typeof this.metric) || 'corners';
    this.cornersThreshold = d.corners_threshold ?? this.cornersThreshold;
    this.goalsThreshold = d.goals_threshold ?? this.goalsThreshold;
    this.offsidesThreshold = d.offsides_threshold ?? this.offsidesThreshold;
    this.shotsThreshold = d.shots_threshold ?? this.shotsThreshold;
    this.shotsOnTargetThreshold = d.shots_on_target_threshold ?? this.shotsOnTargetThreshold;
    this.fixedOdd = d.fixed_odd ?? undefined;

    this.homeAway = d.home_away ?? '';
    this.lastNGames = d.last_n_games ?? 0;
    // AUD-004: opponent_tier de uma definição antiga é deliberadamente ignorado
    // ao recarregar — reenviá-lo faria o backtest ser recusado.
    this.maxOdds = d.max_odds ?? undefined;
    this.stake = d.stake ?? this.stake;
    this.selectedTeamId = d.team_id ?? undefined;
    // O banner "veio de uma descoberta" só vale quando veio mesmo. Um estado
    // restaurado da URL é do próprio usuário — dizer o contrário seria mentir
    // sobre a procedência do recorte.
    this.loadedFromDiscovery.set(vindaDeDescoberta);

    // includeAll=true: uma descoberta pode agregar várias temporadas (ex.: 2024+2025+
    // 2026). Se usássemos a lista já filtrada pra "temporadas recentes" aqui, o
    // intersect abaixo descartaria as temporadas antigas em silêncio, sobrando só a
    // atual — que sozinha pode não ter jogo nenhum batendo com o padrão, dando
    // "0 partidas encontradas" mesmo pra uma descoberta com 100+ ocorrências.
    this.api.listSeasons(this.selectedLeagueId!, true).subscribe(s => {
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
    // REV-P3 / correção 6: a URL passa a descrever o backtest que está na tela.
    this.writeUrlState();
    this.api.runFilter({
      league_id: this.selectedLeagueId,
      season_ids: this.selectedSeasonIds,
      team_id: this.selectedTeamId ?? null,
      last_n_games: this.lastNGames || undefined,
      home_away: this.homeAway || undefined,
      // corners_threshold sempre vai (ignorado no backend quando metric=goals);
      // para gols enviamos metric/goals_threshold/fixed_odd.
      corners_threshold: this.cornersThreshold,
      max_odds: this.usesRealOdds ? (this.maxOdds || undefined) : undefined,
      stake: this.stake || undefined,
      metric: this.metric,
      goals_threshold: this.isGoals ? this.goalsThreshold : undefined,
      offsides_threshold: this.isOffsides ? this.offsidesThreshold : undefined,
      shots_threshold: this.isShots ? this.shotsThreshold : undefined,
      shots_on_target_threshold: this.isShotsOnTarget ? this.shotsOnTargetThreshold : undefined,
      fixed_odd: this.fixedOdd || undefined,
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
