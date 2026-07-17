package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/pkg/email"
	"github.com/devdsfr/cornerlab/pkg/jwtutil"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("credenciais inválidas")
var ErrEmailAlreadyUsed = errors.New("e-mail já cadastrado")
var ErrInvalidResetToken = errors.New("link de redefinição inválido ou expirado")

// resetTokenTTL é por quanto tempo um link de "esqueci minha senha" fica válido
// depois de enviado — curto o suficiente para limitar o risco se o e-mail do
// usuário for comprometido, longo o suficiente para não expirar antes de ele abrir
// a caixa de entrada.
const resetTokenTTL = 1 * time.Hour

type AuthUsecase struct {
	users       repository.UserRepository
	resetTokens repository.PasswordResetRepository
	emailSender email.Sender
	jwtSecret   string
	jwtExpiry   time.Duration
	frontendURL string
}

func NewAuthUsecase(users repository.UserRepository, resetTokens repository.PasswordResetRepository, emailSender email.Sender, jwtSecret string, jwtExpiry time.Duration, frontendURL string) *AuthUsecase {
	return &AuthUsecase{
		users:       users,
		resetTokens: resetTokens,
		emailSender: emailSender,
		jwtSecret:   jwtSecret,
		jwtExpiry:   jwtExpiry,
		frontendURL: frontendURL,
	}
}

func (u *AuthUsecase) Register(ctx context.Context, name, email, password string) (*domain.User, string, error) {
	existing, _ := u.users.GetByEmail(ctx, email)
	if existing != nil {
		return nil, "", ErrEmailAlreadyUsed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	user := &domain.User{Name: name, Email: email, PasswordHash: string(hash)}
	if err := u.users.Create(ctx, user); err != nil {
		return nil, "", err
	}

	token, err := jwtutil.GenerateToken(u.jwtSecret, u.jwtExpiry, user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (u *AuthUsecase) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, err := u.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := jwtutil.GenerateToken(u.jwtSecret, u.jwtExpiry, user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// ForgotPassword gera um token de redefinição e envia o link por e-mail. Sempre
// retorna sucesso quando o serviço de e-mail está configurado — mesmo se o e-mail
// informado não existir na base — para não revelar quais e-mails têm conta
// (enumeração de usuários). Retorna email.ErrNotConfigured se RESEND_API_KEY não
// estiver definida, para o handler devolver um 503 claro (esse erro não vaza
// informação sobre contas específicas, só sobre a configuração do backend).
func (u *AuthUsecase) ForgotPassword(ctx context.Context, userEmail string) error {
	if !u.emailSender.Configured() {
		return email.ErrNotConfigured
	}

	user, err := u.users.GetByEmail(ctx, userEmail)
	if err != nil || user == nil {
		// E-mail não cadastrado — não revela isso ao chamador, apenas não envia nada.
		return nil
	}

	// Invalida qualquer link enviado anteriormente antes de criar o novo, para que
	// só o link mais recente funcione.
	if err := u.resetTokens.InvalidateAllForUser(ctx, user.ID); err != nil {
		return err
	}

	rawToken, err := randomToken()
	if err != nil {
		return err
	}

	resetToken := &domain.PasswordResetToken{
		UserID:    user.ID,
		Token:     rawToken,
		ExpiresAt: time.Now().Add(resetTokenTTL),
	}
	if err := u.resetTokens.Create(ctx, resetToken); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/redefinir-senha?token=%s", u.frontendURL, rawToken)
	subject := "Redefinir sua senha — CornerLab"
	html := fmt.Sprintf(`
		<p>Olá, %s!</p>
		<p>Recebemos um pedido para redefinir a senha da sua conta CornerLab. Clique no link abaixo para escolher uma nova senha:</p>
		<p><a href="%s">%s</a></p>
		<p>Este link expira em 1 hora. Se você não pediu essa redefinição, pode ignorar este e-mail com segurança — sua senha atual continua válida.</p>
	`, user.Name, link, link)

	return u.emailSender.Send(ctx, user.Email, subject, html)
}

// ResetPassword troca a senha do usuário se o token informado ainda for válido
// (existe, não expirou, não foi usado). Marca o token como usado mesmo se a troca
// de senha falhar depois, para nunca permitir reuso do mesmo link.
func (u *AuthUsecase) ResetPassword(ctx context.Context, token, newPassword string) error {
	resetToken, err := u.resetTokens.GetValidByToken(ctx, token)
	if err != nil {
		return ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := u.users.UpdatePassword(ctx, resetToken.UserID, string(hash)); err != nil {
		return err
	}
	return u.resetTokens.MarkUsed(ctx, resetToken.ID)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
