import { Component, OnDestroy, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatIconModule } from '@angular/material/icon';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { ProviderSummary, StorageUsage, SyncProgress, SyncRun, SyncRunResult, UsageEntry } from '../../core/models';
import { SimpleChartComponent } from '../../shared/simple-chart.component';

interface ProviderView extends ProviderSummary {
  testing: boolean;
  testResult: { ok: boolean; message: string } | null;
  chartLabels: string[];
  chartDatasets: { label: string; data: number[]; color?: string }[];
}

// Se o carregamento inicial passar disso, mostramos um aviso de "está
// demorando mais que o esperado" com opção de tentar de novo — em vez de
// deixar o usuário olhando para um spinner sem qualquer contexto por tempo
// indeterminado (ver template).
const SLOW_LOAD_TIMEOUT_MS = 8000;

// Intervalo do polling da barra de progresso. 2s dá movimento perceptível sem
// martelar o servidor: cada passo do ciclo leva ~6,5s por causa do throttle.
const POLL_MS = 2000;

@Component({
  selector: 'app-integrations',
  standalone: true,
  imports: [
    CommonModule,
    MatCardModule,
    MatButtonModule,
    MatProgressSpinnerModule,
    MatProgressBarModule,
    MatTableModule,
    MatChipsModule,
    MatTooltipModule,
    MatIconModule,
    SimpleChartComponent,
  ],
  templateUrl: './integrations.component.html',
})
export class IntegrationsComponent implements OnInit, OnDestroy {
  loading = signal(true);
  loadingSlow = signal(false);
  private slowLoadTimer?: ReturnType<typeof setTimeout>;
  error = signal<string | null>(null);
  providers = signal<ProviderView[]>([]);

  // Consumo do banco. Opcional no payload: se o backend não conseguir consultar
  // (ex: permissão negada em pg_stat_user_tables), o card simplesmente não aparece.
  storage = signal<StorageUsage | null>(null);

  recentEntries = signal<UsageEntry[]>([]);
  showHistory = signal(false);
  historyLoading = signal(false);
  entryColumns = ['created_at', 'provider', 'endpoint', 'success', 'status_code', 'duration_ms'];

  readonly panelTooltip =
    'Este painel mostra apenas o consumo técnico das integrações externas (nº de chamadas, tokens, latência, sucesso/erro) — não expõe as chaves de API nem dados financeiros do usuário.';

  // Botão "Sincronizar agora" — dispara o mesmo ciclo de descoberta + atualização de
  // partidas que o Render Cron Job roda periodicamente, para quando o usuário notar
  // dados desatualizados e não quiser esperar o próximo ciclo agendado.
  syncLoading = signal(false);
  syncResult = signal<SyncRunResult | null>(null);
  syncError = signal<string | null>(null);

  // Andamento do ciclo, consultado por polling enquanto ele roda.
  progress = signal<SyncProgress | null>(null);
  private pollTimer?: ReturnType<typeof setInterval>;

  // Status rápido "API-Football está de pé?" — independente do painel completo de
  // consumo (que pode demorar mais, ver load()), pra decidir antes de clicar em
  // "Sincronizar agora".
  apiFootballStatus = signal<'checking' | 'up' | 'down' | null>(null);
  apiFootballMessage = signal<string | null>(null);

  // Última sincronização registrada no banco (manual ou via Render Cron Job) — vem do
  // backend (tabela sync_runs), não de estado local do navegador, para o usuário poder
  // conferir "já foi sincronizado hoje?" mesmo depois de recarregar a página.
  lastSyncRun = signal<SyncRun | null>(null);
  lastSyncLoading = signal(false);

  constructor(private api: ApiService, public auth: AuthService) {}

  ngOnInit(): void {
    this.load();
    this.checkApiFootball();
    this.loadSyncStatus();

    // Se um ciclo já estiver rodando (disparado em outra aba, ou a página foi
    // recarregada no meio), a barra reaparece sozinha em vez de sumir.
    this.api.getSyncProgress().subscribe({
      next: p => {
        if (p.running) {
          this.progress.set(p);
          this.syncLoading.set(true);
          this.startPolling();
        }
      },
      error: () => {},
    });
  }

  loadSyncStatus(): void {
    this.lastSyncLoading.set(true);
    this.api.getSyncStatus().subscribe({
      next: res => {
        this.lastSyncRun.set(res.last_run);
        this.lastSyncLoading.set(false);
      },
      error: () => this.lastSyncLoading.set(false),
    });
  }

  formatLastSync(run: SyncRun): string {
    const d = new Date(run.created_at);
    const dd = String(d.getDate()).padStart(2, '0');
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const hh = String(d.getHours()).padStart(2, '0');
    const min = String(d.getMinutes()).padStart(2, '0');
    const origem = run.triggered_by === 'cron' ? 'automática' : 'manual';
    const duracao = this.formatDuration(run.duration_ms);
    return `${dd}/${mm} ${hh}:${min} (${origem}, durou ${duracao})`;
  }

  // Duração do ciclo completo (descoberta + atualização) — pedido do usuário para
  // decidir o horário do Render Cron Job com base em quanto tempo isso realmente leva.
  formatDuration(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    const totalSeconds = Math.round(ms / 1000);
    if (totalSeconds < 60) return `${totalSeconds}s`;
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return `${minutes}min ${seconds}s`;
  }

