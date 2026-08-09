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
// La spec 18 emite DOS hechos —`attempt_started` y `stale_clone_used`— porque
// son los dos que ocurren en su capa. Los otros ocho tienen dueño escrito
// (spec 19 §5.1) y todavía no tienen emisor, así que aquí no tienen forma
// serializada: inventarla ahora sería fijar el formato de un hecho sin ver el
// dato que lo llena, que es cómo se congelan campos que luego no encajan.
//
// Un tipo sin traducción es un ERROR y no una línea vacía: el emisor está
// escribiendo un hecho que nadie sabe leer, y un registro con líneas mudas es
// peor que uno incompleto —el pliegue las cuenta y no dicen nada—.
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

	default:
		return nil, fmt.Errorf(
			"jsonl event: el hecho '%s' no tiene forma serializada todavía (la define la spec 19)",
			payload.Type())
	}
}
