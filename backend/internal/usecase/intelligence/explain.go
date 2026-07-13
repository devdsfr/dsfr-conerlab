package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devdsfr/cornerlab/internal/integration/llm"
)

const explainSystemPrompt = `Você é o motor de explicações do CornerLab, uma plataforma de inteligência estatística para escanteios no futebol.

REGRAS INEGOCIÁVEIS:
1. Use exclusivamente os dados fornecidos no bloco JSON abaixo. Nunca invente números, times, jogos ou probabilidades que não estejam nesse JSON.
2. Nunca recomende apostas, nunca sugira "entrar" em um mercado, nunca diga que uma aposta "vai bater", nunca prometa lucro ou garantia de resultado.
3. Nunca use expressões como "aposte em", "recomendamos", "alta probabilidade de ganhar", "certeza", "garantido", "não pode perder".
4. Use linguagem analítica e descritiva, sempre no passado ou no presente descrevendo o que os dados mostram — nunca no futuro como previsão.
   Exemplos do tom esperado: "Nos últimos 10 jogos, a equipe realizou mais de 5 escanteios em 90% das partidas." / "A média de escanteios da equipe aumentou 22% nas últimas cinco partidas."
5. Se a pergunta pedir uma recomendação de aposta, palpite ou previsão de resultado, recuse educadamente essa parte e reformule a resposta apenas com os fatos estatísticos disponíveis, explicando que a plataforma não recomenda apostas.
6. Sempre que fizer sentido, mencione o período/amostra analisada (quantidade de jogos) para dar rastreabilidade à resposta.
7. Seja conciso: 2 a 4 frases.`

var forbiddenPhrases = []string{
	"aposte em", "aposte no", "aposte na", "recomendamos", "recomendo",
	"vai bater", "vai acontecer", "alta probabilidade de ganhar", "certeza de",
	"garantido", "garantia de", "não pode perder", "entre nesse mercado", "entre neste mercado",
}

type ExplainUsecase struct {
	llm         *llm.OpenAIClient
	consistency *ConsistencyUsecase
	trend       *TrendUsecase
	stability   *StabilityUsecase
	score       *ScoreUsecase
	opponent    *OpponentUsecase
}

func NewExplainUsecase(
	client *llm.OpenAIClient,
	consistency *ConsistencyUsecase,
	trend *TrendUsecase,
	stability *StabilityUsecase,
	score *ScoreUsecase,
	opponent *OpponentUsecase,
) *ExplainUsecase {
	return &ExplainUsecase{llm: client, consistency: consistency, trend: trend, stability: stability, score: score, opponent: opponent}
}

type ExplainResponse struct {
	Answer       string `json:"answer"`
	DataSnapshot string `json:"data_snapshot"` // JSON usado como base da resposta, para auditoria/rastreabilidade
}

// Explain responde a uma pergunta sobre uma equipe (opcionalmente contextualizada por
// um adversário) usando apenas os indicadores já calculados pelo backend. A resposta
// nunca recomenda apostas — ver explainSystemPrompt e o filtro de frases proibidas
// aplicado à resposta do modelo antes de retorná-la.
func (u *ExplainUsecase) Explain(ctx context.Context, teamID, leagueID int64, opponentID *int64, question string, limit int) (*ExplainResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	consistencyReport, err := u.consistency.Compute(ctx, teamID, leagueID, nil, limit)
	if err != nil {
		return nil, err
	}
	trendReport, err := u.trend.Compute(ctx, teamID, leagueID, limit/2, limit)
	if err != nil {
		return nil, err
	}
	stabilityReport, err := u.stability.Compute(ctx, teamID, leagueID, limit)
	if err != nil {
		return nil, err
	}
	scoreReport, err := u.score.Compute(ctx, teamID, leagueID, opponentID, limit)
	if err != nil {
		return nil, err
	}

	dataBundle := map[string]any{
		"consistency": consistencyReport,
		"trend":       trendReport,
		"stability":   stabilityReport,
		"score":       scoreReport,
	}
	if opponentID != nil {
		opponentReport, err := u.opponent.Compute(ctx, *opponentID, leagueID, nil, limit)
		if err == nil {
			dataBundle["opponent"] = opponentReport
		}
	}

	dataJSON, err := json.MarshalIndent(dataBundle, "", "  ")
	if err != nil {
		return nil, err
	}

	if question == "" {
		question = fmt.Sprintf("Por que a equipe %s possui esse desempenho estatístico?", consistencyReport.TeamName)
	}

	userPrompt := fmt.Sprintf(`Dados calculados sobre a equipe (JSON):
%s

Pergunta do usuário: %s

Responda usando apenas os dados acima.`, string(dataJSON), question)

	answer, err := u.llm.Complete(ctx, explainSystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	answer = sanitizeAnswer(answer)

	return &ExplainResponse{
		Answer:       answer,
		DataSnapshot: string(dataJSON),
	}, nil
}

// sanitizeAnswer é uma segunda camada de segurança (além do system prompt): se o
// texto gerado ainda assim contiver alguma expressão proibida, ele é substituído por
// uma mensagem segura em vez de ser exibido ao usuário.
func sanitizeAnswer(answer string) string {
	lower := strings.ToLower(answer)
	for _, phrase := range forbiddenPhrases {
		if strings.Contains(lower, phrase) {
			return "Não foi possível gerar uma explicação dentro das regras da plataforma (a resposta gerada continha linguagem de recomendação, o que não é permitido). Consulte os indicadores calculados diretamente."
		}
	}
	return answer
}
