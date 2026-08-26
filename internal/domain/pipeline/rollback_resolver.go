package pipeline

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

// RollbackResolver resuelve el ancla de un rollback: **qué registro estuvo
// vigente en cada step de la ejecución destino** (spec 28 §5.3).
//
// # El ancla se LEE, no se deriva
//
// Sale de los hechos de aquella ejecución —`evidence_from` en cada
// `step_finished`, que la spec 19 emite en las dos mitades: cuando el step
// revivió y cuando ejecutó—. La alternativa era derivarla por fechas —«el último
// registro anterior a la fecha de aquella ejecución»— y se descartó porque es
// una RECONSTRUCCIÓN con dos modos de fallo silenciosos, solapamiento y reloj,
// justo en la operación que menos puede permitírselos (§4, alternativa D).
//
// # Rechaza antes del primer step, y enumera por qué
//
// Un destino inválido no se descubre a mitad (§5.2). Se rechaza cuando:
//
//	no consta ni un hecho de ese intento          — el identificador no existe
//	el intento no terminó bien                    — o algún step falló o quedó abierto
//	algún step del objeto no llegó a alcanzarse   — la completitud que `IsValidTarget`
//	                                                no puede comprobar sola (22 §9)
//	no se puede enlazar su objeto                 — sin él no hay lista de steps que
//	                                                cotejar, y seguir con el
//	                                                `content_id` equivocado sería peor
//
// El último merece la frase que la spec 22 le dedica: el enlace despliegue→objeto
// no está escrito en ninguna parte y se DERIVA recomputando `dep-v1`, así que un
// destino al que un empuje anterior no llegó tiene el objeto y no tiene la tira
// de su padre. Ahí el motor **rechaza explícitamente** en vez de continuar.
//
// # El puerto vive aquí porque quien lo consume es el handler 09
//
// La implementación escanea `events/` y `objects/` del DESTINO, que es donde la
// spec 21 los empuja: es lo que hace que un rollback pueda anclarse en una
// ejecución de otra máquina que compartía destino, y no un efecto colateral —es
// lo que hace que «volver a lo de ayer» no dependa de en qué portátil se desplegó
// ayer—.
type RollbackResolver interface {
	// Resolve compone el ancla del destino pedido.
	//
	// Un destino CERO devuelve un ancla cero sin error: es la ejecución normal, y
	// no pedir rollback no es un caso que haya que diagnosticar.
	Resolve(
		ctx *context.Context,
		target deployment.RollbackTarget) (deployment.RollbackAnchor, error)
}
