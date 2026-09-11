import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';

// Página "Como Funciona" — explicação do CornerLab para quem não é da área.
//
// REGRA DESTA PÁGINA: ela explica o método, não vende resultado. Todo texto aqui
// descreve o que o sistema FAZ com dados passados. Nada promete lucro, nada
// sugere aposta, nada projeta futuro. Se um dia alguém for editar este arquivo,
// esse é o limite a respeitar — é o mesmo princípio que vale para o texto gerado
// automaticamente nas descobertas (ver describe() em usecase/discovery/engine.go).
//
// A página também assume uma coisa incomum: ela conta o que o sistema NÃO sabe.
// A seção "limites" existe de propósito e não deve ser removida para a página
// ficar mais bonita. Um usuário que entende as limitações usa a ferramenta
// melhor do que um que acha que ela adivinha resultado.
@Component({
  selector: 'app-how-it-works',
  standalone: true,
  imports: [CommonModule, RouterLink, MatIconModule],
  templateUrl: './how-it-works.component.html',
})
export class HowItWorksComponent {
  // Os quatro passos do caminho que um dado percorre, do jogo até a tela.
  readonly steps = [
    {
      n: 1,
      icon: 'cloud_download',
      title: 'O jogo acontece e os números são coletados',
      text:
        'Quando uma partida termina, um provedor de dados esportivos registra o que aconteceu: quantos escanteios cada time bateu, gols, chutes, posse de bola, cartões. O CornerLab busca essas informações automaticamente, algumas vezes por dia, e guarda cada partida no seu banco de dados.',
      detail:
        'Nada aqui é digitado à mão nem estimado. Se o provedor não tem o dado de uma partida, ela fica sem esse dado — o sistema não inventa um valor para preencher o buraco.',
    },
    {
      n: 2,
      icon: 'functions',
      title: 'Os números viram estatística',
      text:
        'Com o histórico guardado, o sistema calcula médias e frequências: quantos escanteios um time costuma ter, com que frequência a partida passa de 8 escanteios, se o número muda quando ele joga em casa ou fora, se vem subindo ou caindo nos últimos jogos.',
      detail:
        'Frequência não é previsão. "Passou de 8 escanteios em 7 de cada 10 jogos" descreve o que já aconteceu. O próximo jogo pode ser o terceiro.',
    },
    {
      n: 3,
      icon: 'history',
      title: 'Você monta uma regra e vê como ela teria se saído',
      text:
        'No Simulador de Filtros você define um critério — por exemplo: "jogos do Brasileirão, time jogando em casa, mais de 8 escanteios no total". O sistema então procura no histórico todas as partidas que se encaixam e mostra quantas vezes aquilo aconteceu, e quanto a regra teria rendido ou perdido.',
      detail:
        'Isso se chama backtest: rodar a regra no passado. É a única forma honesta de avaliar um critério sem arriscar dinheiro — e continua sendo passado, não garantia.',
    },
    {
      n: 4,
      icon: 'travel_explore',
      title: 'O sistema testa milhares de regras sozinho',
      text:
        'Em vez de esperar você pensar em todas as combinações possíveis, o CornerLab monta centenas delas automaticamente e testa cada uma no histórico. As que passam em todos os critérios de qualidade aparecem na tela de Descobertas.',
      detail:
        'É aqui que mora o maior risco de se enganar, e é aqui que o sistema é mais rigoroso. A seção seguinte explica por quê.',
    },
  ];

  // Por que testar muita coisa é perigoso, e o que o sistema faz a respeito.
  // Este bloco é o coração da página: é o que separa uma ferramenta de análise
  // de um gerador de coincidências.
  readonly rigor = [
    {
      icon: 'casino',
      title: 'O problema de testar muita coisa',
      text:
        'Jogue uma moeda 10 vezes e é raro sair cara nas 10. Agora peça para mil pessoas jogarem: alguém vai conseguir. Essa pessoa não tem uma moeda especial — ela teve sorte, e só apareceu porque muita gente tentou.',
      text2:
        'Testar centenas de regras contra o mesmo histórico tem exatamente esse efeito. Sempre vai existir uma combinação que parece excelente por acaso. Um sistema que simplesmente mostra "a melhor" está mostrando sorte e chamando de descoberta.',
    },
    {
      icon: 'content_cut',
      title: 'Primeira trava: metade do histórico fica escondida',
      text:
        'O CornerLab divide o histórico de cada campeonato em duas partes por data. A busca só enxerga a parte mais antiga. A parte mais recente fica guardada, intocada.',
      text2:
        'Quando uma regra passa na primeira parte, ela é testada de novo na segunda — um período que a busca nunca viu. Padrão que existe só por sorte não tem motivo para se repetir num período que não participou da procura. É o mesmo que estudar com uma lista de exercícios e fazer a prova com outra.',
    },
    {
      icon: 'balance',
      title: 'Segunda trava: acertar muito não é o mesmo que ter vantagem',
      text:
        'Uma regra que acerta 90% parece ótima. Mas se a odd paga apenas 1,05, acertar 90% dá prejuízo. A pergunta certa não é "acerta muito?", e sim "acerta mais do que a odd já estava dizendo?".',
      text2:
        'O sistema compara a taxa de acerto observada com a probabilidade que a própria odd embutia, e calcula a chance daquele resultado ter saído por acaso. Quanto mais combinações foram testadas, mais exigente esse limite fica — testar mais passa a custar mais caro, em vez de aumentar a chance de achar sorte.',
    },
  ];

