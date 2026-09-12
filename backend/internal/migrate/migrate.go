// Package migrate aplica as migrations pendentes do banco na inicialização.
//
// POR QUE EXISTE. Três vezes em uma semana o código subiu para produção antes do
// SQL correspondente ter sido rodado à mão, e as três vezes o resultado foi o
// mesmo: `ERROR: column "..." does not exist` em toda consulta de partidas, com
// o Simulador e o ciclo de descoberta fora do ar até alguém perceber.
//
//	013_odds_source          -> derrubou o Simulador por ~3h
//	014_tier_nao_classificado -> evitada por pouco
//	015_result_odds          -> derrubou o Simulador de novo
//
// O erro não foi de desatenção: foi de PROCESSO. Um passo manual que precisa
// acontecer na ordem certa, toda vez, por alguém que lembre — vai falhar. A
// correção é tirar o passo da mão de quem faz o deploy.
//
// GARANTIA QUE ESTE PACOTE DÁ: ou o schema está na versão que o código espera,
// ou o processo não sobe. Falhar ao iniciar é barulhento e óbvio; subir com o
// schema errado é silencioso e só aparece quando um usuário clica em algo.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey serializa a aplicação entre processos.
//
// A API e o worker sobem separados e podem inicializar ao mesmo tempo; no Render,
// um deploy também pode ter duas instâncias da API se sobrepondo. Sem o lock, duas
// tentariam aplicar a mesma migration simultaneamente. O número é arbitrário, só
// precisa ser estável e não colidir com outro advisory lock da aplicação.
const advisoryLockKey int64 = 8074531

// ChecksumAdotada marca uma migration que foi aplicada À MÃO, antes deste runner
// existir, e depois apenas registrada para que ele não a repita.
//
// Existe porque o checksum real dessas não pode ser conferido: ninguém sabe se o
// SQL que rodou no banco é byte a byte o que está no arquivo hoje. Fingir um
// checksum válido seria afirmar uma verificação que não aconteceu; usar o
// checksum do arquivo atual faria o runner "confirmar" uma igualdade que não
// verificou. Este valor diz a verdade: aplicada, origem não conferida.
const ChecksumAdotada = "adotada-sem-verificacao"

// migrationName aceita só arquivos com prefixo numérico: 001_init.sql,
// 015_result_odds.sql. É o que separa migration de script avulso — a pasta também
// guarda coisas como diagnostico_duplicatas_serie_b.sql, que NUNCA deve rodar
// sozinho num deploy.
var migrationName = regexp.MustCompile(`^(\d{3,})_[A-Za-z0-9_\-]+\.sql$`)

type migration struct {
	version string // nome do arquivo, que é a chave em schema_migrations
	sql     string
	sum     string
}

// Run aplica, em ordem, toda migration ainda não registrada. Devolve erro na
// primeira que falhar — o chamador deve encerrar o processo.
func Run(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	pendentes, err := carregar(fsys)
	if err != nil {
		return err
	}
	if len(pendentes) == 0 {
		return fmt.Errorf("nenhuma migration encontrada no binário — o go:embed não pegou os arquivos")
	}

	// Uma conexão só, do começo ao fim: advisory lock vive na conexão, não na
	// sessão do pool. Pegar do pool a cada passo poderia soltar o lock no meio.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrations: obter conexão: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			checksum   TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("migrations: criar schema_migrations: %w", err)
	}

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("migrations: adquirir lock: %w", err)
	}
	defer func() {
		// Best-effort: se o unlock falhar, a conexão é devolvida ao pool e o
		// Postgres solta o lock quando ela fecha.
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	aplicadas, err := jaAplicadas(ctx, conn)
	if err != nil {
		return err
	}

	var rodadas int
	for _, m := range pendentes {
		sumAnterior, ok := aplicadas[m.version]
		if ok {
			// Já aplicada. Se o arquivo mudou depois disso, avisa mas não bloqueia:
			// editar um comentário numa migration antiga não pode impedir o deploy.
			// O que importa é que ninguém fique sem saber.
			if sumAnterior != m.sum && sumAnterior != ChecksumAdotada {
				log.Warn("migration já aplicada foi alterada depois; o banco NÃO reflete o arquivo atual",
					"version", m.version)
			}
			continue
		}

		log.Info("aplicando migration", "version", m.version)

		// O SQL do arquivo e o registro em schema_migrations vão no MESMO envio,
		// pelo protocolo simples: o Postgres trata um lote multi-statement como
		// uma transação implícita, então ou os dois acontecem ou nenhum. Sem isso,
		// um crash entre aplicar e registrar deixaria a migration rodando de novo
		// no próximo boot.
		//
		// PRESSUPOSTO: nenhum arquivo de migration contém BEGIN/COMMIT explícito —
		// isso quebraria a atomicidade do lote.
		lote := m.sql + "\n;\nINSERT INTO schema_migrations (version, checksum) VALUES (" +
			quote(m.version) + ", " + quote(m.sum) + ");"

		if err := conn.Conn().PgConn().Exec(ctx, lote).Close(); err != nil {
			return fmt.Errorf("migrations: falha em %s (nada foi aplicado desta migration): %w", m.version, err)
		}
		rodadas++
	}

	if rodadas == 0 {
		log.Info("schema do banco já está atualizado", "migrations", len(pendentes))
	} else {
		log.Info("migrations aplicadas", "aplicadas_agora", rodadas, "total", len(pendentes))
	}
	return nil
}

// jaAplicadas devolve version -> checksum do que já rodou. Usa a MESMA conexão
// que segura o advisory lock: ler de outra conexão do pool poderia enxergar um
// estado anterior ao de quem está com o lock.
func jaAplicadas(ctx context.Context, conn *pgxpool.Conn) (map[string]string, error) {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations: ler schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var v, c string
		if err := rows.Scan(&v, &c); err != nil {
			return nil, err
		}
		out[v] = c
	}
	return out, rows.Err()
}

// carregar lê os arquivos embutidos, filtra os que são migration e ordena.
func carregar(fsys fs.FS) ([]migration, error) {
	entradas, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations: ler diretório embutido: %w", err)
	}

	var out []migration
	for _, e := range entradas {
		if e.IsDir() || !migrationName.MatchString(e.Name()) {
			continue
		}
		conteudo, err := fs.ReadFile(fsys, "migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrations: ler %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(conteudo)
		out = append(out, migration{
			version: e.Name(),
			sql:     string(conteudo),
			sum:     hex.EncodeToString(sum[:]),
		})
	}

	// Ordem lexicográfica funciona porque o prefixo é numérico com largura fixa
	// (001..015). Se um dia passar de 999, o próximo precisa ter 4 dígitos — a
	// regex já aceita.
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// quote monta um literal SQL seguro. Os valores aqui são nomes de arquivo e
// hashes hexadecimais (não vêm de usuário), mas escapar é barato e evita que uma
// futura mudança transforme isto num vetor de injeção.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
