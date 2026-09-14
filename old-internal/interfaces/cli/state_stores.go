package cli

import (
	"os"
	"path/filepath"

	cacheDom "github.com/jairoprogramador/vex-engine/old-internal/domain/cache"
	deploymentDom "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	pipelineDom "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	stateDom "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	syncDom "github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/syncconfig"
	cacheInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/cache"
	deploymentInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/deployment"
	recordInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
	stateInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/state"
	syncInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/sync"
)

// stateStores es la FAMILIA que un destino produce: el almacén de registros, el
// índice de contenido, la cabeza del linaje y —desde la spec 21— el `Sink` que
// empuja hacia él lo que se bufferizó. Vienen juntas a propósito (Abstract
// Factory, spec 16 §5.3'): mezclar un índice local con un almacén remoto es un
// estado que no debe ser construible, porque el índice apunta a registros por
// identificador y esos identificadores sólo significan algo dentro de su almacén.
type stateStores struct {
	records  stateDom.Records
	entries  cacheDom.Entries
	lineages deploymentDom.LineageStore

	// rollbacks resuelve el ancla de un rollback (spec 28) y es la QUINTA pieza
	// de la familia. Sale del mismo `switch` que las demás por lo mismo que el
	// `Sink`, y además por una razón propia: el ancla tiene dos mitades que llegan
	// por caminos distintos —la referencia sale de los HECHOS y el valor del
	// ALMACÉN— y sólo significan lo mismo si las dos son del mismo destino. Un
	// resolutor colgado por fuera podría apuntar a otra raíz y resolvería
	// `record_id` contra un almacén que no es el que los escribió (spec 21,
	// recuadro).
	rollbacks pipelineDom.RollbackResolver

	// sink es la CUARTA pieza de la familia, y es de otra naturaleza que las tres
	// de arriba: aquéllas se leen antes de decidir y por eso van directas al
	// destino; ésta EMPUJA hacia él lo que se bufferizó en el área de trabajo
	// (spec 21 §5.1). Sale del mismo `switch` que las demás porque añadir un
	// `type` al vocabulario sin su familia tiene que fallar al arrancar, y un
	// `Sink` nuevo colgado por fuera no daría esa garantía.
	sink syncDom.Sink

	// digestSecret es el secreto local con el que se derivan las claves de
	// resumen de los parámetros (spec 20 §5.2). Sale de aquí y no del área de
	// trabajo del motor por una razón que sólo se ve en la topología real: dos
	// intentos del mismo proyecto corren en dos máquinas efímeras, y lo único que
	// comparten por configuración es el destino. Un secreto por máquina daría
	// digests incomparables **en silencio**, que es lo contrario de aquello para
	// lo que existen. Ver `recordInfra.ReadOrCreateDigestSecret`.
	//
	// **No es parte del registro** y no se sincroniza: vive en `keys/`, fuera de
	// `state/`, `cache/` y `lineage/`.
	digestSecret []byte
}

// Los dos directorios existen separados para que sus reglas de vida se vean
// desde `ls`, y desde la spec 16 cuelgan del DESTINO y no del $HOME del proceso:
//
//	state/  es la VERDAD. Un registro por ejecución real de un step, nunca
//	        sobrescrito, con los identificadores de recursos que existen de
//	        verdad en la nube. No se borra nunca (spec 11).
//	cache/  es el ÍNDICE. Derivable, desechable, y no participa en ninguna
//	        decisión: `rm -rf` sobre él no cambia lo que el motor decide.
//	lineage/ es la CABEZA de la historia de cada ambiente (spec 18). Cuelga del
//	        destino y no del área de trabajo por la misma razón que las otras
//	        dos: hay que leerla ANTES de decidir, y una historia que empieza
//	        vacía en cada máquina efímera derivaría dos veces la misma posición.
//	keys/   NO es registro y no se sincroniza nunca: guarda el secreto local del
//	        que se derivan las claves de resumen (spec 20 §5.2). Está aquí porque
//	        es lo único que dos máquinas efímeras comparten, y separado del resto
//	        para que «lo que se sincroniza» siga siendo enumerable.
//
// Las tres primeras llegan al destino DIRECTAS, y no por el `Sink`: hay que
// LEERLAS antes de decidir. Lo que la spec 21 empuja es lo otro —`objects/` y
// `events/`, que se bufferizan en el área de trabajo— y los dos directorios
// aparecen bajo el destino sólo cuando el primer empuje los crea.
const (
	stateDirName   = "state"
	cacheDirName   = "cache"
	lineageDirName = "lineage"
	keysDirName    = "keys"
)

