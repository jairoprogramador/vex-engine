package record

import (
	"fmt"
	"time"

	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
)

// jsonlEventSchemaVersion versiona la forma de CADA LÍNEA, no el vocabulario de
// hechos. Un tipo de evento nuevo no la mueve: el vocabulario es cerrado en
// valores y abierto en extensión, y esto describe el sobre.
const jsonlEventSchemaVersion = 1

// eventTimeLayout es RFC3339Nano porque va y vuelve sin perder precisión.
const eventTimeLayout = time.RFC3339Nano

// JSONLEventDTO es una línea del archivo de hechos.
//
// El `deployment_id` NO está en el sobre: viaja en la carga de `attempt_started`
// y en la RUTA del archivo (spec 17). Repetirlo en cada hecho sería guardar el
// mismo dato tantas veces como hechos haya y dejar que las copias se
// contradigan.
type JSONLEventDTO struct {
	SchemaVersion int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	Seq           uint64 `json:"seq"`
	At            string `json:"at"`
	Attempt       int    `json:"attempt"`
	Type          string `json:"type"`

	// Payload es lo propio de cada tipo. Es un mapa y no una unión de structs
	// porque el lector genérico —el pliegue del backend (spec 26), `record log`
	// (spec 22)— tiene que poder leer una línea de un tipo que no conoce sin
	// fallar, y eso lo da un mapa y no un tipo cerrado.
	Payload map[string]any `json:"payload"`
}

// ToJSONLEventDTO traduce un hecho a su línea.
//
// # El vocabulario serializado crece con quien lo emite
//
// La spec 18 traducía DOS hechos —`attempt_started` y `stale_clone_used`—
// porque eran los dos que ocurrían en su capa, y dejaba el resto sin forma
// serializada a propósito: fijar el formato de un hecho sin ver el dato que lo
// llena es cómo se congelan campos que luego no encajan. La spec 19 pone los
// emisores de los otros ocho, así que aquí entran sus ocho casos.
//
// Un tipo sin traducción sigue siendo un ERROR y no una línea vacía: el emisor
// estaría escribiendo un hecho que nadie sabe leer, y un registro con líneas
// mudas es peor que uno incompleto —el pliegue las cuenta y no dicen nada—. El
// compilador no ayuda aquí, así que el `default` es lo que avisa.
//
// # Los campos opcionales se OMITEN, no se escriben en cero
//
// `exit_code`, `evidence_from` y `error_class` sólo aparecen cuando existen. Es
// la misma disciplina del resto del paquete: un `exit_code: 0` en un step que
// revivió afirmaría que un proceso salió con éxito, y no hubo ninguno. La
// ausencia de una clave es «no consta»; su presencia es un hecho.
func ToJSONLEventDTO(event domRecord.Event) (JSONLEventDTO, error) {
	if event.IsZero() {
		return JSONLEventDTO{}, fmt.Errorf("jsonl event: no hay hecho que escribir")
	}

	payload, err := payloadOf(event.Payload())
	if err != nil {
		return JSONLEventDTO{}, err
	}

	return JSONLEventDTO{
		SchemaVersion: jsonlEventSchemaVersion,
		EventID:       event.ID().String(),
		Seq:           event.Seq().Position(),
		At:            event.At().UTC().Format(eventTimeLayout),
		Attempt:       event.Attempt().Number(),
		Type:          event.Type().String(),
		Payload:       payload,
	}, nil
}

