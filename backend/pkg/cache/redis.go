package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(addr, password string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
}

// GetJSON busca uma chave no Redis e desserializa em `out`. Retorna hit=false (sem
// erro) quando a chave não existe — cache miss é um caso esperado, não uma falha.
func GetJSON(ctx context.Context, client *redis.Client, key string, out any) (hit bool, err error) {
	raw, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON serializa `value` e grava no Redis com o TTL informado. Implementa a regra
// geral "os resultados deverão ser cacheados / atualização automática diária" do
// módulo de Inteligência Estatística — o TTL padrão usado pelos handlers é de 24h
// (config.Config.IntelligenceCacheTTL).
func SetJSON(ctx context.Context, client *redis.Client, key string, ttl time.Duration, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return client.Set(ctx, key, raw, ttl).Err()
}
