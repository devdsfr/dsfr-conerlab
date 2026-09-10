// cmd/oddsprobe responde, com dados e não com suposição, se a API-Football
// serve como fonte de odds REAIS de escanteios para o CornerLab.
//
// NÃO escreve nada — nem no banco, nem em disco. Só consulta e imprime.
//
// POR QUE EXISTE. O AUD-001 mostrou que todas as odds do sistema são sintéticas,
// derivadas da média do próprio lote, e que sem odd de mercado o Discovery não
// tem como validar estratégia. A pergunta seguinte — "dá para usar a
// API-Football?" — tem três partes, e nenhuma delas se responde lendo
// documentação:
//
//  1. Existe mercado de escanteios? Se a API só cobre 1X2 e over/under de gols,
//     todo o resto é irrelevante.
//  2. Dá para preencher o passado? A documentação diz que não ("We keep a
//     7-days history"), e essa é a pergunta mais cara: se não dá, o histórico
//     de odds reais tem que ser ACUMULADO daqui pra frente, uma rodada por vez.
//  3. A cota do plano aguenta? Odds vêm paginadas, 10 por página, e o plano
//     gratuito tem teto diário.
//
// Uso:
//
//	go run ./cmd/oddsprobe
//	go run ./cmd/oddsprobe -league 71 -season 2026
//
// Requer API_FOOTBALL_KEY no ambiente (ver backend/.env.example).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devdsfr/cornerlab/pkg/config"
)

const baseURL = "https://v3.football.api-sports.io"

// entre chamadas: o plano gratuito corta em ~10 req/min (ver o comentário de
// minRequestInterval em internal/integration/sportsdata/apifootball/client.go).
const pause = 6500 * time.Millisecond

func main() {
	league := flag.Int("league", 71, "id do campeonato no provedor (71 = Brasileirão Série A)")
	season := flag.Int("season", time.Now().Year(), "temporada")
	flag.Parse()

	cfg := config.Load()
	if cfg.APIFootballKey == "" {
		log.Fatal("API_FOOTBALL_KEY não configurada — sem chave não há o que sondar")
	}
	p := &prober{key: cfg.APIFootballKey, hc: &http.Client{Timeout: 25 * time.Second}}

	fmt.Println("=== SONDAGEM DE ODDS REAIS — API-FOOTBALL ===")

	p.plano()
	corners := p.mercadoDeEscanteios()
	p.bookmakers()
	p.amostra(*league, *season)

	fmt.Println("\n=== O QUE ISSO SIGNIFICA ===")
	if !corners {
		fmt.Println(`
NÃO foi encontrado mercado de escanteios. Sem ele, integrar odds desta API não
resolve o AUD-001: daria para precificar gols, não escanteios, que é a métrica
central do CornerLab. Nesse caso a decisão vira "trocar de provedor" ou "mudar a
métrica principal", e não "implementar a integração".`)
	} else {
		fmt.Println(`
Existe mercado de escanteios. Mas repare no ponto do histórico acima: a API
mantém apenas alguns dias de odds. Não dá para preencher o passado.

Consequência prática, que precisa estar clara antes de qualquer estimativa de
prazo: o histórico de odds REAIS começa do zero no dia em que a coleta entrar no
ar e cresce uma rodada por vez. Até acumular amostra suficiente (o doc 08 exige
100 jogos por combinação), o Discovery continua publicando zero — corretamente.
As 3.547 partidas que já estão no banco NÃO ganham odd real nunca.`)
	}
	fmt.Println(`
Nada foi gravado. Esta ferramenta só lê.`)
}

type prober struct {
	key string
	hc  *http.Client
}

func (p *prober) get(path string, q map[string]string) ([]byte, error) {
	u, _ := url.Parse(baseURL + path)
	if len(q) > 0 {
		vals := u.Query()
		for k, v := range q {
			vals.Set(k, v)
		}
		u.RawQuery = vals.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apisports-key", p.key)

	resp, err := p.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("status %d", resp.StatusCode)
	}
	time.Sleep(pause)
	return body, nil
}

