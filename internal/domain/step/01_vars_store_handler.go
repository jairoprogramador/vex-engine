package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// VarsStoreHandler carga lo que este step dejó en las corridas anteriores: el
// ámbito de PROYECTO primero, el del AMBIENTE en ejecución después.
//
// Lee los DOS, y eso NO contradice que el step declare uno solo (spec 13 §5.4):
//
//	Cargar:   ámbito project  →  ámbito del ambiente
//	Escribir: SOLO el ámbito declarado por el step
//
// Es lo que hace posible el caso que motiva la spec entera: dos despliegues a
// ambientes distintos comparten lo que produjo un step de ámbito `project`,
// mientras que lo que produce un step de ambiente sólo lo ve ese ambiente.
//
// Hasta la spec 13 esto eran DOS handlers, uno por ámbito, con el mismo cuerpo
// duplicado. Se colapsan en uno porque las dos cargas comparten una sola
// posición del enum `command.Origin` (`OriginState`), así que unirlas no toca el
// vocabulario de precedencia: lo único que cambia es cuántas veces se alimenta
// esa posición.
//
// El ORDEN entre las dos sigue importando, y por la regla de la spec 12 y no por
// el cableado: como los dos aportan `OriginState`, gana el que llega DESPUÉS
// —por la igualdad de `ExecutionVariableMap.Add`—, y lo específico debe llegar
// después de lo común.
//
// Lee el registro VIGENTE de cada clave, y desde la spec 28 «vigente» es una
// política y no una constante: en una ejecución normal es el último, y en un
// rollback es el que estuvo vigente en la ejecución anclada (`RecordProvider`).
//
// Que ESTA lectura pase también por el proveedor es lo que la 28 §5.3 no
// nombraba y la 25 dejó al descubierto: si el ancla alcanzara sólo al bucle de
// decisión, un rollback **decidiría** con el registro anclado y **resolvería**
// con los valores de hoy — la partición de §1 reaparecida dentro de una sola
// ejecución. Lo que este handler llena es el mapa acumulado, o sea con qué
// valores se despliega.
type VarsStoreHandler struct {
	StepBaseHandler
	records RecordProvider
}

var _ StepHandler = (*VarsStoreHandler)(nil)

func NewVarsStoreHandler(records RecordProvider) StepHandler {
	return &VarsStoreHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		records:         records,
	}
}

func (h *VarsStoreHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	projectKey, err := request.ProjectStateKey()
	if err != nil {
		return fmt.Errorf("componer la clave de estado del proyecto: %w", err)
	}
	environmentKey, err := request.EnvironmentStateKey()
	if err != nil {
		return fmt.Errorf("componer la clave de estado del ambiente: %w", err)
	}

	for _, key := range []state.Key{projectKey, environmentKey} {
		record, found, err := h.records.Current(ctx, key)
		if err != nil {
			return fmt.Errorf("cargar el estado del ámbito %s: %w", key.Scope(), err)
		}
		if !found {
			continue
		}

		// Las variables se cargan TAL COMO se guardaron. Antes se reconstruían
		// aquí con `shared=true` cableado, y era el adaptador quien decidía si la
		// marca sobrevivía al viaje: el de archivo la perdía y el de Supabase la
		// deducía del ámbito, así que el mismo proyecto producía una huella
		// distinta según dónde corriera (spec 02 §5.2). Desde la spec 13 esa marca
		// no existe, así que el modo de fallo tampoco.
		//
		// Lo mismo vale para el origen: llegan con `OriginState` puesto por el
		// adaptador de lectura, y este handler tampoco se lo fabrica (spec 12 §9.3).
		// Al colapsar los dos handlers en uno esa propiedad tenía que sobrevivir, y
		// sobrevive: aquí no se construye ninguna `Variable`.
		for _, variable := range record.Variables() {
			request.AddAccumulatedVars(variable)
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
