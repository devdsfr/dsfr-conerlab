import { Injectable, computed, signal } from '@angular/core';

import { ApiService } from './api.service';
import { AuthService } from './auth.service';
import { BillingStatus } from './models';

// BillingService centraliza o status da Assinatura Premium (ver
// ESTRATEGIA-MONETIZACAO.md) para que qualquer tela (navbar, Gestão de Banca,
// Projeções, página de assinatura) leia o mesmo signal em vez de cada uma chamar
// GET /billing/status por conta própria. refresh() é chamado explicitamente pelas
// telas que precisam do dado mais atual (não há polling automático).
@Injectable({ providedIn: 'root' })
export class BillingService {
  status = signal<BillingStatus | null>(null);
  loading = signal(false);

  // true assim que sabemos que o usuário NÃO é premium (status já carregado e
  // is_premium=false) — usado pelas telas de paywall para não "piscar" o conteúdo
  // pago antes do status chegar.
  isPremium = computed(() => this.status()?.is_premium ?? false);
  loaded = computed(() => this.status() !== null);

  constructor(private api: ApiService, private auth: AuthService) {}

  refresh(): void {
    if (!this.auth.isAuthenticated()) {
      this.status.set(null);
      return;
    }
    this.loading.set(true);
    this.api.getBillingStatus().subscribe({
      next: s => {
        this.status.set(s);
        this.loading.set(false);
      },
      error: () => {
        this.loading.set(false);
      },
    });
  }
}