// digestSecretFileName es el archivo del secreto local dentro de `keys/`. Lleva
// versión por lo mismo que la lleva el prefijo del resumen: cambiar de
// convención tiene que poder verse, no deducirse de que los digests dejaron de
// coincidir.
const digestSecretFileName = "digest-v1.key"

// newStateStores construye la familia del destino configurado.
//
// **`type: local` no es un Null Object.** No se puede garantizar que el volumen
// esté montado; si no lo está, escribir a ciegas dejaría al motor registrando en
// el filesystem efímero del contenedor SIN NINGUNA SEÑAL, y lo que se perdería
// no es velocidad sino el identificador del recurso que se acaba de crear en la
// nube. Por eso el destino se comprueba aquí, antes del primer step, y un fallo
// es del mismo tipo que una configuración ausente.
//
// `stagingPath` entra aquí porque el `Sink` es el paso de UN sitio a OTRO: sin
// saber de dónde lee, la familia estaría a medias. Es también la razón por la
// que el área de trabajo se resuelve antes que la familia en `BuildRunCommand`.
func newStateStores(cfg syncconfig.Config, stagingPath string) (stateStores, error) {
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
		lineagePath := filepath.Join(base, lineageDirName)
		for _, dir := range []string{statePath, cachePath, lineagePath} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return stateStores{}, inputErrorf(
					"vexd run: el destino 'local' no es escribible (%s): %w", dir, err)
			}
		}

		// El secreto se resuelve AQUÍ, con el resto de la familia y antes del
		// primer step, por lo mismo que el destino se comprueba aquí: un secreto
		// que no se puede leer ni crear no es un problema que descubrir a mitad
		// del despliegue, cuando ya hay hechos emitidos con los que no compara.
		secreto, err := recordInfra.ReadOrCreateDigestSecret(
			filepath.Join(base, keysDirName, digestSecretFileName))
		if err != nil {
			return stateStores{}, inputErrorf("vexd run: %w", err)
		}

		return stateStores{
			records:  stateInfra.NewFileRecordsRepository(statePath),
			entries:  cacheInfra.NewFileEntriesRepository(cachePath),
			lineages: deploymentInfra.NewFileLineageStore(lineagePath),
			sink:     syncInfra.NewLocalSink(stagingPath, base),

			// El resolutor del ancla lee `events/` y `objects/` del DESTINO y no del
			// área de trabajo, aunque el área sea un superconjunto y sea lo que
			// `vexd record` mira por defecto. La razón es de corrección y no de
			// preferencia: el registro anclado se resuelve contra `state/`, que está
			// aquí, y las dos mitades del ancla tienen que salir del mismo sitio. Es
			// además lo que permite volver a una ejecución de OTRA máquina que
			// compartía destino, que es la capacidad que la spec 21 hizo posible.
			rollbacks: recordInfra.NewScanRollbackResolver(base),

			digestSecret: secreto,
		}, nil

	default:
		// Inalcanzable: syncconfig.New no construye ninguna otra cosa. Está aquí
		// para que añadir un `type` al vocabulario sin añadir su familia falle al
		// arrancar en vez de construir medio motor.
		return stateStores{}, inputErrorf(
			"vexd run: destino %q sin familia de repositorios", cfg.Type())
	}
}
