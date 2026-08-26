package record

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// El lector del JSONL, PUBLICADO por la spec 22.
//
// Hasta aquí sólo existía dentro de `harness_test.go`, y era deliberado: el motor
// sólo ESCRIBE hechos, así que publicar el lector antes de tener un consumidor
// habría fijado una superficie sin nadie detrás (spec 19 §9). El consumidor es
// `record log`/`record show`, y el decodificador del harness es la especificación
// ejecutable de esto: cubre los diez tipos del vocabulario.
//
// # La asimetría con el escritor, y es la mitad lectora del OCP
//
// En `ToJSONLEventDTO` un tipo sin traducción es un ERROR: el emisor estaría
// escribiendo un hecho que nadie sabe leer. Aquí es lo contrario — un tipo
// DESCONOCIDO no es un archivo corrupto, es un motor más nuevo que escribió algo
// que este binario no conoce, y la respuesta correcta es saltarlo y contarlo, no
// negarse a leer la tira. De ahí `ErrTipoDesconocido`, que es un centinela y no
// un fallo.
//
// # Y no reutiliza el lector del `LocalSink`, que no es éste
//
// Aquél lee la tira como ARCHIVO y sólo mira el `seq` del sobre: no decodifica la
// carga a propósito, para no rechazar un tipo que un motor más nuevo hubiera
// escrito (spec 21 §9). Lo que sí se hereda de él son sus dos reglas de
// tolerancia, ya probadas: línea vacía se salta, línea ilegible se salta.

// ErrTipoDesconocido marca el hecho cuyo tipo no está en el vocabulario de este
// binario.
//
// Se envuelve, no se devuelve pelado: el mensaje nombra el tipo, y quien lo
// atrapa con `errors.Is` decide si lo cuenta o lo ignora.
var ErrTipoDesconocido = errors.New("jsonl event: tipo de hecho desconocido")

// maxStripLine acota la línea que el escáner acepta, con el mismo valor que el
// empuje: una tira es un archivo que puede haber quedado a medias.
const maxStripLine = 1 << 20

// Strip es una tira LEÍDA: los hechos que se entendieron y la cuenta honesta de
// lo que no.
//
// Las dos partes viajan juntas porque la segunda no es un detalle de
// implementación: `record verify` tiene que poder decir «esta tira tiene una
// línea ilegible», y un lector que devolviera sólo los hechos habría convertido
// una corrupción en un silencio.
type Strip struct {
	// Path es el archivo del que salió, para que un diagnóstico pueda nombrarlo.
	Path string

	// Events son los hechos decodificados, en el orden del archivo.
	Events []domRecord.Event

	// Raw son los sobres tal como están, incluidos los de tipo desconocido. Es lo
	// que `record log` imprime: un tipo que este binario no sabe plegar sigue
	// siendo un hecho que se puede mostrar.
	Raw []JSONLEventDTO

	// Ilegibles son los números de línea (base 1) que no se pudieron decodificar.
	// Pueden estar EN MEDIO y no sólo al final: el empuje nunca borra nada del
	// destino, así que una línea rota se conserva y lo nuevo se anexa detrás
	// (spec 21 §9.7). Un lector que asumiera «lo ilegible está al final» leería
	// bien el área de trabajo y se caería contra el destino.
	Ilegibles []int

	// Desconocidos son los tipos que aparecieron y este binario no conoce, una
	// entrada por línea.
	Desconocidos []string
}

// ExecutionID es la ejecución que escribió la tira, que es lo que nombra el
// archivo (spec 17 §5.8).
func (s Strip) ExecutionID() string {
	name := filepath.Base(s.Path)
	return name[:len(name)-len(filepath.Ext(name))]
}

// Attempt es el intento al que pertenece la tira, o el valor cero si no se pudo
// leer ni un hecho.
func (s Strip) Attempt() domDeployment.Attempt {
	for _, event := range s.Events {
		if !event.IsZero() {
			return event.Attempt()
		}
	}
	return domDeployment.Attempt{}
}

// MaxSeq es la posición mayor que la tira alcanzó, o cero si no hay ninguna.
//
// Se mide sobre los SOBRES y no sobre los hechos decodificados, y la diferencia
// importa: una línea de tipo desconocido tiene posición y no produce hecho, así que
// contar hechos diría que la tira llegó menos lejos de lo que llegó. «Hasta dónde
// llegó» es una propiedad de las posiciones, no de lo que este binario entiende.
func (s Strip) MaxSeq() uint64 {
	var mayor uint64
	for _, sobre := range s.Raw {
		if sobre.Seq > mayor {
			mayor = sobre.Seq
		}
	}
	return mayor
}

