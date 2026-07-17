-- CornerLab - Redefinição de senha via e-mail ("esqueci minha senha")
-- Tokens de uso único, com expiração curta, gerados por AuthUsecase.ForgotPassword
-- e consumidos por AuthUsecase.ResetPassword. Não reaproveita nenhuma tabela
-- existente porque um usuário pode solicitar vários resets (ex: clicou em "esqueci
-- a senha" mais de uma vez) e cada solicitação deve invalidar as anteriores.

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);
