// cmd/findleague pergunta ao provedor qual é o external_id de um campeonato.
// NÃO escreve nada — nem no banco, nem em lugar nenhum. Só lê e imprime.
//
// POR QUE EXISTE. A Champions League "não carregava" porque nunca foi cadastrada,
// e cadastrar exige saber o id do campeonato no provedor. Descobrir esse número
// dependia de abrir o painel da API-Football e procurar na mão — ou de chutar.
// Chutar é a pior opção: um id errado não deixa a liga vazia, ele carrega dados
// de OUTRA competição com o nome que você escolheu, e ninguém percebe.
//
// Aqui o número vem de quem tem autoridade sobre ele: o próprio provedor.
//
// Uso:
//
//	go run ./cmd/findleague -name "Europa League"
//	go run ./cmd/findleague -name "Champions League"
//	go run ./cmd/findleague -name "Serie A" -country Italy
//
// O parâmetro -country é opcional e serve para desambiguar ("Serie A" existe no
// Brasil e na Itália). Competições continentais (Champions, Europa League,
// Libertadores) normalmente não têm país — deixe -country vazio.
//
// A saída já traz o SQL de cadastro pronto, com o id preenchido.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devdsfr/cornerlab/internal/integration/sportsdata/apifootball"
	"github.com/devdsfr/cornerlab/pkg/config"
)

func main() {
	name := flag.String("name", "", "nome (ou parte do nome) do campeonato — obrigatório")
	country := flag.String("country", "", "país, para desambiguar. Vazio = qualquer")
	season := flag.Int("season", 0, "ano da temporada a sugerir no SQL (0 = não sugerir)")
	flag.Parse()

	if *name == "" {
		fmt.Fprintln(os.Stderr, "informe -name (ex: -name \"Europa League\")")
		flag.Usage()
		os.Exit(2)
	}

	cfg := config.Load()
	if cfg.APIFootballKey == "" {
		log.Fatal("API_FOOTBALL_KEY não configurada — sem chave não há como consultar o provedor")
	}

	// recorder nil: consulta pontual e manual, não faz parte do consumo
	// monitorado dos workers.
	client := apifootball.New(cfg.APIFootballKey, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	comps, err := client.SyncCompetitions(ctx, *name, *country)
	if err != nil {
		log.Fatalf("erro ao consultar o provedor: %v", err)
	}
	if len(comps) == 0 {
		fmt.Printf("Nenhum campeonato encontrado para name=%q country=%q.\n", *name, *country)
		fmt.Println("Tente um trecho menor do nome, ou remova o -country.")
		return
	}

	fmt.Printf("\n%d resultado(s) para name=%q country=%q:\n\n", len(comps), *name, *country)
	fmt.Printf("  %-10s  %-45s  %s\n", "EXTERNAL_ID", "NOME NO PROVEDOR", "PAÍS")
	fmt.Printf("  %-10s  %-45s  %s\n", "----------", "---------------------------------------------", "----")
	for _, c := range comps {
		fmt.Printf("  %-10s  %-45s  %s\n", c.ExternalID, c.Name, c.Country)
	}

	if len(comps) > 1 {
		fmt.Println("\nMais de um resultado: confira o nome e o país antes de escolher.")
	}

	// SQL pronto só quando o resultado é inequívoco. Com vários candidatos, quem
	// escolhe é a pessoa — imprimir um SQL "provável" convidaria a colar sem ler.
	if len(comps) == 1 && *season > 0 {
		c := comps[0]
		fmt.Printf(`
SQL de cadastro (confira o nome e o país acima antes de rodar):

WITH nova AS (
    INSERT INTO leagues (external_id, name, country, tier)
    VALUES ('%s', '%s', '%s', '1')
    ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name
    RETURNING id
)
INSERT INTO seasons (league_id, year, label)
SELECT id, %d, '%d' FROM nova
ON CONFLICT (league_id, year) DO NOTHING;

Depois do próximo ciclo do worker, confirme que os clubes que apareceram são
mesmo dessa competição — é a prova definitiva de que o id está certo:

SELECT t.name FROM teams t
  JOIN league_teams lt ON lt.team_id = t.id
  JOIN leagues l ON l.id = lt.league_id
 WHERE l.external_id = '%s' ORDER BY t.name;
`, c.ExternalID, c.Name, c.Country, *season, *season, c.ExternalID)
	}
}
