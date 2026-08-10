package command

import (
	"context"
	"time"
)

// FactSink es por donde la cadena de COMANDO publica lo que observa.
//
// # Por qué es un puerto y no el emisor de `record` directamente
//
// Es una razón de DIRECCIÓN, no de gusto. `record` importa `command` para
// transportar `StepStatus`, `CommandStatus` y `Origin` sin duplicarlos —decisión
// de la spec 17: inventar un vocabulario paralelo habría dejado el original
// muerto y añadido una traducción que envejece—, así que la dependencia de
// vuelta sería un ciclo. El puerto se declara donde se CONSUME y lo implementa
// `record`, que es la regla de este repositorio aplicada entre dos paquetes de
// dominio en vez de entre dominio e infraestructura.
//
// Las otras dos cadenas no lo necesitan: `step` y `pipeline` importan `record`
// sin que nadie importe de vuelta, así que emiten con el `record.Emitter`.
//
// # Lo que entra son HECHOS, no eventos ya compuestos
//
// El sobre —`event_id`, `seq`, instante, intento— lo pone el emisor. Quien
// presencia entrega sólo lo que vio, y por eso estos structs llevan un `error` y
// no una `ErrorClass`: clasificar es de quien tiene el vocabulario.
type FactSink interface {
	CommandStarted(ctx *context.Context, fact CommandStartedFact) error
	CommandFinished(ctx *context.Context, fact CommandFinishedFact) error
	ParameterResolved(ctx *context.Context, fact ParameterFact) error
}

// CommandStartedFact abre un comando.
type CommandStartedFact struct {
	StepID      string
	CommandName string
}

// CommandFinishedFact cierra un comando, y es donde `CommandResult` deja de
// morir dentro de su handler (BL-30).
//
// El exit code viaja como valor y no como puntero porque **todo comando que
// termina tiene uno**: el del proceso cuando el proceso corrió, y el genérico
// cuando el comando ni llegó a lanzarse —una plantilla que no se pudo
// interpolar—. Es la diferencia con el de un step, que puede no tener ninguno.
type CommandFinishedFact struct {
	StepID      string
	CommandName string
	Status      CommandStatus
	Duration    time.Duration
	ExitCode    int

	// Err es el error tal cual se observó. La clase la deriva el adaptador, que
	// es quien tiene el vocabulario cerrado; traducir aquí obligaría a `command`
	// a conocerlo.
	Err error
}

// ParameterFact es con qué se resolvió un parámetro.
//
// Lleva el VALOR y no su resumen: la convención del digest es del registro
// (spec 20), y decidirla aquí la duplicaría en los dos sitios que emiten este
// hecho —los resolutores de declaraciones y el extractor de salidas—.
type ParameterFact struct {
	Name   string
	Source Origin
	Value  string
}

// AQUÍ NO HAY un `NoFacts`, y su ausencia es deliberada: el puerto tiene una
// sola implementación —el adaptador del registro— y los hechos de esta cadena se
// verifican de punta a punta, contra el archivo que el motor escribe. Un doble
// mudo publicado aquí sería una invitación a cablearlo, y registrar es
// incondicional (§5.6). El de la cadena de step (`step.NoFacts`) sí existe,
// porque sus tests de unidad miden lo que se PERSISTE y no lo que se emite.
