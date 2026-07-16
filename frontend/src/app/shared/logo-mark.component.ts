import { Component, input } from '@angular/core';

// Marca do CornerLab: o "L" verde é a linha de fundo + linha lateral + arco de
// escanteio do campo (o elemento mais reconhecível do jogo que a plataforma
// analisa), e as barras ciano ascendentes representam a estatística/análise
// em cima desses dados. Mesmo desenho usado no favicon (public/favicon.svg)
// e no ícone de app (apple-touch-icon.png, gerado a partir do mesmo traçado)
// — mantém a marca consistente entre a aba do navegador e o app em si.
@Component({
  selector: 'app-logo-mark',
  template: `
    <svg
      [attr.width]="size()"
      [attr.height]="size()"
      viewBox="0 0 100 100"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="58" y="58" width="12" height="27" rx="2.5" fill="#38bdf8" />
      <rect x="76" y="40" width="12" height="45" rx="2.5" fill="#38bdf8" />
      <path d="M15 85 L46 85" stroke="#22c55e" stroke-width="9" stroke-linecap="round" />
      <path d="M15 85 L15 48" stroke="#22c55e" stroke-width="9" stroke-linecap="round" />
      <path
        d="M15 67 A18 18 0 0 1 33 85"
        stroke="#22c55e"
        stroke-width="9"
        stroke-linecap="round"
        fill="none"
      />
    </svg>
  `,
})
export class LogoMarkComponent {
  size = input<number>(28);
}
