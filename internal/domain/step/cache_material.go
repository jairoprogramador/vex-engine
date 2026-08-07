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
			Name:  variable.Name(),
			Value: variable.Value(),
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
