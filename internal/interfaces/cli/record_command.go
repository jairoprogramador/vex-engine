package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/syncconfig"
	deploymentInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
	recordInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/record"
)

// `vexd record` es la superficie de INSPECCIÓN DEL PROPIO MOTOR (spec 22 §5.2).
//
// # No es una CLI de usuario, y por eso vive aquí
//
// Su función es validar lo que `vexd` escribe, **mientras todavía hay pocos datos
// permanentes emitidos**. Los objetos y los eventos no se corrigen borrando: un
// error en su forma se paga para siempre, y la única defensa es mirarlos pronto con
// una herramienta que ejecute exactamente el mismo `Fold` que ejecutará el
// consumidor real (spec 26). Un `vex log` bonito para el usuario final es otro
// proyecto y llega después.
//
// # Las dos raíces, y de cuál se lee POR DEFECTO
//
// `objects/` y `events/` existen en dos sitios y no son equivalentes: el área de
// trabajo los tiene siempre —registrar es incondicional— y el destino los tiene si
// el empuje funcionó. El área de trabajo es un SUPERCONJUNTO, porque un intento
// interrumpido no llega a empujarse nunca por la vía del cierre (spec 21 §9.11).
//
//	--from work         →  «¿qué ocurrió aquí?»       (DEFAULT)
//	--from destination  →  «¿qué se publicó?»
//
// El default es el área de trabajo y es una decisión, no un descubrimiento: lo que
// este comando existe para validar es lo que el motor ESCRIBE, y el intento que
// murió antes de empujar es justo el que más falta hace poder mirar.
//
// # Y no redacta nada, que es lo que compró la alternativa B de la spec 20
//
// El registro no contiene valores: `parameter_resolved` lleva un digest con clave
// por proyecto, no el valor. Así que aquí no hay nada que enmascarar al imprimir. Si
// algún día `record log` tuviera que redactar, la que estaría mal es la emisión.

// FromWork y FromDestination son el vocabulario cerrado de `--from`.
const (
	FromWork        = "work"
	FromDestination = "destination"
)

// RecordArgs son los flags de `vexd record`.
type RecordArgs struct {
	// StateConfigFile es la MISMA vía que usa el motor: `--state-config` o la env
	// var VEX_STATE_CONFIG. Es lo que hace que el `gc` barra el directorio que el
	// motor escribe y no uno derivado del `$HOME` de quien ejecuta la herramienta.
	StateConfigFile string

	// StagingDir es el área de trabajo, con la misma resolución que en `run`: si
	// difiriera, `record gc` podaría un búfer que no es el del motor.
	StagingDir string

	// VexHome es la raíz bajo la que vive `.vex/` —clones y workdirs—. La necesita
	// sólo el `gc`, para listar los huérfanos del almacén viejo (spec 11).
	VexHome string

	// From elige la raíz de lectura. Ver el comentario del paquete.
	From string

	// Limit acota `record history`. Cero o menos significa «todos».
	Limit int

	// Attempt acota `record log` a un intento. Cero significa «todos los que haya».
	Attempt int

	// Los selectores del `gc`. Sin ninguno, `gc` sólo INFORMA.
	Cache   bool
	Staging bool

	// All poda el índice ENTERO en vez de por edad. Es seguro por construcción: el
	// índice no participa en ninguna decisión del motor.
	All bool

	// MaxAge es la política de retención PROPIA del `gc` sobre el índice.
	//
	// Propia porque no puede heredar ninguna: el TTL global de 30 días que el motor
	// tenía lo retiró la spec 15 —hoy la caducidad la declara cada step (`max_age`)
	// y se mide contra la edad de su REGISTRO, no de la entrada de índice, que desde
	// la spec 11 ni siquiera tiene fecha—. Así que la edad se mide por el `mtime`
	// del archivo, que es lo único que hay.
	MaxAge time.Duration

	// Apply es lo que separa informar de borrar. Sin él nada se toca, y ése es el
	// default: quien decide qué se poda tiene que poder ver antes qué se podaría.
	Apply bool
}

// DefaultCacheMaxAge es la retención por defecto del índice cuando no se pide
// `--all`. No hereda de nada: ver `RecordArgs.MaxAge`.
const DefaultCacheMaxAge = 30 * 24 * time.Hour

