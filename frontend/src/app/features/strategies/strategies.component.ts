import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { Strategy, StrategyBundle } from '../../core/models';

// Strategy Workspace (Remodelagem F5 — docs 14/22): cada estratégia com
// backtest, health, DSFR score e lifecycle "em uma única página". Os números
// são calculados pelo backend (Formula Catalog); aqui só exibimos e explicamos.
@Component({
  selector: 'app-strategies',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    RouterLink,
    MatButtonModule,
    MatIconModule,
    MatProgressSpinnerModule,
    MatTooltipModule,
  ],
  templateUrl: './strategies.component.html',
})
export class StrategiesComponent implements OnInit {
  strategies = signal<Strategy[]>([]);
  selected = signal<StrategyBundle | null>(null);
  loadingList = signal(false);
  loadingDetail = signal(false);
  running = signal(false);
  error = signal<string | null>(null);

  readonly stageLabels: Record<string, string> = {
    nascimento: 'Nascimento',
    crescimento: 'Crescimento',
    maturidade: 'Maturidade',
    declinio: 'Declínio',
    obsoleta: 'Obsoleta',
  };

  readonly dsfrTooltip =
    'DSFR Score (0–100): nota proprietária que resume a qualidade da estratégia — pondera ROI, EV, taxa de acerto, yield, drawdown, tamanho da amostra, consistência e variância.';
  readonly healthTooltip =
    'Health Score (0–100): compara a execução mais recente com a anterior. 50 = estável; acima, melhorando; abaixo, perdendo eficiência.';

  constructor(private api: ApiService, public auth: AuthService) {}

  ngOnInit(): void {
    if (this.auth.isAuthenticated()) this.reload();
  }

  reload(): void {
    this.loadingList.set(true);
    this.api.listStrategies().subscribe({
      next: list => {
        this.strategies.set(list ?? []);
        this.loadingList.set(false);
        const current = this.selected();
        if (!current && list?.length) this.select(list[0]);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao listar estratégias');
        this.loadingList.set(false);
      },
    });
  }

  select(s: Strategy): void {
    this.loadingDetail.set(true);
    this.error.set(null);
    this.api.getStrategy(s.id).subscribe({
      next: bundle => {
        this.selected.set(bundle);
        this.loadingDetail.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar estratégia');
        this.loadingDetail.set(false);
      },
    });
  }

  run(): void {
    const bundle = this.selected();
    if (!bundle) return;
    this.running.set(true);
    this.api.runStrategy(bundle.strategy.id).subscribe({
      next: () => {
        this.running.set(false);
        this.select(bundle.strategy); // recarrega health/scores/backtests
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao executar estratégia');
        this.running.set(false);
      },
    });
  }

  toggleFavorite(s: Strategy, event: Event): void {
    event.stopPropagation();
    this.api.updateStrategyFlags(s.id, { favorite: !s.favorite }).subscribe({
      next: updated => {
        this.strategies.set(this.strategies().map(x => (x.id === updated.id ? updated : x)));
        const sel = this.selected();
        if (sel?.strategy.id === updated.id) this.selected.set({ ...sel, strategy: updated });
      },
    });
  }

  toggleActive(): void {
    const bundle = this.selected();
    if (!bundle) return;
    const s = bundle.strategy;
    this.api.updateStrategyFlags(s.id, { active: !s.active }).subscribe({
      next: updated => {
        this.selected.set({ ...bundle, strategy: updated });
        this.strategies.set(this.strategies().map(x => (x.id === updated.id ? updated : x)));
      },
    });
  }

  remove(): void {
    const bundle = this.selected();
    if (!bundle || !confirm(`Excluir a estratégia "${bundle.strategy.name}"?`)) return;
    this.api.deleteStrategy(bundle.strategy.id).subscribe({
      next: () => {
        this.selected.set(null);
        this.reload();
      },
      error: err => this.error.set(err?.error?.error ?? 'Erro ao excluir'),
    });
  }

  isOwner(s: Strategy): boolean {
    return s.origin === 'user';
  }

  // ---- helpers de exibição --------------------------------------------------

  stageLabel(stage?: string): string {
    return (stage && this.stageLabels[stage]) || '—';
  }

  scoreColor(v?: number | null): string {
    if (v == null) return 'text-slate-400';
    if (v >= 65) return 'text-cornerlab-primary';
    if (v >= 45) return 'text-amber-400';
    return 'text-red-400';
  }

  healthColor(v?: number | null): string {
    if (v == null) return 'text-slate-400';
    if (v > 55) return 'text-cornerlab-primary';
    if (v >= 45) return 'text-slate-200';
    return 'text-red-400';
  }

  // resumo legível da definition (JSON do Simulador) para o card.
  definitionSummary(s: Strategy): string {
    try {
      const d = JSON.parse(s.definition);
      const metricLabels: Record<string, string> = {
        corners: 'Escanteios', goals: 'Gols', offsides: 'Impedimentos',
        shots: 'Chutes', shots_on_target: 'Chutes no gol',
      };
      const parts: string[] = [metricLabels[d.metric] ?? 'Escanteios'];
      const th = d.corners_threshold || d.goals_threshold || d.offsides_threshold || d.shots_threshold || d.shots_on_target_threshold;
      if (th != null) parts.push(`acima de ${th}`);
      if (d.home_away === 'home') parts.push('em casa');
      if (d.home_away === 'away') parts.push('fora');
      if (d.last_n_games) parts.push(`últimos ${d.last_n_games}`);
      return parts.join(' · ');
    } catch {
      return '';
    }
  }

  variationEntries(variation?: string): { label: string; value: number }[] {
    if (!variation) return [];
    try {
      const v = JSON.parse(variation);
      const labels: Record<string, string> = {
        roi: 'ROI', ev: 'EV', drawdown: 'Drawdown', consistency: 'Consistência',
      };
      return Object.entries(v).map(([k, val]) => ({ label: labels[k] ?? k, value: Number(val) }));
    } catch {
      return [];
    }
  }

  winRate(b: { games: number; wins: number }): string {
    if (!b.games) return '—';
    return ((b.wins / b.games) * 100).toFixed(1) + '%';
  }
}
