// Package billing implementa a Assinatura Premium do CornerLab via Stripe Checkout
// (hospedado, sem Stripe.js no frontend) e Stripe Billing Portal (autoatendimento
// de cancelamento/troca de cartão). A fonte de verdade do acesso é sempre
// users.subscription_status, atualizado só pelos webhooks do Stripe — nunca pela
// resposta do Checkout em si, que pode chegar antes ou depois do webhook.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/devdsfr/cornerlab/internal/repository"
	"github.com/stripe/stripe-go/v78"
	portalsession "github.com/stripe/stripe-go/v78/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/webhook"
)

// ErrNotConfigured é retornado por todas as operações quando o backend ainda não
// tem uma chave do Stripe configurada (STRIPE_SECRET_KEY/STRIPE_PRICE_ID vazias) —
// isso é esperado em desenvolvimento antes do usuário criar a conta Stripe, e o
// handler HTTP traduz este erro em 503 com uma mensagem clara.
var ErrNotConfigured = errors.New("assinatura premium ainda não configurada (STRIPE_SECRET_KEY/STRIPE_PRICE_ID ausentes)")

// Status é a visão de assinatura exposta em GET /api/v1/billing/status.
type Status struct {
	Plan               string     `json:"plan"`
	SubscriptionStatus string     `json:"subscription_status"`
	IsPremium          bool       `json:"is_premium"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	Configured         bool       `json:"configured"`
}

type Usecase struct {
	users         repository.UserRepository
	secretKey     string
	webhookSecret string
	priceID       string
	trialDays     int
	frontendURL   string
}

func New(users repository.UserRepository, secretKey, webhookSecret, priceID string, trialDays int, frontendURL string) *Usecase {
	if secretKey != "" {
		stripe.Key = secretKey
	}
	return &Usecase{
		users:         users,
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		priceID:       priceID,
		trialDays:     trialDays,
		frontendURL:   frontendURL,
	}
}

// Configured indica se há chave e price configurados — usado tanto para decidir
// se os endpoints de billing respondem normalmente quanto para o frontend saber se
// deve mostrar o botão de assinar ou uma mensagem de "em breve".
func (u *Usecase) Configured() bool {
	return u.secretKey != "" && u.priceID != ""
}

func (u *Usecase) Status(ctx context.Context, userID int64) (*Status, error) {
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Status{
		Plan:               user.Plan,
		SubscriptionStatus: user.SubscriptionStatus,
		IsPremium:          user.IsPremium(),
		TrialEndsAt:        user.TrialEndsAt,
		CurrentPeriodEnd:   user.CurrentPeriodEnd,
		Configured:         u.Configured(),
	}, nil
}

// CreateCheckoutSession cria uma Stripe Checkout Session hospedada (modo
// subscription, com trial de u.trialDays dias) e retorna a URL para redirecionar o
// usuário. Reaproveita o Customer do Stripe já vinculado ao usuário, se existir.
func (u *Usecase) CreateCheckoutSession(ctx context.Context, userID int64) (string, error) {
	if !u.Configured() {
		return "", ErrNotConfigured
	}
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(u.priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(u.frontendURL + "/assinatura?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String(u.frontendURL + "/assinatura"),
		ClientReferenceID: stripe.String(strconv.FormatInt(user.ID, 10)),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(int64(u.trialDays)),
		},
	}
	if user.StripeCustomerID != nil && *user.StripeCustomerID != "" {
		params.Customer = stripe.String(*user.StripeCustomerID)
	} else {
		params.CustomerEmail = stripe.String(user.Email)
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// CreatePortalSession cria uma sessão do Stripe Billing Portal (autoatendimento:
// trocar cartão, ver faturas, cancelar). Exige que o usuário já tenha um
// Customer do Stripe — ou seja, já ter passado ao menos uma vez pelo Checkout.
func (u *Usecase) CreatePortalSession(ctx context.Context, userID int64) (string, error) {
	if !u.Configured() {
		return "", ErrNotConfigured
	}
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.StripeCustomerID == nil || *user.StripeCustomerID == "" {
		return "", errors.New("usuário ainda não possui assinatura no Stripe")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*user.StripeCustomerID),
		ReturnURL: stripe.String(u.frontendURL + "/assinatura"),
	}
	sess, err := portalsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// HandleWebhook valida a assinatura do evento e aplica o efeito correspondente:
//   - checkout.session.completed: grava o vínculo usuário <-> Customer do Stripe
//     (via ClientReferenceID, setado na criação da sessão acima).
//   - customer.subscription.created/updated: sincroniza status/plano/datas.
//   - customer.subscription.deleted: marca a assinatura como cancelada.
//
// payload deve ser o corpo bruto (não decodificado) da requisição, e signatureHeader
// o valor do header "Stripe-Signature" — ambos exigidos pela verificação HMAC do
// Stripe, por isso o handler HTTP precisa ler o raw body antes de qualquer bind JSON.
func (u *Usecase) HandleWebhook(ctx context.Context, payload []byte, signatureHeader string) error {
	if !u.Configured() || u.webhookSecret == "" {
		return ErrNotConfigured
	}
	event, err := webhook.ConstructEvent(payload, signatureHeader, u.webhookSecret)
	if err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return err
		}
		return u.handleCheckoutCompleted(ctx, sess)

	case "customer.subscription.created", "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return u.syncSubscription(ctx, sub)

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return u.users.UpdateSubscriptionByCustomerID(ctx, sub.Customer.ID, sub.ID, "canceled", "free", nil, nil)
	}

	// Outros tipos de evento (ex.: invoice.payment_failed) não têm efeito hoje —
	// ignorados silenciosamente, retornando 200 para o Stripe não tentar reenviar.
	return nil
}

func (u *Usecase) handleCheckoutCompleted(ctx context.Context, sess stripe.CheckoutSession) error {
	if sess.ClientReferenceID == "" || sess.Customer == nil {
		return nil
	}
	userID, err := strconv.ParseInt(sess.ClientReferenceID, 10, 64)
	if err != nil {
		return nil
	}
	return u.users.SetStripeCustomerID(ctx, userID, sess.Customer.ID)
}

func (u *Usecase) syncSubscription(ctx context.Context, sub stripe.Subscription) error {
	if sub.Customer == nil {
		return nil
	}
	plan := "free"
	status := string(sub.Status)
	if status == string(stripe.SubscriptionStatusActive) || status == string(stripe.SubscriptionStatusTrialing) {
		plan = "premium"
	}

	var trialEndsAt, currentPeriodEnd *time.Time
	if sub.TrialEnd > 0 {
		t := time.Unix(sub.TrialEnd, 0)
		trialEndsAt = &t
	}
	if sub.CurrentPeriodEnd > 0 {
		t := time.Unix(sub.CurrentPeriodEnd, 0)
		currentPeriodEnd = &t
	}

	return u.users.UpdateSubscriptionByCustomerID(ctx, sub.Customer.ID, sub.ID, status, plan, trialEndsAt, currentPeriodEnd)
}

