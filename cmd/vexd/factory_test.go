package main

// El único test de `package main`, y existe por una razón concreta: lo que se
// comprueba aquí es que el binario NO destruye nada al arrancar, y hasta la spec
// 16 sí lo hacía. `linkVexHome` hacía `os.RemoveAll($HOME/.vex)` para symlinkear
// ese directorio al volumen montado, así que ejecutar `vexd run --mode local`
// fuera del contenedor —lo más natural del mundo al depurar— borraba el `~/.vex`
// real de quien lo corriera, con sus registros de step dentro.
//
// La spec 01 lo sacó del cableado y lo dejó aquí, inalcanzable desde un test de
// `internal/`. Mover el peligro no es quitarlo; esto lo quita y lo fija.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

func TestBuildRunCommand_NoBorraElHomeDelUsuario(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	destino := filepath.Join(base, "destino")
	require.NoError(t, os.MkdirAll(destino, 0o755))

	// Un $HOME poblado como el de alguien que lleva meses usando vex: registros
	// de step con identificadores de recursos reales dentro.
	registro := filepath.Join(home, cli.VexHomeDirName, "state", "demo-app", "project", "02-supply", "01.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(registro), 0o755))
	require.NoError(t, os.WriteFile(registro, []byte(`{"variables":[{"name":"acr_name"}]}`), 0o644))

	stateConfig := filepath.Join(base, "state-config.yaml")
	require.NoError(t, os.WriteFile(stateConfig,
		[]byte("type: local\nlocal:\n  path: "+destino+"\n"), 0o644))

	t.Setenv("HOME", home)
	t.Setenv(cli.StateConfigEnvVar, "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "xdg"))

	runCmd, err := buildRunCommand(
		cli.RunArgs{StateConfigFile: stateConfig, Quiet: true},
		enginePaths{projectPath: filepath.Join(base, "proyecto")},
	)

	require.NoError(t, err)
	require.NotNil(t, runCmd)
	assert.FileExists(t, registro, "HOY lo borraba: el arranque del binario no toca el $HOME del proceso")

	// Y la otra mitad del arreglo: el área de trabajo del motor existe y está
	// FUERA del volumen, sin que nadie haya tenido que symlinkear nada.
	assert.DirExists(t, runCmd.StagingDir())
	assert.NotContains(t, runCmd.StagingDir(), filepath.Join(home, cli.VexHomeDirName))
}
