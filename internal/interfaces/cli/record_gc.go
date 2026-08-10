package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cacheDom "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	stateDom "github.com/jairoprogramador/vex-engine/internal/domain/state"
	cacheInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	deploymentInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
	recordInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/record"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	syncInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/sync"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

// El `gc`, y las reglas de vida de las tiendas hechas ejecutables (spec 22 §5.3).
//
//	objects/   permanente                    →  NUNCA
//	events/    permanente (en el DESTINO)     →  NUNCA
//	lineage/   puntero, parece desechable     →  NUNCA. Ver abajo.
//	keys/      no es registro                →  NUNCA, y ni siquiera se lista
//	state/     no se borra nunca              →  NUNCA; sólo se LISTAN los huérfanos
//	cache/     índice derivable               →  sí, por edad o entero
//	staging/   búfer de solo-anexar           →  sí, tiras con `ack` completo
//
// # `lineage/` parece desechable y no lo es
//
// Es un puntero, y los objetos y los eventos siguen ahí. Pero borrar la cabeza hace
// que el despliegue siguiente derive una posición de RAÍZ que **ya existe**, y
// `deployment_id` es permanente: dos despliegues distintos con la misma identidad,
// para siempre. Va en la fila de «nunca» (spec 18 §9).
//
// # `keys/` es peor que `state/`
//
// `<destino>/keys/digest-v1.key` es el secreto del que se deriva la clave HMAC de
// cada digest. Borrarlo no pierde un archivo: **invalida en silencio todos los
// resúmenes emitidos contra ese destino**, porque el siguiente arranque genera otro
// y ningún digest nuevo coincide con ninguno viejo. Es peor que borrar `state/`
// —donde al menos se listan los huérfanos y decide el operador— porque aquí no hay
// nada que decidir y el daño no se ve al ocurrir. Se excluye POR NOMBRE, no por
// accidente (spec 20 §9).
//
// # Por qué esto no pasa por `state.Records`
//
// Porque aquel puerto ofrece `Append`, `Last` y `Get`, y **no tiene `Delete`, ni
// `Update`, ni `List`** — que la interfaz no los ofrezca es más fuerte que
// documentar que no se deben usar (spec 11 §5.3'). El `gc` necesita su propio camino
// de lectura y borrado, FUERA del puerto que el motor usa, y es exactamente la forma
// que se quería: lo peligroso no está al alcance del bucle de ejecución.
//
// # Y el destino puede ser COMPARTIDO
//
// Dos máquinas pueden apuntar al mismo destino (spec 16 §7), así que un `gc` no es
// una operación local sobre lo que uno mismo escribió: puede estar barriendo el
// caché de otro. Para `cache/` da igual —es derivable— y para `state/` es el segundo
// motivo de que aquí se LISTE en vez de borrar.

// protegidosDelDestino son los directorios del destino que el `gc` no puede tocar,
// pase lo que pase.
//
// Es una guarda de verdad y no un comentario: `podable` la comprueba antes de cada
// borrado, así que un selector nuevo mal escrito falla en vez de barrer la verdad.
// `cache/` no está en la lista, que es justo lo que la distingue.
var protegidosDelDestino = []string{
	stateDirName, lineageDirName, keysDirName,
	deploymentInfra.ObjectsDirName, recordInfra.EventsDirName,
}

// StripInfo es una tira del área de trabajo vista por el `gc`.
type StripInfo struct {
	Path        string
	AckPath     string
	Deployment  domDeployment.DeploymentID
	ExecutionID string

	// MaxSeq es la posición mayor de la tira, y UltimoAck la que el destino
	// confirmó. La tira está confirmada cuando la segunda iguala a la primera
	// (spec 21 §9): el criterio es comprobable y no una intuición.
	MaxSeq    uint64
	UltimoAck uint64

	// AckPresente separa «no confirmada» de «NO CONSTA», que es la trampa que hay
	// que escribir: el `ack` va por (despliegue, ejecución), así que una tira sin
	// archivo de `ack` no es una tira sin confirmar — y las dos se podan igual de
	// mal. Ante la ausencia hay que empujar y comprobar, no suponer.
	AckPresente bool

	// Ilegible marca la tira con al menos una línea que no se pudo decodificar. No
	// se poda: no se puede demostrar que el destino tenga lo que hay en ella, porque
	// no se sabe qué posición ocupa.
	Ilegible bool

	// Vacia marca el archivo que existe y no tiene ni un hecho: un proceso que murió
	// entre crear la tira y escribir su primer hecho.
	//
	// **No se poda tampoco, y no por prudencia con el dato** —no hay ninguno— sino
	// porque un motor VIVO puede tenerla abierta en `O_APPEND` en este instante, y
	// borrarla dejaría sus escrituras yendo a un inodo desenlazado: se perdería el
	// intento entero, en silencio. Lo que se gana distinguiéndola es que su línea diga
	// «vacía» en vez de hacerse pasar por un intento pendiente de publicar.
	Vacia bool
}

