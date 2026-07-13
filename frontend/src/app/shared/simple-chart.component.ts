import { AfterViewInit, Component, ElementRef, Input, NgZone, OnChanges, OnDestroy, SimpleChanges, ViewChild } from '@angular/core';
import Chart from 'chart.js/auto';

// Componente utilitário para renderizar gráficos de linha/barra a partir de dados
// simples (rótulos + valores), usado no Dashboard (tendência) e Comparador.
//
// Nota sobre um bug corrigido aqui: o Chart.js usa requestAnimationFrame para
// desenhar e animar. Se isso rodar dentro da zone do Angular, cada frame de
// animação dispara um novo ciclo de change detection; se o template do
// consumidor passar arrays/objetos literais novos a cada ciclo (ex.:
// `[datasets]="[{...}]"`), o Angular entende isso como uma mudança de input e
// chama ngOnChanges de novo, que destrói e recria o canvas no meio da
// animação — o resultado observado era um canvas presente no DOM, do tamanho
// certo, mas completamente em branco. A correção tem duas partes: 1) criar e
// atualizar o gráfico fora da Angular zone (para o próprio Chart.js não gerar
// mais ciclos de CD), e 2) os componentes que usam <cl-simple-chart> agora
// calculam os inputs uma única vez (ao chegar o resultado), em vez de recriar
// arrays a cada render do template.
@Component({
  selector: 'cl-simple-chart',
  standalone: true,
  template: `<canvas #canvasRef></canvas>`,
  styles: [':host { display:block; height: 220px; width: 100%; }', 'canvas { max-height: 220px; }'],
})
export class SimpleChartComponent implements AfterViewInit, OnChanges, OnDestroy {
  @Input() labels: (string | number)[] = [];
  @Input() datasets: { label: string; data: number[]; color?: string }[] = [];
  @Input() type: 'line' | 'bar' = 'line';

  @ViewChild('canvasRef') canvasRef!: ElementRef<HTMLCanvasElement>;
  private chart?: Chart;
  private viewReady = false;

  constructor(private zone: NgZone) {}

  ngAfterViewInit(): void {
    this.viewReady = true;
    this.render();
  }

  ngOnChanges(_: SimpleChanges): void {
    if (this.viewReady) this.render();
  }

  ngOnDestroy(): void {
    this.zone.runOutsideAngular(() => this.chart?.destroy());
  }

  private render(): void {
    if (!this.canvasRef) return;
    // Executa fora da zone: evita que o loop de animação/resize do Chart.js
    // dispare change detection do Angular (e um novo destroy+create em cascata).
    this.zone.runOutsideAngular(() => {
      this.chart?.destroy();
      const palette = ['#22c55e', '#38bdf8', '#f97316', '#a855f7'];
      this.chart = new Chart(this.canvasRef.nativeElement, {
        type: this.type,
        data: {
          labels: this.labels,
          datasets: this.datasets.map((d, i) => ({
            label: d.label,
            data: d.data,
            borderColor: d.color ?? palette[i % palette.length],
            backgroundColor: (d.color ?? palette[i % palette.length]) + (this.type === 'bar' ? 'cc' : '33'),
            tension: 0.3,
            fill: this.type === 'line',
          })),
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { labels: { color: '#cbd5e1' } },
          },
          scales: {
            x: { ticks: { color: '#94a3b8' }, grid: { color: '#1f2937' } },
            y: { ticks: { color: '#94a3b8' }, grid: { color: '#1f2937' } },
          },
        },
      });
    });
  }
}