// EstaVacia dice que el archivo existe y no tiene ni una línea aprovechable.
//
// Es un estado real y distinto de «no llegó a empujarse»: un proceso que muere entre
// crear el archivo y escribir su primer hecho deja exactamente esto, y no hay nada
// que empujar de ahí. Distinguirlo es lo que impide que un `gc` lo liste para siempre
// como si fuera un intento pendiente de publicar.
func (s Strip) EstaVacia() bool {
	return len(s.Raw) == 0 && len(s.Ilegibles) == 0
}

// ReadStrip lee la tira de un archivo.
//
// AUSENCIA es error y no una tira vacía: quien llama compuso la ruta a partir de
// algo que encontró recorriendo el directorio, así que un archivo que no está es
// una carrera o una ruta mal compuesta, no «no consta».
//
// Lo que NO es error es una tira que no se entiende del todo: líneas vacías,
// líneas ilegibles y tipos desconocidos se cuentan y la lectura sigue. Un byte
// roto no puede dejar ilegible el resto de un registro permanente.
func ReadStrip(path string) (Strip, error) {
	file, err := os.Open(path)
	if err != nil {
		return Strip{}, fmt.Errorf("jsonl event reader: abrir %s: %w", path, err)
	}
	defer file.Close()

	strip := Strip{Path: path}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStripLine)

	numero := 0
	for scanner.Scan() {
		numero++
		linea := scanner.Bytes()
		if len(bytes.TrimSpace(linea)) == 0 {
			continue
		}

		var dto JSONLEventDTO
		if err := json.Unmarshal(linea, &dto); err != nil {
			strip.Ilegibles = append(strip.Ilegibles, numero)
			continue
		}
		strip.Raw = append(strip.Raw, dto)

		event, err := FromJSONLEventDTO(dto)
		if err != nil {
			if errors.Is(err, ErrTipoDesconocido) {
				strip.Desconocidos = append(strip.Desconocidos, dto.Type)
				continue
			}
			// Un sobre que el propio modelo rechaza —sin `event_id`, con `seq` cero,
			// con un instante que no parsea— es una línea ilegible en el sentido que
			// importa: no se puede plegar. Se cuenta como tal en vez de tumbar la
			// lectura, y `verify` la reporta.
			strip.Ilegibles = append(strip.Ilegibles, numero)
			continue
		}
		strip.Events = append(strip.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return Strip{}, fmt.Errorf("jsonl event reader: leer %s: %w", path, err)
	}
	return strip, nil
}

// FromJSONLEventDTO reconstruye el hecho desde su línea.
//
// Es el inverso exacto de `ToJSONLEventDTO`, y el `switch` de abajo tiene los
// mismos diez casos: cada campo que un emisor añada y este lector no sepa leer se
// nota al recomponer el hecho, no seis meses después en el consumidor.
//
// El sobre se reconstruye entero y por los constructores del dominio, invariantes
// incluidos: un hecho que el modelo rechaza no entra en el pliegue disfrazado de
// hecho válido.
func FromJSONLEventDTO(dto JSONLEventDTO) (domRecord.Event, error) {
	if dto.SchemaVersion != jsonlEventSchemaVersion {
		return domRecord.Event{}, fmt.Errorf(
			"jsonl event: esquema de línea %d no soportado (este binario lee el %d)",
			dto.SchemaVersion, jsonlEventSchemaVersion)
	}

	payload, err := payloadFrom(dto)
	if err != nil {
		return domRecord.Event{}, err
	}

	id, err := domRecord.ParseEventID(dto.EventID)
	if err != nil {
		return domRecord.Event{}, err
	}
	seq, err := domRecord.NewSeq(dto.Seq)
	if err != nil {
		return domRecord.Event{}, err
	}
	at, err := time.Parse(eventTimeLayout, dto.At)
	if err != nil {
		return domRecord.Event{}, fmt.Errorf(
			"jsonl event: interpretar at %q: %w", dto.At, err)
	}
	attempt, err := domDeployment.NewAttempt(dto.Attempt)
	if err != nil {
		return domRecord.Event{}, err
	}

	return domRecord.NewEvent(id, seq, at, attempt, payload)
}

