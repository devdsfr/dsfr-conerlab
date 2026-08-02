import { Component, OnInit, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { DiscoveredStrategy, DiscoveryRunResult, League } from '../../core/models';
import { PageLoaderComponent } from '../../shared/page-loader.component';

// Página "Descobertas" — Strategy Discovery Engine (Remodelagem F6, doc 08).
//
// Inversão de fluxo em relação ao Simulador de Filtros: aqui o usuário não monta
// nada. O sistema minerou o histórico, testou centenas de combinações, descartou
// tudo que não passou nos critérios mínimos e apresenta somente os padrões
// consistentes — já explicados em texto.
//
// Nenhum número desta tela é calculado no frontend: o backend entrega a linha do
// ranking pronta (inclusive a classificação). O componente só formata e explica.
@Component({
  selector: 'app-discovery',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatButtonModule,
    MatIconModule,
    MatProgressSpinnerModule,
    MatTooltipModule,
    PageLoaderComponent,
  ],
  templateUrl: './discovery.component.html',
})
export class DiscoveryComponent implements OnInit {
  leagues = signal<League[]>([]);
  leagueId = signal<number | null>(null);

  strategies = signal<DiscoveredStrategy[]>([]);
  disclaimer = signal('');
  expanded = signal<number | null>(null);

  loading = signal(false);
  running = signal(false);
  error = signal<string | null>(null);
  lastRun = signal<DiscoveryRunResult | null>(null);

  /** Faixas do doc 08, para explicar a classificação exibida em cada card. */
  readonly classificationTooltips: Record<string, string> = {
    Elite: 'DSFR Score 91–100: padrão histórico excepcional em todos os indicadores avaliados.',
    Excelente: 'DSFR Score 81–90: padrão histórico muito forte e consistente.',
    'Muito Boa': 'DSFR Score 71–80: padrão histórico forte.',
    Boa: 'DSFR Score 61–70: padrão histórico sólido.',
    Regular: 'DSFR Score 40–60: padrão aprovado nos critérios mínimos, mas sem destaque.',
  };

  readonly stageLabels: Record<string, string> = {
    nascimento: 'Nascimento',
    crescimento: 'Crescimento',
    maturidade: 'Maturidade',
    declinio: 'Declínio',
    obsoleta: 'Obsoleta',
  };

  readonly dsfrTooltip =
    'DSFR Score (0–100): nota proprietária que resume a qualidade do padrão — pondera ROI, EV, taxa de acerto, yield, drawdown, tamanho da amostra, consistência e variância.';
  readonly confidenceTooltip =
    'Confiabilidade (0–100): quanta evidência estatística sustenta o número — cresce com o tamanho da amostra e a estabilidade dos resultados.';
  readonly riskTooltip =
    'Risco (0–100): quanto MENOR, melhor. Combina drawdown, variância, volatilidade e taxa de erro.';
  readonly drawdownTooltip =
    'Maior queda acumulada do histórico, em unidades de aposta — o pior momento pelo qual o padrão passou.';

  /** Motivos de descarte devolvidos pelo ciclo, em linguagem de negócio. */
  readonly rejectionLabels: Record<string, string> = {
    amostra_insuficiente: 'amostra insuficiente',
    win_rate_baixo: 'taxa de acerto baixa',
    roi_baixo: 'ROI baixo',
    yield_baixo: 'yield baixo',
    ev_nao_positivo: 'EV não positivo',
    drawdown_alto: 'drawdown alto',
    score_baixo: 'score baixo',
  };

  /** Resumo do último ciclo em uma frase, para o usuário entender o que aconteceu. */
  lastRunSummary = computed(() => {
    const r = this.lastRun();
    if (!r) return '';
    const scope = r.league_name ? `no ${r.league_name}` : `em ${r.leagues ?? 0} campeonato(s)`;
    return `${r.combinations} combinações testadas ${scope} · ${r.published} padrão(ões) aprovado(s) e publicado(s)`;
  });

  constructor(private api: ApiService, public auth: AuthService, private router: Router) {}

  ngOnInit(): void {
    this.api.listLeagues().subscribe({
      next: list => this.leagues.set(list ?? []),
    });
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set(null);
    this.api.listDiscoveredStrategies(this.leagueId() ?? undefined).subscribe({
      next: res => {
        this.strategies.set(res.strategies ?? []);
        this.disclaimer.set(res.disclaimer ?? '');
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar as descobertas');
        this.loading.set(false);
      },
    });
  }

  onLeagueChange(value: string): void {
    this.leagueId.set(value ? Number(value) : null);
    this.expanded.set(null);
    this.load();
  }

  /**
   * "Procurar agora": só faz sentido quando há uma liga escolhida — varrer todas
   * de uma vez pode levar minutos e o usuário ficaria sem retorno na tela.
   */
  run(): void {
    this.running.set(true);
    this.error.set(null);
    this.api.runDiscovery(this.leagueId() ?? undefined).subscribe({
      next: result => {
        this.lastRun.set(result);
        this.running.set(false);
        this.load();
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao executar a busca por estratégias');
        this.running.set(false);
      },
    });
  }

  toggle(id: number): void {
    this.expanded.set(this.expanded() === id ? null : id);
  }

  /**
   * Abre a descoberta no Simulador de Filtros. Reproduzibilidade é um princípio da
   * plataforma: o usuário precisa poder reexecutar o backtest que gerou o número e
   * conferir jogo por jogo, em vez de confiar no ranking.
   */
  openInSimulator(s: DiscoveredStrategy): void {
    this.router.navigate(['/filtros'], { state: { definition: s.definition } });
  }

  // ---- helpers de exibição --------------------------------------------------

  classificationTooltip(c: string): string {
    return this.classificationTooltips[c] ?? 'Faixa de qualidade calculada a partir do DSFR Score.';
  }

  classificationClass(c: string): string {
    switch (c) {
      case 'Elite':
        return 'bg-cornerlab-primary/20 text-cornerlab-primary border-cornerlab-primary/40';
      case 'Excelente':
      case 'Muito Boa':
        return 'bg-emerald-500/15 text-emerald-300 border-emerald-500/40';
      case 'Boa':
        return 'bg-sky-500/15 text-sky-300 border-sky-500/40';
      default:
        return 'bg-slate-700/40 text-slate-300 border-slate-600';
    }
  }

  stageLabel(stage: string): string {
    return this.stageLabels[stage] ?? '—';
  }

  scoreColor(v: number): string {
    if (v >= 65) return 'text-cornerlab-primary';
    if (v >= 45) return 'text-amber-400';
    return 'text-red-400';
  }

  // Risco é invertido: quanto menor, melhor.
  riskColor(v: number): string {
    if (v <= 35) return 'text-cornerlab-primary';
    if (v <= 60) return 'text-amber-400';
    return 'text-red-400';
  }

  signed(v: number): string {
    return (v > 0 ? '+' : '') + v.toFixed(2);
  }

  rejectionEntries(r?: Record<string, number>): { label: string; count: number }[] {
    if (!r) return [];
    return Object.entries(r)
      .map(([k, count]) => ({ label: this.rejectionLabels[k] ?? k, count }))
      .sort((a, b) => b.count - a.count);
  }
}
