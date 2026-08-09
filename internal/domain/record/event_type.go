package record

// EventType es el vocabulario CERRADO de hechos que el motor sabe observar
// (spec 17 §5.2).
//
// Cerrado por la misma razón que las clases de fallo: **texto libre no es
// agregable**. Es la diferencia entre «3 fallos por `source_unavailable`» y no
// poder contar nada.
//
// Cerrado en VALORES, abierto en EXTENSIÓN: añadir un tipo es añadir una
// constante y su carga útil, y `Fold` sigue plegando los existentes sin tocarse.
type EventType string

const (
	// TypeAttemptStarted abre el intento. Lo emite el resolutor del objeto
	// (spec 18), que es quien lo compone.
	TypeAttemptStarted EventType = "attempt_started"

	// TypeStepStarted y TypeStepFinished los emite `StepExecutable` (spec 19),
	// que posee el ciclo before/exec/after y VE el error. El runner decide y
	// ANOTA; el ejecutable emite.
	TypeStepStarted  EventType = "step_started"
	TypeStepFinished EventType = "step_finished"

	// TypeCommandStarted y TypeCommandFinished, por el mismo argumento, los emite
	// `CommandExecutable`. Se emiten a nivel comando —A2 resuelta que sí— porque
	// «¿qué comando falló y cuánto tardó?» es la primera pregunta de cualquier
	// diagnóstico, y recortar después es posible mientras que recuperar el pasado
	// no lo es.
	TypeCommandStarted  EventType = "command_started"
	TypeCommandFinished EventType = "command_finished"

	// TypeParameterResolved es un hecho POR PARÁMETRO, no un digest agregado
	// (N-3). No cuesta trabajo extra: el par (declaración, valor resuelto) existe
	// junto y completo en la carga del step, antes del primer comando (spec 14).
	TypeParameterResolved EventType = "parameter_resolved"

	// TypeArtifactProduced es lo que un step dejó construido. La convención del
	// digest está DIFERIDA (A1/BL-10): el artefacto se construye en `package`, es
	// salida del resultado y no entrada del objeto, así que fijarla después no
	// invalida nada.
	TypeArtifactProduced EventType = "artifact_produced"

	// TypeStaleCloneUsed es la ventana de reutilización del clon (spec 18 §5.4):
	// el motor procedió con un clon viejo en vez de fallar. Que la explicación de
	// por qué un despliegue corrió contra otra versión del pipeline no dependa de
	// la memoria de nadie.
	TypeStaleCloneUsed EventType = "stale_clone_used"

	// TypeSyncFailed es el empuje del registro que no llegó a su destino
	// (spec 21). Es un hecho del registro sobre sí mismo, y por eso viaja dentro
	// de él: un registro que no puede contar sus propios fallos de entrega
	// obligaría a mirar los logs, que son descartables.
	TypeSyncFailed EventType = "sync_failed"

	// TypeAttemptFinished cierra el intento. Lo emite `CreateExecutionUseCase`,
	// que es la ÚNICA capa que ve tanto el éxito como el fallo.
	//
	// **Su ausencia es informativa**: un intento sin él plegó a `interrupted`, y
	// ése es el caso que justifica el modelo entero.
	TypeAttemptFinished EventType = "attempt_finished"
)

// String es la forma externa del tipo, la misma que se escribe en el archivo.
func (t EventType) String() string { return string(t) }

// IsKnown dice si el tipo pertenece al vocabulario de este motor.
//
// Existe para el LECTOR, no para el emisor: un motor viejo leyendo un archivo
// escrito por uno nuevo se encuentra tipos que no conoce, y la respuesta correcta
// es ignorarlos al plegar —no fallar—, porque un hecho que no se entiende sigue
// siendo un hecho de otro y perderlo no ayuda a nadie.
func (t EventType) IsKnown() bool {
	switch t {
	case TypeAttemptStarted, TypeStepStarted, TypeStepFinished,
		TypeCommandStarted, TypeCommandFinished, TypeParameterResolved,
		TypeArtifactProduced, TypeStaleCloneUsed, TypeSyncFailed,
		TypeAttemptFinished:
		return true
	default:
		return false
	}
}
