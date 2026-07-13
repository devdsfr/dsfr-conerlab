import { AfterViewInit, Component, ElementRef, Input, OnChanges, OnDestroy, SimpleChanges, ViewChild } from '@angular/core';
import Chart from 'chart.js/auto';

// Componente utilitário para renderizar gráficos de linha/barra a partir de dados
// simples (rótulos + valores), usado no Dashboard (tendência) e Comparador.
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

  ngAfterViewInit(): void {
    this.render();
  }

  ngOnChanges(_: SimpleChanges): void {
    this.render();
  }

  ngOnDestroy(): void {
    this.chart?.destroy();
  }

  private render(): void {
    if (!this.canvasRef) return;
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
  }
}
