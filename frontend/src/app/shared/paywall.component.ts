import { Component, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';

// Cartão de bloqueio exibido no lugar de um recurso exclusivo da Assinatura
// Premium (Gestão de Banca evolutiva, Cálculo de Projeções, Alertas, Exportações
// — ver ESTRATEGIA-MONETIZACAO.md). Reaproveitado em qualquer tela premium para
// manter a mensagem e o CTA consistentes.
@Component({
  selector: 'app-paywall',
  standalone: true,
  imports: [CommonModule, RouterLink, MatIconModule],
  template: `
    <div class="cl-card max-w-md mx-auto my-12 p-8 flex flex-col items-center text-center gap-2">
      <mat-icon class="!text-4xl !w-10 !h-10 text-cyan-400 mb-1">workspace_premium</mat-icon>
      <h2 class="text-lg font-semibold text-slate-100">{{ title() }}</h2>
      <p class="text-sm text-slate-400 mb-2">{{ description() }}</p>
      <a
        routerLink="/assinatura"
        class="inline-flex items-center justify-center rounded-lg bg-cornerlab-primary px-4 py-2 text-sm font-medium text-slate-900 hover:opacity-90 transition-opacity"
      >
        Assinar Premium
      </a>
    </div>
  `,
})
export class PaywallComponent {
  title = input<string>('Recurso exclusivo da Assinatura Premium');
  description = input<string>(
    'Assine o CornerLab Premium para desbloquear este recurso, com 7 dias grátis para testar.',
  );
}
