package step

import (
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

// Este archivo es la traducción entre el modelo de EJECUCIÓN —`command.Command`,
// `StepConfig`, `VariableDeclaration`— y el material que las reglas de huella
// entienden.
//
// Vive aquí y no en `fingerprint` a propósito: las reglas se especifican y se
// validan con vectores en memoria, y para eso no pueden depender del modelo de
// ejecución. El precio es este paso de traducción; la ventaja es que un campo
// nuevo de `commands.yaml` o de `config.yaml` aparece aquí, en el compilador,
// obligando a decidir si entra en la huella o no.

// NewDeclarationFingerprint es la huella `pipe-v1` de lo que un step DECLARA
// hacer: sus tres archivos de declaración, normalizados, más el árbol crudo del
// resto de su directorio (spec 27 §5.2 y §5.3).
//
// # Es EL punto de traducción, y tiene dos consumidores
//
// La huella del step (`NewStepFingerprint`, la cadena 2) y el material del
// objeto de despliegue (`deployment.StepContent`, el resolutor de la cadena 1).
// Las dos hacen la misma pregunta —«¿qué declara este step?»— así que componerla
// dos veces con dos traducciones sería la segunda fuente de verdad que un
// identificador por contenido no tolera. Sustituyó a
// `NewInstructionsFingerprint`, que existía por lo mismo (spec 18 §9).
//
// # El árbol llega como puerto, no como ruta
//
// `tree` es el árbol del directorio del step. Que sea un `TreeSource` y no una
// ruta es lo que mantiene la regla en el dominio y el recorrido de disco en
// infraestructura, y lo que permite que los vectores corran en memoria.
func NewDeclarationFingerprint(
	loaded LoadedStep, tree fingerprint.TreeSource) (fingerprint.Fingerprint, error) {

	return fingerprint.ComputeStepDeclaration(fingerprint.StepDeclarationMaterial{
		// El `scope` DECLARADO, que es contenido. La dirección que de él se deriva
		// —`state/<subject>/<scope>/<step_id>/`— es la clave, y no entra en ningún
		// hash (spec 27 §5.2bis). Son dos cosas que se llaman igual y están en
		// lados opuestos de la frontera.
		Scope: loaded.Config.Scope().String(),
		// El VALOR DE DOMINIO del conjunto de reglas, no el texto del archivo:
		// `- state_changed` y `- state_changed: [pipeline, project]` producen la
		// misma cadena, y `max_age: 60m` y `max_age: 1h` también.
		Rules:     loaded.Config.Rules().Canonical(),
		Commands:  instructionMaterialOf(loaded.Commands),
		Variables: variableMaterialOf(loaded.Declarations),
	}, tree)
}

// NewStepFingerprint es la huella `sf-v1` del step: su declaración y, si el step
// lo declara, la huella del árbol del proyecto.
//
// # La regla llega como parámetro, y no se deduce del request
//
// `watched` es la regla `state_changed` que el step DECLARA (spec 15 §5.2), y es
// lo que decide si el código del proyecto entra. Se pasa explícita —en vez de
// sacarla del `StepConfig`, que está a mano— porque el valor cero de un
// `StateChangedRule` es «no vigila el proyecto», y de ahí saldría una huella SIN
// el código del proyecto por omisión. Un default silencioso en esa dirección es
// un step que deja de re-ejecutarse ante un cambio de código; obligar al llamador
// a decir cuál es la regla hace que ese caso no exista.
func NewStepFingerprint(
	declaration fingerprint.Fingerprint,
	projectStatus string,
	watched StateChangedRule) (fingerprint.Fingerprint, error) {

	// El término CONDICIONAL (spec 15 §5.2, spec 27 §5.4). Un step que declara
	// `state_changed: [pipeline]` afirma que su trabajo no depende del código de
	// la aplicación —crear un registro de contenedores, provisionar una red—, y
	// meterlo en su huella lo re-ejecutaría en cada commit, que es lo que vaciaba
	// de sentido haber separado el ámbito de proyecto.
	//
	// La ausencia va DECLARADA, no dejando el argumento a cero: es lo que
	// distingue «este step no lo vigila» de «no se pudo componer la huella», y las
	// dos cosas tienen consecuencias opuestas.
	if !watched.WatchesProject() {
		return fingerprint.ComputeStepFingerprint(declaration, fingerprint.Fingerprint{}, true)
	}

	// La huella del árbol llega ya calculada por el handler 08 del pipeline, en
	// su forma canónica CON prefijo. Se vuelve a parsear en vez de transportarse
	// como cadena para que un valor corrupto o vacío falle aquí —y el step se
	// ejecute— en vez de colarse en la huella (spec 08 §5.3).
	project, err := fingerprint.Parse(projectStatus)
	if err != nil {
		return fingerprint.Fingerprint{}, fmt.Errorf("huella del código del proyecto: %w", err)
	}
	return fingerprint.ComputeStepFingerprint(declaration, project, false)
}

func instructionMaterialOf(commands []command.Command) []fingerprint.InstructionMaterial {
	material := make([]fingerprint.InstructionMaterial, 0, len(commands))
	for _, c := range commands {
		templates := c.TemplatePaths()
		rutas := make([]string, 0, len(templates))
		for _, template := range templates {
			rutas = append(rutas, filepath.ToSlash(template.String()))
		}

		outputs := c.Outputs()
		salidas := make([]fingerprint.OutputMaterial, 0, len(outputs))
		for _, output := range outputs {
			salidas = append(salidas, fingerprint.OutputMaterial{
				Name:  output.Name(),
				Probe: output.Probe(),
			})
		}

		material = append(material, fingerprint.InstructionMaterial{
			Name:    c.Name(),
			Cmd:     c.Cmd(),
			Workdir: filepath.ToSlash(c.Workdir().String()),
			// `show` entra (spec 10 §5.1bis). Antes no, y por eso añadir
			// `show: true` para depurar un comando no invalidaba el caché: el
			// paso se saltaba y no se imprimía nada.
			Show:      c.Show(),
			Templates: rutas,
			Outputs:   salidas,
		})
	}
	return material
}

// variableMaterialOf traduce lo que el step DECLARA en
// `variables/<ambiente>/<paso>.yaml`, y sólo eso.
//
//	entra:    name, y la forma canónica de la declaración —el literal
//	          entrecomillado, o resolve/from/key/scope—
//	no entra: el valor RESUELTO, el instante, el actor, el orden real
//
// # Lo que este cambio de material cierra, y son dos defectos de signo contrario
//
// Hasta la spec 27 el material era el MAPA ACUMULADO RESUELTO, filtrado por la
// lista de volátiles. De ahí salían los dos:
//
//   - las salidas del propio step, persistidas al terminar, se cargaban antes de
//     componer la huella de la corrida siguiente, así que **todo step con un
//     `outputs` se re-ejecutaba exactamente una vez de más**;
//   - y `step_fingerprint` transportaba, por composición, un digest SIN SAL de
//     los valores de configuración del paso — que se emite en los hechos
//     (spec 19) y se empuja al destino (spec 21). Un paso con UNA sola variable
//     no volátil dejaba ahí lo que es efectivamente el `sha256` de ese valor
//     (spec 20 §8).
//
// Con declaraciones, un valor producido en runtime **deja de entrar en
// absoluto**, y el `Sink` no filtra nada (21 §9.7): lo que no debe salir tiene
// que no entrar.
//
// # Y el filtro de volátiles desaparece de aquí sin dejar hueco
//
// Ninguna de las seis se declara nunca en un `variables/<ambiente>/<paso>.yaml`:
// las inyecta el motor. No hay nada que filtrar porque no hay nada que pueda
// entrar. `command.VolatileVarNames` sobrevive con su otra razón —el filtro de lo
// que se persiste en el registro (spec 11)— y deja de ser normativa de ninguna
// regla de huella.
//
// Por la misma vía sale `environment`, que era una variable del mapa acumulado y
// entraba en `vars-v1` por derecho propio. Era la segunda de las dos vías por las
// que el ambiente entraba en la huella de un step; la primera era la dimensión
// `Scope` de `cache.Material`. **Cerrar sólo una no habría cambiado nada**: las
// dos se cierran con la spec 27, y por eso un step con `scope: project` empieza a
// revivir entre ambientes.
func variableMaterialOf(declarations []VariableDeclaration) []fingerprint.VariableMaterial {
	material := make([]fingerprint.VariableMaterial, 0, len(declarations))
	for _, declaration := range declarations {
		material = append(material, fingerprint.VariableMaterial{
			Name:        declaration.Name(),
			Declaration: declaration.Canonical(),
		})
	}
	return material
}
