import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatIconModule } from '@angular/material/icon';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { BillingService } from '../../core/billing.service';

const PREMIUM_FEATURES = [
  'Gestão Evolutiva de Banca (fases, critérios de promoção/rebaixamento)',
  'Cálculo de Projeções (simulador de reinvestimento composto)',
  'Histórico completo no Simulador de Filtros (sem o limite de 90 dias do plano gratuito)',
  'Exportação de dados (CSV/XLSX) de todos os módulos',
  'Alertas personalizados',
  'Sem anúncios',
];

@Component({
  selector: 'app-billing',
  standalone: true,
  imports: [CommonModule, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatProgressSpinnerModule, MatIconModule],
  templateUrl: './billing.component.html',
})
export class BillingComponent implements OnInit {
  readonly features = PREMIUM_FEATURES;

  checkoutLoading = signal(false);
  portalLoading = signal(false);
  actionError = signal<string | null>(null);
  // Setado quando a URL tem ?session_id= (retorno do Stripe Checkout) — o webhook
  // pode levar alguns segundos para atualizar o status, então mostramos uma
  // mensagem de "processando" enquanto o BillingService.refresh() é repetido.
  justCheckedOut = signal(false);

  // Autenticação mínima (mesmo padrão do módulo de Gestão de Banca)
  loginMode = signal<'login' | 'register'>('login');
  authLoading = signal(false);
  authError = signal<string | null>(null);
  loginEmail = '';
  loginPassword = '';
  registerName = '';
  registerEmail = '';
  registerPassword = '';

  constructor(
    private api: ApiService,
    public auth: AuthService,
    public billing: BillingService,
    private route: ActivatedRoute,
  ) {}

  ngOnInit(): void {
    if (this.auth.isAuthenticated()) {
      this.billing.refresh();
    }
    if (this.route.snapshot.queryParamMap.has('session_id')) {
      this.justCheckedOut.set(true);
      // O webhook do Stripe costuma chegar em poucos segundos; tenta algumas vezes
      // para o status virar "premium" sem exigir um reload manual da página.
      let attempts = 0;
      const interval = setInterval(() => {
        attempts++;
        this.billing.refresh();
        if (this.billing.isPremium() || attempts >= 6) clearInterval(interval);
      }, 3000);
    }
  }

  submitLogin(): void {
    this.authLoading.set(true);
    this.authError.set(null);
    this.auth.login(this.loginEmail, this.loginPassword).subscribe({
      next: () => {
        this.authLoading.set(false);
        this.billing.refresh();
      },
      error: err => {
        this.authLoading.set(false);
        this.authError.set(err?.error?.error ?? 'E-mail ou senha inválidos');
      },
    });
  }

  submitRegister(): void {
    this.authLoading.set(true);
    this.authError.set(null);
    this.auth.register(this.registerName, this.registerEmail, this.registerPassword).subscribe({
      next: () => {
        this.authLoading.set(false);
        this.billing.refresh();
      },
      error: err => {
        this.authLoading.set(false);
        this.authError.set(err?.error?.error ?? 'Não foi possível criar a conta');
      },
    });
  }

  assinar(): void {
    this.checkoutLoading.set(true);
    this.actionError.set(null);
    this.api.createCheckoutSession().subscribe({
      next: res => {
        window.location.href = res.url;
      },
      error: err => {
        this.checkoutLoading.set(false);
        this.actionError.set(err?.error?.error ?? 'Não foi possível iniciar o checkout');
      },
    });
  }

  gerenciarAssinatura(): void {
    this.portalLoading.set(true);
    this.actionError.set(null);
    this.api.createPortalSession().subscribe({
      next: res => {
        window.location.href = res.url;
      },
      error: err => {
        this.portalLoading.set(false);
        this.actionError.set(err?.error?.error ?? 'Não foi possível abrir o portal de gerenciamento');
      },
    });
  }
}