  checkApiFootball(): void {
    this.apiFootballStatus.set('checking');
    this.api.testConnection('api_football').subscribe({
      next: res => {
        this.apiFootballStatus.set(res.ok ? 'up' : 'down');
        this.apiFootballMessage.set(res.message);
      },
      error: err => {
        this.apiFootballStatus.set('down');
        this.apiFootballMessage.set(err?.error?.error ?? 'Falha ao verificar a API-Football');
      },
    });
  }

  ngOnDestroy(): void {
    clearTimeout(this.slowLoadTimer);
    // Sair da tela não cancela o ciclo (ele roda no servidor), mas o polling
    // precisa parar, senão fica um setInterval órfão batendo na API.
    clearInterval(this.pollTimer);
  }

  load(): void {
    this.loading.set(true);
    this.loadingSlow.set(false);
    this.error.set(null);
    clearTimeout(this.slowLoadTimer);
    this.slowLoadTimer = setTimeout(() => this.loadingSlow.set(true), SLOW_LOAD_TIMEOUT_MS);

    this.api.getUsageSummary().subscribe({
      next: res => {
        this.providers.set(res.providers.map(p => this.toView(p)));
        this.storage.set(res.storage ?? null);
        this.loading.set(false);
        this.loadingSlow.set(false);
        clearTimeout(this.slowLoadTimer);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar o status das integrações');
        this.loading.set(false);
        this.loadingSlow.set(false);
        clearTimeout(this.slowLoadTimer);
      },
    });
  }

  private toView(p: ProviderSummary): ProviderView {
    // Defesa extra: se algum dia a API voltar a mandar null aqui (provedor sem
    // nenhuma chamada), não deixa a página inteira quebrar — mesmo bug já
    // corrigido no backend para o campo "mode" do Dashboard.
    const dailyCalls = p.daily_calls ?? [];
    return {
      ...p,
      testing: false,
      testResult: null,
      chartLabels: dailyCalls.map(d => (d.date ? d.date.slice(5) : '')),
      chartDatasets: [{ label: 'Chamadas/dia (7 dias)', data: dailyCalls.map(d => d.count), color: '#38bdf8' }],
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

  // O ciclo roda em segundo plano no backend (POST responde 202 na hora) porque,
  // com o throttle da API-Football, ele leva minutos. Aqui só disparamos e passamos
  // a perguntar o andamento a cada POLL_MS para desenhar a barra.
  runSync(): void {
    if (!this.auth.isAuthenticated()) {
      this.syncError.set('Faça login (aba Assinatura) para sincronizar manualmente.');
      return;
    }
    this.syncLoading.set(true);
    this.syncError.set(null);
    this.syncResult.set(null);

    this.api.syncRun().subscribe({
      next: res => {
        this.progress.set(res.progress ?? null);
        this.startPolling();
      },
      error: err => {
        // 409 = já existe um ciclo rodando (outra aba, ou o cron). Em vez de tratar
        // como falha, entra no modo de acompanhamento do ciclo que já está em curso.
        if (err?.status === 409) {
          this.progress.set(err?.error?.progress ?? null);
          this.startPolling();
          return;
        }
        this.syncLoading.set(false);
        this.syncError.set(err?.error?.error ?? 'Não foi possível sincronizar agora');
      },
    });
  }

  private startPolling(): void {
    clearInterval(this.pollTimer);
    this.pollTimer = setInterval(() => this.pollProgress(), POLL_MS);
    this.pollProgress();
  }

  private pollProgress(): void {
    this.api.getSyncProgress().subscribe({
      next: p => {
        this.progress.set(p);
        if (p.running) return;

        // Terminou: para o polling e recarrega o painel com os números novos.
        this.stopPolling();
        if (p.phase === 'erro') {
          this.syncError.set(p.error || 'A sincronização falhou');
        } else if (p.phase === 'concluido') {
          this.syncResult.set({
            discovery: {
              Targets: p.discovery?.Targets ?? 0,
              FixturesFound: p.discovery?.FixturesFound ?? 0,
              FixturesUpserted: p.discovery?.FixturesUpserted ?? 0,
              Errors: p.discovery?.Errors ?? 0,
            },
            update: {
              Checked: p.update?.Checked ?? 0,
              Finalized: p.update?.Finalized ?? 0,
              StillOpen: p.update?.StillOpen ?? 0,
              Errors: p.update?.Errors ?? 0,
            },
            duration_ms: p.duration_ms,
          });
          this.load();
          this.loadSyncStatus();
        }
      },
      // Falha pontual no polling não cancela o acompanhamento: o ciclo segue
      // rodando no servidor e a próxima tentativa provavelmente responde.
      error: () => {},
    });
  }

  private stopPolling(): void {
    clearInterval(this.pollTimer);
    this.pollTimer = undefined;
    this.syncLoading.set(false);
  }

  /** Rótulo curto do que está acontecendo agora, para acompanhar a barra. */
  progressLabel(p: SyncProgress): string {
    if (p.total > 0) return `${p.phase_label} — ${p.current} de ${p.total}`;
    return p.phase_label;
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
