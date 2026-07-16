import { Component, signal } from '@angular/core';
import { NavigationStart, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { LogoMarkComponent } from './shared/logo-mark.component';

interface NavItem {
  label: string;
  route: string;
  icon: string;
}

@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    MatToolbarModule,
    MatButtonModule,
    MatIconModule,
    LogoMarkComponent,
  ],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App {
  protected readonly title = 'CornerLab';

  protected readonly navItems: NavItem[] = [
    { label: 'Dashboard', route: '/dashboard', icon: 'query_stats' },
    { label: 'Comparador', route: '/comparador', icon: 'compare_arrows' },
    { label: 'Simulador de Filtros', route: '/filtros', icon: 'tune' },
    { label: 'Gestão de Banca', route: '/banca', icon: 'account_balance_wallet' },
    { label: 'Projeções', route: '/projecoes', icon: 'trending_up' },
    { label: 'Integrações', route: '/integracoes', icon: 'cable' },
  ];

  menuOpen = signal(false);

  constructor(router: Router) {
    router.events.subscribe(e => {
      if (e instanceof NavigationStart) this.menuOpen.set(false);
    });
  }

  toggleMenu(): void {
    this.menuOpen.set(!this.menuOpen());
  }
}
