import { Component, OnInit, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTooltipModule } from '@angular/material/tooltip';

import { ApiService } from '../../core/api.service';
import { UpcomingMatch } from '../../core/models';

interface CalendarDay {
  date: Date;
  key: string; // YYYY-MM-DD, chave de matchesByDay
  inCurrentMonth: boolean;
  isToday: boolean;
}

const WEEKDAY_LABELS = ['Dom', 'Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb'];
const MONTH_LABELS = [
  'Janeiro', 'Fevereiro', 'Março', 'Abril', 'Maio', 'Junho',
  'Julho', 'Agosto', 'Setembro', 'Outubro', 'Novembro', 'Dezembro',
];

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// Página "Visão Geral" — tela inicial do CornerLab: um calendário com os próximos
// jogos já mapeados pelo Worker de Descoberta, para o usuário ter uma visão clara do
// que vem a seguir antes mesmo de escolher time/campeonato (pedido explícito do
// usuário: "assim eu já tenho uma visão clara de qual o próximo jogo").
@Component({
  selector: 'app-overview',
  standalone: true,
  imports: [CommonModule, MatButtonModule, MatIconModule, MatProgressSpinnerModule, MatTooltipModule],
  templateUrl: './overview.component.html',
  styleUrl: './overview.component.scss',
})
export class OverviewComponent implements OnInit {
  readonly weekdayLabels = WEEKDAY_LABELS;

  loading = signal(true);
  error = signal<string | null>(null);
  matches = signal<UpcomingMatch[]>([]);

  private today = new Date();
  currentMonth = signal(new Date(this.today.getFullYear(), this.today.getMonth(), 1));
  selectedDate = signal<Date>(new Date(this.today.getFullYear(), this.today.getMonth(), this.today.getDate()));

  // Agrupa os jogos por dia (chave local YYYY-MM-DD) uma única vez por carregamento —
  // evita recalcular a cada change detection ao navegar entre meses/dias.
  private matchesByDay = computed<Map<string, UpcomingMatch[]>>(() => {
    const map = new Map<string, UpcomingMatch[]>();
    for (const m of this.matches()) {
      const k = dayKey(new Date(m.match_date));
      const list = map.get(k) ?? [];
      list.push(m);
      map.set(k, list);
    }
    return map;
  });

  monthLabel = computed(() => {
    const m = this.currentMonth();
    return `${MONTH_LABELS[m.getMonth()]} de ${m.getFullYear()}`;
  });

  // Grade de 6 semanas (42 dias) começando no domingo anterior (ou igual) ao dia 1 do
  // mês — padrão de calendário mensal, com dias de outros meses esmaecidos.
  calendarDays = computed<CalendarDay[]>(() => {
    const monthStart = this.currentMonth();
    const gridStart = new Date(monthStart);
    gridStart.setDate(gridStart.getDate() - gridStart.getDay());

    const days: CalendarDay[] = [];
    for (let i = 0; i < 42; i++) {
      const d = new Date(gridStart);
      d.setDate(gridStart.getDate() + i);
      days.push({
        date: d,
        key: dayKey(d),
        inCurrentMonth: d.getMonth() === monthStart.getMonth(),
        isToday: dayKey(d) === dayKey(this.today),
      });
    }
    return days;
  });

  selectedDayMatches = computed<UpcomingMatch[]>(() => {
    const list = this.matchesByDay().get(dayKey(this.selectedDate())) ?? [];
    return [...list].sort((a, b) => a.match_date.localeCompare(b.match_date));
  });

  constructor(private api: ApiService, private router: Router) {}

  ngOnInit(): void {
    this.loading.set(true);
    this.api.getUpcomingMatches().subscribe({
      next: res => {
        this.matches.set(res.matches ?? []);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error ?? 'Erro ao carregar os próximos jogos');
        this.loading.set(false);
      },
    });
  }

  matchCountFor(day: CalendarDay): number {
    return this.matchesByDay().get(day.key)?.length ?? 0;
  }

  // Resumo rápido pro tooltip ao passar o mouse num dia com jogos (pedido do
  // redesign) — lista os primeiros confrontos, com "+N" se houver mais.
  daySummary(day: CalendarDay): string {
    const list = this.matchesByDay().get(day.key);
    if (!list || list.length === 0) return '';
    const sorted = [...list].sort((a, b) => a.match_date.localeCompare(b.match_date));
    const lines = sorted.slice(0, 5).map(m => `${m.home_team_name} x ${m.away_team_name}`);
    if (sorted.length > 5) lines.push(`+${sorted.length - 5} jogo(s)`);
    return lines.join('\n');
  }

  isSelected(day: CalendarDay): boolean {
    return day.key === dayKey(this.selectedDate());
  }

  selectDay(day: CalendarDay): void {
    this.selectedDate.set(day.date);
    // Navegar pra um dia de outro mês também troca o mês exibido — evita o usuário
    // clicar num dia esmaecido e não entender por que "sumiu" da grade.
    if (!day.inCurrentMonth) {
      this.currentMonth.set(new Date(day.date.getFullYear(), day.date.getMonth(), 1));
    }
  }

  prevMonth(): void {
    const m = this.currentMonth();
    this.currentMonth.set(new Date(m.getFullYear(), m.getMonth() - 1, 1));
  }

  nextMonth(): void {
    const m = this.currentMonth();
    this.currentMonth.set(new Date(m.getFullYear(), m.getMonth() + 1, 1));
  }

  goToday(): void {
    this.currentMonth.set(new Date(this.today.getFullYear(), this.today.getMonth(), 1));
    this.selectedDate.set(new Date(this.today.getFullYear(), this.today.getMonth(), this.today.getDate()));
  }

  selectedDateLabel(): string {
    const d = this.selectedDate();
    const isToday = dayKey(d) === dayKey(this.today);
    const formatted = d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric', weekday: 'long' });
    return isToday ? `Hoje, ${formatted}` : formatted;
  }

  // Clicar num time do calendário leva direto pro Dashboard já filtrado naquele
  // campeonato/equipe — o calendário é o ponto de partida, não um beco sem saída.
  openTeamDashboard(m: UpcomingMatch, teamId: number): void {
    this.router.navigate(['/dashboard'], { queryParams: { league_id: m.league_id, team_id: teamId } });
  }
}
