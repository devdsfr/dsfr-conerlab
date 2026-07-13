import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
  League,
  Season,
  Team,
  DashboardResult,
  ComparisonResult,
  FilterRunRequest,
  BacktestResult,
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

  // Catálogo
  listLeagues(): Observable<League[]> {
    return this.http.get<League[]>(`${this.base}/leagues`);
  }

  listSeasons(leagueId: number): Observable<Season[]> {
    return this.http.get<Season[]>(`${this.base}/leagues/${leagueId}/seasons`);
  }

  listTeams(leagueId?: number, query?: string): Observable<Team[]> {
    let url = `${this.base}/teams?`;
    if (leagueId) url += `league_id=${leagueId}&`;
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
}
