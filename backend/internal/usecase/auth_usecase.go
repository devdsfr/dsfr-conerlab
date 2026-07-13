package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/devdsfr/cornerlab/internal/domain"
	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/devdsfr/cornerlab/pkg/jwtutil"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("credenciais inválidas")
var ErrEmailAlreadyUsed = errors.New("e-mail já cadastrado")

type AuthUsecase struct {
	users     repository.UserRepository
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthUsecase(users repository.UserRepository, jwtSecret string, jwtExpiry time.Duration) *AuthUsecase {
	return &AuthUsecase{users: users, jwtSecret: jwtSecret, jwtExpiry: jwtExpiry}
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