// RecordCommand es la implementación de los subcomandos de `vexd record`.
//
// Tiene las rutas resueltas y nada más: cada subcomando construye lo que necesita.
// En particular **no construye la familia de repositorios del destino**
// (`newStateStores`), y no es una omisión — aquélla crea el secreto de los digests
// con `O_EXCL` si no está, y un comando de CONSULTA no debe acuñar el secreto de un
// proyecto por el hecho de mirarlo (spec 20 §9.1).
type RecordCommand struct {
	destino     syncconfig.Config
	destinoPath string
	stagingDir  string
	vexHome     string
}

// BuildRecordCommand lee la configuración de destino y resuelve el área de trabajo.
//
// Las dos por las MISMAS funciones que `BuildRunCommand`, que es el punto: una
// consulta que mirara otro sitio que el motor no sería una consulta, sería una
// segunda opinión.
//
// # Lo que NO hace es comprobar que el volumen esté montado
//
// El motor sí lo comprueba al arrancar, porque va a escribir. Esto va a LEER, y su
// default es el área de trabajo — que es donde queda todo cuando el volumen falla.
// Exigir el destino aquí dejaría `record show --from work` inservible **exactamente
// en la máquina cuyo empuje no llegó**, que es el caso que esta superficie existe
// para poder mirar. El destino se resuelve cuando alguien lo pide: `gc`, `rebuild` o
// `--from destination`.
func BuildRecordCommand(cfg EngineConfig, args RecordArgs) (*RecordCommand, error) {
	if cfg.RootVexPath == "" {
		return nil, fmt.Errorf("vexd record: raíz de almacenamiento vacía")
	}
	if err := validarFrom(args.From); err != nil {
		return nil, err
	}

	destino, err := readStateConfigFrom(args.StateConfigFile)
	if err != nil {
		return nil, err
	}
	stagingDir, err := resolveStagingDir(args.StagingDir, cfg.RootVexPath)
	if err != nil {
		return nil, err
	}

	return &RecordCommand{
		destino:    destino,
		stagingDir: stagingDir,
		vexHome:    cfg.RootVexPath,
	}, nil
}

// destinoDisponible es la ruta del destino, comprobada.
//
// Sólo `type: local` tiene una: `type: http` está en el vocabulario y CONGELADO
// (spec 16), y su error se distingue de una configuración malformada por
// `errors.Is(err, syncconfig.ErrCongelado)`. Un `gc` contra un destino remoto no es
// un `rm -rf` de un directorio, es otra operación entera.
//
// Se llama desde cada operación que de verdad necesita el destino, y no al cablear:
// ver `BuildRecordCommand`.
func (c *RecordCommand) destinoDisponible() (string, error) {
	if c.destinoPath != "" {
		return c.destinoPath, nil
	}
	if c.destino.Type() != syncconfig.TypeLocal {
		return "", inputErrorf(
			"vexd record: el destino %q no tiene una ruta local que inspeccionar", c.destino.Type())
	}

	base := c.destino.Path()
	info, err := os.Stat(base)
	if err != nil {
		return "", inputErrorf(
			"vexd record: destino 'local' inaccesible en %q (¿el volumen no está montado?): %w",
			base, err)
	}
	if !info.IsDir() {
		return "", inputErrorf("vexd record: el destino 'local' %q no es un directorio", base)
	}

	c.destinoPath = base
	return base, nil
}

func validarFrom(from string) error {
	switch from {
	case "", FromWork, FromDestination:
		return nil
	default:
		return inputErrorf(
			"vexd record: --from %q no está en el vocabulario (%s | %s)",
			from, FromWork, FromDestination)
	}
}

// Destino y StagingDir son las rutas con las que se cableó, para que quien las
// resolvió pueda decir cuáles son.
func (c *RecordCommand) Destino() syncconfig.Config { return c.destino }
func (c *RecordCommand) StagingDir() string         { return c.stagingDir }

