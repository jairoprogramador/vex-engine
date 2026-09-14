package step

import (
	"fmt"
	"slices"
	"sort"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
)

// LoadedStep es TODO lo que el pipelinecode declara sobre un step, leído una
// sola vez: sus comandos, su configuración y sus declaraciones de variables.
//
// Son los tres archivos del directorio del step —`commands.yaml`, `config.yaml`
// y `variables/<ambiente>/<paso>.yaml`— y viajan juntos porque los tres se leen
// en el mismo momento y por el mismo motivo: componer la identidad de la
// operación ANTES de ejecutar el primer comando (spec 18 §5.1).
//
// Es un struct con campos exportados y no un value object con constructor, por
// lo mismo que `cache.Material`: es material explícito y exhaustivo, y lo que se
// quiere es que añadir un archivo por step sea un cambio de tipo visible en el
// compilador.
type LoadedStep struct {
	// Commands son los comandos declarados, en su orden. Vacío es LEGÍTIMO: un
	// step sin comandos es `skipped{no_commands}` (spec 04 §5.3), que es un
	// desenlace y no un fallo de carga.
	Commands []command.Command

	// Config es el ámbito y las reglas del step. `NoStepConfig()` es legítimo:
	// un step sin `config.yaml` se ejecuta siempre y no persiste registro
	// (spec 13 §5.3).
	Config StepConfig

	// Declarations son las variables que el step declara para el ambiente en
	// ejecución, sin resolver.
	Declarations []VariableDeclaration

	// Declaration es la huella `pipe-v1` de los tres campos de arriba MÁS el
	// árbol crudo del directorio del step (spec 27 §5.1).
	//
	// Se calcula en el resolutor, con los tres archivos recién leídos y el
	// directorio a mano, y viaja aquí por la misma razón que el resto: la cadena
	// de step no lee el disco desde la spec 18, y ésta es la única pieza del
	// material que vive fuera de los archivos de declaración.
	//
	// Que se calcule ANTES de abrir el step es lo que el cambio de material de la
	// spec 27 §5.2 hace posible: con la huella en la declaración —y no en el mapa
	// acumulado resuelto— ya no hace falta haber resuelto nada para conocerla.
	Declaration fingerprint.Fingerprint
}

// LoadedPipelinecode es el material de los steps DE LA OPERACIÓN, cargado por la
// cadena de pipeline y consumido por la de step (spec 18 §5.2).
//
// # Por qué existe: una sola lectura, una sola verdad
//
// Hasta la spec 18 cada step leía sus propios archivos justo antes de
// ejecutarse, dentro de la cadena 2. Eso tenía dos consecuencias y las dos son
// del mismo defecto: un `04-deploy/commands.yaml` malformado se descubría
// DESPUÉS de haber corrido `01-test` y `02-supply` de verdad, y la identidad de
// la operación no se podía componer antes de ejecutar nada porque la mitad del
// material todavía no se había leído.
//
// Cargarlo dos veces —una para identificar y otra para ejecutar— era la
// alternativa B de §4 y se descartó: dos lecturas del mismo archivo pueden
// diferir (el clon se rehace, alguien edita), y dos fuentes de verdad para el
// mismo material es exactamente lo que un identificador por contenido no tolera.
//
// # Y por qué es un objeto compartido y no un campo del ExecutionContext
//
// Porque el `ExecutionContext` vive en `command`, y `command` no puede importar
// `step` sin un ciclo. El objeto se construye una vez en el cableado y se inyecta
// en los dos lados —el resolutor que lo llena y los handlers de step que lo
// leen—, que es la misma forma en que `state.Records` cruza las capas.
//
// No es una caché: nadie relee si falta. Pedir el material de un step que no se
// cargó es un ERROR y no una ausencia, porque el conjunto vacío significa otra
// cosa —un step sin comandos se salta— y devolverlo aquí convertiría un defecto
// del motor en un step silenciosamente saltado.
type LoadedPipelinecode struct {
	steps map[string]LoadedStep
}

func NewLoadedPipelinecode() *LoadedPipelinecode {
	return &LoadedPipelinecode{steps: make(map[string]LoadedStep, 8)}
}

// Put anota el material de un step. La clave es el nombre del DIRECTORIO con su
// prefijo de orden (`02-supply`): la identidad de un step es su ruta, la misma
// que usan `state.Key` y `deployment.StepContent`.
func (p *LoadedPipelinecode) Put(stepID string, loaded LoadedStep) {
	p.steps[stepID] = loaded
}

// Get devuelve el material ya cargado de un step.
//
// La ausencia es un error del MOTOR, no del pipelinecode: significa que la
// cadena de step llegó a un step que el resolutor no cargó, y las dos únicas
// formas de que eso pase son un recorte mal hecho de `request.Steps()` o un
// handler que se saltó al resolutor. Devolver material vacío diría «este step no
// tiene comandos», que es un desenlace legítimo y distinto.
func (p *LoadedPipelinecode) Get(stepID string) (LoadedStep, error) {
	loaded, found := p.steps[stepID]
	if !found {
		return LoadedStep{}, fmt.Errorf(
			"step: no hay material cargado para %q (cargados: %v)", stepID, p.StepIDs())
	}
	return loaded, nil
}

// StepIDs son los steps cargados, ordenados. Existe para que el error de `Get`
// diga contra qué se comparó: un «no está» sin la lista obliga a instrumentar el
// motor para entenderlo.
func (p *LoadedPipelinecode) StepIDs() []string {
	ids := make([]string, 0, len(p.steps))
	for stepID := range p.steps {
		ids = append(ids, stepID)
	}
	sort.Strings(ids)
	return ids
}

// CommandsCopy son los comandos del step, copiados: el material lo comparten
// dos cadenas, y quien lo reciba no puede alterar lo que otro step va a leer.
func (s LoadedStep) CommandsCopy() []command.Command { return slices.Clone(s.Commands) }

// DeclarationsCopy son las declaraciones del step, copiadas, por lo mismo.
func (s LoadedStep) DeclarationsCopy() []VariableDeclaration {
	return slices.Clone(s.Declarations)
}
