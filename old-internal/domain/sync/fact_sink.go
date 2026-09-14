package sync

import "context"

// FactSink es por donde el sincronizador cuenta lo que le pasó a él mismo.
//
// # Por qué es un puerto y no `*record.Emitter`
//
// Por la dirección de la dependencia. Este paquete importa `record` —el lote
// habla de tiras y de posiciones—, así que `record` no puede importarlo de
// vuelta. Es la misma forma que resuelve el ciclo de las cadenas de step y de
// comando (spec 19 §9.1): **el puerto se declara donde se consume**, y quien lo
// implementa es `record.Facts`, que puede hacerlo sin importar este paquete
// porque la firma sólo habla de cadenas de texto.
//
// La comprobación de contrato vive en el cableado
// (`var _ sync.FactSink = (*record.Facts)(nil)`), que es el único sitio que ve
// los dos lados.
type FactSink interface {
	// SyncFailed registra que el empuje no llegó.
	//
	// **No interrumpe el pipeline ni cambia su resultado** (§5.5). Existe para
	// que un `vex log` o un `vex plan` posterior pueda EXPLICAR por qué el
	// registro remoto tiene huecos, en vez de que se descubra por casualidad.
	// Misma disciplina que el resto del vocabulario: guardar hechos, no
	// conclusiones — «nunca se pudo sincronizar» es un hecho tan válido como
	// cualquier otro.
	SyncFailed(ctx *context.Context, destination, cause string) error
}
