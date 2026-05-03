package command

import (
	"fmt"
)

func ExtractVars(commandOutput string, outputs []CommandOutput) (map[string]string, error) {
	extractedVars := make(map[string]string)

	for _, output := range outputs {
		re := output.CompiledProbe()
		if re == nil {
			return nil, fmt.Errorf("probe de la salida '%s' tiene una regex inválida o no compilada", output.Name())
		}
		matches := re.FindStringSubmatch(commandOutput)

		if output.Name() != "" {
			if len(matches) < 2 {
				return nil, fmt.Errorf("no se encontró la variable de salida '%s' en la salida del comando. Sonda utilizada: %s",
					output.Name(), output.Probe())
			}
			if matches[1] == "" {
				return nil, fmt.Errorf("la variable de salida '%s' extrajo un valor vacío. Sonda utilizada: %s",
					output.Name(), output.Probe())
			}
			extractedVars[output.Name()] = matches[1]
		}
	}

	return extractedVars, nil
}
