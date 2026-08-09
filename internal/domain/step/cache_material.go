package step

import (
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

// NewCacheMaterial compone las siete dimensiones que determinan el resultado de
// un paso (spec 10 §5.1, `cache/SPEC-v1.md` §3).
//
// Este archivo es la traducción entre el modelo de EJECUCIÓN —`command.Command`,
// `command.Variable`, el `ProjectStatus` de la cadena de pipeline— y el material
// que las reglas de huella entienden. Vive aquí y no en `fingerprint` a
// propósito: las reglas se especifican y se validan con vectores en memoria, y
// para eso no pueden depender del modelo de ejecución. El precio es este paso
// de traducción; la ventaja es que un campo nuevo de `commands.yaml` aparece
// aquí, en el compilador, obligando a decidir si entra en la huella o no.
//
// # La regla llega como parámetro, y no se deduce del request
//
// `watched` es la regla `state_changed` que el step DECLARA (spec 15 §5.2), y es
// lo que decide si el código del proyecto entra en el material. Se pasa explícita
// —en vez de sacarla de `request.StepConfig()`, que está a mano— porque el valor
// cero de un `RuleSet` es «sin reglas», y de ahí saldría un material SIN el
// código del proyecto por omisión. Un default silencioso en esa dirección es un
// step que deja de re-ejecutarse ante un cambio de código; obligar al llamador a
// decir cuál es la regla hace que ese caso no exista.
func NewCacheMaterial(
	request *StepRequestHandler,
	commands []command.Command,
	watched StateChangedRule) (cache.Material, error) {

	instructions, err := NewInstructionsFingerprint(commands)
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella de las instrucciones: %w", err)
	}

	variables, err := fingerprint.ComputeVariables(
		variableMaterialOf(request.AccumulatedVars(), request.SourcedDeclarations()))
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella de las variables: %w", err)
	}

	material := cache.Material{
		Subject:  request.ProjectUrl(),
		Pipeline: request.PipelineUrl(),
		// Esta dimensión sigue siendo el AMBIENTE, también para un step
		// `scope: project`, y la spec 13 no la toca (§6: no toca la huella).
		//
		// Consecuencia declarada, y conviene tenerla escrita: un step de ámbito de
		// proyecto desplegado a dos ambientes escribe y lee la MISMA clave de
		// estado —eso ya funciona— pero no revive el registro del otro ambiente,
		// porque su huella lleva el ambiente aquí y además `environment` es una
		// variable del mapa acumulado. La lectura compartida, que es lo que §5.4
		// promete, sí ocurre. El salto compartido llega con la spec 27, que saca
		// las dimensiones de DIRECCIÓN de la huella (§5.2bis).
		Scope:        request.Environment(),
		Step:         request.StepNameExe(),
		Instructions: instructions,
		Variables:    variables,
	}

	// El término CONDICIONAL (spec 15 §5.2). Un step que declara
	// `state_changed: [pipeline]` afirma que su trabajo no depende del código de
	// la aplicación —crear un registro de contenedores, provisionar una red—, y
	// meterlo en su huella lo re-ejecutaría en cada commit, que es lo que vaciaba
	// de sentido haber separado el ámbito de proyecto.
	//
	// La ausencia va DECLARADA en el material, no dejando el campo a cero: es lo
	// que distingue «este step no lo vigila» de «no se pudo componer la huella»,
	// y las dos cosas tienen consecuencias opuestas.
	if !watched.WatchesProject() {
		material.CodeExcluded = true
		return material, nil
	}

	// La huella del árbol llega ya calculada por el handler 08 del pipeline, en
	// su forma canónica CON prefijo. Se vuelve a parsear en vez de transportarse
	// como cadena para que un valor corrupto o vacío falle aquí —y el paso se
	// ejecute— en vez de colarse en la clave (spec 08 §5.3).
	code, err := fingerprint.Parse(request.ProjectStatus())
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella del código del proyecto: %w", err)
	}
	material.Code = code

	return material, nil
}

// NewInstructionsFingerprint es la huella `inst-v1` de los comandos que un step
// DECLARA.
//
// Se expone porque tiene dos consumidores desde la spec 18 y las dos preguntas
// son la misma: la huella del step —material de `cache.Material`— y el material
// del objeto de despliegue —`deployment.StepContent`—. Componerla dos veces con
// dos traducciones sería la segunda fuente de verdad que un identificador por
// contenido no tolera, y la traducción es justo lo que vive en este archivo.
func NewInstructionsFingerprint(commands []command.Command) (fingerprint.Fingerprint, error) {
	return fingerprint.ComputeInstructions(instructionMaterialOf(commands))
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

// variableMaterialOf aplica el filtro de volátiles de
// `fingerprint/SPEC-VARIABLES-v1.md` §3.1 y traduce el resto.
//
// Con una sustitución, y es LA regla de la spec 14 §5.3: de una variable que el
// consumidor declaró con `resolve`, entra su DECLARACIÓN y no su valor resuelto.
//
//	entra:    name, resolve, from, key, scope, y el value literal cuando lo hay
//	no entra: el valor resuelto, el instante, el actor, el orden real
//
// Las dos mitades importan. Que el valor no entre es lo que permite conocer la
// identidad ANTES de ejecutar, que es la precondición dura de todo el registro:
// un valor de runtime no se puede saber por adelantado, pero la declaración de
// cómo se obtiene sí. Y que la declaración entre ENTERA es lo que la hace
// discriminante — es el argumento contra el marcador genérico `dynamic: true`,
// que habría producido la misma cadena para todos los casos, y una parte
// constante de un hash no aporta identidad.
//
// La regla `vars-v1` no cambia y no tiene por qué: sigue hasheando el par
// (nombre, valor) que se le da. Lo que cambia es la TRADUCCIÓN, que vive aquí
// desde siempre precisamente para que las reglas se puedan especificar y validar
// con vectores en memoria sin conocer el modelo de ejecución.
func variableMaterialOf(
	variables *command.ExecutionVariableMap,
	declarations []VariableDeclaration) []fingerprint.VariableMaterial {

	// Una declaración por nombre: el pipelinecode no puede declarar dos veces la
	// misma variable en el mismo archivo sin que la segunda gane, y eso ya lo
	// resuelve el mapa.
	declared := make(map[string]string, len(declarations))
	for _, declaration := range declarations {
		declared[declaration.Name()] = declaration.Canonical()
	}

	material := make([]fingerprint.VariableMaterial, 0, len(*variables))
	for _, variable := range *variables {
		if command.IsVolatileVar(variable.Name()) {
			continue
		}

		value := variable.Value()
		if canonical, isDeclared := declared[variable.Name()]; isDeclared {
			value = canonical
		}

		material = append(material, fingerprint.VariableMaterial{
			Name:  variable.Name(),
			Value: value,
			// `Shared` queda FIJO en su valor cero, y ésa es la forma de no tocar la
			// huella al retirar `isShared` del dominio (spec 13 §6).
			//
			// `vars-v1` está CONGELADA: su §3.2 exige tres campos por entrada, así
			// que quitar el tercero sería una regla distinta y obligaría a un
			// `vars-v2` con su propia especificación. No hace falta, porque el valor
			// que llegaba aquí era `false` para toda variable que existió: el
			// mecanismo que ponía `true` —el primer segmento de `workdir`— nunca se
			// activó en ningún template (spec 13 §1), y el ámbito de proyecto sólo
			// recibía el conjunto vacío. O sea: ninguna huella se mueve.
			//
			// El campo muere con la regla en la spec 27, que retira `vars-v1`.
		})
	}
	return material
}
