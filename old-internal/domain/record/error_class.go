package record

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
)

// ErrorClass es el vocabulario CERRADO de por qué algo falló.
//
// # `unknown` es un valor legítimo, y preferible a forzar una clasificación
//
// Es la parte más valiosa de la disciplina de este paquete: *guardar hechos,
// nunca conclusiones*. Un fallo que el motor no sabe clasificar es exactamente
// eso —uno que no sabe clasificar— y decirlo es un hecho. Meterlo a martillazos
// en la clase que más se le parezca produciría un agregado que cuenta cosas que
// no pasaron, y un agregado en el que no se puede confiar no se usa.
//
// # Crece añadiendo un valor, no admitiendo texto libre
//
// Cada valor nuevo tiene que corresponder a algo que el motor OBSERVA, no a algo
// que se imagina: las siete de aquí abajo son las que hoy tienen un sitio en el
// código de donde salir. Una clase sin observador es una que nunca se emite y que
// aun así aparece en el vocabulario del consumidor.
type ErrorClass string

const (
	// ErrorClassNone es la ausencia de error: lo que lleva un hecho que terminó
	// bien. Es el valor cero del tipo, y eso hace que un `command_finished`
	// exitoso no tenga que decir nada sobre errores.
	ErrorClassNone ErrorClass = ""

	// ErrorClassUnknown es el fallo que no se pudo clasificar. Ver arriba: no es
	// un hueco a rellenar.
	ErrorClassUnknown ErrorClass = "unknown"

	// ErrorClassCommandFailed es el comando que devolvió un código de salida
	// distinto de cero (`command.CommandFailedError`). Es el fallo más común y el
	// único que trae un exit code de verdad.
	ErrorClassCommandFailed ErrorClass = "command_failed"

	// ErrorClassCancelled es la ejecución que alguien paró: `SIGINT`/`SIGTERM`
	// cancelan el contexto y la cadena se desenrolla (spec 07). No lleva exit
	// code, y ésa es exactamente la diferencia con un fallo.
	ErrorClassCancelled ErrorClass = "cancelled"

	// ErrorClassInvalidPipelinecode es el pipelinecode que no se sostiene: un
	// `NN-nombre` mal formado, un orden duplicado, un `scope` fuera del
	// vocabulario, un grafo de declaraciones que no cierra (specs 04, 13, 14, 15).
	ErrorClassInvalidPipelinecode ErrorClass = "invalid_pipelinecode"

	// ErrorClassSourceUnavailable es la fuente que no se pudo obtener: el remoto
	// que no responde al clonar el proyecto o el pipelinecode (spec 18).
	ErrorClassSourceUnavailable ErrorClass = "source_unavailable"

	// ErrorClassStateUnavailable es el destino del estado que no se puede usar:
	// el volumen sin montar, la ruta que no existe o no es escribible (spec 16).
	// El motor sale con código 2 y no ejecuta nada.
	ErrorClassStateUnavailable ErrorClass = "state_unavailable"
)

// String es la forma externa de la clase.
func (c ErrorClass) String() string { return string(c) }

// IsZero dice que el hecho no reporta ningún error.
func (c ErrorClass) IsZero() bool { return c == ErrorClassNone }

// IsKnown dice si la clase pertenece al vocabulario de este motor.
//
// Igual que `EventType.IsKnown`, existe para el lector: una clase desconocida
// llegada de un motor más nuevo se conserva tal cual al plegar. Un consumidor que
// no la entienda puede contarla aparte; convertirla en `unknown` sería perder un
// dato que alguien sí observó.
func (c ErrorClass) IsKnown() bool {
	switch c {
	case ErrorClassNone, ErrorClassUnknown, ErrorClassCommandFailed,
		ErrorClassCancelled, ErrorClassInvalidPipelinecode,
		ErrorClassSourceUnavailable, ErrorClassStateUnavailable:
		return true
	default:
		return false
	}
}

// ClassifyError traduce el error que el motor OBSERVA a la clase que el hecho
// transporta.
//
// Vive aquí y no en cada emisor por lo mismo que `Fold` vive en dominio puro:
// tres capas ven fallar cosas —el comando, el step, el intento— y una sola
// traducción es lo que impide que el mismo fallo se cuente de tres maneras.
//
// # `unknown` no es un hueco a rellenar
//
// Lo que no se sabe clasificar se dice. Meterlo a martillazos en la clase que
// más se le parezca produciría un agregado que cuenta cosas que no pasaron, y de
// un agregado en el que no se puede confiar no se deriva nada. Las clases que
// hoy no se derivan de un error —`invalid_pipelinecode`, `source_unavailable`,
// `state_unavailable`— se quedan sin emisor a propósito: no hay un tipo de error
// del que salgan, y una clase adivinada por el texto del mensaje sería una
// conclusión disfrazada de hecho.
//
// La cancelación se mira ANTES que el fallo del comando y no al revés: al
// cancelar el contexto el comando en curso muere y devuelve un exit code, así
// que quedarse con el primero contaría una decisión como una desgracia.
func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ErrorClassNone
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrorClassCancelled
	}
	var failed *command.CommandFailedError
	if errors.As(err, &failed) {
		return ErrorClassCommandFailed
	}
	return ErrorClassUnknown
}

// ExitCodeOf recupera el código de salida del comando que falló, si lo hubo.
//
// La segunda salida es la que hace que `step_finished.ExitCode` sea un puntero:
// un step exitoso, revivido o saltado no tiene código que reportar, y un `0`
// diría lo contrario. Un fallo que no es de un comando —un clon, una validación
// del pipelinecode— tampoco tiene uno propio.
func ExitCodeOf(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var failed *command.CommandFailedError
	if errors.As(err, &failed) {
		return failed.ExitCode(), true
	}
	return 0, false
}