func (p *prober) plano() {
	fmt.Println("\n--- 1. Plano e cota (não consome a cota diária) ---")
	body, err := p.get("/status", nil)
	if err != nil {
		fmt.Printf("  ERRO: %v\n", err)
		return
	}
	var r struct {
		Response struct {
			Subscription struct {
				Plan   string `json:"plan"`
				Active bool   `json:"active"`
				End    string `json:"end"`
			} `json:"subscription"`
			Requests struct {
				Current  int `json:"current"`
				LimitDay int `json:"limit_day"`
			} `json:"requests"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Printf("  resposta inesperada: %s\n", trunc(body))
		return
	}
	s := r.Response
	fmt.Printf("  plano: %s (ativo: %v, expira: %s)\n", s.Subscription.Plan, s.Subscription.Active, s.Subscription.End)
	fmt.Printf("  uso hoje: %d de %d requisições\n", s.Requests.Current, s.Requests.LimitDay)
	if s.Requests.LimitDay > 0 && s.Requests.LimitDay <= 100 {
		fmt.Println("  ATENÇÃO: com 100 req/dia, coletar odds de todas as ligas cadastradas")
		fmt.Println("  não cabe na cota. Odds vêm paginadas (10 por página).")
	}
}

// mercadoDeEscanteios devolve true se a API oferece algum mercado de escanteios.
func (p *prober) mercadoDeEscanteios() bool {
	fmt.Println("\n--- 2. Existe mercado de escanteios? ---")
	body, err := p.get("/odds/bets", map[string]string{"search": "corner"})
	if err != nil {
		fmt.Printf("  ERRO: %v\n  %s\n", err, trunc(body))
		return false
	}
	var r struct {
		Response []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Printf("  resposta inesperada: %s\n", trunc(body))
		return false
	}
	if len(r.Response) == 0 {
		fmt.Println("  NENHUM mercado de escanteios encontrado.")
		return false
	}
	fmt.Printf("  %d mercado(s) encontrado(s):\n", len(r.Response))
	for _, b := range r.Response {
		marca := ""
		n := strings.ToLower(b.Name)
		if strings.Contains(n, "over") || strings.Contains(n, "under") {
			marca = "   <-- linha over/under: é este o formato que o CornerLab usa"
		}
		fmt.Printf("    id=%-5s %s%s\n", b.ID.String(), b.Name, marca)
	}
	return true
}

func (p *prober) bookmakers() {
	fmt.Println("\n--- 3. Casas disponíveis ---")
	body, err := p.get("/odds/bookmakers", nil)
	if err != nil {
		fmt.Printf("  ERRO: %v\n", err)
		return
	}
	var r struct {
		Response []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Printf("  resposta inesperada: %s\n", trunc(body))
		return
	}
	fmt.Printf("  %d casas. Primeiras 10:\n", len(r.Response))
	for i, b := range r.Response {
		if i >= 10 {
			break
		}
		fmt.Printf("    id=%-5s %s\n", b.ID.String(), b.Name)
	}
	fmt.Println("  A escolha da casa importa: odd de casa diferente muda o break-even")
	fmt.Println("  e, portanto, o p-valor do teste de significância (AUD-003).")
}

func (p *prober) amostra(league, season int) {
	fmt.Printf("\n--- 4. Amostra real de odds (liga %d, temporada %d) ---\n", league, season)
	body, err := p.get("/odds", map[string]string{
		"league": fmt.Sprint(league),
		"season": fmt.Sprint(season),
		"page":   "1",
	})
	if err != nil {
		fmt.Printf("  ERRO: %v\n  %s\n", err, trunc(body))
		return
	}
	var r struct {
		Results json.Number `json:"results"`
		Paging  struct {
			Current json.Number `json:"current"`
			Total   json.Number `json:"total"`
		} `json:"paging"`
		Response []struct {
			Fixture struct {
				ID   json.Number `json:"id"`
				Date string      `json:"date"`
			} `json:"fixture"`
			Update    string `json:"update"`
			Bookmaker []struct {
				Name string `json:"name"`
				Bets []struct {
					Name   string `json:"name"`
					Values []struct {
						Value string `json:"value"`
						Odd   string `json:"odd"`
					} `json:"values"`
				} `json:"bets"`
			} `json:"bookmakers"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Printf("  resposta inesperada: %s\n", trunc(body))
		return
	}

	fmt.Printf("  resultados nesta página: %s | páginas: %s\n", r.Results.String(), r.Paging.Total.String())
	if len(r.Response) == 0 {
		fmt.Println(`
  VAZIO. Com a documentação na mão, o vazio é informativo e provavelmente
  correto: a API só publica odds de 1 a 14 dias ANTES do jogo e guarda cerca de
  7 dias de histórico. Partida antiga simplesmente não tem odd para devolver.
  Tente uma liga com jogos nos próximos dias.`)
		return
	}

	for _, o := range r.Response {
		fmt.Printf("\n  fixture %s (%s) — atualizado em %s\n", o.Fixture.ID.String(), o.Fixture.Date, o.Update)
		for _, bk := range o.Bookmaker {
			for _, bet := range bk.Bets {
				if !strings.Contains(strings.ToLower(bet.Name), "corner") {
					continue
				}
				fmt.Printf("    [%s] %s\n", bk.Name, bet.Name)
				for i, v := range bet.Values {
					if i >= 6 {
						fmt.Println("      ...")
						break
					}
					fmt.Printf("      %-16s %s\n", v.Value, v.Odd)
				}
			}
		}
	}
	fmt.Println(`
  Repare na data das partidas acima: se todas forem futuras, está confirmado
  que não há como preencher o passado.`)
}

func trunc(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "..."
	}
	if s == "" {
		return "(vazio)"
	}
	return s
}
