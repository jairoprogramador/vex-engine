package cli

import (
	"os"
	"path/filepath"

	cacheDom "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	stateDom "github.com/jairoprogramador/vex-engine/internal/domain/state"
	"github.com/jairoprogramador/vex-engine/internal/domain/syncconfig"
	cacheInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
)

// stateStores es la FAMILIA que un destino produce: el almacén de registros y el
// índice de contenido. Vienen juntas a propósito (Abstract Factory, spec 16
// §5.3'): mezclar un índice local con un almacén remoto es un estado que no
// debe ser construible, porque el índice apunta a registros por identificador y
// esos identificadores sólo significan algo dentro de su almacén.
type stateStores struct {
	records stateDom.Records
	entries cacheDom.Entries
}

// Los dos directorios existen separados para que sus reglas de vida se vean
// desde `ls`, y desde la spec 16 cuelgan del DESTINO y no del $HOME del proceso:
//
//	state/  es la VERDAD. Un registro por ejecución real de un step, nunca
//	        sobrescrito, con los identificadores de recursos que existen de
//	        verdad en la nube. No se borra nunca (spec 11).
//	cache/  es el ÍNDICE. Derivable, desechable, y no participa en ninguna
//	        decisión: `rm -rf` sobre él no cambia lo que el motor decide.
const (
	stateDirName = "state"
	cacheDirName = "cache"
)

// newStateStores construye la familia del destino configurado.
//
// **`type: local` no es un Null Object.** No se puede garantizar que el volumen
// esté montado; si no lo está, escribir a ciegas dejaría al motor registrando en
// el filesystem efímero del contenedor SIN NINGUNA SEÑAL, y lo que se perdería
// no es velocidad sino el identificador del recurso que se acaba de crear en la
// nube. Por eso el destino se comprueba aquí, antes del primer step, y un fallo
// es del mismo tipo que una configuración ausente.
func newStateStores(cfg syncconfig.Config) (stateStores, error) {
	switch cfg.Type() {
	case syncconfig.TypeLocal:
		base := cfg.Path()

		info, err := os.Stat(base)
		if err != nil {
			return stateStores{}, inputErrorf(
				"vexd run: destino 'local' inaccesible en %q (¿el volumen no está montado?): %w",
				base, err)
		}
		if !info.IsDir() {
			return stateStores{}, inputErrorf(
				"vexd run: el destino 'local' %q no es un directorio", base)
		}

		statePath := filepath.Join(base, stateDirName)
		cachePath := filepath.Join(base, cacheDirName)
		for _, dir := range []string{statePath, cachePath} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return stateStores{}, inputErrorf(
					"vexd run: el destino 'local' no es escribible (%s): %w", dir, err)
			}
		}

		return stateStores{
			records: stateInfra.NewFileRecordsRepository(statePath),
			entries: cacheInfra.NewFileEntriesRepository(cachePath),
		}, nil

	default:
		// Inalcanzable: syncconfig.New no construye ninguna otra cosa. Está aquí
		// para que añadir un `type` al vocabulario sin añadir su familia falle al
		// arrancar en vez de construir medio motor.
		return stateStores{}, inputErrorf(
			"vexd run: destino %q sin familia de repositorios", cfg.Type())
	}
}
