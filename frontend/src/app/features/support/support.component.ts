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
