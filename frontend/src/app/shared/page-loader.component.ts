import { Component, input } from '@angular/core';
import { LogoMarkComponent } from './logo-mark.component';

// Substitui o <mat-spinner> genérico nas telas de carregamento de página
// inteira (Visão Geral, Dashboard, Comparador, Simulador, Descobertas,
// Projeções, Gestão de Banca). Em vez de um indicador neutro, usa a própria
// marca do CornerLab — anel cônico verde→ciano girando em torno do "L" que
// pulsa suavemente — reforçando identidade mesmo nos estados de espera.
// Spinners pequenos e contextuais (dentro de botões, campos, seções
// secundárias) continuam usando mat-spinner: este componente é só para o
// estado "carregando a página toda".
@Component({
  selector: 'app-page-loader',
  imports: [LogoMarkComponent],
  template: `
    <div class="cl-loader" role="status" [attr.aria-label]="label() || 'Carregando'">
      <span class="cl-loader-ring">
        <app-logo-mark [size]="28" />
      </span>
      @if (label()) {
        <span class="cl-loader-label">{{ label() }}</span>
      }
    </div>
  `,
  styles: [`
    .cl-loader {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 0.85rem;
      padding: 3rem 0;
    }
    .cl-loader-ring {
      position: relative;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 58px;
      height: 58px;
      border-radius: 999px;
    }
    .cl-loader-ring::before {
      content: '';
      position: absolute;
      inset: 0;
      border-radius: 999px;
      background: conic-gradient(from 0deg, #22c55e 0deg, #38bdf8 130deg, transparent 200deg, transparent 360deg);
      -webkit-mask: radial-gradient(farthest-side, transparent calc(100% - 2.5px), #fff calc(100% - 2.5px));
      mask: radial-gradient(farthest-side, transparent calc(100% - 2.5px), #fff calc(100% - 2.5px));
      animation: cl-loader-spin 1s linear infinite;
    }
    .cl-loader-ring::after {
      content: '';
      position: absolute;
      inset: 0;
      border-radius: 999px;
      border: 1px solid rgba(148, 163, 184, 0.14);
    }
    .cl-loader-ring app-logo-mark {
      display: flex;
      animation: cl-loader-pulse 1.8s ease-in-out infinite;
    }
    .cl-loader-label {
      font-size: 0.6875rem;
      font-weight: 500;
      letter-spacing: 0.09em;
      text-transform: uppercase;
      color: #64748b;
    }
    @keyframes cl-loader-spin {
      to { transform: rotate(360deg); }
    }
    @keyframes cl-loader-pulse {
      0%, 100% { opacity: 0.6; transform: scale(0.92); }
      50% { opacity: 1; transform: scale(1); }
    }
  `],
})
export class PageLoaderComponent {
  label = input<string>('');
}
