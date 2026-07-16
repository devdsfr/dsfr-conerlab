import { Component, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTooltipModule } from '@angular/material/tooltip';

interface CycleStep {
  label: string;
  value: number;
}

interface ScenarioConfig {
  nome: string;
  ciclos: number;
  badge: string;
  avaliacao: string;
}

interface ScenarioResult extends ScenarioConfig {
  mensal: number;
  trimestral: number;
  semestral: number;
  anual: number;
}

// As 4 faixas de ciclos/mês (4, 8, 12 e 20) e a avaliação de plausibilidade de
// cada uma são fixas — vieram de uma análise qualitativa do usuário sobre o
// que é sustentável na prática (limite das casas, disponibilidade de jogos
// que atendam aos filtros, variação natural de resultados), não dependem do
// valor da banca. Só o lucro projetado em cima delas muda com os parâmetros.
const SCENARIOS: ScenarioConfig[] = [
  {
    nome: 'Conservador',
    ciclos: 4,
    badge: '🟢',
    avaliacao: 'Bastante plausível, desde que a estratégia seja consistente.',
  },
  {
    nome: 'Realista',
    ciclos: 8,
    badge: '🟡',
    avaliacao: 'Possível, mas exige taxa de acerto alta e bom volume de oportunidades — bom cenário para meta de longo prazo.',
  },
  {
    nome: 'Otimista',
    ciclos: 12,
    badge: '🟠',
    avaliacao: 'Bastante exigente: requer muitas oportunidades que atendam de fato aos filtros, sem forçar entradas.',
  },
  {
    nome: 'Muito agressivo',
    ciclos: 20,
    badge: '🔴',
    avaliacao: 'Muito difícil de sustentar na prática (limite das casas, variação natural, disponibilidade de jogos) — tratar apenas como projeção matemática.',
  },
];

// Módulo de Cálculo de Projeções: simula um ciclo de reinvestimento de 100%
// do lucro em N vitórias consecutivas com uma odd média fixa, realiza o lucro
// no fim do ciclo e reinicia sempre a partir da banca inicial (nunca do saldo
// acumulado) — é exatamente a lógica e a fórmula (Banca × odd^vitórias)
// especificadas pelo usuário. Tudo aqui é calculado no cliente (sem chamada
// de API): é uma calculadora de "e se", não um registro de apostas reais —
// por isso mora fora do módulo de Gestão de Banca, que é o histórico real do
// usuário.
@Component({
  selector: 'app-projections',
  standalone: true,
  imports: [CommonModule, FormsModule, MatFormFieldModule, MatInputModule, MatTooltipModule],
  templateUrl: './projections.component.html',
})
export class ProjectionsComponent {
  banca = signal(10000);
  oddMedia = signal(1.5);
  vitorias = signal(3);

  readonly explicacaoFormula =
    'A cada vitória, o valor apostado (banca + lucro acumulado do ciclo) é totalmente reinvestido na próxima. Depois da última vitória do ciclo, o lucro é realizado e o próximo ciclo sempre recomeça do zero, com a banca inicial — nunca com o saldo acumulado.';

  // Fator de crescimento do ciclo: odd^vitórias (ex.: 1,5³ = 3,375).
  fator = computed(() => Math.pow(this.oddMedia(), this.vitorias()));

  saldoFinal = computed(() => this.banca() * this.fator());
  lucroPorCiclo = computed(() => this.saldoFinal() - this.banca());
  capitalRisco = computed(() => this.banca());

  evolucao = computed<CycleStep[]>(() => {
    const steps: CycleStep[] = [{ label: 'Inicial', value: this.banca() }];
    let valor = this.banca();
    for (let i = 1; i <= this.vitorias(); i++) {
      valor *= this.oddMedia();
      steps.push({ label: `${i}ª vitória`, value: valor });
    }
    return steps;
  });

  cenarios = computed<ScenarioResult[]>(() => {
    const lucro = this.lucroPorCiclo();
    return SCENARIOS.map(s => ({
      ...s,
      mensal: lucro * s.ciclos,
      trimestral: lucro * s.ciclos * 3,
      semestral: lucro * s.ciclos * 6,
      anual: lucro * s.ciclos * 12,
    }));
  });

  // Inputs inválidos (banca <= 0, odd <= 1, vitórias < 1) quebrariam a lógica
  // de crescimento composto — em vez de deixar o template mostrar NaN/valores
  // negativos sem explicação, o card de resultado mostra um aviso claro.
  entradasValidas = computed(
    () => this.banca() > 0 && this.oddMedia() > 1 && Number.isInteger(this.vitorias()) && this.vitorias() >= 1,
  );
}
