package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

const vexHomeMountPoint = "/vexHome"

// buildRunCommand resuelve las rutas del entorno real del binario y delega el
// cableado del motor en cli.BuildRunCommand. Todo lo que muta el sistema del
// usuario —el symlink de $HOME/.vex— vive aquí y no en el factory, para que el
// cableado pueda ensamblarse en un test contra un directorio temporal.
func buildRunCommand(args cli.RunArgs) (*cli.RunCommand, error) {
	if err := cli.ValidateMode(args.Mode); err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}

	if args.Mode == cli.ModeLocal {
		if err := linkVexHome(home); err != nil {
			return nil, err
		}
	}

	return cli.BuildRunCommand(cli.EngineConfig{RootVexPath: home}, args)
}

func linkVexHome(homeDir string) error {
	vexDir := filepath.Join(homeDir, cli.VexHomeDirName)

	if err := os.RemoveAll(vexDir); err != nil {
		return fmt.Errorf("link vex home: eliminar %q: %w", vexDir, err)
	}

	if err := os.Symlink(vexHomeMountPoint, vexDir); err != nil {
		return fmt.Errorf("link vex home: crear symlink %q → %q: %w", vexDir, vexHomeMountPoint, err)
	}

	return nil
}
