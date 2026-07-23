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
    path: 'suporte',
    loadComponent: () => import('./features/support/support.component').then(m => m.SupportComponent),
  },
  { path: '**', redirectTo: 'visao-geral' },
];
