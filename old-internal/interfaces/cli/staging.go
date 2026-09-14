package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// El área de trabajo del motor. Hay una tensión aparente entre «el motor nunca
// tiene un default propio» y esto, y conviene enunciarla para que nadie la lea
// al revés (spec 16 §5.3):
//
//	Sin default para el DESTINO. El motor sí es dueño de su ÁREA DE TRABAJO.
//
// Registrar es incondicional: el motor escribe siempre aquí, haya volumen
// montado o no. La configuración de destino decide únicamente si, *además*, hay
// a dónde empujar lo ya escrito (spec 21).
const (
	stagingVendorDir = "vex"
	stagingLeafDir   = "staging"

	// xdgStateHomeEnv es la primera opción de la cadena, y es la del estándar:
	// lo que se escribe aquí es estado de la aplicación, no caché ni
	// configuración.
	xdgStateHomeEnv = "XDG_STATE_HOME"
)

// resolveStagingDir devuelve —creado y comprobado escribible— el directorio de
// trabajo del motor:
//
//	--staging-dir  →  $XDG_STATE_HOME/vex/staging
//	               →  $HOME/.local/state/vex/staging
//	               →  os.TempDir()/vex/staging
//
// La ruta que los planes proponían (`/var/lib/vex/…`) es INESCRIBIBLE: el
// contenedor corre como `USER vex` (uid 1001, `useradd --system`) y `/var/lib`
// es `root:root 0755`, así que el primer arranque devolvía EACCES y la escritura
// «incondicional» dejaba de serlo. De ahí que la cadena baje hasta `TempDir` y
// que cada candidato se PRUEBE en vez de darse por bueno.
//
// **Nunca bajo `<rootVexPath>/.vex`**, que es el volumen montado: si el área de
// trabajo cayera dentro del destino, un `type: local` mal configurado dejaría de
// ser detectable —el motor estaría escribiendo su búfer en el mismo sitio donde
// cree estar sincronizando— y la spec 21 no tendría desde dónde re-empujar.
func resolveStagingDir(flagDir, rootVexPath string) (string, error) {
	volumen := filepath.Join(rootVexPath, VexHomeDirName)

	// La flag es una afirmación del caller: si es inservible se le dice, no se
	// la sustituye en silencio por otra cosa.
	if flagDir != "" {
		dir := filepath.Clean(flagDir)
		if estaBajo(dir, volumen) {
			return "", inputErrorf(
				"vexd: --staging-dir %q cae bajo el volumen %q: el área de trabajo del motor"+
					" tiene que estar fuera del destino", dir, volumen)
		}
		if err := prepararDirectorio(dir); err != nil {
			return "", inputErrorf("vexd: --staging-dir %q: %w", dir, err)
		}
		return dir, nil
	}

	candidatos := make([]string, 0, 3)
	if xdg := strings.TrimSpace(os.Getenv(xdgStateHomeEnv)); xdg != "" {
		candidatos = append(candidatos, filepath.Join(xdg, stagingVendorDir, stagingLeafDir))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidatos = append(candidatos, filepath.Join(home, ".local", "state", stagingVendorDir, stagingLeafDir))
	}
	candidatos = append(candidatos, filepath.Join(os.TempDir(), stagingVendorDir, stagingLeafDir))

	intentados := make([]string, 0, len(candidatos))
	for _, dir := range candidatos {
		intentados = append(intentados, dir)
		if estaBajo(dir, volumen) {
			continue
		}
		if err := prepararDirectorio(dir); err != nil {
			continue
		}
		return dir, nil
	}

	return "", inputErrorf(
		"vexd: no hay dónde crear el área de trabajo del motor (probados: %s); pásala con"+
			" --staging-dir", strings.Join(intentados, ", "))
}

// prepararDirectorio crea el directorio y comprueba que se puede ESCRIBIR en él.
// Que `MkdirAll` no falle no basta: un directorio que ya existe sin permiso de
// escritura lo deja pasar, y el fallo aparecería a mitad de la ejecución.
func prepararDirectorio(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe, err := os.CreateTemp(dir, ".vexd-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	_ = probe.Close()
	return os.Remove(name)
}

// estaBajo responde si `path` es `root` o cuelga de él. Compara rutas limpias y
// exige el separador para que `/vexHome-otro` no cuente como dentro de
// `/vexHome`.
func estaBajo(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
