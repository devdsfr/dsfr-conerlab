import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { ProviderSummary, UsageEntry } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';

interface ProviderView extends ProviderSummary {
  testing: boolean;
  testResult: { ok: boolean; message: string } | null;
  chartLabels: string[];
  chartDatasets: { label: string; data: number[]; color?: string }[];
}

@Component({
  selector: 'app-integrations',
  standalone: true,
  imports: [
    CommonModule,
    MatCardModule,
    MatButtonModule,
    MatProgressSpinnerModule,
    MatTableModule,
    MatChipsModule,
    MatTooltipModule,
    SimpleChartComponent,
  ],
  templateUrl: './integrations.component.html',
})
export class IntegrationsComponent implements OnInit {
  loading = signal(true);
  error = signal<string | null>(null);
  providers = signal<ProviderView[]>([]);

  recentEntries = signal<UsageEntry[]>([]);
  showHistory = signal(false);
  historyLoading = signal(false);
  entryColumns = ['created_at', 'provider', 'endpoint', 'success', 'status_code', 'duration_ms'];

  readonly panelTooltip =
    'Este painel mostra apenas o consumo técnico das integrações externas (nº de chamadas, tokens, latência, sucesso/erro) — não expõe as chaves de API nem dados financeiros do usuário.';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set(null);
    this.api.getUsageSummary().subscribe({
      next: res => {
        this.providers.set(res.providers.map(p => this.toView(p)));
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar o status das integrações');
        this.loading.set(false);
      },
    });
  }

  private toView(p: ProviderSummary): ProviderView {
    return {
      ...p,
      testing: false,
      testResult: null,
      chartLabels: p.daily_calls.map(d => d.date.slice(5)),
      chartDatasets: [{ label: 'Chamadas/dia (7 dias)', data: p.daily_calls.map(d => d.count), color: '#38bdf8' }],
    };
  }

  testConnection(provider: ProviderView): void {
    provider.testing = true;
    provider.testResult = null;
    this.providers.set([...this.providers()]);
    this.api.testConnection(provider.provider).subscribe({
      next: res => {
        provider.testing = false;
        provider.testResult = { ok: res.ok, message: res.message };
        this.providers.set([...this.providers()]);
        this.load();
      },
      error: err => {
        provider.testing = false;
        provider.testResult = { ok: false, message: err?.error?.error ?? 'Falha ao testar a conexão' };
        this.providers.set([...this.providers()]);
      },
    });
  }

  toggleHistory(): void {
    this.showHistory.set(!this.showHistory());
    if (this.showHistory() && this.recentEntries().length === 0) {
      this.historyLoading.set(true);
      this.api.getRecentUsage(undefined, 30).subscribe({
        next: res => {
          this.recentEntries.set(res.entries);
          this.historyLoading.set(false);
        },
        error: () => this.historyLoading.set(false),
      });
    }
  }

  statusLabel(p: ProviderView): string {
    if (!p.configured) return 'Não configurada';
    if (p.total_calls === 0) return 'Configurada — sem chamadas ainda';
    if (p.last_success_at && (!p.last_error_at || p.last_success_at >= p.last_error_at)) return 'Configurada — funcionando';
    return 'Configurada — com falhas recentes';
  }

  statusClass(p: ProviderView): string {
    if (!p.configured) return 'text-slate-400';
    if (p.total_calls === 0) return 'text-slate-400';
    if (p.last_success_at && (!p.last_error_at || p.last_success_at >= p.last_error_at)) return 'text-cornerlab-primary';
    return 'text-red-400';
  }
}
