import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', redirectTo: 'visao-geral', pathMatch: 'full' },
  {
    path: 'visao-geral',
    loadComponent: () => import('./features/overview/overview.component').then(m => m.OverviewComponent),
  },
  {
    path: 'dashboard',
    loadComponent: () => import('./features/dashboard/dashboard.component').then(m => m.DashboardComponent),
  },
  {
    path: 'comparador',
    loadComponent: () => import('./features/comparator/comparator.component').then(m => m.ComparatorComponent),
  },
  {
    path: 'filtros',
    loadComponent: () => import('./features/filters/filters.component').then(m => m.FiltersComponent),
  },
  {
    path: 'estrategias',
    loadComponent: () => import('./features/strategies/strategies.component').then(m => m.StrategiesComponent),
  },
  {
    // Strategy Discovery Engine (Remodelagem F6, doc 08) — leitura pública: o
    // ranking já vem calculado do backend, sem exigir login para consultar.
    path: 'descobertas',
    loadComponent: () => import('./features/discovery/discovery.component').then(m => m.DiscoveryComponent),
  },
  {
    path: 'banca',
    loadComponent: () => import('./features/bankroll/bankroll.component').then(m => m.BankrollComponent),
  },
  {
    path: 'projecoes',
    loadComponent: () => import('./features/projections/projections.component').then(m => m.ProjectionsComponent),
  },
  {
    path: 'assinatura',
    loadComponent: () => import('./features/billing/billing.component').then(m => m.BillingComponent),
  },
  {
    path: 'redefinir-senha',
    loadComponent: () => import('./features/reset-password/reset-password.component').then(m => m.ResetPasswordComponent),
  },
  {
    path: 'integracoes',
    loadComponent: () => import('./features/integrations/integrations.component').then(m => m.IntegrationsComponent),
  },
  {
    // Explicação do método para quem não é da área. Pública e sem login: é a
    // página que o usuário abre ANTES de decidir se confia nos números.
    path: 'como-funciona',
    loadComponent: () => import('./features/how-it-works/how-it-works.component').then(m => m.HowItWorksComponent),
  },
  {
    path: 'suporte',
    loadComponent: () => import('./features/support/support.component').then(m => m.SupportComponent),
  },
  { path: '**', redirectTo: 'visao-geral' },
];
