package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/usecase/statsync"
)

// Resultados parciais, como os que um ciclo produz antes de ser interrompido.
func discoveryParcial(upserted int) statsync.DiscoveryResult {
	return statsync.DiscoveryResult{
		Targets: 12, FixturesFound: upserted, FixturesUpserted: upserted,
	}
}

func updateParcial(checked, finalized int) statsync.UpdateResult {
	return statsync.UpdateResult{Checked: checked, Finalized: finalized}
}

// P0 — INTEGRIDADE E SINCRONIZAÇÃO.
//
// Estes testes existem por causa de duas falhas observadas em PRODUÇÃO, não por
// hipótese:
//
//  1. O Cron Job nunca existiu no Render. Todas as 17 linhas de sync_runs em
//     12/09/2026 tinham triggered_by = 'manual'. A tela, que mostrava só o ciclo
//     mais recente de qualquer origem, nunca teve como revelar isso — bastava um
//     clique em "Sincronizar agora" para a linha parecer saudável.
//
//  2. O ciclo manual de 12/09 ~17:00 morreu com "context deadline exceeded".
//     api_usage_log registrou as 17 chamadas falhas; sync_runs não registrou
//     linha nenhuma. A tela continuou anunciando "última sincronização: 11/09
//     22:40". Falhar deixava o sistema com aparência MELHOR do que rodar mal.

// repoFake implementa repository.SyncRunRepository em memória.
type repoFake struct {
	runs []*domain.SyncRun
	err  error
}

func (r *repoFake) AddRun(_ context.Context, e *domain.SyncRun) error {
	if r.err != nil {
		return r.err
	}
	cp := *e
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	r.runs = append(r.runs, &cp)
	return nil
}

func (r *repoFake) LastRun(context.Context) (*domain.SyncRun, error) {
	if len(r.runs) == 0 {
		return nil, nil
	}
	return r.runs[len(r.runs)-1], nil
}

func (r *repoFake) LastSuccessfulRun(context.Context) (*domain.SyncRun, error) {
	for i := len(r.runs) - 1; i >= 0; i-- {
		e := r.runs[i]
		if e.Status == domain.SyncStatusSuccess && e.Errors == 0 &&
			(e.FixturesUpserted > 0 || e.MatchesFinalized > 0) {
			return e, nil
		}
	}
	return nil, nil
}

func (r *repoFake) LastRunBySource(_ context.Context, triggeredBy string) (*domain.SyncRun, error) {
	for i := len(r.runs) - 1; i >= 0; i-- {
		if r.runs[i].TriggeredBy == triggeredBy {
			return r.runs[i], nil
		}
	}
	return nil, nil
}

func (r *repoFake) LastProviderError(context.Context) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func chamarStatus(t *testing.T, repo *repoFake) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := &SyncHandler{runs: repo}
	router := gin.New()
	router.GET("/sync/status", h.Status)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sync/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status HTTP = %d, esperado 200. corpo: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	return body
}

// Reproduz a situação real de 12/09: vários ciclos manuais, nenhum automático.
// A resposta precisa dizer isso explicitamente.
func TestStatusRevelaQueOCicloAutomaticoNuncaRodou(t *testing.T) {
	repo := &repoFake{}
	for i := 0; i < 5; i++ {
		_ = repo.AddRun(context.Background(), &domain.SyncRun{
			TriggeredBy:      domain.SyncTriggerManual,
			Status:           domain.SyncStatusSuccess,
			FixturesUpserted: 100,
			MatchesFinalized: 40,
		})
	}

	body := chamarStatus(t, repo)

	if body["last_cron_run"] != nil {
		t.Errorf("last_cron_run = %v, esperado null: nenhum ciclo automático foi registrado", body["last_cron_run"])
	}
	if body["cron_never_ran"] != true {
		t.Errorf("cron_never_ran = %v, esperado true", body["cron_never_ran"])
	}
	if body["cron_stale"] != true {
		t.Errorf("cron_stale = %v, esperado true: nunca ter rodado é o pior caso, não o melhor", body["cron_stale"])
	}
	if body["last_manual_run"] == nil {
		t.Error("last_manual_run = null, mas cinco ciclos manuais foram registrados")
	}

	// O ponto do teste: ANTES desta correção o endpoint só devolvia last_run, que
	// aqui apontaria para um ciclo manual saudável. A ausência do automático era
	// indistinguível de um automático em dia.
	if _, ok := body["last_run"]; !ok {
		t.Error("last_run sumiu da resposta — a correção não pode remover o que já existia")
	}
}

// Um ciclo automático recente e saudável não pode ser marcado como atrasado.
func TestStatusNaoAlarmaComCicloAutomaticoRecente(t *testing.T) {
	repo := &repoFake{}
	_ = repo.AddRun(context.Background(), &domain.SyncRun{
		TriggeredBy:      domain.SyncTriggerCron,
		Status:           domain.SyncStatusSuccess,
		FixturesUpserted: 37,
		MatchesFinalized: 82,
	})

	body := chamarStatus(t, repo)

	if body["cron_stale"] != false {
		t.Errorf("cron_stale = %v, esperado false para um ciclo de agora", body["cron_stale"])
	}
	if _, presente := body["cron_never_ran"]; presente {
		t.Error("cron_never_ran não deveria aparecer quando existe ciclo automático registrado")
	}
}