// raiz es la base de lectura que `--from` elige.
//
// Devuelve error porque una de las dos ramas puede no existir: el área de trabajo se
// creó al cablear, el destino puede no estar montado. Quien pide el destino se lleva
// el diagnóstico; quien lee el área de trabajo no lo necesita.
func (c *RecordCommand) raiz(from string) (string, error) {
	if from == FromDestination {
		return c.destinoDisponible()
	}
	return c.stagingDir, nil
}

func (c *RecordCommand) projection(from string) (*recordInfra.ScanProjection, string, error) {
	base, err := c.raiz(from)
	if err != nil {
		return nil, "", err
	}
	return recordInfra.NewScanProjection(
		filepath.Join(base, recordInfra.EventsDirName),
		filepath.Join(base, deploymentInfra.ObjectsDirName)), base, nil
}

// ── record show ─────────────────────────────────────────────────────────────

// Show imprime la intención congelada de un despliegue y lo que su intento fue.
//
// Las dos cosas y no una: el objeto responde «¿qué se pretendía hacer?» y el
// pliegue «¿qué pasó?», y son las dos preguntas que un despliegue permanente tiene
// que poder contestar por separado. Un intento INTERRUMPIDO no es un error aquí —es
// la respuesta correcta, con el último step alcanzado—, que es la propiedad que
// justificó el event sourcing (spec 17).
//
// El objeto puede faltar y el intento se imprime igual. Ver `ObjectIndex.Link`: el
// enlace despliegue→objeto no está escrito en ninguna parte y se DERIVA de la regla
// `dep-v1`, así que «no consta el objeto» es una ausencia honesta y no un fallo.
func (c *RecordCommand) Show(ctx context.Context, out io.Writer, args RecordArgs, id string) error {
	deploymentID, err := domDeployment.ParseDeploymentID(strings.TrimSpace(id))
	if err != nil {
		return inputErrorf("vexd record show: %w", err)
	}

	projection, base, err := c.projection(args.From)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "despliegue %s\n", deploymentID)
	fmt.Fprintf(out, "  leído de: %s\n", base)

	object, enlazado, err := c.objetoDe(projection, base, deploymentID)
	if err != nil {
		return err
	}
	if enlazado {
		imprimirObjeto(out, object)
	} else {
		fmt.Fprintf(out, "\nno consta el objeto de este despliegue en %s\n",
			filepath.Join(base, deploymentInfra.ObjectsDirName))
	}

	strips, err := projection.Strips(&ctx, deploymentID)
	if err != nil {
		return err
	}
	if len(strips) == 0 {
		return fmt.Errorf("%w: despliegue %s", domRecord.ErrAttemptNoConsta, deploymentID)
	}
	for _, strip := range strips {
		imprimirResultado(out, strip, domRecord.Fold(strip.Events))
	}
	return nil
}

// objetoDe resuelve el objeto de UN despliegue.
//
// # Los candidatos a padre son TODOS los despliegues de la raíz, no el pedido
//
// Y es la diferencia entre funcionar y no funcionar. `deployment_id = H(content_id,
// parent)`, y **todo despliegue menos el primero de su ambiente cuelga del
// anterior**: pasar sólo el pedido deja la lista de candidatos en «la raíz y él
// mismo», así que la derivación no puede acertar nunca a partir del segundo
// despliegue. El fallo es silencioso —«no consta el objeto» es una respuesta
// legítima— y por eso hay un test con DOS corridas.
func (c *RecordCommand) objetoDe(
	projection *recordInfra.ScanProjection,
	base string,
	deploymentID domDeployment.DeploymentID) (deploymentInfra.IndexedObject, bool, error) {

	index, err := deploymentInfra.ScanObjects(
		filepath.Join(base, deploymentInfra.ObjectsDirName))
	if err != nil {
		return deploymentInfra.IndexedObject{}, false, err
	}

	// Los despliegues de la raíz son los que tienen tira. El pedido se añade aunque no
	// esté entre ellos: es el que hay que buscar.
	conocidos, err := projection.Deployments()
	if err != nil {
		return deploymentInfra.IndexedObject{}, false, err
	}
	conocidos = append(conocidos, deploymentID)

	object, enlazado := index.Link(conocidos)[deploymentID.String()]
	return object, enlazado, nil
}

