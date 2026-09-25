package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/pkg/adminaccess"
	"github.com/devdsfr/cornerlab/pkg/jwtutil"
	"github.com/gin-gonic/gin"
)

// REV-P4 (B4) — prova de LIGAÇÃO: a rota real POST /api/v1/discovery/run do
// roteador de produção passa pelo RequireAdmin. (O comportamento do middleware
// em si é testado em middleware/admin_test.go.)

type usersRoteador struct{}

func (usersRoteador) Create(context.Context, *domain.User) error { return nil }
func (usersRoteador) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, errors.New("n/a")
}
func (usersRoteador) GetByID(_ context.Context, id int64) (*domain.User, error) {
	return &domain.User{ID: id, Email: "comum@cornerlab.test"}, nil
}
func (usersRoteador) GetByStripeCustomerID(context.Context, string) (*domain.User, error) {
	return nil, errors.New("n/a")
}
func (usersRoteador) SetStripeCustomerID(context.Context, int64, string) error { return nil }
func (usersRoteador) UpdateSubscriptionByCustomerID(context.Context, string, string, string, string, *time.Time, *time.Time) error {
	return nil
}
func (usersRoteador) UpdatePassword(context.Context, int64, string) error { return nil }

func TestRotaDiscoveryRun_UsuarioComumRecebe403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminaccess.Configure([]string{"admin@cornerlab.test"})
	defer adminaccess.Configure(nil)

	const segredo = "segredo-de-teste"
	r := NewRouter(Handlers{}, segredo, usersRoteador{})

	token, err := jwtutil.GenerateToken(segredo, time.Hour, 2, "comum@cornerlab.test")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/discovery/run", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != nethttp.StatusForbidden {
		t.Fatalf("usuário comum: status %d, esperado 403 — a rota real não está protegida", w.Code)
	}

	// E sem token continua 401 (AuthRequired antes do RequireAdmin).
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(nethttp.MethodPost, "/api/v1/discovery/run", nil))
	if w2.Code != nethttp.StatusUnauthorized {
		t.Errorf("sem token: status %d, esperado 401", w2.Code)
	}
}
