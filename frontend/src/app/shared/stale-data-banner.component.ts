import { Component, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';

import { ApiService } from '../core/api.service';
import { SyncStatusResponse } from '../core/models';

/**
 * Faixa de aviso, no topo de toda a aplicação, quando os dados param de ser
 * atualizados.
 *
 * POR QUE EXISTE. Entre 02/08 e 12/09 o banco ficou seis semanas sem receber
 * partida nova e o produto continuou mostrando médias, frequências e tendências
 * com a cara de sempre. Ninguém percebeu porque nada na interface distinguia
 * "dado de ontem" de "dado de seis semanas atrás" — e a tela de Integrações
 * ainda exibia "Última sincronização: hoje", porque contava TENTATIVAS.
 *
 * Num produto de estatística, dado velho apresentado como atual é pior do que
 * tela vazia: a pessoa toma decisão com número que parece vivo e está morto.
 *
 * POR QUE FAIXA E NÃO MODAL. Modal bloqueia, e o que se aprende com modal é a
 * fechá-lo — em dois dias vira reflexo e o aviso perde a função justamente
 * quando mais importa. A faixa fica visível o tempo todo enquanto a condição
 * durar, não interrompe o uso, e não tem botão de "não mostrar mais": ela
 * desaparece sozinha quando a sincronização voltar, e só assim.
 */
@Component({
  selector: 'app-stale-data-banner',
  standalone: true,
  imports: [CommonModule, RouterLink, MatIconModule],
  template: `
    @if (status(); as s) {
      @if (s.stale) {
        <div
          role="status"
          aria-live="polite"
          class="w-full bg-amber-950/60 border-b border-amber-700/60 px-4 py-2.5"
        >
          <div class="max-w-[1600px] mx-auto flex items-start gap-3 flex-wrap">
            <mat-icon aria-hidden="true" class="!text-amber-400 !w-5 !h-5 !text-[20px] !leading-5 shrink-0 mt-0.5">
              warning
            </mat-icon>

            <div class="min-w-0 flex-1">
              <div class="text-sm text-amber-100 font-medium">
                {{ titulo(s) }}
              </div>
              <div class="text-xs text-amber-200/80 mt-0.5 leading-relaxed">
                As estatísticas abaixo continuam sendo exibidas, mas refletem apenas
                os jogos que já estavam no banco — partidas recentes podem estar faltando.
                @if (s.provider_error) {
                  <span class="block mt-1">
                    Motivo relatado pelo provedor de dados:
                    <strong class="text-amber-100">{{ s.provider_error }}</strong>
                  </span>
                }
              </div>
            </div>

            <a
              routerLink="/integracoes"
              class="text-xs font-medium text-amber-200 hover:text-amber-100 underline whitespace-nowrap mt-0.5"
            >
              Ver diagnóstico
            </a>
          </div>
        </div>
      }
    }
  `,
})
export class StaleDataBannerComponent implements OnInit {
  private api = inject(ApiService);

  status = signal<SyncStatusResponse | null>(null);

  ngOnInit(): void {
    this.api.getSyncStatus().subscribe({
      next: s => this.status.set(s),
      // Silêncio proposital: se a checagem falhar, o usuário não ganha nada com
      // um erro sobre o verificador de erros. A faixa simplesmente não aparece.
      error: () => this.status.set(null),
    });
  }

  titulo(s: SyncStatusResponse): string {
    if (s.hours_since_success === null) {
      return 'Os dados nunca foram sincronizados com sucesso.';
    }
    const dias = Math.floor(s.hours_since_success / 24);
    if (dias >= 2) {
      return `Dados desatualizados: a última sincronização bem-sucedida foi há ${dias} dias.`;
    }
    return 'Dados possivelmente desatualizados: a última sincronização não trouxe partidas novas.';
  }
}