// Confirmada es el criterio comprobable de §5.3.
func (s StripInfo) Confirmada() bool {
	return s.AckPresente && !s.Ilegible && s.MaxSeq > 0 && s.UltimoAck >= s.MaxSeq
}

// StateKeyInfo es una clave de posición del almacén, con lo que se puede decir de
// ella sin abrir el pipelinecode.
type StateKeyInfo struct {
	Dir      string
	Subject  string
	Scope    string
	StepID   string
	Records  int
	UltimoAt time.Time
}

// GCPlan es lo que el `gc` haría, y lo que hizo si se le dijo `--apply`.
type GCPlan struct {
	Aplicado bool

	CachePodables []string
	CacheTotal    int

	Confirmadas   []StripInfo
	SinConfirmar  []StripInfo
	AckHuerfanos  []string
	VarsHuerfanos []string
	Estado        []StateKeyInfo

	Borrados int
	Errores  []string
}

// GC informa de lo que se puede podar y —sólo con `--apply`— lo poda.
//
// # El default es INFORMAR, y es la decisión de seguridad de este comando
//
// Sin selectores no se toca nada: se enseña qué se podaría del índice, qué tiras
// están confirmadas y cuáles no, y qué huérfanos hay en el almacén. Quien decide qué
// se poda tiene que poder ver antes qué se podaría — y en un destino compartido, lo
// que vería podría no ser suyo.
func (c *RecordCommand) GC(ctx context.Context, out io.Writer, args RecordArgs) (GCPlan, error) {
	plan := GCPlan{Aplicado: args.Apply}

	// El `gc` SÍ necesita el destino —el índice y el almacén cuelgan de él—, así que
	// aquí es donde se exige que esté montado. `record show --from work` no lo pide
	// (ver `BuildRecordCommand`): son dos operaciones con dos necesidades.
	if _, err := c.destinoDisponible(); err != nil {
		return plan, err
	}

	maxAge := args.MaxAge
	if maxAge <= 0 {
		maxAge = DefaultCacheMaxAge
	}

	if err := c.planificarCache(&plan, args.All, maxAge); err != nil {
		return plan, err
	}
	if err := c.planificarStaging(&plan); err != nil {
		return plan, err
	}
	c.listarHuerfanos(&plan)

	if args.Apply {
		if args.Cache {
			c.podar(&plan, plan.CachePodables)
		}
		if args.Staging {
			rutas := make([]string, 0, len(plan.Confirmadas)*2+len(plan.AckHuerfanos))
			for _, strip := range plan.Confirmadas {
				rutas = append(rutas, strip.Path, strip.AckPath)
			}
			rutas = append(rutas, plan.AckHuerfanos...)
			c.podar(&plan, rutas)
		}
	}

	imprimirPlanGC(out, plan, args, maxAge)
	return plan, nil
}

