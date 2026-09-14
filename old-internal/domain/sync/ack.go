package sync

import (
	"context"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// AckStore guarda hasta dónde confirmó el destino, POR TIRA.
//
// # Es una optimización de ancho de banda, no un checkpoint (§5.3)
//
// Su única función es evitar reenviar el intento completo en cada step: sin él,
// el tráfico crecería O(n²) en número de steps. Tres propiedades lo mantienen
// inofensivo, y las tres son del contrato y no del adaptador:
//
//   - **Su pérdida degrada a reenvío total**, nunca a duplicado ni a hueco. Por
//     eso puede vivir en el área de trabajo, fuera del volumen: si desaparece se
//     asume la posición cero. Un adaptador que no pueda leerlo debe decir
//     «no consta» en vez de fallar — la respuesta correcta ante la duda es
//     mandarlo todo otra vez, que es idempotente.
//   - **Se avanza DESPUÉS de que el destino confirma, jamás antes.** Por eso
//     `Sink.Push` devuelve la posición: lo que se guarda es lo que el destino
//     dijo, no lo que el llamador creyó mandar.
//   - **El área de trabajo NO se vacía tras empujar.** Es un búfer de
//     solo-anexar con un puntero, y no una bandeja de salida: el intento queda
//     íntegro aunque el push haya funcionado, así que siempre se puede
//     re-empujar desde cero contra un destino corrupto o desactualizado. Es la
//     razón por la que el patrón Outbox se descartó (§5.3').
type AckStore interface {
	// Last es la última posición confirmada para esta tira.
	//
	// La AUSENCIA no es un error: es la posición cero, y aguas arriba significa
	// «manda la tira entera». Un archivo ilegible se trata igual, por lo mismo
	// que el índice de caché resuelve la duda hacia la ausencia (spec 11 §5.6):
	// aquí equivocarse cuesta ancho de banda, no correctitud.
	Last(ctx *context.Context, stream record.EventStream) (record.Seq, error)

	// Save publica la posición confirmada. Sustituye: el `ack` es un puntero al
	// último, no una historia.
	Save(ctx *context.Context, stream record.EventStream, confirmed record.Seq) error
}