// imprimirObjeto vuelca el DTO del objeto: la intención, con su `content_id`.
//
// Se imprime el DTO y no un `Content` rehidratado a propósito: rehidratarlo sería
// LOSSY —las reglas del step se serializan por su forma canónica, no por su
// gramática— y enseñar un objeto reconstruido a medias es precisamente el defecto
// que `record verify` existe para detectar, introducido por la herramienta que lo
// detecta (spec 18 §9).
func imprimirObjeto(out io.Writer, object deploymentInfra.IndexedObject) {
	dto := object.Object
	fmt.Fprintf(out, "\nintención (objeto %s)\n", object.Path)
	fmt.Fprintf(out, "  content_id:  %s\n", dto.ContentID)
	fmt.Fprintf(out, "  subject:     %s\n", dto.Subject)
	fmt.Fprintf(out, "  operación:   %s\n", dto.Operation)
	fmt.Fprintf(out, "  ambiente:    %s\n", dto.Destination)
	fmt.Fprintf(out, "  proyecto:    %s", dto.Source.Project)
	if dto.Source.ProjectCommit != "" {
		fmt.Fprintf(out, " (commit %s)", dto.Source.ProjectCommit)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  pipelinecode:%s", " "+dto.Source.Pipeline)
	if dto.Source.PipelineCommit != "" {
		fmt.Fprintf(out, " (commit %s)", dto.Source.PipelineCommit)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  formato:     schema_version %d, declarado %t\n",
		dto.Format.SchemaVersion, dto.Format.Declared)

	// «Material incompleto» es un HECHO del objeto y no una advertencia sobre él: un
	// pipelinecode sin `vexpipeline.yaml` no declara de dónde salen sus variables, y
	// el objeto se emite igual, marcado (spec 18 §5.5).
	fmt.Fprintf(out, "  completo:    %t\n", dto.Complete)

	for _, step := range dto.Steps {
		fmt.Fprintf(out, "  step %s\n", step.StepID)
		fmt.Fprintf(out, "    ámbito:       %s\n", vacioONo(step.Scope))
		// Las reglas van ENTRECOMILLADAS porque lo que el objeto guarda es su forma
		// CANÓNICA, y esa forma lleva separadores de control (`U+001E`) que en una
		// terminal son invisibles: sin las comillas, `state_changed` y sus
		// dimensiones se leen pegados y parece otro valor.
		fmt.Fprintf(out, "    reglas:       %s\n", canonicaONo(step.Rules))
		fmt.Fprintf(out, "    declaración:%s\n", " "+step.Declaration)
		for _, parameter := range step.Parameters {
			fmt.Fprintf(out, "    parámetro %s = %s\n", parameter.Name, parameter.Declaration)
		}
	}
}

// imprimirResultado vuelca el pliegue de una tira.
func imprimirResultado(out io.Writer, strip recordInfra.Strip, result domRecord.AttemptResult) {
	fmt.Fprintf(out, "\nintento %s (ejecución %s)\n",
		vacioONo(result.Attempt.String()), strip.ExecutionID())
	fmt.Fprintf(out, "  estado:      %s\n", result.Status)
	fmt.Fprintf(out, "  destino válido de rollback: %t\n", result.IsValidTarget())
	fmt.Fprintf(out, "  actor:       %s\n", vacioONo(result.Actor))
	fmt.Fprintf(out, "  runner:      %s\n", vacioONo(result.Runner))
	fmt.Fprintf(out, "  empezó:      %s\n", instante(result.StartedAt))
	fmt.Fprintf(out, "  terminó:     %s\n", instante(result.FinishedAt))
	if result.Duration > 0 {
		fmt.Fprintf(out, "  duración:    %s\n", result.Duration)
	}
	fmt.Fprintf(out, "  último step: %s\n", vacioONo(result.LastStep))
	if result.ExitCode != nil {
		fmt.Fprintf(out, "  exit code:   %d\n", *result.ExitCode)
	}
	if !result.ErrorClass.IsZero() {
		fmt.Fprintf(out, "  clase:       %s\n", result.ErrorClass)
	}
	fmt.Fprintf(out, "  comandos:    %d (%d fallidos)\n", result.Commands, result.FailedCommands)
	fmt.Fprintf(out, "  artefactos:  %d\n", result.Artifacts)
	fmt.Fprintf(out, "  clones viejos: %d\n", result.StaleClones)

	for _, step := range result.Steps {
		fmt.Fprintf(out, "  step %s: %s", step.StepID, step.Status)
		if !step.Finished {
			fmt.Fprint(out, " (abierto: nadie escribió su cierre)")
		}
		if step.Reason != "" {
			fmt.Fprintf(out, " reason=%s", step.Reason)
		}
		if step.FromCache {
			fmt.Fprint(out, " from_cache=true")
		}
		if step.Duration > 0 {
			fmt.Fprintf(out, " %s", step.Duration)
		}
		if !step.Scope.IsZero() {
			fmt.Fprintf(out, " scope=%s", step.Scope)
		}
		if step.StepFingerprint != "" {
			fmt.Fprintf(out, " fingerprint=%s", step.StepFingerprint)
		}
		if step.ExitCode != nil {
			fmt.Fprintf(out, " exit=%d", *step.ExitCode)
		}
		if !step.ErrorClass.IsZero() {
			fmt.Fprintf(out, " clase=%s", step.ErrorClass)
		}
		fmt.Fprintln(out)
		if !step.Evidence.IsZero() {
			fmt.Fprintf(out, "    evidencia: registro %s de %s, escrito por %s\n",
				step.Evidence.RecordID, instante(step.Evidence.At), step.Evidence.ExecutionID)
		}
	}
	imprimirTolerancias(out, strip)
}

// imprimirTolerancias dice lo que el lector toleró.
//
// Se imprime SIEMPRE que haya algo, en `show` y en `log`: una tira con una línea
// ilegible se lee igual —es la regla— pero leerla en silencio convertiría una
// corrupción en un resultado limpio.
func imprimirTolerancias(out io.Writer, strip recordInfra.Strip) {
	if len(strip.Ilegibles) > 0 {
		fmt.Fprintf(out, "  AVISO: %d línea(s) ilegible(s) en %s: %v\n",
			len(strip.Ilegibles), strip.Path, strip.Ilegibles)
	}
	if len(strip.Desconocidos) > 0 {
		fmt.Fprintf(out, "  AVISO: %d hecho(s) de tipo desconocido para este binario: %v\n",
			len(strip.Desconocidos), strip.Desconocidos)
	}
}

// ── record log ──────────────────────────────────────────────────────────────

// Log imprime los hechos en orden de `seq`.
//
// # Imprime el SOBRE CRUDO, no el hecho plegado
//
// Y es deliberado: un tipo que este binario no conoce —escrito por un motor más
// nuevo— se puede MOSTRAR aunque no se pueda plegar. Un `log` que sólo enseñara lo
// que entiende esconderia exactamente la línea que hay que mirar cuando el
// vocabulario creció.
//
// # Es el único sitio donde un hueco del destino se puede explicar
//
// `sync_failed` es el hecho que el registro hace sobre sí mismo, y `Fold` lo ignora
// a propósito: no cambia el resultado del intento. Así que no aparece en ningún
// `AttemptResult` y sólo se ve aquí, que es literalmente lo que la spec 21 §5.5
// prometió y dejó a deber.
func (c *RecordCommand) Log(ctx context.Context, out io.Writer, args RecordArgs, id string) error {
	deploymentID, err := domDeployment.ParseDeploymentID(strings.TrimSpace(id))
	if err != nil {
		return inputErrorf("vexd record log: %w", err)
	}

	projection, _, err := c.projection(args.From)
	if err != nil {
		return err
	}
	strips, err := projection.Strips(&ctx, deploymentID)
	if err != nil {
		return err
	}
	if len(strips) == 0 {
		return fmt.Errorf("%w: despliegue %s", domRecord.ErrAttemptNoConsta, deploymentID)
	}

	for _, strip := range strips {
		fmt.Fprintf(out, "tira %s\n", strip.Path)
		sobres := make([]recordInfra.JSONLEventDTO, 0, len(strip.Raw))
		for _, sobre := range strip.Raw {
			if args.Attempt > 0 && sobre.Attempt != args.Attempt {
				continue
			}
			sobres = append(sobres, sobre)
		}
		sort.SliceStable(sobres, func(i, j int) bool { return sobres[i].Seq < sobres[j].Seq })

		for _, sobre := range sobres {
			carga, err := json.Marshal(sobre.Payload)
			if err != nil {
				carga = []byte("{}")
			}
			fmt.Fprintf(out, "  %4d  %s  %-18s  %s\n",
				sobre.Seq, sobre.At, sobre.Type, carga)
		}
		imprimirTolerancias(out, strip)
	}
	return nil
}

// ── record history ──────────────────────────────────────────────────────────

// History lista los intentos de un ambiente, del más reciente al más antiguo, y
// MARCA cuáles son destinos válidos.
//
// La marca es el motivo de que el listado exista y no un adorno: nombrar un destino
// de rollback (spec 28) sin saber antes si ese intento sirve deja al usuario
// eligiendo uno que falla después.
func (c *RecordCommand) History(
	ctx context.Context, out io.Writer, args RecordArgs, environment string) error {

	destination, err := domDeployment.NewDestination(strings.TrimSpace(environment))
	if err != nil {
		return inputErrorf("vexd record history: %w", err)
	}

	projection, base, err := c.projection(args.From)
	if err != nil {
		return err
	}
	results, err := projection.History(&ctx, destination, args.Limit)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintf(out, "el ambiente '%s' no tiene historia en %s\n", destination, base)
		return nil
	}

	fmt.Fprintf(out, "%d intento(s) contra '%s', del más reciente al más antiguo\n",
		len(results), destination)
	for _, result := range results {
		marca := " "
		if result.IsValidTarget() {
			marca = "*"
		}
		fmt.Fprintf(out, " %s %s  intento %s  %-11s  hasta '%s'  %s\n",
			marca, result.Deployment, vacioONo(result.Attempt.String()),
			result.Status, vacioONo(result.LastStep), instante(result.StartedAt))
	}
	fmt.Fprintln(out, "\n(*) destino válido: terminó bien y todos sus steps cerraron correctos")
	return nil
}

