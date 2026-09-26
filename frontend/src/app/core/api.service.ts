import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import {
  League,
  Season,
  Team,
  DashboardResult,
  ComparisonResult,
  ComparatorVenue,
  ComparatorMetric,
  ComparatorPerspective,
  FilterRunRequest,
  BacktestResult,
  UsageSummaryResponse,
  TestConnectionResult,
  UsageEntry,
  BankrollPhase,
  BankrollCriteria,
  BankrollStatus,
  BankrollHistoryEntry,
  BankrollRound,
  SyncRunResult,
  SyncStartResponse,
  SyncProgress,
  SyncStatusResponse,
  BillingStatus,
  UpcomingMatch,
  Strategy,
  StrategyBundle,
  StrategyEvaluation,
  DiscoveredStrategiesResponse,
  DiscoveryRunResult,
  DiscoveryProgress,
  DiscoveryLastRunResponse,
} from './models';

// URL base da API. Em produção (docker-compose) o frontend é servido pelo nginx, que
// encaminha /api para o serviço backend — nesse caso usamos um caminho relativo. Em
// desenvolvimento local (`ng serve`, porta 4200) apontamos direto para o backend Go
// rodando em :8080.
function resolveApiBaseUrl(): string {
  const override = (globalThis as any)['__CORNERLAB_API_URL__'];
  if (override) return override;
  if (typeof window !== 'undefined' && window.location.port === '4200') {
    return 'http://localhost:8080/api/v1';
  }
  return '/api/v1';
}

