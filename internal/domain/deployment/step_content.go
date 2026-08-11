package deployment

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	"github.com/jairoprogramador/vex-engine/internal/domain/step"
)

// StepContent es lo que un step del pipelinecode DECLARA sobre sí mismo, y es
// material del objeto: instrucciones, ámbito, reglas y parámetros.
//
// No es un nodo del grafo (spec 17 §5.6): un step interno entra en la identidad
// del despliegue como material y sale a la luz por los EVENTOS, nunca como un
// objeto propio. Pedir `deploy` produce UN objeto con cuatro `StepContent`
// dentro, no cuatro objetos.
//
// # Lo que entra, y de dónde sale ya construido
//
//	step_id        el nombre del DIRECTORIO, con su prefijo de orden
//	scope+rules    el `config.yaml` del step (specs 13 y 15), ya validado
//	declaration    la huella `pipe-v1` de lo que el step declara (spec 27)
//	parameters     las declaraciones de variables (spec 14), NUNCA sus valores
//
// Ninguna de las cuatro se vuelve a parsear ni a normalizar aquí: las tres
// últimas llegan como los value objects que ya existen, y `VariableDeclaration`
// trae su `Canonical()` en uso desde la spec 14. Un segundo parseo del mismo
// archivo con otro propósito sería una segunda fuente de verdad para el mismo
// material, que es justo lo que un identificador por contenido no tolera.
// # La huella que transporta pasó de `inst-v1` a `pipe-v1`, y eso cambió todos
// los `content_id`
//
// Era la propiedad que la spec 17 compró y la spec 18 §9.12 dio por contraída:
// como el material compone sobre la forma canónica COMPLETA de la huella,
// sustituir la regla cambia todas las identidades **sin una línea de código**.
// El precio, ya pagado, es que los `content_id` emitidos antes de la spec 27
// quedan permanentes e incomparables con los de después.
//
// Lo que NO cambia es la forma canónica de `cnt-v1`: `scope`, `rules` y las
// declaraciones siguen ahí como campos, aunque `pipe-v1` los lleve también
// dentro. Quitarlos sería un `cnt-v2` —una decisión aparte, con su propio coste
// de generación huérfana— y conservarlos es inofensivo para el hash. Además son
// lo que `record show` imprime: el objeto tiene que poder responder «¿qué
// declaraba este step?» sin recomputar ninguna regla.
type StepContent struct {
	stepID      string
	config      step.StepConfig
	declaration fingerprint.Fingerprint
	parameters  []step.VariableDeclaration
}