func payloadOf(payload domRecord.Payload) (map[string]any, error) {
	switch carga := payload.(type) {
	case domRecord.AttemptStarted:
		// Es el único hecho que lleva el `deployment_id`, porque es el que lo
		// acaba de derivar. `actor` y `runner` son CIRCUNSTANCIA y por eso están
		// aquí y no en el objeto: quién lanzó el despliegue y sobre qué máquina
		// corrió no cambian qué se pretendía hacer.
		return map[string]any{
			"deployment_id": carga.Deployment.String(),
			"actor":         carga.Actor,
			"runner":        carga.Runner,
		}, nil

	case domRecord.StaleCloneUsed:
		return map[string]any{
			"source":    carga.Source,
			"age_hours": carga.AgeHours,
		}, nil

	case domRecord.StepStarted:
		// El ámbito y la huella van como CAMPOS y no dentro de un hash, que es el
		// rename que la spec 19 anunciaba y la 17 dejó hecho: un consumidor filtra
		// por ámbito sin descomponer nada. Desde este motor van vacías —el dato
		// existe dentro de la cadena, y este hecho se emite antes de entrar— y
		// viajan en el cierre.
		return map[string]any{
			"step_id":          carga.StepID,
			"scope":            carga.Scope.String(),
			"step_fingerprint": carga.StepFingerprint,
		}, nil

	case domRecord.StepFinished:
		return stepFinishedPayload(carga), nil

	case domRecord.CommandStarted:
		return map[string]any{
			"step_id": carga.StepID,
			"command": carga.CommandName,
		}, nil

	case domRecord.CommandFinished:
		// El exit code va SIEMPRE y como número: todo comando que termina tiene
		// uno, y el cero significa lo que significa. Es la diferencia con el de un
		// step, que puede no tener ninguno.
		linea := map[string]any{
			"step_id":     carga.StepID,
			"command":     carga.CommandName,
			"status":      carga.Status.String(),
			"duration_ms": durationMillis(carga.Duration),
			"exit_code":   carga.ExitCode,
		}
		if !carga.ErrorClass.IsZero() {
			linea["error_class"] = carga.ErrorClass.String()
		}
		return linea, nil

	case domRecord.ParameterResolved:
		// UNO POR PARÁMETRO (N-3). El `source` es el `Origin` cuyo orden ES la
		// precedencia, y viaja como dato: el consumidor deriva de él quién ganó sin
		// reconstruir el cableado del motor.
		return map[string]any{
			"name":   carga.Name,
			"source": carga.Source.String(),
			"digest": carga.Digest,
		}, nil

	case domRecord.ArtifactProduced:
		// `type` en la línea y `Kind` en Go: el nombre del campo es del vocabulario
		// de §5.2 y el del método choca con la interfaz.
		return map[string]any{
			"step_id": carga.StepID,
			"type":    carga.Kind,
			"digest":  carga.Digest,
		}, nil

	case domRecord.SyncFailed:
		return map[string]any{
			"destination": carga.Destination,
			"cause":       carga.Cause,
		}, nil

	case domRecord.AttemptFinished:
		return map[string]any{
			"status": carga.Status.String(),
		}, nil

	default:
		return nil, fmt.Errorf(
			"jsonl event: el hecho '%s' no tiene forma serializada",
			payload.Type())
	}
}

// stepFinishedPayload es la línea del hecho con más carga del vocabulario.
func stepFinishedPayload(carga domRecord.StepFinished) map[string]any {
	linea := map[string]any{
		"step_id":     carga.StepID,
		"scope":       carga.Scope.String(),
		"status":      carga.Status.String(),
		"duration_ms": durationMillis(carga.Duration),
		"from_cache":  carga.FromCache,
		"reason":      carga.Reason.String(),
	}
	if carga.StepFingerprint != "" {
		linea["step_fingerprint"] = carga.StepFingerprint
	}
	// La ausencia del código es informativa: un step exitoso, revivido o saltado
	// no tiene ninguno, y un `0` diría lo contrario.
	if carga.ExitCode != nil {
		linea["exit_code"] = *carga.ExitCode
	}
	if !carga.ErrorClass.IsZero() {
		linea["error_class"] = carga.ErrorClass.String()
	}
	if !carga.Evidence.IsZero() {
		linea["evidence_from"] = evidencePayload(carga.Evidence)
	}
	return linea
}

// evidencePayload es la referencia al registro que estuvo vigente.
//
// `deployment_id` y `attempt` NO aparecen todavía, y su ausencia es una verdad y
// no un hueco: `state.Provenance` es hoy `{ExecutionID, At}` y no sabe de qué
// despliegue salió el registro. Entrarán cuando ella los lleve.
func evidencePayload(evidence domRecord.EvidenceRef) map[string]any {
	linea := map[string]any{
		"execution_id": evidence.ExecutionID,
		"at":           evidence.At.UTC().Format(eventTimeLayout),
		"record_id":    evidence.RecordID.String(),

		// Los tres componentes de la clave POR SEPARADO, igual que el índice
		// (`FileStateKeyDTO`) y por la misma razón: `Key.String()` es una forma
		// legible para diagnósticos y lo dice de sí misma, no un formato de
		// serialización. Componerla en una cadena obligaría a inventar un escape
		// del separador para una url o un ambiente que lo contenga.
		"state_key": map[string]any{
			"subject": evidence.StateKey.Subject(),
			"scope":   evidence.StateKey.Scope().String(),
			"step_id": evidence.StateKey.StepID(),
		},
	}
	if !evidence.Deployment.IsZero() {
		linea["deployment_id"] = evidence.Deployment.String()
	}
	if !evidence.Attempt.IsZero() {
		linea["attempt"] = evidence.Attempt.Number()
	}
	return linea
}

// durationMillis es la unidad de la SERIALIZACIÓN, no la del dominio: dentro son
// `time.Duration` y aquí `duration_ms` (spec 17 §5.2).
//
// Se redondea a entero y no se emite en coma flotante porque el consumidor
// agrega —suma, promedia, compara—, y un tipo que a veces es entero y a veces no
// obliga a cada lector a decidir cómo tratarlo.
func durationMillis(duration time.Duration) int64 {
	if duration < 0 {
		return 0
	}
	return duration.Milliseconds()
}
