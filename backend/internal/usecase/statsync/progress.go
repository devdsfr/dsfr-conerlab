package statsync

import "github.com/devdsfr/cornerlab/internal/progress"

// O acompanhamento genérico vive em internal/progress (mesmo mecanismo usado pela
// varredura de descobertas). Aqui ficam só os nomes das fases da sincronização e o
// formato do resultado final entregue à tela.

const (
	PhaseDiscovery = "descoberta"
	PhaseUpdate    = "atualizacao"
)

// SyncOutcome é o resultado do ciclo, colocado em Snapshot.Result quando termina.
type SyncOutcome struct {
	Discovery DiscoveryResult `json:"discovery"`
	Update    UpdateResult    `json:"update"`
}

// reporter e noopReporter são apelidos locais para manter as assinaturas dos
// usecases curtas e não espalhar o import do pacote progress.
type reporter = progress.Reporter

type noopReporter = progress.Noop
