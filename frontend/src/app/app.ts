import { Component, signal } from '@angular/core';
import { NavigationStart, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { LogoMarkComponent } from './shared/logo-mark.component';

// Um único array alimenta tanto o menu desktop quanto o mobile (ver app.html)
// — evita duplicar os 5 links em dois lugares e correr o risco de um ficar
// desatualizado (ex.: adicionar uma página nova e esquecer de repetir no
// menu hamburguer).
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
    { label: 'Integrações', route: '/integracoes', icon: 'cable' },
  ];

  // Navbar mobile: em telas estreitas os 5 itens não cabem em linha, então
  // ficam escondidos atrás de um botão hamburguer (ver app.html) — sem isso a
  // navegação quebra/vaza em viewports pequenos.
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
