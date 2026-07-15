import { inject } from '@angular/core';
import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { catchError, throwError } from 'rxjs';

import { AuthService } from './auth.service';

const TOKEN_KEY = 'cornerlab_token';

// Anexa "Authorization: Bearer <token>" em toda requisição, quando houver um usuário
// autenticado (ver AuthService). O token em si é lido diretamente do localStorage
// (mais simples que injetar AuthService só para isso).
//
// Também trata globalmente o caso de token ausente/inválido/expirado (401 vindo do
// middleware Go — ver backend/internal/delivery/http/middleware/auth.go): em vez de
// deixar cada tela exibir o erro cru da API ("token inválido"), desloga o usuário
// automaticamente (AuthService.logout('expired')). Isso faz as telas que dependem de
// auth.isAuthenticated() (ex.: Gestão de Banca) voltarem sozinhas para a tela de
// login, já com um aviso de sessão expirada — sem deixar o cabeçalho "logado"
// inconsistente com um erro de autenticação na tela.
// Login e registro ficam de fora: um 401 ali é credencial errada, não sessão expirada.
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  // inject() só é válido no contexto síncrono da chamada do interceptor —
  // por isso é resolvido aqui em cima, não dentro do callback do catchError.
  const auth = inject(AuthService);

  const token = localStorage.getItem(TOKEN_KEY);
  const authReq = token ? req.clone({ setHeaders: { Authorization: `Bearer ${token}` } }) : req;

  const isAuthEndpoint = /\/auth\/(login|register)$/.test(req.url);

  return next(authReq).pipe(
    catchError((err: unknown) => {
      if (
        token &&
        !isAuthEndpoint &&
        err instanceof HttpErrorResponse &&
        (err.status === 401 || err.status === 403)
      ) {
        auth.logout('expired');
      }
      return throwError(() => err);
    }),
  );
};