  // O que o sistema não faz. Fica numa seção própria, com destaque, porque é
  // informação que o usuário precisa ANTES de confiar em qualquer número.
  readonly limits = [
    {
      icon: 'block',
      title: 'Não prevê resultado',
      text:
        'Nenhum número desta plataforma diz o que vai acontecer no próximo jogo. Tudo aqui descreve o que já aconteceu em partidas passadas. Futebol tem lesão, expulsão, chuva, time poupando jogador para a próxima rodada — nada disso está no histórico de escanteios.',
    },
    {
      icon: 'thumb_down_off_alt',
      title: 'Não recomenda aposta',
      text:
        'O CornerLab não indica em que apostar, nem quanto. Ele organiza dados e mostra o que os números dizem. A decisão, o risco e as consequências são inteiramente de quem aposta.',
    },
    {
      icon: 'inbox',
      title: 'Lista vazia é resultado válido',
      text:
        'Se a tela de Descobertas não mostrar nenhuma estratégia, o sistema está funcionando. Significa que nenhuma combinação passou nos critérios naquele momento. Mostrar algo fraco só para a tela não ficar vazia seria o comportamento errado.',
    },
    {
      icon: 'price_change',
      title: 'Odd estimada é claramente marcada',
      text:
        'Nem toda partida do histórico tem a odd que o mercado realmente oferecia. Quando a odd é uma estimativa, o resultado aparece com um aviso e os valores de retorno devem ser lidos como simulação — não como desempenho observado.',
    },
  ];

  // Glossário do mínimo necessário para ler as telas sem se perder.
  readonly glossary = [
    { term: 'Escanteio', def: 'O córner. A métrica principal do CornerLab: quase toda análise aqui gira em torno de quantos escanteios uma partida teve.' },
    { term: 'Linha', def: 'O número que serve de corte. "Linha 8,5" significa "mais de 8 escanteios" — o meio ponto existe para nunca haver empate.' },
    { term: 'Odd', def: 'Quanto se recebe por cada 1 apostado, se acertar. Odd 2,00 devolve 2 (1 de lucro). Quanto menor a odd, mais provável o mercado considera aquele resultado.' },
    { term: 'Amostra', def: 'Quantas partidas entraram na conta. Dez jogos não sustentam conclusão nenhuma; algumas centenas já dizem alguma coisa. Desconfie sempre de porcentagem com amostra pequena.' },
    { term: 'Taxa de acerto', def: 'De todas as vezes que a regra foi aplicada no histórico, em quantas o resultado saiu como ela esperava.' },
    { term: 'ROI', def: 'Retorno sobre o valor apostado, em porcentagem. ROI de 10% significa que, no período analisado, a regra teria devolvido 10% a mais do que foi colocado nela.' },
    { term: 'Backtest', def: 'Rodar uma regra no histórico para ver como ela teria se comportado. É medição do passado — não é teste do futuro.' },
    { term: 'Drawdown', def: 'A maior queda acumulada dentro do período. Mede quanto a regra chegou a estar perdendo no pior momento, mesmo que tenha terminado bem.' },
    { term: 'Fora da amostra', def: 'O teste feito num período que a busca não usou. É o que separa um padrão de verdade de uma coincidência bem apresentada.' },
  ];

  // Para onde ir depois de entender o funcionamento.
  readonly next = [
    { route: '/visao-geral', icon: 'calendar_month', label: 'Visão Geral', text: 'Os próximos jogos mapeados.' },
    { route: '/dashboard', icon: 'query_stats', label: 'Dashboard', text: 'As estatísticas de uma equipe.' },
    { route: '/filtros', icon: 'tune', label: 'Simulador', text: 'Monte uma regra e teste no histórico.' },
    { route: '/descobertas', icon: 'travel_explore', label: 'Descobertas', text: 'O que o sistema encontrou sozinho.' },
  ];
}
