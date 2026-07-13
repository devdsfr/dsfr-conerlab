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
  { path: '**', redirectTo: 'dashboard' },
];
