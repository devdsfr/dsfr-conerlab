import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap } from 'rxjs';

import { API_BASE_URL } from './api.service';
import { AuthResponse, AuthUser } from './models';

const TOKEN_KEY = 'cornerlab_token';
const USER_KEY = 'cornerlab_user';

// AuthService cobre o mínimo necessário para módulos que são "por usuário" (ex:
// Gestão Evolutiva de Banca, que depende das apostas já registradas por cada
// usuário). O token JWT é persistido em localStorage e anexado automaticamente pelo
// authInterceptor (ver auth.interceptor.ts).
@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly base = API_BASE_URL;

  user = signal<AuthUser | null>(this.readUser());
  token = signal<string | null>(localStorage.getItem(TOKEN_KEY));

  constructor(private http: HttpClient) {}

  isAuthenticated(): boolean {
    return !!this.token();
  }

  login(email: string, password: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.base}/auth/login`, { email, password }).pipe(
      tap(res => this.persist(res)),
    );
  }

  register(name: string, email: string, password: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.base}/auth/register`, { name, email, password }).pipe(
      tap(res => this.persist(res)),
    );
  }

  logout(): void {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    this.token.set(null);
    this.user.set(null);
  }

  private persist(res: AuthResponse): void {
    localStorage.setItem(TOKEN_KEY, res.token);
    localStorage.setItem(USER_KEY, JSON.stringify(res.user));
    this.token.set(res.token);
    this.user.set(res.user);
  }

  private readUser(): AuthUser | null {
    const raw = localStorage.getItem(USER_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as AuthUser;
    } catch {
      return null;
    }
  }
}