export const API_BASE_URL = resolveApiBaseUrl();

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly base = API_BASE_URL;

  constructor(private http: HttpClient) {}

  /** Calendário da Visão Geral — próximos jogos mapeados (ligas com dado real). */
  getUpcomingMatches(): Observable<{ matches: UpcomingMatch[] }> {
    return this.http.get<{ matches: UpcomingMatch[] }>(`${this.base}/overview/upcoming`);
  }

  // Catálogo
  listLeagues(): Observable<League[]> {
    return this.http.get<League[]>(`${this.base}/leagues`);
  }

  /**
   * includeAll=true pula o filtro de "temporadas recentes" abaixo — necessário quando
   * vamos RESTAURAR uma seleção existente de temporadas (ex.: estratégia descoberta
   * que agrega várias temporadas), porque intersectar season_ids reais contra a lista
   * já filtrada descartaria silenciosamente as temporadas antigas que faziam parte do
   * cálculo original. Ver FiltersComponent.applyDefinition.
   */
  listSeasons(leagueId: number, includeAll = false): Observable<Season[]> {
    // Mostra só temporadas do ano corrente pra frente — temporadas antigas (ex: 2024)
    // poluíam os seletores e traziam times já rebaixados. É filtro de interface: o
    // dado continua no banco. Fallback: se nada sobrar (liga só com histórico antigo),
    // devolve tudo pra não deixar o seletor vazio.
    const currentYear = new Date().getFullYear();
    return this.http.get<Season[]>(`${this.base}/leagues/${leagueId}/seasons`).pipe(
      map(seasons => {
        if (includeAll) return seasons;
        const recent = seasons.filter(s => s.year >= currentYear);
        return recent.length ? recent : seasons;
      }),
    );
  }

  /** seasonId restringe a equipes que de fato jogaram naquela liga+temporada — evita
   * listar equipes de temporadas passadas (ex: rebaixadas) como se ainda
   * estivessem na liga atual. Omitir seasonId mantém o comportamento "todas as
   * temporadas" (vínculo histórico da liga). */
  listTeams(leagueId?: number, query?: string, seasonId?: number | number[]): Observable<Team[]> {
    let url = `${this.base}/teams?`;
    if (leagueId) url += `league_id=${leagueId}&`;
    // season_id aceita um valor (Dashboard) ou vários (Simulador multi-temporada);
    // repetido na query string vira season_id=1&season_id=2 no backend.
    const seasons = seasonId == null ? [] : Array.isArray(seasonId) ? seasonId : [seasonId];
    for (const s of seasons) url += `season_id=${s}&`;
    if (query) url += `q=${encodeURIComponent(query)}&`;
    return this.http.get<Team[]>(url);
  }

  // Módulo 1
  getDashboard(teamId: number, leagueId?: number, seasonId?: number, limit = 10): Observable<DashboardResult> {
    let url = `${this.base}/dashboard?team_id=${teamId}&limit=${limit}`;
    if (leagueId) url += `&league_id=${leagueId}`;
    if (seasonId) url += `&season_id=${seasonId}`;
    return this.http.get<DashboardResult>(url);
  }

  // Módulo 2
  compare(opts: {
    teamA: number;
    teamB: number;
    leagueId?: number;
    seasonId?: number;
    limit?: number;
    venue?: ComparatorVenue;
    metric?: ComparatorMetric;
    perspective?: ComparatorPerspective;
  }): Observable<ComparisonResult> {
    const p = new URLSearchParams({
      team_a: String(opts.teamA),
      team_b: String(opts.teamB),
      limit: String(opts.limit ?? 10),
      venue: opts.venue ?? 'geral',
      metric: opts.metric ?? 'corners',
      perspective: opts.perspective ?? 'total',
    });
    if (opts.leagueId) p.set('league_id', String(opts.leagueId));
    // Sem season_id a amostra atravessa temporadas: em 23/09/2026 o Celta Vigo
    // devolvia 20 jogos que eram 7 de 2026 + 13 de 2025, com média que não
    // correspondia a nenhuma das duas.
    if (opts.seasonId) p.set('season_id', String(opts.seasonId));
    return this.http.get<ComparisonResult>(`${this.base}/comparator?${p.toString()}`);
  }

  // Módulo 3
  runFilter(req: FilterRunRequest): Observable<BacktestResult> {
    return this.http.post<BacktestResult>(`${this.base}/filters/run`, req);
  }

  // Strategy Engine (Remodelagem F5) — CRUD + execução de estratégias.
  listStrategies(): Observable<Strategy[]> {
    return this.http.get<Strategy[]>(`${this.base}/strategies`);
  }

  // REV-P3 / correção 5: `origin` registra de onde a estratégia veio. O Simulador
  // manda 'simulator' — um recorte montado à mão, sem holdout e sem correção de
  // múltiplas comparações. O backend só aceita 'simulator'; 'discovery' é gravado
  // exclusivamente pelo motor, para que salvar não pareça validar.
  createStrategy(payload: { name: string; description?: string; definition: string; favorite?: boolean; origin?: 'simulator' }): Observable<Strategy> {
    return this.http.post<Strategy>(`${this.base}/strategies`, payload);
  }

  getStrategy(id: number): Observable<StrategyBundle> {
    return this.http.get<StrategyBundle>(`${this.base}/strategies/${id}`);
  }

  runStrategy(id: number): Observable<StrategyEvaluation> {
    return this.http.post<StrategyEvaluation>(`${this.base}/strategies/${id}/run`, {});
  }

  updateStrategyFlags(id: number, flags: { active?: boolean; favorite?: boolean }): Observable<Strategy> {
    return this.http.patch<Strategy>(`${this.base}/strategies/${id}`, flags);
  }

  deleteStrategy(id: number): Observable<void> {
    return this.http.delete<void>(`${this.base}/strategies/${id}`);
  }

  // Strategy Discovery Engine (Remodelagem F6, doc 08)

  /**
   * Ranking de estratégias descobertas automaticamente. Leitura pública: o
   * worker noturno já deixou tudo calculado no banco.
   */
  listDiscoveredStrategies(leagueId?: number, limit = 50): Observable<DiscoveredStrategiesResponse> {
    const params: Record<string, string> = { limit: String(limit) };
    if (leagueId) params['league_id'] = String(leagueId);
    return this.http.get<DiscoveredStrategiesResponse>(`${this.base}/discovery/strategies`, { params });
  }

  /**
   * Dispara uma varredura agora. Exige login pelo custo do ciclo (centenas de
   * backtests) — o caminho normal é esperar o worker diário.
   * Sem leagueId, varre todas as ligas.
   */
  runDiscovery(leagueId?: number): Observable<{ started: boolean; message: string; progress: DiscoveryProgress }> {
    return this.http.post<{ started: boolean; message: string; progress: DiscoveryProgress }>(
      `${this.base}/discovery/run`,
      leagueId ? { league_id: leagueId } : {},
    );
  }

  /** Andamento da varredura — alimenta a barra de progresso da tela Descobertas. */
  getDiscoveryProgress(): Observable<DiscoveryProgress> {
    return this.http.get<DiscoveryProgress>(`${this.base}/discovery/progress`);
  }

  // REV-P4 (item 27): último ciclo concluído (cron ou manual), persistido.
  // Requer login; executar a varredura continua restrito a admin.
  getDiscoveryLastRun(): Observable<DiscoveryLastRunResponse> {
    return this.http.get<DiscoveryLastRunResponse>(`${this.base}/discovery/last-run`);
  }

  /**
   * Documento de contexto do CornerLab em Markdown, para alimentar outra IA.
   * Gerado pelo backend a partir das constantes reais do motor e da lista viva
   * de rotas — por isso vem da API e não de um arquivo estático em /assets.
   *
   * A rota EXIGE LOGIN (antes era pública) e não tem mais botão na interface:
   * ela descreve o método do produto — pesos do DSFR, critérios de aprovação do
   * Discovery, espaço de busca. Mantida aqui porque continua sendo útil ao dono
   * do produto.
   */
  downloadAIContext(): Observable<Blob> {
    return this.http.get(`${this.base}/docs/contexto.md`, { responseType: 'blob' });
  }

  /**
   * Exporta os dados do próprio usuário: filtros salvos, apostas, gestão de
   * banca, alertas e histórico de estratégias. Exige login; não exige premium.
   */
  downloadMyData(): Observable<Blob> {
    return this.http.get(`${this.base}/exports/meus-dados`, { responseType: 'blob' });
  }

  // Painel "Integrações" — status/consumo das APIs externas
  getUsageSummary(): Observable<UsageSummaryResponse> {
    return this.http.get<UsageSummaryResponse>(`${this.base}/diagnostics/usage`);
  }

  testConnection(provider: string): Observable<TestConnectionResult> {
    return this.http.post<TestConnectionResult>(`${this.base}/diagnostics/test/${provider}`, {});
  }

  /** Botão "Sincronizar agora" — exige login (ver router.go, grupo authGroup).
   *  Responde 202 na hora: o ciclo roda em segundo plano e o andamento é lido por
   *  getSyncProgress(). Responde 409 se já houver um ciclo em execução. */
  syncRun(): Observable<SyncStartResponse> {
    return this.http.post<SyncStartResponse>(`${this.base}/sync/run`, {});
  }

  /** Andamento do ciclo em execução — alimenta a barra de progresso. Leitura
   *  pública: continua funcionando mesmo se o token expirar durante o ciclo. */
  getSyncProgress(): Observable<SyncProgress> {
    return this.http.get<SyncProgress>(`${this.base}/sync/progress`);
  }

  /** Última sincronização registrada (manual ou via Cron Job) — leitura pública. */
  getSyncStatus(): Observable<SyncStatusResponse> {
    return this.http.get<SyncStatusResponse>(`${this.base}/sync/status`);
  }

  getRecentUsage(provider?: string, limit = 30): Observable<{ entries: UsageEntry[] }> {
    let url = `${this.base}/diagnostics/recent?limit=${limit}`;
    if (provider) url += `&provider=${provider}`;
    return this.http.get<{ entries: UsageEntry[] }>(url);
  }

  // Módulo de Gestão Evolutiva de Banca (requer usuário autenticado — ver AuthService)
  getBankrollStatus(): Observable<BankrollStatus> {
    return this.http.get<BankrollStatus>(`${this.base}/bankroll/status`);
  }

  getBankrollPhases(): Observable<{ phases: BankrollPhase[] }> {
    return this.http.get<{ phases: BankrollPhase[] }>(`${this.base}/bankroll/phases`);
  }

  setBankrollPhases(phases: { sequence: number; name: string; amount: number }[]): Observable<{ phases: BankrollPhase[] }> {
    return this.http.put<{ phases: BankrollPhase[] }>(`${this.base}/bankroll/phases`, { phases });
  }

  getBankrollCriteria(): Observable<BankrollCriteria> {
    return this.http.get<BankrollCriteria>(`${this.base}/bankroll/criteria`);
  }

  setBankrollCriteria(criteria: BankrollCriteria): Observable<BankrollCriteria> {
    return this.http.put<BankrollCriteria>(`${this.base}/bankroll/criteria`, criteria);
  }

  promoteBankroll(notes: string): Observable<BankrollHistoryEntry> {
    return this.http.post<BankrollHistoryEntry>(`${this.base}/bankroll/promote`, { notes });
  }

  demoteBankroll(reason: string, notes: string): Observable<BankrollHistoryEntry> {
    return this.http.post<BankrollHistoryEntry>(`${this.base}/bankroll/demote`, { reason, notes });
  }

  getBankrollHistory(): Observable<{ history: BankrollHistoryEntry[] }> {
    return this.http.get<{ history: BankrollHistoryEntry[] }>(`${this.base}/bankroll/history`);
  }

  confirmBankrollRound(phaseSequence: number, result: number, notes: string): Observable<BankrollRound> {
    return this.http.post<BankrollRound>(`${this.base}/bankroll/rounds`, { phase_sequence: phaseSequence, result, notes });
  }

  getBankrollRounds(): Observable<{ rounds: BankrollRound[] }> {
    return this.http.get<{ rounds: BankrollRound[] }>(`${this.base}/bankroll/rounds`);
  }

  // Assinatura Premium (Stripe) — ver ESTRATEGIA-MONETIZACAO.md
  getBillingStatus(): Observable<BillingStatus> {
    return this.http.get<BillingStatus>(`${this.base}/billing/status`);
  }

  createCheckoutSession(): Observable<{ url: string }> {
    return this.http.post<{ url: string }>(`${this.base}/billing/checkout`, {});
  }

  createPortalSession(): Observable<{ url: string }> {
    return this.http.post<{ url: string }>(`${this.base}/billing/portal`, {});
  }
}