// NewStepContent compone el material de un step.
//
// # `step_id` lleva el prefijo de orden, y eso tiene un coste conocido
//
// Es el mismo identificador que `state.Key.StepID()`: la identidad de un step ES
// SU RUTA. Renumerar `02-supply` a `03-supply` cambia el `content_id` y pierde la
// historia del step, y se acepta por lo mismo que allí — el efecto es una
// re-ejecución, nunca un despliegue omitido, y la alternativa era inventar un
// identificador estable que el autor tendría que mantener a mano.
//
// # `parameters` lleva TODAS las declaraciones, no sólo las que declaran fuente
//
// Es la decisión que la spec 17 dejó abierta, y se cierra por el lado
// conservador. El material de la huella de variables sustituye el valor sólo en
// las que declaran `resolve` porque allí el resto del mapa acumulado ya aporta
// los literales YA RESUELTOS (14 §9.5); aquí no hay mapa acumulado —el objeto se
// compone ANTES de ejecutar— así que un literal que no entrara no entraría por
// ninguna otra vía. Dos pipelinecode que sólo difieren en el valor de un literal
// declaran ejecutar cosas distintas, y deben tener identidades distintas.
func NewStepContent(
	stepID string,
	config step.StepConfig,
	declaration fingerprint.Fingerprint,
	parameters []step.VariableDeclaration) (StepContent, error) {

	if stepID == "" {
		return StepContent{}, fmt.Errorf("deployment: un step del contenido no tiene step_id")
	}
	if strings.ContainsAny(stepID, `/\`) || stepID == "." || stepID == ".." {
		return StepContent{}, fmt.Errorf("deployment: %q no puede usarse como step_id", stepID)
	}

	// La huella de la declaración es OBLIGATORIA y tiene que ser la de su regla.
	// Un step sin comandos, sin `config.yaml` y sin variables declara la nada, y
	// la huella de la nada existe: la ausencia aquí no sería «no declara nada»,
	// sería «no se pudo componer», y las dos cosas tienen consecuencias opuestas.
	if declaration.IsZero() {
		return StepContent{}, fmt.Errorf(
			"deployment: el step %q no tiene la huella de su declaración", stepID)
	}
	// La guarda de versión es deliberada y es lo contrario de la de `sf-v1`: aquí
	// el material entra en una identidad PERMANENTE, así que el día que `pipe-v1`
	// salte de versión esto tiene que fallar en compilación y en test — un cambio
	// de regla no puede colarse en un `content_id` por omisión (spec 17 §9).
	if declaration.Version() != fingerprint.DeclarationVersion {
		return StepContent{}, fmt.Errorf(
			"deployment: el step %q trae una huella %q donde se espera una %q",
			stepID, declaration.Version(), fingerprint.DeclarationVersion)
	}

	declared := make(map[string]bool, len(parameters))
	for _, parameter := range parameters {
		if parameter.Name() == "" {
			return StepContent{}, fmt.Errorf(
				"deployment: el step %q declara un parámetro sin nombre", stepID)
		}
		// Dos declaraciones del mismo nombre son un error y no «gana la última»:
		// la forma canónica ordena por nombre, así que cuál gana dependería del
		// orden de lectura del archivo y la identidad dejaría de ser reproducible.
		if declared[parameter.Name()] {
			return StepContent{}, fmt.Errorf(
				"deployment: el step %q declara el parámetro %q dos veces",
				stepID, parameter.Name())
		}
		declared[parameter.Name()] = true
	}

	sorted := slices.Clone(parameters)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name() < sorted[j].Name() })

	return StepContent{
		stepID:      stepID,
		config:      config,
		declaration: declaration,
		parameters:  sorted,
	}, nil
}

// StepID es el nombre del directorio del step, con su prefijo de orden.
func (c StepContent) StepID() string { return c.stepID }

// Config es lo que el step declara sobre sí mismo: su ámbito y sus reglas.
func (c StepContent) Config() step.StepConfig { return c.config }

// Declaration es la huella `pipe-v1` de lo que el step declara hacer: sus tres
// archivos de declaración y el árbol de su directorio.
func (c StepContent) Declaration() fingerprint.Fingerprint { return c.declaration }

// Parameters son las declaraciones de variables, ORDENADAS POR NOMBRE.
func (c StepContent) Parameters() []step.VariableDeclaration {
	return slices.Clone(c.parameters)
}

// IsProjectScoped dice si este step pertenece al sub-bloque cuyo material es
// INDEPENDIENTE DEL DESTINO (spec 17 §5.1, spec 13).
//
// Queda declarado aquí para que nadie meta el ambiente en el material de un step
// de ámbito de proyecto más adelante: son steps completos —no fases de otro—, y
// su contenido es el mismo se despliegue a `sand` o a `prod`.
func (c StepContent) IsProjectScoped() bool {
	return c.config.IsDeclared() && c.config.Scope().IsProject()
}

// IsZero indica que no hay material de step.
func (c StepContent) IsZero() bool { return c.stepID == "" }

// canonical es la forma normativa de un step dentro del material del objeto
// (`SPEC-CONTENT-v1.md` §5.2). Devuelve una línea de cabecera y una línea por
// parámetro.
func (c StepContent) canonical() []string {
	lines := []string{
		strings.Join([]string{
			strconv.Quote(c.stepID),
			strconv.Quote(c.config.Scope().String()),
			strconv.Quote(c.config.Rules().Canonical()),
			// La forma canónica COMPLETA de la huella, con su prefijo: componer
			// sobre el hash pelado haría iguales una `pipe-v1` y una `pipe-v2`
			// del mismo material.
			strconv.Quote(c.declaration.String()),
			strconv.Itoa(len(c.parameters)),
		}, canonicalFieldSep),
	}
	for _, parameter := range c.parameters {
		lines = append(lines, strings.Join([]string{
			strconv.Quote(parameter.Name()),
			// La DECLARACIÓN, nunca el valor resuelto. Es la regla de la spec 14
			// §5.3 y la precondición dura de todo el registro: un valor de
			// runtime no se puede saber por adelantado; la declaración de cómo se
			// obtiene, sí.
			strconv.Quote(parameter.Canonical()),
		}, canonicalFieldSep))
	}
	return lines
}
