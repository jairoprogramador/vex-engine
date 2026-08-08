package main

import (
	"fmt"
	"os"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// enginePaths son las dos rutas del entorno real que el binario resuelve y le
// dicta al cableado. Son lo que queda del enum `--mode` (spec 16 §5.4): una
// decide dónde vive el área de clones del motor y la otra si el proyecto ya está
// en disco o hay que clonarlo.
//
// Viven aquí y no en cli.RunArgs porque no las lee el cableado: las CONSUME
// cli.EngineConfig, que es el contrato que el harness de la spec 01 inyecta.
type enginePaths struct {
	vexHome     string
	projectPath string
}

// buildRunCommand resuelve las rutas del entorno real del binario y delega el
// cableado del motor en cli.BuildRunCommand.
//
// **Ya no muta nada del sistema del usuario.** Hasta la spec 16 esta función
// hacía `os.RemoveAll($HOME/.vex)` para symlinkear ese directorio al volumen
// montado, de modo que ejecutar `vexd run --mode local` FUERA del contenedor
// borraba el `~/.vex` real de quien lo corriera. Con la raíz de almacenamiento
// inyectable (spec 01) y el estado en un destino explícito (spec 16), el motor
// recibe las rutas y las usa: no necesita reescribir el $HOME del proceso.
func buildRunCommand(args cli.RunArgs, paths enginePaths) (*cli.RunCommand, error) {
	root := paths.vexHome
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home: %w", err)
		}
		root = home
	}

	return cli.BuildRunCommand(cli.EngineConfig{
		RootVexPath:      root,
		LocalProjectPath: paths.projectPath,
	}, args)
}
