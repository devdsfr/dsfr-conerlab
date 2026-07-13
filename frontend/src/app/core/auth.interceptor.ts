import { HttpInterceptorFn } from '@angular/common/http';

const TOKEN_KEY = 'cornerlab_token';

// Anexa "Authorization: Bearer <token>" em toda requisição, quando houver um usuário
// autenticado (ver AuthService). Lido diretamente do localStorage para evitar
// dependência circular com AuthService dentro do interceptor.
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (!token) return next(req);
  return next(req.clone({ setHeaders: { Authorization: `Bearer ${token}` } }));
};
