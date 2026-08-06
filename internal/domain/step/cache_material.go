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
func NewCacheMaterial(request *StepRequestHandler, commands []command.Command) (cache.Material, error) {
	instructions, err := fingerprint.ComputeInstructions(instructionMaterialOf(commands))
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella de las instrucciones: %w", err)
	}

	variables, err := fingerprint.ComputeVariables(variableMaterialOf(request.AccumulatedVars()))
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella de las variables: %w", err)
	}

	// La huella del árbol llega ya calculada por el handler 08 del pipeline, en
	// su forma canónica CON prefijo. Se vuelve a parsear en vez de transportarse
	// como cadena para que un valor corrupto o vacío falle aquí —y el paso se
	// ejecute— en vez de colarse en la clave (spec 08 §5.3).
	code, err := fingerprint.Parse(request.ProjectStatus())
	if err != nil {
		return cache.Material{}, fmt.Errorf("huella del código del proyecto: %w", err)
	}

	return cache.Material{
		Subject:  request.ProjectUrl(),
		Pipeline: request.PipelineUrl(),
		// Hasta la spec 15 el ámbito es siempre el ambiente. El campo se llama
		// Scope y no Environment porque desde la 15 puede valer "shared", y
		// cambiarlo entonces invalidaría todas las claves ya emitidas.
		Scope:        request.Environment(),
		Step:         request.StepNameExe(),
		Instructions: instructions,
		Variables:    variables,
		Code:         code,
	}, nil
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
func variableMaterialOf(variables *command.ExecutionVariableMap) []fingerprint.VariableMaterial {
	material := make([]fingerprint.VariableMaterial, 0, len(*variables))
	for _, variable := range *variables {
		if command.IsVolatileVar(variable.Name()) {
			continue
		}
		material = append(material, fingerprint.VariableMaterial{
			Name:   variable.Name(),
			Value:  variable.Value(),
			Shared: variable.IsShared(),
		})
	}
	return material
}
