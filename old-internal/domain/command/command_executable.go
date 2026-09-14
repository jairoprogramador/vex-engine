package command

import (
	"errors"
	"time"
)

// CommandExecutable posee el ciclo de un comando y VE su error, así que es quien
// emite `command_started` y `command_finished` (spec 19 §5.1).
//
// Se emite a nivel COMANDO —A2 resuelta que sí— y el coste es volumen: los tres
// templates tienen del orden de seis comandos por step, así que un `deploy`
// produce unas decenas de hechos, no miles. A cambio se responde «¿qué comando
// falló y cuánto tardó?», que es la primera pregunta de cualquier diagnóstico.
// Recortar a nivel step se puede hacer después sin tocar el modelo; añadirlo
// después significaría no tener los datos de los meses anteriores.
type CommandExecutable struct {
	BaseExecutable
	handler CommandHandler
	facts   FactSink
}

var _ Executable = (*CommandExecutable)(nil)

func NewCommandExecutable(handler CommandHandler, facts FactSink) *CommandExecutable {
	return &CommandExecutable{
		handler: handler,
		facts:   facts,
	}
}

func (c *CommandExecutable) Execute(executionContext *ExecutionContext) error {
	stepID := executionContext.StepFullName()
	commandName := executionContext.Command().name

	var request *CommandRequestHandler
	var startedAt time.Time
	opened := false

	return c.Run(
		executionContext,
		func() error {
			// El hecho de apertura es lo PRIMERO del `before`, y el instante se toma
			// antes de emitirlo: la duración mide el comando, no lo que cueste
			// escribir su hecho.
			//
			// Aquí vivía `Emit("Comando … en ejecución")`. La línea no desaparece: se
			// DERIVA de este hecho (§5.5), que es la inversión que la spec compra.
			startedAt = executionContext.Now()
			if err := c.facts.CommandStarted(executionContext.Ctx(), CommandStartedFact{
				StepID:      stepID,
				CommandName: commandName,
			}); err != nil {
				return err
			}
			opened = true
			return nil
		},
		func() error {
			request = NewCommandRequestHandler(executionContext, executionContext.Command())
			err := c.handler.Handle(executionContext.Ctx(), request)
			if err == nil {
				request.MarkCommandSuccess()
				return nil
			}
			// El TEXTO del fallo se sigue emitiendo por el canal de líneas: es
			// diagnóstico, y su hecho equivalente —el extracto acotado y redactado—
			// es de la spec 20. Hasta entonces, ningún evento lleva stdout ni stderr.
			executionContext.Emit(err.Error())
			return err
		},
		func() error {
			return nil
		},
		// El cierre corre SIEMPRE, incluso si el `before` falló después de abrir
		// —hoy no puede, pero el par no se apoya en esa coincidencia (§5.2')—. Lo
		// único que no cierra es lo que nunca se abrió: si la apertura falló, no hay
		// par, y emitir sólo el cierre sería inventar un hueco en vez de taparlo.
		func(err error) error {
			if !opened {
				return nil
			}
			return c.facts.CommandFinished(executionContext.Ctx(), CommandFinishedFact{
				StepID:      stepID,
				CommandName: commandName,
				Status:      commandStatusOf(request, err),
				Duration:    executionContext.Now().Sub(startedAt),
				ExitCode:    commandExitCodeOf(request, err),
				Err:         err,
			})
		},
	)
}

// commandStatusOf lee el status que el request ya calculaba y NADIE leía (BL-4).
//
// El error manda sobre lo anotado por un caso real: `exec` puede terminar bien y
// fallar la limpieza después, y un comando cuyo ciclo acabó en error no terminó
// con éxito por mucho que su handler lo marcara.
func commandStatusOf(request *CommandRequestHandler, err error) CommandStatus {
	if err != nil {
		return CommandFailure
	}
	if request == nil {
		return CommandFailure
	}
	return request.CommandStatus()
}

// commandExitCodeOf es donde `CommandResult` deja de morir dentro de su handler
// (BL-30).
//
// Se propaga hasta aquí en vez de reconstruirse del error porque son dos casos y
// sólo uno lleva el número dentro del error: un comando que falló trae su código
// en `CommandFailedError`, y uno que terminó bien lo trae en su resultado. El
// tercero —el comando que ni llegó a lanzarse, con una plantilla que no se pudo
// interpolar— no tiene código propio y se reporta con el genérico.
func commandExitCodeOf(request *CommandRequestHandler, err error) int {
	if err == nil {
		if request == nil {
			return 0
		}
		return request.CommandResult().ExitCode()
	}
	var failed *CommandFailedError
	if errors.As(err, &failed) {
		return failed.ExitCode()
	}
	return DefaultFailureExitCode
}
