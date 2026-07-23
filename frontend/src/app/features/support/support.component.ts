import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';

// Página de Suporte — canais de contato (WhatsApp e e-mail) + FAQ curto. Estática,
// sem backend: os links abrem o WhatsApp/cliente de e-mail do próprio usuário.
@Component({
  selector: 'app-support',
  standalone: true,
  imports: [CommonModule, MatIconModule],
  templateUrl: './support.component.html',
})
export class SupportComponent {
  // Contatos do CornerLab.
  readonly whatsappDisplay = '(62) 99434-5604';
  readonly whatsappLink =
    'https://wa.me/5562994345604?text=' +
    encodeURIComponent('Olá! Preciso de ajuda com o CornerLab.');
  readonly email = 'traderdsfr@gmail.com';
  readonly emailLink =
    'mailto:traderdsfr@gmail.com?subject=' + encodeURIComponent('Suporte CornerLab');

  // Visão geral da aplicação — o que cada área do CornerLab faz. Ícones batem com
  // os do menu principal (app.ts) para o usuário associar rápido.
  readonly features = [
    { icon: 'calendar_month', name: 'Visão Geral', text: 'Calendário dos próximos jogos já mapeados. É a tela inicial — dá uma visão rápida do que vem a seguir; clique num time para ir direto às estatísticas dele.' },
    { icon: 'query_stats', name: 'Dashboard', text: 'Estatísticas de escanteios de uma equipe: médias, frequências (acima de N), tendência, casa x fora e os últimos jogos com estatísticas detalhadas (posse, chutes, cartões etc.).' },
    { icon: 'compare_arrows', name: 'Comparador', text: 'Coloca duas equipes lado a lado para comparar os perfis de escanteios — a favor, sofridos, casa/fora e consistência.' },
    { icon: 'tune', name: 'Simulador de Filtros', text: 'Monta uma estratégia (liga, mando, limite de escanteios, período) e testa como ela teria se saído no histórico — backtesting com ROI/yield.' },
    { icon: 'account_balance_wallet', name: 'Gestão de Banca', text: 'Gestão evolutiva de banca por fases, com critérios de promoção e o registro de rodadas confirmadas (saldo real acumulado). Recurso Premium.' },
    { icon: 'trending_up', name: 'Projeções', text: 'Projeções e cenários a partir das suas estratégias. Recurso Premium.' },
    { icon: 'workspace_premium', name: 'Assinatura', text: 'Planos e assinatura Premium — libera Gestão de Banca, Projeções, alertas e exportações, e remove os anúncios.' },
    { icon: 'sync', name: 'Integrações', text: 'Status e consumo das APIs externas (API-Football etc.) e o botão "Sincronizar agora" para buscar jogos novos sem esperar o ciclo automático.' },
  ];

  readonly faq = [
    {
      q: 'De onde vêm as estatísticas?',
      a: 'Os dados de partidas e escanteios vêm da API-Football e são atualizados automaticamente. O CornerLab organiza esses dados históricos e calcula estatísticas — nunca recomenda apostas.',
    },
    {
      q: 'Uma liga ou jogo não apareceu. E agora?',
      a: 'Só exibimos ligas com dado real verificado. Se algum jogo recente ainda não consta, o ciclo de sincronização pode não tê-lo processado ainda — você pode forçar em Integrações › Sincronizar agora, ou nos avisar pelos canais acima.',
    },
    {
      q: 'Como funciona a assinatura Premium?',
      a: 'A Premium libera Gestão de Banca, Projeções, Alertas e exportações, além de remover os anúncios. Detalhes e planos na página Assinatura.',
    },
    {
      q: 'Encontrei um erro ou tenho uma sugestão.',
      a: 'Manda pra gente no WhatsApp ou e-mail — quanto mais detalhe (print, qual página, o que esperava), mais rápido a gente resolve.',
    },
  ];
}
