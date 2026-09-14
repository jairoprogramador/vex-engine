package state

import (
	"fmt"
	"slices"
	"time"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

// fileStepRecordSchemaVersion versiona la FORMA del archivo, no la regla de la
// huella que lleva dentro. Son dos cosas distintas y conviene no fundirlas.
//
// Sube a 2 con la spec 13: `variables[].shared` desaparece del registro porque
// el ámbito es del step (§5.6), y eso cambia la forma del archivo.
const fileStepRecordSchemaVersion = 2

// fileStepRecordReadableVersions son las formas que ESTE binario sabe leer.
//
// LA DECISIÓN QUE LA SPEC 13 EXIGE TOMAR EXPLÍCITAMENTE, y se toma a favor de
// leer el 1: un registro v1 se lee sin migración y sin pérdida.
//
// Se puede porque lo que se quitó es un campo, y quitarlo no deja un hueco: el
// lector ya no tiene dónde poner `shared`, así que ignorarlo no pierde nada que
// el dominio pueda representar. `encoding/json` descarta los campos que el DTO
// no declara, de modo que un archivo v1 entra tal cual.
//
// La alternativa —rechazar el 1— no es «romper ruidosamente y ya»: `state/` es
// LA VERDAD y no se borra nunca, así que negarse a leerlo dejaría huérfanos los
// identificadores de recursos que existen de verdad en la nube, y el motor
// volvería a crearlos. Romper así no avisa de nada: destruye. Ver spec 11 §5.6
// para la asimetría que sí se conserva —un registro ILEGIBLE sigue siendo un
// error, no una ausencia—: una versión conocida no es un registro ilegible.
//
// Se escribe siempre en la versión de arriba: los registros v1 no se reescriben
// —el almacén es append-only— sino que envejecen bajo su clave.
var fileStepRecordReadableVersions = []int{1, fileStepRecordSchemaVersion}

// recordTimeLayout es RFC3339Nano porque va y vuelve sin perder precisión: el
// instante que se lee es exactamente el que se escribió.
const recordTimeLayout = time.RFC3339Nano

// FileStepRecordDTO es la forma en disco de un registro (spec 11 §5.3).
//
// JSON y no gob, por la misma razón que el índice: un registro de estado es lo
// que un humano abre cuando el motor revivió un step y no entiende por qué. Un
// gob no es inspeccionable ni portable.
//
// La clave no viaja dentro: la ruta la dice entera —`<subject>/<scope>/<step>/`—
// y aquí, a diferencia del índice, no hay hash que haga la ruta opaca.
type FileStepRecordDTO struct {
	SchemaVersion   int               `json:"schema_version"`
	RecordID        string            `json:"record_id"`
	StepFingerprint string            `json:"step_fingerprint"`
	Variables       []FileVariableDTO `json:"variables"`
	ProducedBy      FileProvenanceDTO `json:"produced_by"`
}

// FileVariableDTO persiste los dos campos que le quedan a command.Variable.
//
// `shared` vivía aquí, y perderlo por el camino hacía que el mismo proyecto
// produjera una huella distinta según corriera en local o en remoto (spec 02
// §5.2). Ese defecto DEJA DE SER POSIBLE con la spec 13: el dato que se perdía
// ya no viaja. El arreglo de la 02 no se revierte —se queda sin objeto—, y su
// contract test sigue verde por todo lo demás.
type FileVariableDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type FileProvenanceDTO struct {
	ExecutionID string `json:"execution_id"`
	At          string `json:"at"`
}

func ToFileStepRecordDTO(record domState.StepRecord) FileStepRecordDTO {
	variables := record.Variables()
	dtos := make([]FileVariableDTO, 0, len(variables))
	for i := range variables {
		dtos = append(dtos, FileVariableDTO{
			Name:  variables[i].Name(),
			Value: variables[i].Value(),
		})
	}

	return FileStepRecordDTO{
		SchemaVersion:   fileStepRecordSchemaVersion,
		RecordID:        record.ID().String(),
		StepFingerprint: record.StepFingerprint(),
		Variables:       dtos,
		ProducedBy: FileProvenanceDTO{
			ExecutionID: record.ProducedBy().ExecutionID,
			At:          record.ProducedBy().At.UTC().Format(recordTimeLayout),
		},
	}
}

// ToDomain reconstruye el registro por el constructor del dominio, invariantes
// incluidos.
//
// Es la asimetría que la spec 02 §9.3 dejó abierta y que la 11 §5.6 decide: en
// el índice, ilegible ⇒ ausente; aquí, ilegible ⇒ ERROR. Un registro que no se
// puede leer no se puede resolver ejecutando —la ejecución crearía un recurso
// duplicado—, así que la ejecución falla ruidosamente en vez de continuar sobre
// un estado que nadie conoce.
func (dto FileStepRecordDTO) ToDomain() (domState.StepRecord, error) {
	if !slices.Contains(fileStepRecordReadableVersions, dto.SchemaVersion) {
		return domState.StepRecord{}, fmt.Errorf(
			"esquema de registro %d no soportado (este binario lee %v)",
			dto.SchemaVersion, fileStepRecordReadableVersions)
	}

	recordID, err := domState.ParseRecordID(dto.RecordID)
	if err != nil {
		return domState.StepRecord{}, err
	}

	producedAt, err := time.Parse(recordTimeLayout, dto.ProducedBy.At)
	if err != nil {
		return domState.StepRecord{}, fmt.Errorf(
			"interpretar produced_by.at %q: %w", dto.ProducedBy.At, err)
	}

	variables := make([]command.Variable, 0, len(dto.Variables))
	for _, variableDTO := range dto.Variables {
		// `OriginState` nace AQUÍ, en el adaptador, y no en los handlers 01 y 02
		// que consumen el registro: leer del almacén es lo que hace que un valor
		// sea del almacén, y así los handlers siguen añadiendo la variable tal como
		// se guardó sin fabricarle ninguna marca (spec 11, heredado por la 12).
		//
		// El registro NO persiste el origen: lo que se guardó fue un valor, y al
		// volver a entrar en una ejecución nueva todo lo que viene de ahí es «de
		// una corrida anterior», venga de un `outputs` o de un literal declarado.
		//
		// El `shared` de un registro v1 se ignora aquí, y no hay dónde ponerlo:
		// el ámbito lo dice la CLAVE bajo la que está este registro (spec 13).
		variable, err := command.NewVariable(
			variableDTO.Name, variableDTO.Value, command.OriginState)
		if err != nil {
			return domState.StepRecord{}, fmt.Errorf("variable %q: %w", variableDTO.Name, err)
		}
		variables = append(variables, variable)
	}

	return domState.NewStepRecord(recordID, dto.StepFingerprint, variables, domState.Provenance{
		ExecutionID: dto.ProducedBy.ExecutionID,
		At:          producedAt,
	})
}
