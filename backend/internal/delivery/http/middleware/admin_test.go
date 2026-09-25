package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/pkg/adminaccess"
	"github.com/gin-gonic/gin"
)

// REV-P4 (B4) — "Procurar agora" restrito a administrador, no BACKEND.

type fakeUsers struct{ byID map[int64]*domain.User }

func (f fakeUsers) Create(context.Context, *domain.User) error { return nil }
func (f fakeUsers) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, errors.New("n/a")
}
func (f fakeUsers) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("não encontrado")
}
func (f fakeUsers) GetByStripeCustomerID(context.Context, string) (*domain.User, error) {
	return nil, errors.New("n/a")
}
func (f fakeUsers) SetStripeCustomerID(context.Context, int64, string) error { return nil }
func (f fakeUsers) UpdateSubscriptionByCustomerID(context.Context, string, string, string, string, *time.Time, *time.Time) error {
	return nil
}
func (f fakeUsers) UpdatePassword(context.Context, int64, string) error { return nil }

// rotaProtegida monta uma rota com o user_id já no contexto (papel do
// AuthRequired, testado à parte) e o RequireAdmin na frente do handler.
func rotaProtegida(userID int64, users fakeUsers) (*httptest.ResponseRecorder, *bool) {
	gin.SetMode(gin.TestMode)
	chegou := false
	r := gin.New()
	r.POST("/discovery/run", func(c *gin.Context) {
		if userID != 0 {
			c.Set(ContextUserIDKey, userID)
		}
		c.Next()
	}, RequireAdmin(users), func(c *gin.Context) {
		chegou = true
		c.Status(http.StatusAccepted)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/discovery/run", nil))
	return w, &chegou
}

func usuarios() fakeUsers {
	return fakeUsers{byID: map[int64]*domain.User{
		1: {ID: 1, Email: "admin@cornerlab.test"},
		2: {ID: 2, Email: "comum@cornerlab.test"},
	}}
}

func TestRequireAdmin_UsuarioComumRecebe403(t *testing.T) {
	adminaccess.Configure([]string{"admin@cornerlab.test"})
	defer adminaccess.Configure(nil)

	w, chegou := rotaProtegida(2, usuarios())
	if w.Code != http.StatusForbidden {
		t.Errorf("status %d, esperado 403", w.Code)
	}
	if *chegou {
		t.Error("o handler do Discovery foi executado para usuário comum")
	}
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "admin_required" {
		t.Errorf("code = %q, esperado admin_required", body["code"])
	}
}

func TestRequireAdmin_AdminPassa(t *testing.T) {
	adminaccess.Configure([]string{"  ADMIN@CornerLab.test "}) // normaliza caixa e espaços
	defer adminaccess.Configure(nil)

	w, chegou := rotaProtegida(1, usuarios())
	if w.Code != http.StatusAccepted || !*chegou {
		t.Errorf("admin: status %d, chegou ao handler = %v", w.Code, *chegou)
	}
}

func TestRequireAdmin_SemTokenRecebe401(t *testing.T) {
	adminaccess.Configure([]string{"admin@cornerlab.test"})
	defer adminaccess.Configure(nil)

	w, chegou := rotaProtegida(0, usuarios())
	if w.Code != http.StatusUnauthorized || *chegou {
		t.Errorf("sem token: status %d, chegou = %v", w.Code, *chegou)
	}
}

func TestRequireAdmin_ListaVaziaNinguemEAdmin(t *testing.T) {
	// Falha fechada: sem ADMIN_EMAILS configurado, nem o dono entra.
	adminaccess.Configure(nil)
	w, chegou := rotaProtegida(1, usuarios())
	if w.Code != http.StatusForbidden || *chegou {
		t.Errorf("lista vazia: status %d, chegou = %v — deveria ser 403", w.Code, *chegou)
	}
}

func TestUserJSON_TrazIsAdmin(t *testing.T) {
	adminaccess.Configure([]string{"admin@cornerlab.test"})
	defer adminaccess.Configure(nil)

	for _, c := range []struct {
		u    domain.User
		quer bool
	}{
		{domain.User{ID: 1, Email: "admin@cornerlab.test", PasswordHash: "segredo"}, true},
		{domain.User{ID: 2, Email: "comum@cornerlab.test", PasswordHash: "segredo"}, false},
	} {
		raw, err := json.Marshal(c.u)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if m["is_admin"] != c.quer {
			t.Errorf("%s: is_admin = %v, esperado %v", c.u.Email, m["is_admin"], c.quer)
		}
		// O MarshalJSON novo não pode vazar o hash (json:"-" tem de continuar valendo).
		if _, vazou := m["PasswordHash"]; vazou {
			t.Errorf("hash de senha vazou no JSON: %s", raw)
		}
		if _, vazou := m["password_hash"]; vazou {
			t.Errorf("hash de senha vazou no JSON: %s", raw)
		}
		if m["email"] != c.u.Email {
			t.Errorf("campos originais perdidos: %s", raw)
		}
	}
}