// planificarCache elige qué entradas del índice se pueden podar.
//
// # «Por TTL» ya no puede heredar ninguna constante
//
// El TTL global de 30 días que el motor tenía lo RETIRÓ la spec 15: hoy la caducidad
// la declara cada step (`max_age`) y se mide contra la edad de su REGISTRO, no de la
// entrada de índice —que desde la spec 11 ni siquiera tiene fecha—. Así que la edad
// se mide por el `mtime` del archivo, que es lo único que hay, y la política es
// PROPIA del `gc`, ajena al pipelinecode.
//
// # Y «entero» es seguro por construcción
//
// Nada del motor consulta el índice para decidir: tras un `--cache --all` la
// ejecución siguiente re-ejecuta todo y ninguna variable con efecto real desaparece.
// Si desapareciera alguna, la que está mal es la clasificación de la spec 11, no
// esto.
//
// Nada se borra solo, además: una entrada no se elimina al consultarla —se sustituye
// bajo la misma clave cuando el paso vuelve a ejecutarse con éxito—, así que un
// contenido que nunca vuelve deja su archivo ahí para siempre. Es exactamente el
// caso que este `gc` existe para recoger, y desde la spec 15 el único.
func (c *RecordCommand) planificarCache(plan *GCPlan, todo bool, maxAge time.Duration) error {
	base := filepath.Join(c.destinoPath, cacheDirName)

	// `time.Now()` aquí es legítimo y no una fuga del `shared.Clock`: la prohibición
	// es del DOMINIO, donde el instante decide qué se ejecuta. Esto es la política de
	// retención de una herramienta de mantenimiento comparada contra el `mtime` de un
	// archivo, y no entra en ninguna decisión del motor.
	corte := time.Now().Add(-maxAge)

	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		plan.CacheTotal++
		if todo {
			plan.CachePodables = append(plan.CachePodables, path)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(corte) {
			plan.CachePodables = append(plan.CachePodables, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("vexd record gc: recorrer %s: %w", base, err)
	}
	sort.Strings(plan.CachePodables)
	return nil
}

// planificarStaging clasifica las tiras del área de trabajo.
//
// # Y aquí es donde se ve el trabajo que la spec 21 declaró suyo y no hizo
//
// Un intento que muere de forma no controlada deja su tira ÍNTEGRA en el área de
// trabajo y sin confirmar, y **nadie la empuja jamás**. Recogerla sería «empuja las
// tiras que nadie confirmó», y eso este comando no lo puede hacer: el empuje necesita
// el `Content` del objeto, y rehidratarlo desde el archivo es LOSSY por decisión
// explícita de la spec 18 §9 —las reglas del step se serializan por su forma
// canónica, no por su gramática—. Así que el barrido de arranque sigue pendiente y
// depende de completar la serialización de `RuleSet`, no de este `gc`.
//
// Lo que sí se hace es lo que hace falta para que ese trabajo sea posible: LISTARLAS
// y no podarlas. Una tira sin empujar que se borra es un intento que no ocurrió.
func (c *RecordCommand) planificarStaging(plan *GCPlan) error {
	eventsBase := filepath.Join(c.stagingDir, recordInfra.EventsDirName)
	ackBase := filepath.Join(c.stagingDir, syncInfra.AckDirName)

	vistas := make(map[string]struct{}, 8)
	err := filepath.WalkDir(eventsBase, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		hashDir := filepath.Dir(path)
		id, parseErr := domDeployment.ParseDeploymentID(
			filepath.Base(filepath.Dir(hashDir)) + ":" + filepath.Base(hashDir))
		if parseErr != nil {
			plan.Errores = append(plan.Errores, fmt.Sprintf(
				"%s: la ruta no compone un deployment_id, no se toca", path))
			return nil
		}

		strip, readErr := recordInfra.ReadStrip(path)
		if readErr != nil {
			plan.Errores = append(plan.Errores, readErr.Error())
			return nil
		}

		info := StripInfo{
			Path:        path,
			Deployment:  id,
			ExecutionID: strip.ExecutionID(),
			Ilegible:    len(strip.Ilegibles) > 0,
			Vacia:       strip.EstaVacia(),
			MaxSeq:      strip.MaxSeq(),
		}

		info.AckPath = filepath.Join(
			ackBase, recordInfra.StreamDir(id), info.ExecutionID+".json")
		vistas[info.AckPath] = struct{}{}
		if seq, presente := leerAck(info.AckPath); presente {
			info.AckPresente = true
			info.UltimoAck = seq
		}

		if info.Confirmada() {
			plan.Confirmadas = append(plan.Confirmadas, info)
		} else {
			plan.SinConfirmar = append(plan.SinConfirmar, info)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("vexd record gc: recorrer %s: %w", eventsBase, err)
	}

	// Un `ack` cuya tira ya no está es residuo puro: es la cosa más segura de borrar
	// de todo el motor —derivable, y su pérdida degrada a reenvío total, que es
	// idempotente—. Lo contrario exacto de `keys/`.
	ackErr := filepath.WalkDir(ackBase, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		if _, tiene := vistas[path]; !tiene {
			plan.AckHuerfanos = append(plan.AckHuerfanos, path)
		}
		return nil
	})
	if ackErr != nil && !os.IsNotExist(ackErr) {
		return fmt.Errorf("vexd record gc: recorrer %s: %w", ackBase, ackErr)
	}

	sort.Strings(plan.AckHuerfanos)
	sort.SliceStable(plan.Confirmadas, func(i, j int) bool {
		return plan.Confirmadas[i].Path < plan.Confirmadas[j].Path
	})
	sort.SliceStable(plan.SinConfirmar, func(i, j int) bool {
		return plan.SinConfirmar[i].Path < plan.SinConfirmar[j].Path
	})
	return nil
}

// leerAck lee la posición confirmada. Ausente o ilegible responden «no consta», con
// la misma política que el almacén de `ack` del motor: ante la duda, reenviar.
func leerAck(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var dto syncInfra.FileAckDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return 0, false
	}
	return dto.Seq, dto.Seq > 0
}

// listarHuerfanos recoge lo que quedó sin productor, y sólo LO LISTA.
//
// # Los huérfanos son DOS conjuntos, y sólo uno está al alcance de este binario
//
//  1. LOCAL: `<vex-home>/.vex/projects/<proyecto>/store/**/*.vars`, los archivos gob
//     del almacén viejo que la spec 11 dejó en su sitio a propósito. Inertes: nada
//     los lee ya.
//  2. REMOTO: las filas de la edge function `store-vars` escritas con el `scope`
//     antiguo. **No se pueden listar desde aquí** —viven en otra base de datos, y su
//     limpieza es de la spec 26—, y decirlo es mejor que dar una lista que parece
//     completa.
//
// # Y de `state/` se da un INVENTARIO, no una lista de huérfanos
//
// Porque «huérfano» no es una propiedad del almacén: un registro queda sin productor
// cuando su step se RENUMERA (`02-supply` → `03-supply` es un step que nunca corrió)
// o cuando deja de declarar `rules` (spec 15 §5.5), y las dos cosas se saben mirando
// el pipelinecode, que este comando no tiene. Dar por huérfano lo que no se puede
// demostrar que lo sea, en la única tienda cuya regla es «no se borra nunca», sería
// exactamente el error que la regla existe para impedir.
//
// Lo que se puede dar es qué claves hay, cuántos registros tiene cada una y de cuándo
// es el último — que es con lo que un operador decide.
func (c *RecordCommand) listarHuerfanos(plan *GCPlan) {
	base := filepath.Join(c.vexHome, VexHomeDirName, "projects")
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Un directorio ilegible se ANOTA y el recorrido sigue: «0 huérfanos» y «no
			// pude mirar» son dos respuestas distintas, y esta lista se le da a un
			// operador para que decida. Una lista incompleta que se presenta como
			// completa es peor que no dar ninguna.
			plan.Errores = append(plan.Errores, fmt.Sprintf("listar %s: %v", path, err))
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".vars" {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"store"+string(filepath.Separator)) {
			plan.VarsHuerfanos = append(plan.VarsHuerfanos, path)
		}
		return nil
	})
	// La ausencia del área de clones no es nada que reportar: es un `$HOME` en el que
	// este motor no ha corrido todavía.
	if err != nil && !os.IsNotExist(err) {
		plan.Errores = append(plan.Errores, fmt.Sprintf("recorrer %s: %v", base, err))
	}
	sort.Strings(plan.VarsHuerfanos)

	claves, err := c.inventarioDeEstado()
	if err != nil {
		plan.Errores = append(plan.Errores, err.Error())
		return
	}
	plan.Estado = claves
}

