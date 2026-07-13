import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatButtonToggleModule } from '@angular/material/button-toggle';

import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { BankrollCriteria, BankrollHistoryEntry, BankrollPhase, BankrollStatus } from '../../core/models';

type Section = 'dashboard' | 'phases' | 'criteria' | 'history';

interface PhaseDraft {
  sequence: number;
  name: string;
  amount: number;
}

@Component({
  selector: 'app-bankroll',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    MatCheckboxModule,
    MatProgressSpinnerModule,
    MatTableModule,
    MatTooltipModule,
    MatButtonToggleModule,
  ],
  templateUrl: './bankroll.component.html',
})
export class BankrollComponent implements OnInit {
  section = signal<Section>('dashboard');

  // Autenticação mínima (o módulo é por usuário — usa as apostas já registradas)
  loginMode = signal<'login' | 'register'>('login');
  authLoading = signal(false);
  authError = signal<string | null>(null);
  loginEmail = '';
  loginPassword = '';
  registerName = '';
  registerEmail = '';
  registerPassword = '';

  // Dashboard
  status = signal<BankrollStatus | null>(null);
  statusLoading = signal(false);
  statusError = signal<string | null>(null);

  confirmingPromote = signal(false);
  confirmingDemote = signal(false);
  promoteNotes = '';
  demoteReason = '';
  demoteNotes = '';
  actionLoading = signal(false);
  actionError = signal<string | null>(null);

  // Fases
  phasesDraft: PhaseDraft[] = [];
  phasesLoading = signal(false);
  phasesSaving = signal(false);
  phasesError = signal<string | null>(null);

  // Critérios
  criteriaDraft: BankrollCriteria | null = null;
  criteriaLoading = signal(false);
  criteriaSaving = signal(false);
  criteriaError = signal<string | null>(null);

  // Histórico
  history = signal<BankrollHistoryEntry[]>([]);
  historyLoading = signal(false);
  historyColumns = ['created_at', 'direction', 'from_amount', 'to_amount', 'reason'];

  readonly smeTooltip =
    'Score de Maturidade da Estratégia (0-100): combina Win Rate (25%), ROI (20%), Yield (15%), Drawdown (15%), tamanho da amostra (15%) e consistência mensal (10%) — quanto mais alto, mais objetivamente pronta a estratégia está para evoluir de banca.';

  constructor(private api: ApiService, public auth: AuthService) {}

  ngOnInit(): void {
    if (this.auth.isAuthenticated()) {
      this.loadStatus();
    }
  }

  // --- Autenticação ---

  submitLogin(): void {
    this.authLoading.set(true);
    this.authError.set(null);
    this.auth.login(this.loginEmail, this.loginPassword).subscribe({
      next: () => {
        this.authLoading.set(false);
        this.loadStatus();
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
        this.loadStatus();
      },
      error: err => {
        this.authLoading.set(false);
        this.authError.set(err?.error?.error ?? 'Não foi possível criar a conta');
      },
    });
  }

  logout(): void {
    this.auth.logout();
    this.status.set(null);
  }

  // --- Dashboard ---

  loadStatus(): void {
    this.statusLoading.set(true);
    this.statusError.set(null);
    this.api.getBankrollStatus().subscribe({
      next: s => {
        this.status.set(s);
        this.statusLoading.set(false);
      },
      error: err => {
        this.statusError.set(err?.error?.error ?? 'Erro ao carregar o status da banca');
        this.statusLoading.set(false);
      },
    });
  }

  confirmPromote(): void {
    this.actionLoading.set(true);
    this.actionError.set(null);
    this.api.promoteBankroll(this.promoteNotes).subscribe({
      next: () => {
        this.actionLoading.set(false);
        this.confirmingPromote.set(false);
        this.promoteNotes = '';
        this.loadStatus();
      },
      error: err => {
        this.actionLoading.set(false);
        this.actionError.set(err?.error?.error ?? 'Não foi possível confirmar a evolução');
      },
    });
  }