// ── record verify ───────────────────────────────────────────────────────────

// VerifyReport es lo que la verificación encontró.
type VerifyReport struct {
	Objetos     int
	Despliegues int
	Tiras       int

	// Faults son los defectos, con el nombre del sujeto delante.
	Faults []string

	// Corruptos son los defectos que ACUSAN AL REGISTRO, separados de los que sólo
	// acusan al binario que lo lee. Es la cuenta que decide el exit code: un
	// `not_comparable` no puede convertir la actualización del motor en una alarma
	// de corrupción masiva.
	Corruptos int
}

// Verify comprueba los invariantes del registro sobre TODO lo que hay.
//
// # Es la pieza de más valor de la spec 22, y la razón de no diferirla
//
// Codifica los invariantes como algo EJECUTABLE en vez de como prosa en un
// documento: `seq` monótono sin huecos, pares abiertos y cerrados, `content_id`
// reproducible. Un invariante que no se puede comprobar no es un invariante — y es
// lo que detecta un defecto de forma mientras todavía hay pocos datos permanentes
// emitidos.
//
// # Sin argumentos verifica la tienda entera, y eso es lo que se quiere
//
// El coste de un escaneo completo es aceptable ahora y el beneficio no admite
// muestreo: un defecto en un objeto que nadie mira es tan permanente como uno en el
// que se mira.
//
// # Lo que NO recomputa: las huellas del índice
//
// El índice es desechable y no tiene invariantes que preservar. Una entrada que no
// recompute simplemente no se acierta, y el step se ejecuta.
func (c *RecordCommand) Verify(
	ctx context.Context, out io.Writer, args RecordArgs) (VerifyReport, error) {

	report := VerifyReport{}

	projection, base, err := c.projection(args.From)
	if err != nil {
		return report, err
	}
	fmt.Fprintf(out, "verificando %s\n", base)

	index, err := deploymentInfra.ScanObjects(
		filepath.Join(base, deploymentInfra.ObjectsDirName))
	if err != nil {
		return report, err
	}
	for _, ilegible := range index.Ilegibles() {
		report.anotar(domRecord.FaultObjetoIlegible, ilegible,
			"el objeto no se puede decodificar")
	}

	report.Objetos = len(index.All())
	for _, object := range index.All() {
		verdict := object.Verify()
		switch {
		case verdict.Malformed:
			// Antes que `Comparable`, y el orden importa: un objeto malformado también
			// es incomparable, y clasificarlo por eso lo dejaría fuera de la cuenta de
			// corrupción — que es la que decide el exit code.
			report.anotar(domRecord.FaultObjetoMalformado, object.ContentID(), verdict.Detail)
		case !verdict.Comparable:
			report.anotar(domRecord.FaultNoComparable, object.ContentID(), verdict.Detail)
		case !verdict.Matches:
			report.anotar(domRecord.FaultContentIDNoRecomputa, object.ContentID(), verdict.Detail)
		}
	}

	deployments, err := projection.Deployments()
	if err != nil {
		return report, err
	}
	report.Despliegues = len(deployments)
	enlace := index.Link(deployments)

	for _, id := range deployments {
		if _, enlazado := enlace[id.String()]; !enlazado {
			// «No se puede enlazar», no «no está»: el enlace se deriva de `dep-v1` y
			// derivarlo necesita el padre, así que un destino al que un empuje anterior
			// no llegó da este aviso con el objeto delante y sano. Por eso no cuenta como
			// corrupción — ver `FaultKind.EsCorrupcion`.
			report.anotar(domRecord.FaultObjetoAusente, id.String(),
				"no se puede enlazar su objeto desde aquí: falta el padre del que "+
					"deriva su posición, o el objeto no está en esta raíz")
		}

		// Una tira que no se puede ABRIR se anota y se sigue con las demás. Abortar
		// devolvería un informe vacío para el resto de la tienda, y este comando
		// existe justamente para enumerar lo que está mal: pararse en el primer
		// problema es lo contrario de lo que se le pide.
		strips, err := projection.Strips(&ctx, id)
		if err != nil {
			report.anotar(domRecord.FaultTiraIlegible, id.String(), err.Error())
			continue
		}
		for _, strip := range strips {
			report.Tiras++
			for _, linea := range strip.Ilegibles {
				report.anotar(domRecord.FaultLineaIlegible, strip.Path,
					fmt.Sprintf("la línea %d no se puede decodificar", linea))
			}
			result := domRecord.Fold(strip.Events)
			for _, fault := range domRecord.CheckStrip(strip.Events, result) {
				report.anotar(fault.Kind, strip.Path, fault.Detail)
			}
		}
	}

	for _, fault := range report.Faults {
		fmt.Fprintf(out, "  %s\n", fault)
	}
	fmt.Fprintf(out, "%d objeto(s), %d despliegue(s), %d tira(s): %d defecto(s), %d de ellos corrupción\n",
		report.Objetos, report.Despliegues, report.Tiras, len(report.Faults), report.Corruptos)
	if report.Corruptos == 0 && len(report.Faults) > 0 {
		fmt.Fprintln(out,
			"ninguno acusa al registro: son límites de este binario para verificarlo aquí")
	}
	return report, nil
}

func (r *VerifyReport) anotar(kind domRecord.FaultKind, sujeto, detalle string) {
	r.Faults = append(r.Faults, fmt.Sprintf("%s  %s: %s", kind, sujeto, detalle))
	if kind.EsCorrupcion() {
		r.Corruptos++
	}
}

// ── utilidades de impresión ─────────────────────────────────────────────────

// vacioONo dice «no consta» donde el dato no está, en vez de imprimir un hueco.
//
// La ausencia es informativa en casi todo este modelo —un `actor` vacío es el
// contrato de entrada que no lo trae, un `scope` vacío es un `step_started` de este
// motor— y una línea en blanco se lee como un error de formato.
func vacioONo(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return "(no consta)"
	}
	return valor
}

// canonicaONo imprime una forma canónica sin dejar que sus separadores de control
// se pierdan por el camino.
func canonicaONo(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return "(no consta)"
	}
	return strconv.Quote(valor)
}

func instante(at time.Time) string {
	if at.IsZero() {
		return "(no consta)"
	}
	return at.UTC().Format(time.RFC3339Nano)
}