// inventarioDeEstado recorre `state/` agrupando por clave de posición.
//
// El `subject` que se muestra es el NOMBRE DE DIRECTORIO y no la url del proyecto, y
// eso es una limitación real y no una elección: el directorio es
// `<último segmento><8 del sha256 de la url>`, que no se puede invertir. La url la
// recupera `record rebuild` leyendo `lineage/` y `objects/`, que sí la llevan dentro.
func (c *RecordCommand) inventarioDeEstado() ([]StateKeyInfo, error) {
	base := filepath.Join(c.destinoPath, stateDirName)
	porClave := make(map[string]*StateKeyInfo, 8)

	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		dir := filepath.Dir(path)
		rel, relErr := filepath.Rel(base, dir)
		if relErr != nil {
			return nil
		}
		segmentos := strings.Split(rel, string(filepath.Separator))
		if len(segmentos) < 3 {
			return nil
		}

		info, ok := porClave[dir]
		if !ok {
			info = &StateKeyInfo{
				Dir:     dir,
				Subject: segmentos[0],
				Scope:   strings.Join(segmentos[1:len(segmentos)-1], ":"),
				StepID:  segmentos[len(segmentos)-1],
			}
			porClave[dir] = info
		}
		info.Records++
		if fileInfo, infoErr := entry.Info(); infoErr == nil {
			if fileInfo.ModTime().After(info.UltimoAt) {
				info.UltimoAt = fileInfo.ModTime()
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("vexd record gc: recorrer %s: %w", base, err)
	}

	claves := make([]StateKeyInfo, 0, len(porClave))
	for _, info := range porClave {
		claves = append(claves, *info)
	}
	sort.SliceStable(claves, func(i, j int) bool { return claves[i].Dir < claves[j].Dir })
	return claves, nil
}

// podar borra, comprobando ANTES que cada ruta se puede borrar.
func (c *RecordCommand) podar(plan *GCPlan, rutas []string) {
	for _, ruta := range rutas {
		if ruta == "" {
			continue
		}
		if err := c.podable(ruta); err != nil {
			plan.Errores = append(plan.Errores, err.Error())
			continue
		}
		if err := os.Remove(ruta); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			plan.Errores = append(plan.Errores, fmt.Sprintf("borrar %s: %v", ruta, err))
			continue
		}
		plan.Borrados++
	}
}

