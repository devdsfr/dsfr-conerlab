import { Component, signal } from '@angular/core';
import { NavigationStart, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatToolbarModule, MatButtonModule, MatIconModule],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App {
  protected readonly title = 'CornerLab';

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
