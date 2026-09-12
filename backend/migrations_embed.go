// Package cornerlab existe por um motivo só: embutir os arquivos SQL de
// migrations dentro do binário.
//
// go:embed não enxerga diretórios acima do pacote, e as migrations moram em
// backend/migrations/. Em vez de mover a pasta (quebrando as referências nos
// documentos de auditoria, que citam caminhos como backend/migrations/013_...),
// o embed fica aqui na raiz do módulo e o runner recebe o sistema de arquivos
// por parâmetro — o que também deixa o runner testável com um FS falso.
//
// Embutir no binário, em vez de ler do disco, garante que o SQL que roda é
// exatamente o da versão que subiu. Não há como o container ter um binário novo
// e arquivos de migration velhos.
package cornerlab

import "embed"

// MigrationsFS contém todos os .sql de backend/migrations.
//
// Inclui arquivos que NÃO são migration (ex.: diagnostico_duplicatas_serie_b.sql).
// A seleção do que é migration fica no runner, que exige o prefixo numérico —
// ver internal/migrate.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