// podable es la guarda que hace ejecutable la tabla de §5.3.
//
// Comprueba lo que el `gc` NO puede tocar en vez de lo que sí: una lista de
// permitidos deja pasar lo que nadie pensó en prohibir, y aquí lo que nadie pensó en
// prohibir puede ser el único sitio donde vive el identificador de un recurso que
// existe de verdad en la nube.
//
// El área de trabajo NO está protegida —es un búfer, y `events/` ahí sí se poda—, así
// que la guarda se aplica a lo que cuelga del DESTINO. Es la misma asimetría de las
// dos raíces vista desde el lado del borrado.
func (c *RecordCommand) podable(ruta string) error {
	// Falla CERRADO si el destino no está resuelto: sin él la lista de protegidos se
	// compone contra la cadena vacía, que `filepath.Clean` convierte en «.», y la
	// guarda dejaría de guardar nada. Sólo se llega aquí desde `GC`, que lo resuelve
	// primero; esto es para que reordenarlo no abra un agujero en silencio.
	if c.destinoPath == "" {
		return fmt.Errorf(
			"vexd record gc: no se poda nada sin el destino resuelto (%s no comprobable)", ruta)
	}

	limpia := filepath.Clean(ruta)
	for _, protegido := range protegidosDelDestino {
		raiz := filepath.Join(c.destinoPath, protegido)
		if estaBajo(limpia, raiz) {
			return fmt.Errorf(
				"vexd record gc: %s cuelga de %s, cuya regla de vida es 'nunca se borra'",
				limpia, raiz)
		}
	}
	if !estaBajo(limpia, c.destinoPath) && !estaBajo(limpia, c.stagingDir) {
		return fmt.Errorf(
			"vexd record gc: %s no está ni en el destino ni en el área de trabajo", limpia)
	}
	return nil
}

