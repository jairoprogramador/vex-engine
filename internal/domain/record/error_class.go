package record

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
