# 21-runbook-operacional.md

Projeto: DSFR CornerLab

Versão: 1.0

---

# Objetivo

Definir todos os procedimentos operacionais da plataforma.

O objetivo é garantir que qualquer desenvolvedor consiga operar o sistema em produção.

---

# Ambientes

Local

↓

Development

↓

Homologação

↓

Produção

Cada ambiente deverá possuir

Banco próprio

Redis próprio

Variáveis próprias

Logs próprios

---

# Deploy

Fluxo

GitHub

↓

Pull Request

↓

Code Review

↓

Merge

↓

GitHub Actions

↓

Build Docker

↓

Testes

↓

Deploy Render

↓

Health Check

↓

Liberar Produção

---

# Health Check

Verificar

API

Banco

Redis

Workers

OpenAI

API Football

Disco

Memória

CPU

---

# Monitoramento

Monitorar

Tempo de resposta

Quantidade de usuários

Importações

Erros

Jobs

Cache

Banco

Fila

Workers

---

# Workers

Todo Worker deverá possuir

Status

Última execução

Próxima execução

Tempo médio

Quantidade processada

Quantidade de erros

Retry

---

# Scheduler

Import Worker

15 minutos

---

Ranking Worker

30 minutos

---

Statistics Worker

Após Match Finished

---

Discovery Worker

03:00

---

Score Worker

03:15

---

Health Worker

03:30

---

Dashboard Worker

03:45

---

Backup

04:00

---

# Backup

Banco

Diário

Retenção

30 dias

---

Redis

Não obrigatório

---

Uploads

Diário

---

# Recuperação

Caso

API Football indisponível

↓

Utilizar último cache.

---

Caso

Redis indisponível

↓

Consultar PostgreSQL.

---

Caso

Worker falhar

↓

Retry automático.

---

Caso

Banco indisponível

↓

Entrar em modo manutenção.

---

# Logs

Separar

Application

Workers

Importação

IA

Banco

Segurança

---

# Alertas

Enviar alerta quando

Worker parado

API indisponível

Redis indisponível

Banco indisponível

Importação falhar

Uso API acima de 80%

Espaço em disco acima de 90%

---

# Segurança

Rotacionar

JWT Secret

API Keys

Tokens

Senhas

---

# API Football

Monitorar

Quota diária

Quota restante

Tempo médio

Falhas

Rate Limit

---

# IA

Monitorar

Quantidade de Tokens

Tempo médio

Custo diário

Custo mensal

---

# Banco

Executar

VACUUM

ANALYZE

REINDEX

Conforme necessidade

---

# Cache

Limpar

Somente quando necessário.

Nunca limpar Redis inteiro em produção.

---

# Auditoria

Registrar

Login

Logout

Atualização

Importação

Worker

Administração

---

# Recuperação

RTO

30 minutos

RPO

24 horas

---

# Atualização

Deploy

Rolling Update

Sem indisponibilidade

Sempre que possível.

---

# Observabilidade

Prometheus

Grafana

OpenTelemetry

Health Endpoint

Structured Logs

---

# Métricas

Disponibilidade

Tempo médio

Importações

Partidas

Estratégias

Backtests

Usuários ativos

Uso da IA

Uso API Football

---

# Critérios de Aceite

✅ Backup automático.

✅ Retry automático.

✅ Health Check.

✅ Logs.

✅ Auditoria.

✅ Monitoramento.

✅ Recuperação documentada.