func imprimirPlanGC(out io.Writer, plan GCPlan, args RecordArgs, maxAge time.Duration) {
	modo := "PLAN (nada se ha borrado; añade --apply)"
	if plan.Aplicado {
		modo = "APLICADO"
	}
	fmt.Fprintf(out, "vexd record gc — %s\n", modo)

	criterio := fmt.Sprintf("con mtime anterior a %s", maxAge)
	if args.All {
		criterio = "entero (seguro: el índice no participa en ninguna decisión)"
	}
	fmt.Fprintf(out, "\ncache/  %d entrada(s); podables %s: %d\n",
		plan.CacheTotal, criterio, len(plan.CachePodables))
	if !args.Cache && len(plan.CachePodables) > 0 {
		fmt.Fprintln(out, "        (selecciónalo con --cache)")
	}

	fmt.Fprintf(out, "\nstaging/  %d tira(s) confirmada(s), %d sin confirmar\n",
		len(plan.Confirmadas), len(plan.SinConfirmar))
	pendientes := 0
	for _, strip := range plan.SinConfirmar {
		if strip.Vacia {
			// Se dice «vacía» y no «pendiente»: no hay nada que publicar, así que
			// contarla entre las que esperan un empuje sería ruido que nunca se aclara.
			fmt.Fprintf(out, "        vacía     %s (ni un hecho: no hay nada que empujar,"+
				" y no se borra porque un motor vivo puede tenerla abierta)\n", strip.Path)
			continue
		}
		pendientes++
		motivo := fmt.Sprintf("ack %d < seq %d", strip.UltimoAck, strip.MaxSeq)
		switch {
		case strip.Ilegible:
			motivo = "tiene una línea ilegible: no se puede demostrar qué llegó"
		case !strip.AckPresente:
			motivo = "NO CONSTA su ack: hay que empujar y comprobar, no suponer"
		}
		fmt.Fprintf(out, "        pendiente %s (%s)\n", strip.Path, motivo)
	}
	if pendientes > 0 {
		fmt.Fprintln(out,
			"        nadie las empujará solo: el barrido de arranque sigue pendiente (spec 21)")
	}
	fmt.Fprintf(out, "        ack huérfanos (tira ya borrada): %d\n", len(plan.AckHuerfanos))
	if !args.Staging && (len(plan.Confirmadas) > 0 || len(plan.AckHuerfanos) > 0) {
		fmt.Fprintln(out, "        (selecciónalo con --staging)")
	}

	fmt.Fprintf(out, "\nhuérfanos del almacén viejo (spec 11), sólo se LISTAN: %d\n",
		len(plan.VarsHuerfanos))
	for _, path := range plan.VarsHuerfanos {
		fmt.Fprintf(out, "        %s\n", path)
	}
	fmt.Fprintln(out,
		"        las filas remotas de `store-vars` con el scope antiguo no se ven desde aquí (spec 26)")

	fmt.Fprintf(out, "\nstate/  %d clave(s) de posición. NUNCA se borran; el inventario es para decidir\n",
		len(plan.Estado))
	for _, clave := range plan.Estado {
		fmt.Fprintf(out, "        %s / %s / %s: %d registro(s), último %s\n",
			clave.Subject, clave.Scope, clave.StepID, clave.Records, instante(clave.UltimoAt))
	}
	fmt.Fprintln(out,
		"        qué está huérfano depende del pipelinecode (renumerar un step, retirar `rules`),")
	fmt.Fprintln(out, "        y este comando no lo tiene: la decisión es del operador")

	if plan.Aplicado {
		fmt.Fprintf(out, "\nborrados: %d archivo(s)\n", plan.Borrados)
	}
	for _, err := range plan.Errores {
		fmt.Fprintf(out, "AVISO: %s\n", err)
	}
}

// ── record rebuild ──────────────────────────────────────────────────────────

// RebuildReport es lo que la reconstrucción del índice hizo.
type RebuildReport struct {
	Registros   int
	Entradas    int
	SinHuella   int
	SinSubject  []string
	Ignorados   int
	Errores     []string
	Reconstruye string
}

