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
  SyncStatusResponse,
  BillingStatus,
  UpcomingMatch,
  Strategy,
  StrategyBundle,
  StrategyEvaluation,
  DiscoveredStrategiesResponse,
  DiscoveryRunResult,
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
  compare(teamA: number, teamB: number, leagueId?: number, limit = 10): Observable<ComparisonResult> {
    let url = `${this.base}/comparator?team_a=${teamA}&team_b=${teamB}&limit=${limit}`;
    if (leagueId) url += `&league_id=${leagueId}`;
    return this.http.get<ComparisonResult>(url);
  }

  // Módulo 3
  runFilter(req: FilterRunRequest): Observable<BacktestResult> {
    return this.http.post<BacktestResult>(`${this.base}/filters/run`, req);
  }

  // Strategy Engine (Remodelagem F5) — CRUD + execução de estratégias.
  listStrategies(): Observable<Strategy[]> {
    return this.http.get<Strategy[]>(`${this.base}/strategies`);
  }

  createStrategy(payload: { name: string; description?: string; definition: string; favorite?: boolean }): Observable<Strategy> {
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
  runDiscovery(leagueId?: number): Observable<DiscoveryRunResult> {
    return this.http.post<DiscoveryRunResult>(`${this.base}/discovery/run`, leagueId ? { league_id: leagueId } : {});
  }

  // Painel "Integrações" — status/consumo das APIs externas
  getUsageSummary(): Observable<UsageSummaryResponse> {
    return this.http.get<UsageSummaryResponse>(`${this.base}/diagnostics/usage`);
  }

  testConnection(provider: string): Observable<TestConnectionResult> {
    return this.http.post<TestConnectionResult>(`${this.base}/diagnostics/test/${provider}`, {});
  }

  /** Botão "Sincronizar agora" — exige login (ver router.go, grupo authGroup). */
  syncRun(): Observable<SyncRunResult> {
    return this.http.post<SyncRunResult>(`${this.base}/sync/run`, {});
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
