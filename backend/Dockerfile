# --- build stage ---
FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sync ./cmd/sync
# cmd/worker é o processo de background (descoberta/atualização de jogos, analytics,
# strategy engine e discovery engine). Roda no Render como Cron Job apontando para
# /app/worker com SYNC_RUN_ONCE=true — sem este binário na imagem, o cron falha e o
# banco simplesmente para de ser atualizado. Ver render.yaml na raiz do repositório.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /out/seed /app/seed
COPY --from=builder /out/sync /app/sync
COPY --from=builder /out/worker /app/worker
COPY migrations /app/migrations
COPY docs /app/docs
EXPOSE 8080
CMD ["/app/api"]