// Rebuild reconstruye el índice de contenido desde `state/`.
//
// # Es trivial porque la entrada es un puntero y nada más
//
// Una entrada es `{cache_key, state_key, record_id}`, y los tres salen del almacén:
// el `record_id` NOMBRA el archivo, la `state_key` es su RUTA y la `cache_key` es el
// `step_fingerprint` que el registro guarda dentro —la huella del step ES la clave
// `ck-v1` hoy—. Recorrer y reescribir es todo.
//
// # Con una pieza que la ruta no puede dar: la url del proyecto
//
// El directorio del sujeto es `<último segmento><8 del sha256 de la url>`, que no se
// invierte. La url la llevan DENTRO `lineage/` y `objects/`, así que se construye el
// mapa `directorio → url` con las dos tiendas antes de recorrer el almacén. Un
// proyecto cuya url no aparece en ninguna no se puede indexar, y se DICE: una entrada
// con un `subject` inventado apuntaría a un registro que nadie encontraría.
//
// # No decide nada, y por eso se puede hacer sin miedo
//
// El índice no participa en ninguna decisión del motor (spec 11 §5.5). Reconstruirlo
// no cambia lo que el motor hace: devuelve el índice a los consumidores de CONSULTA,
// que son los que lo miran.
func (c *RecordCommand) Rebuild(ctx context.Context, out io.Writer) (RebuildReport, error) {
	// Reconstruye el índice del destino desde el almacén del destino: las dos tiendas
	// son suyas, así que sin volumen montado esto no tiene sujeto.
	destinoPath, err := c.destinoDisponible()
	if err != nil {
		return RebuildReport{}, err
	}
	report := RebuildReport{Reconstruye: filepath.Join(destinoPath, cacheDirName)}

	subjects, err := c.subjectsPorDirectorio()
	if err != nil {
		return report, err
	}

	entries := cacheInfra.NewFileEntriesRepository(report.Reconstruye)
	base := filepath.Join(c.destinoPath, stateDirName)
	faltantes := make(map[string]struct{}, 4)

	err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		report.Registros++

		// Los dos motivos por los que un registro no se indexa NO se colapsan: «no
		// conozco la url de este proyecto» manda a auditar `lineage/` y `objects/`, y
		// «esta ruta no compone una clave» manda a mirar la ruta. Darle el primero a
		// quien tiene el segundo cuesta una tarde buscando en el sitio equivocado.
		key, dirSujeto, claveErr := claveDeLaRuta(base, path, subjects)
		if claveErr != nil {
			if errors.Is(claveErr, errSubjectDesconocido) {
				faltantes[dirSujeto] = struct{}{}
			} else {
				report.Errores = append(report.Errores, claveErr.Error())
			}
			report.Ignorados++
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			report.Errores = append(report.Errores, readErr.Error())
			return nil
		}
		var dto stateInfra.FileStepRecordDTO
		if err := json.Unmarshal(data, &dto); err != nil {
			report.Errores = append(report.Errores, fmt.Sprintf("decodificar %s: %v", path, err))
			return nil
		}
		if dto.StepFingerprint == "" {
			// Un registro sin huella no se indexa y no es un error: es el step que se
			// ejecutó sin poder componer su material, y precisamente lo que no puede
			// hacer es revivir (spec 11 §4). Indexarlo bajo una clave incompleta lo
			// haría colisionar con cualquier otro al que le falte lo mismo.
			report.SinHuella++
			return nil
		}

		cacheKey, err := cacheDom.ParseCacheKey(dto.StepFingerprint)
		if err != nil {
			report.Errores = append(report.Errores, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		recordID, err := stateDom.ParseRecordID(dto.RecordID)
		if err != nil {
			report.Errores = append(report.Errores, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		entradaNueva, err := cacheDom.NewEntry(key, recordID)
		if err != nil {
			report.Errores = append(report.Errores, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		// `Put` desempata a favor del `record_id` MAYOR, así que el orden del recorrido
		// no cambia el resultado: el índice queda apuntando al registro más reciente de
		// cada contenido sea cual sea el orden en que se lean.
		if err := entries.Put(&ctx, cacheKey, entradaNueva); err != nil {
			report.Errores = append(report.Errores, err.Error())
			return nil
		}
		report.Entradas++
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return report, fmt.Errorf("vexd record rebuild: recorrer %s: %w", base, err)
	}

	for dir := range faltantes {
		report.SinSubject = append(report.SinSubject, dir)
	}
	sort.Strings(report.SinSubject)

	fmt.Fprintf(out, "vexd record rebuild — %s\n", report.Reconstruye)
	fmt.Fprintf(out, "  %d registro(s) leídos, %d entrada(s) escritas\n",
		report.Registros, report.Entradas)
	fmt.Fprintf(out, "  %d sin huella (no pueden revivir, no se indexan)\n", report.SinHuella)
	if report.Ignorados > 0 {
		// Se dice aunque sea cero interesante: un recorrido que ignora archivos en
		// silencio se lee como «se indexó todo».
		fmt.Fprintf(out, "  %d ignorado(s): su ruta no compone una clave de posición\n",
			report.Ignorados)
	}
	if len(report.SinSubject) > 0 {
		fmt.Fprintf(out, "  %d proyecto(s) sin url conocida: %v\n",
			len(report.SinSubject), report.SinSubject)
		fmt.Fprintln(out,
			"    su url no está en lineage/ ni en objects/, así que su clave de estado no se puede componer")
	}
	for _, err := range report.Errores {
		fmt.Fprintf(out, "  AVISO: %s\n", err)
	}
	return report, nil
}

// errSubjectDesconocido es el registro cuyo proyecto no se puede nombrar: su
// directorio no aparece ni en `lineage/` ni en `objects/`.
//
// Es un centinela y no un booleano porque es el ÚNICO motivo que se agrupa por
// proyecto y se cuenta aparte. Los demás —una ruta que no compone una clave— son
// defectos de esa ruta concreta y se reportan una por una.
var errSubjectDesconocido = errors.New("la url del proyecto no consta en lineage/ ni en objects/")

// claveDeLaRuta recompone la clave de posición desde la ruta de un registro.
//
// La ruta es `<subject-dir>/<ámbito…>/<step_id>/<record_id>.json`, y el ámbito aporta
// UNO o DOS segmentos —`project`, o `environment/<nombre>`— porque `:` es ilegal en
// rutas de Windows (spec 11 §5.4). Se recompone por la misma partición, no por un
// split del texto lógico.
//
// El error dice CUÁL de los motivos fue, y eso es lo que hace accionable el informe:
// «no conozco la url de este proyecto» y «esta ruta no compone una clave» mandan a
// mirar dos sitios distintos.
func claveDeLaRuta(
	base, path string, subjects map[string]string) (stateDom.Key, string, error) {

	rel, err := filepath.Rel(base, filepath.Dir(path))
	if err != nil {
		return stateDom.Key{}, "", fmt.Errorf("%s: no cuelga de %s: %w", path, base, err)
	}
	segmentos := strings.Split(rel, string(filepath.Separator))
	if len(segmentos) < 3 {
		return stateDom.Key{}, "", fmt.Errorf(
			"%s: su ruta no tiene la forma <proyecto>/<ámbito…>/<step>/<record_id>.json", path)
	}

	dirSujeto := segmentos[0]
	subject, conocido := subjects[dirSujeto]
	if !conocido {
		return stateDom.Key{}, dirSujeto, errSubjectDesconocido
	}

	scope, err := stateDom.ParseScope(strings.Join(segmentos[1:len(segmentos)-1], ":"))
	if err != nil {
		return stateDom.Key{}, dirSujeto, fmt.Errorf("%s: %w", path, err)
	}
	key, err := stateDom.NewKey(subject, scope, segmentos[len(segmentos)-1])
	if err != nil {
		return stateDom.Key{}, dirSujeto, fmt.Errorf("%s: %w", path, err)
	}
	return key, dirSujeto, nil
}

// subjectsPorDirectorio recupera `nombre de directorio → url del proyecto`.
//
// Las dos tiendas que llevan la url dentro son `lineage/` y `objects/`, y las dos se
// leen: la primera tiene una entrada por (proyecto, ambiente) y la segunda una por
// contenido, así que entre ellas cubren cualquier proyecto que haya desplegado alguna
// vez contra este destino. Que la lleven dentro no es casual — las dos lo hacen
// porque su ruta abrevia la url y un archivo tiene que poder decir de qué es.
func (c *RecordCommand) subjectsPorDirectorio() (map[string]string, error) {
	subjects := make(map[string]string, 8)
	anotar := func(url string) {
		if url == "" {
			return
		}
		// La MISMA función que compone la ruta al escribir. Si esto derivara el
		// nombre por su cuenta, el índice reconstruido apuntaría a un directorio que
		// no es el que el almacén usa.
		subjects[utils.GetDirNameFromUrl(url)] = url
	}

	lineageBase := filepath.Join(c.destinoPath, lineageDirName)
	err := filepath.WalkDir(lineageBase, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var dto deploymentInfra.FileLineageDTO
		if json.Unmarshal(data, &dto) == nil {
			anotar(dto.Subject)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("vexd record rebuild: recorrer %s: %w", lineageBase, err)
	}

	for _, raiz := range []string{c.destinoPath, c.stagingDir} {
		index, err := deploymentInfra.ScanObjects(
			filepath.Join(raiz, deploymentInfra.ObjectsDirName))
		if err != nil {
			return nil, err
		}
		for _, object := range index.All() {
			anotar(object.Object.Subject)
		}
	}
	return subjects, nil
}
