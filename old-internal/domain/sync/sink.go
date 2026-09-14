package sync

import (
	"context"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// Sink es por donde lo registrado sale del área de trabajo hacia el destino.
//
//	type: local  →  escribe de `staging/` a `destino.path`
//	type: http   →  POST hacia `destino.endpoint` (CONGELADO, spec 16; lo
//	                descongela la 26)
//
// # Ninguno es «el normal» y el otro «el alterno»
//
// Los dos pueden fallar y se tratan con el mismo reintento (§5.1). La ventaja
// lateral de `local` es que sus fallos son MÁS informativos que los de red
// —ruta inexistente, permisos, disco lleno son inmediatos y específicos—, así
// que un volumen no montado se detecta al instante y con causa clara. Eso lo
// hace un buen primer adaptador, no un caso especial: si uno fallara ruidosamente
// y el otro callara no serían sustituibles (LSP, §5.2').
//
// # Se llama `Sink` y no `Target`
//
// Porque `deployment.Destination` ya existe y es otra cosa: el ambiente al que
// un despliegue va dirigido (C-6, spec 17). Dos conceptos sin relación con
// nombres casi idénticos es exactamente lo que el nombre evita.
type Sink interface {
	// Push empuja la selección y devuelve HASTA DÓNDE el destino confirmó.
	//
	// Devolver la posición confirmada —y no sólo un error— es lo que hace
	// literal la regla de §5.3: el `ack` se avanza **con lo que el destino
	// dijo**, no con lo que el llamador supone que mandó. Un `ack` optimista
	// abriría un hueco real.
	//
	// La posición cero significa «el destino no tiene nada de esta tira», que es
	// lo que devuelve un empuje de un lote vacío contra un destino virgen.
	Push(ctx *context.Context, batch Batch) (record.Seq, error)

	// Destination nombra el destino en la forma que va a leer un humano cuando
	// pregunte por qué el registro remoto tiene huecos. Es lo que viaja en el
	// `destination` de `sync_failed`.
	Destination() string
}