  confirmDemote(): void {
    this.actionLoading.set(true);
    this.actionError.set(null);
    this.api.demoteBankroll(this.demoteReason, this.demoteNotes).subscribe({
      next: () => {
        this.actionLoading.set(false);
        this.confirmingDemote.set(false);
        this.demoteReason = '';
        this.demoteNotes = '';
        this.loadStatus();
      },
      error: err => {
        this.actionLoading.set(false);
        this.actionError.set(err?.error?.error ?? 'Não foi possível confirmar o rebaixamento');
      },
    });
  }

  // --- Navegação entre seções (carrega sob demanda) ---

  goTo(section: Section): void {
    this.section.set(section);
    if (section === 'phases' && this.phasesDraft.length === 0) this.loadPhases();
    if (section === 'criteria' && !this.criteriaDraft) this.loadCriteria();
    if (section === 'history' && this.history().length === 0) this.loadHistory();
  }

  // --- Fases ---

  loadPhases(): void {
    this.phasesLoading.set(true);
    this.phasesError.set(null);
    this.api.getBankrollPhases().subscribe({
      next: res => {
        this.phasesDraft = res.phases.map(p => ({ sequence: p.sequence, name: p.name, amount: p.amount }));
        this.phasesLoading.set(false);
      },
      error: err => {
        this.phasesError.set(err?.error?.error ?? 'Erro ao carregar as fases');
        this.phasesLoading.set(false);
      },
    });
  }

  addPhase(): void {
    const nextSeq = this.phasesDraft.length ? Math.max(...this.phasesDraft.map(p => p.sequence)) + 1 : 1;
    this.phasesDraft = [...this.phasesDraft, { sequence: nextSeq, name: `Fase ${nextSeq}`, amount: 0 }];
  }

  removePhase(index: number): void {
    this.phasesDraft = this.phasesDraft.filter((_, i) => i !== index);
  }

  savePhases(): void {
    this.phasesSaving.set(true);
    this.phasesError.set(null);
    this.api.setBankrollPhases(this.phasesDraft).subscribe({
      next: res => {
        this.phasesDraft = res.phases.map(p => ({ sequence: p.sequence, name: p.name, amount: p.amount }));
        this.phasesSaving.set(false);
        this.loadStatus();
      },
      error: err => {
        this.phasesError.set(err?.error?.error ?? 'Erro ao salvar as fases');
        this.phasesSaving.set(false);
      },
    });
  }

  // --- Critérios ---

  loadCriteria(): void {
    this.criteriaLoading.set(true);
    this.criteriaError.set(null);
    this.api.getBankrollCriteria().subscribe({
      next: c => {
        this.criteriaDraft = c;
        this.criteriaLoading.set(false);
      },
      error: err => {
        this.criteriaError.set(err?.error?.error ?? 'Erro ao carregar os critérios');
        this.criteriaLoading.set(false);
      },
    });
  }

  saveCriteria(): void {
    if (!this.criteriaDraft) return;
    this.criteriaSaving.set(true);
    this.criteriaError.set(null);
    this.api.setBankrollCriteria(this.criteriaDraft).subscribe({
      next: c => {
        this.criteriaDraft = c;
        this.criteriaSaving.set(false);
        this.loadStatus();
      },
      error: err => {
        this.criteriaError.set(err?.error?.error ?? 'Erro ao salvar os critérios');
        this.criteriaSaving.set(false);
      },
    });
  }

  // --- Histórico ---

  loadHistory(): void {
    this.historyLoading.set(true);
    this.api.getBankrollHistory().subscribe({
      next: res => {
        this.history.set(res.history);
        this.historyLoading.set(false);
      },
      error: () => this.historyLoading.set(false),
    });
  }

  stars(n: number): number[] {
    return Array.from({ length: 5 }, (_, i) => (i < n ? 1 : 0));
  }
}