// Um ciclo automático antigo precisa acender o alerta — e o limite do automático
// (36h) é mais curto que o geral (48h) de propósito: o cron roda todo dia.
func TestStatusMarcaCicloAutomaticoAtrasado(t *testing.T) {
	repo := &repoFake{}
	antigo := &domain.SyncRun{
		TriggeredBy:      domain.SyncTriggerCron,
		Status:           domain.SyncStatusSuccess,
		FixturesUpserted: 10,
		MatchesFinalized: 5,
		CreatedAt:        time.Now().Add(-40 * time.Hour),
	}
	repo.runs = append(repo.runs, antigo)

	body := chamarStatus(t, repo)

	if body["cron_stale"] != true {
		t.Errorf("cron_stale = %v, esperado true para ciclo de 40h atrás (limite = %dh)",
			body["cron_stale"], cronStaleAfterHours)
	}
	horas, ok := body["hours_since_cron"].(float64)
	if !ok || horas < 39 || horas > 41 {
		t.Errorf("hours_since_cron = %v, esperado ~40", body["hours_since_cron"])
	}
}

// O caso central da correção: um ciclo que FALHA precisa virar linha em
// sync_runs, com status e motivo. Sem isto o ciclo some e a tela mostra a
// sincronização anterior como se fosse a última — foi o que aconteceu em 12/09.
func TestCicloInterrompidoEhRegistradoComoFalha(t *testing.T) {
	repo := &repoFake{}
	h := &SyncHandler{runs: repo}

	h.recordFailedRun(context.Background(), domain.SyncTriggerManual,
		discoveryParcial(3712), updateParcial(50, 33),
		1_200_000, "atualização falhou: context deadline exceeded")

	if len(repo.runs) != 1 {
		t.Fatalf("ciclos registrados = %d, esperado 1. Um ciclo que falha NÃO pode sumir", len(repo.runs))
	}
	e := repo.runs[0]

	if e.Status != domain.SyncStatusFailed {
		t.Errorf("status = %q, esperado %q", e.Status, domain.SyncStatusFailed)
	}
	if e.ErrorMessage == "" {
		t.Error("error_message vazio: sem o motivo, a linha diz que falhou mas não diz por quê")
	}
	// Os números parciais precisam sobreviver: "morreu depois de gravar 3.712" e
	// "morreu sem fazer nada" apontam para causas diferentes.
	if e.FixturesUpserted != 3712 {
		t.Errorf("fixtures_upserted = %d, esperado 3712 (o que o ciclo alcançou antes de morrer)", e.FixturesUpserted)
	}
	if e.MatchesFinalized != 33 {
		t.Errorf("matches_finalized = %d, esperado 33", e.MatchesFinalized)
	}
}

// Um ciclo que falhou não pode ser confundido com o último ciclo saudável, mesmo
// tendo gravado partidas antes de morrer.
func TestCicloQueFalhouNaoContaComoBemSucedido(t *testing.T) {
	repo := &repoFake{}
	bom := &domain.SyncRun{
		TriggeredBy: domain.SyncTriggerCron, Status: domain.SyncStatusSuccess,
		FixturesUpserted: 20, MatchesFinalized: 20,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	ruim := &domain.SyncRun{
		TriggeredBy: domain.SyncTriggerManual, Status: domain.SyncStatusFailed,
		FixturesUpserted: 3712, MatchesFinalized: 33,
		ErrorMessage: "context deadline exceeded",
		CreatedAt:    time.Now(),
	}
	repo.runs = append(repo.runs, bom, ruim)

	body := chamarStatus(t, repo)

	ultimo, _ := body["last_run"].(map[string]any)
	if ultimo == nil || ultimo["status"] != domain.SyncStatusFailed {
		t.Errorf("last_run deveria ser o ciclo interrompido, veio %v", body["last_run"])
	}

	ok, _ := body["last_successful_run"].(map[string]any)
	if ok == nil {
		t.Fatal("last_successful_run = null, mas existe um ciclo bem-sucedido registrado")
	}
	if ok["status"] != domain.SyncStatusSuccess {
		t.Errorf("last_successful_run.status = %v, esperado success", ok["status"])
	}
	if int(ok["fixtures_upserted"].(float64)) != 20 {
		t.Errorf("last_successful_run apontou para o ciclo errado: fixtures_upserted = %v, esperado 20",
			ok["fixtures_upserted"])
	}
}

// Falhar ao GRAVAR o registro não pode derrubar o ciclo nem o endpoint — perder
// o histórico é ruim, virar um segundo incidente é pior.
func TestFalhaAoGravarHistoricoNaoPropaga(t *testing.T) {
	repo := &repoFake{err: errors.New("banco indisponível")}
	h := &SyncHandler{runs: repo}

	// Não deve entrar em pânico nem propagar.
	h.recordFailedRun(context.Background(), domain.SyncTriggerCron,
		discoveryParcial(0), updateParcial(0, 0), 100, "qualquer erro")

	body := chamarStatus(t, repo)
	if body["stale"] != true {
		t.Errorf("stale = %v, esperado true quando não há ciclo bem-sucedido conhecido: "+
			"afirmar saúde sem evidência é o erro que a auditoria já apontou", body["stale"])
	}
}