// payloadFrom reconstruye la carga útil de cada tipo.
//
// Los campos ausentes dan el valor cero de su tipo, y eso es exactamente lo que
// significan: el escritor OMITE los opcionales en vez de escribirlos en cero
// —`exit_code`, `evidence_from`, `error_class`— así que la ausencia de una clave
// es «no consta» y no un hecho.
func payloadFrom(dto JSONLEventDTO) (domRecord.Payload, error) {
	p := dto.Payload

	switch domRecord.EventType(dto.Type) {
	case domRecord.TypeAttemptStarted:
		id, err := domDeployment.ParseDeploymentID(texto(p, "deployment_id"))
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		rollback, err := destinoDeRollback(p["rollback_to"])
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		return domRecord.AttemptStarted{
			Deployment: id,
			Actor:      texto(p, "actor"),
			Runner:     texto(p, "runner"),
			RollbackTo: rollback,
		}, nil

	case domRecord.TypeStaleCloneUsed:
		return domRecord.StaleCloneUsed{
			Source:   texto(p, "source"),
			AgeHours: numero(p, "age_hours"),
		}, nil

	case domRecord.TypeStepStarted:
		scope, err := ambito(texto(p, "scope"))
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		return domRecord.StepStarted{
			StepID:          texto(p, "step_id"),
			Scope:           scope,
			StepFingerprint: texto(p, "step_fingerprint"),
		}, nil

	case domRecord.TypeStepFinished:
		scope, err := ambito(texto(p, "scope"))
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		evidence, err := evidencia(p["evidence_from"])
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		return domRecord.StepFinished{
			StepID:          texto(p, "step_id"),
			Scope:           scope,
			Status:          command.StepStatus(texto(p, "status")),
			Duration:        duracion(p),
			FromCache:       p["from_cache"] == true,
			Reason:          command.StepReason(texto(p, "reason")),
			StepFingerprint: texto(p, "step_fingerprint"),
			Evidence:        evidence,
			ExitCode:        entero(p, "exit_code"),
			ErrorClass:      domRecord.ErrorClass(texto(p, "error_class")),
		}, nil

	case domRecord.TypeCommandStarted:
		return domRecord.CommandStarted{
			StepID:      texto(p, "step_id"),
			CommandName: texto(p, "command"),
		}, nil

	case domRecord.TypeCommandFinished:
		return domRecord.CommandFinished{
			StepID:      texto(p, "step_id"),
			CommandName: texto(p, "command"),
			Status:      command.CommandStatus(texto(p, "status")),
			Duration:    duracion(p),
			ExitCode:    int(numero(p, "exit_code")),
			ErrorClass:  domRecord.ErrorClass(texto(p, "error_class")),
		}, nil

	case domRecord.TypeParameterResolved:
		fuente, err := origen(texto(p, "source"))
		if err != nil {
			return nil, fmt.Errorf("jsonl event: '%s': %w", dto.Type, err)
		}
		return domRecord.ParameterResolved{
			Name:   texto(p, "name"),
			Source: fuente,
			Digest: texto(p, "digest"),
		}, nil

	case domRecord.TypeArtifactProduced:
		return domRecord.ArtifactProduced{
			StepID: texto(p, "step_id"),
			Kind:   texto(p, "type"),
			Digest: texto(p, "digest"),
		}, nil

	case domRecord.TypeSyncFailed:
		return domRecord.SyncFailed{
			Destination: texto(p, "destination"),
			Cause:       texto(p, "cause"),
		}, nil

	case domRecord.TypeAttemptFinished:
		return domRecord.AttemptFinished{
			Status: domRecord.AttemptStatus(texto(p, "status")),
		}, nil

	default:
		return nil, fmt.Errorf("%w: '%s'", ErrTipoDesconocido, dto.Type)
	}
}

// destinoDeRollback reconstruye la ejecución pasada a la que este intento volvió.
//
// Su ausencia es lo normal y no un error: la inmensa mayoría de los intentos no
// son rollbacks. Lo que SÍ es un error es un `rollback_to` presente y a medias
// —sin identificador, o con `attempt: 0`—: afirmaría que hubo una vuelta atrás y
// no dejaría llegar hasta ella, que es la misma regla que gobierna
// `evidence_from`.
func destinoDeRollback(valor any) (domDeployment.RollbackTarget, error) {
	crudo, ok := valor.(map[string]any)
	if !ok {
		return domDeployment.RollbackTarget{}, nil
	}
	return domDeployment.ParseRollbackTarget(
		texto(crudo, "deployment_id"), int(numero(crudo, "attempt")))
}

