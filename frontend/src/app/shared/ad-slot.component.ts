import { AfterViewInit, Component, input } from '@angular/core';

// ID do publisher AdSense — placeholder até a conta AdSense do CornerLab ser
// criada/aprovada (ver ESTRATEGIA-MONETIZACAO.md, seção "AdSense contextual").
// Enquanto for este placeholder, AdSlotComponent não renderiza nenhum <ins> de
// anúncio (evita solicitar anúncios reais com um client ID inválido) — assim
// que a conta for aprovada, troque este valor pelo ca-pub-XXXXXXXXXXXXXXXX real
// (e o mesmo valor no <script> de src/index.html).
export const ADSENSE_CLIENT_ID = 'ca-pub-0000000000000000';

const ADSENSE_CONFIGURED = ADSENSE_CLIENT_ID !== 'ca-pub-0000000000000000';

// Slot de anúncio contextual reutilizável. Usado só nas páginas gratuitas
// (Dashboard, Comparador, Simulador de Filtros) — nunca em Gestão de Banca ou
// Cálculo de Projeções, para manter "sem anúncios" como incentivo de upgrade
// da Assinatura Premium.
@Component({
  selector: 'app-ad-slot',
  standalone: true,
  template: `
    @if (configured) {
      <ins
        class="adsbygoogle block"
        style="display:block"
        [attr.data-ad-client]="clientId"
        [attr.data-ad-slot]="slot()"
        data-ad-format="auto"
        data-full-width-responsive="true"
      ></ins>
    }
  `,
})
export class AdSlotComponent implements AfterViewInit {
  // ID do bloco de anúncio no painel do AdSense (cada posição na página tem um
  // slot diferente) — passado pelo componente que usa <app-ad-slot>.
  slot = input.required<string>();

  readonly clientId = ADSENSE_CLIENT_ID;
  readonly configured = ADSENSE_CONFIGURED;

  ngAfterViewInit(): void {
    if (!this.configured) return;
    try {
      ((window as any).adsbygoogle = (window as any).adsbygoogle || []).push({});
    } catch {
      // Bloqueadores de anúncio removem/alteram adsbygoogle — falha silenciosa,
      // sem afetar o resto da página.
    }
  }
}
