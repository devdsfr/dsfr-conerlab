import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', redirectTo: 'dashboard', pathMatch: 'full' },
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
  { path: '**', redirectTo: 'dashboard' },
];
