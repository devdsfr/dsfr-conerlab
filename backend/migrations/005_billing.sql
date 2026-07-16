-- CornerLab - Assinatura Premium (Stripe)
-- Adiciona ao usuário os campos necessários para controlar o plano atual e a
-- assinatura no Stripe. plan/subscription_status alimentam o middleware
-- RequirePremium e o gate de acesso do frontend; stripe_customer_id/
-- stripe_subscription_id conectam o usuário ao Customer/Subscription no Stripe;
-- trial_ends_at e current_period_end vêm direto dos eventos de webhook do Stripe
-- e são usados só para exibição (a fonte de verdade de acesso é subscription_status).

ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(20) NOT NULL DEFAULT 'free';
ALTER TABLE users ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(100);
-- subscription_status espelha o status da Subscription no Stripe: 'none' (nunca
-- assinou), 'trialing', 'active', 'past_due', 'canceled', 'incomplete_expired'.
ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_status VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE users ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_stripe_customer ON users(stripe_customer_id);