// evidencia reconstruye la referencia al registro que estuvo vigente.
//
// Su ausencia es LEGÍTIMA y frecuente: un step que se ejecutó por primera vez, o
// uno que no declara ámbito, no tiene ninguno (spec 19). Devolver el valor cero
// sin error es lo que dice eso.
func evidencia(valor any) (domRecord.EvidenceRef, error) {
	crudo, ok := valor.(map[string]any)
	if !ok {
		return domRecord.EvidenceRef{}, nil
	}

	claveCruda, ok := crudo["state_key"].(map[string]any)
	if !ok {
		return domRecord.EvidenceRef{}, fmt.Errorf(
			"la evidencia no dice dónde vive el registro")
	}
	scope, err := ambito(texto(claveCruda, "scope"))
	if err != nil {
		return domRecord.EvidenceRef{}, err
	}
	key, err := domState.NewKey(texto(claveCruda, "subject"), scope, texto(claveCruda, "step_id"))
	if err != nil {
		return domRecord.EvidenceRef{}, err
	}
	recordID, err := domState.ParseRecordID(texto(crudo, "record_id"))
	if err != nil {
		return domRecord.EvidenceRef{}, err
	}
	at, err := time.Parse(eventTimeLayout, texto(crudo, "at"))
	if err != nil {
		return domRecord.EvidenceRef{}, fmt.Errorf(
			"interpretar la fecha de la evidencia %q: %w", texto(crudo, "at"), err)
	}

	evidence := domRecord.EvidenceRef{
		ExecutionID: texto(crudo, "execution_id"),
		At:          at,
		StateKey:    key,
		RecordID:    recordID,
	}

	// Los dos opcionales de la evidencia: entran cuando `state.Provenance` sepa de
	// qué despliegue salió el registro (spec 19). Hasta entonces su ausencia es una
	// verdad y no un hueco.
	if texto(crudo, "deployment_id") != "" {
		deploymentID, err := domDeployment.ParseDeploymentID(texto(crudo, "deployment_id"))
		if err != nil {
			return domRecord.EvidenceRef{}, err
		}
		evidence.Deployment = deploymentID
	}
	if _, presente := numeroDe(crudo["attempt"]); presente {
		attempt, err := domDeployment.NewAttempt(int(numero(crudo, "attempt")))
		if err != nil {
			return domRecord.EvidenceRef{}, err
		}
		evidence.Attempt = attempt
	}
	return evidence, nil
}

// ambito traduce la forma lógica del ámbito. Vacío es el ámbito cero, que es
// legítimo: desde este motor `step_started` lo escribe vacío siempre.
func ambito(text string) (domState.Scope, error) {
	if text == "" {
		return domState.Scope{}, nil
	}
	return domState.ParseScope(text)
}

// origen traduce la forma externa del `Origin` de vuelta al enum.
//
// El orden del enum ES la precedencia, así que no se puede derivar de un número:
// se compara contra el mismo `String()` que lo escribió. Un origen que este
// binario no conoce es un error de la línea y no un cero silencioso — el cero es
// `OriginDeclared`, que es un valor con significado.
func origen(nombre string) (command.Origin, error) {
	for _, candidato := range []command.Origin{
		command.OriginDeclared, command.OriginState, command.OriginInjected,
		command.OriginResolved, command.OriginRuntime,
	} {
		if candidato.String() == nombre {
			return candidato, nil
		}
	}
	return command.OriginDeclared, fmt.Errorf("origen %q desconocido", nombre)
}

// duracion traduce `duration_ms` a la unidad del dominio. La unidad de la
// serialización es el milisegundo y la del dominio `time.Duration` (spec 17 §5.2).
func duracion(payload map[string]any) time.Duration {
	return time.Duration(numero(payload, "duration_ms")) * time.Millisecond
}

func texto(payload map[string]any, clave string) string {
	valor, _ := payload[clave].(string)
	return valor
}

func numero(payload map[string]any, clave string) float64 {
	valor, _ := numeroDe(payload[clave])
	return valor
}

// entero devuelve nil cuando la clave no está, que es lo que la ausencia de
// `exit_code` significa: no hubo ningún proceso del que reportarlo. Un `0` diría
// que un proceso salió con éxito.
func entero(payload map[string]any, clave string) *int {
	valor, ok := numeroDe(payload[clave])
	if !ok {
		return nil
	}
	entero := int(valor)
	return &entero
}

// numeroDe acepta las formas en las que un número puede llegar al mapa.
//
// `encoding/json` decodifica todo número como `float64`, así que leer un archivo da
// siempre eso. Pero el mapa también puede venir del TRADUCTOR DE SALIDA sin pasar
// por disco —es lo que hace la ida y vuelta que estos dos lados comparten, y lo que
// hará un ingestor que reciba el DTO ya construido— y ahí los enteros son enteros.
//
// Aceptar las dos formas no es laxitud: un lector que sólo entendiera `float64`
// funcionaría contra archivos y devolvería ceros silenciosos contra un DTO en
// memoria, que es la peor combinación posible — pasa los tests que leen disco y
// pierde datos en el camino que nadie prueba.
func numeroDe(valor any) (float64, bool) {
	switch numero := valor.(type) {
	case float64:
		return numero, true
	case float32:
		return float64(numero), true
	case int:
		return float64(numero), true
	case int32:
		return float64(numero), true
	case int64:
		return float64(numero), true
	case uint64:
		return float64(numero), true
	case json.Number:
		convertido, err := numero.Float64()
		return convertido, err == nil
	default:
		return 0, false
	}
}
