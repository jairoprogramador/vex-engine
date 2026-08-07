package state

import (
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// fileStepRecordSchemaVersion versiona la FORMA del archivo, no la regla de la
// huella que lleva dentro. Son dos cosas distintas y conviene no fundirlas.
const fileStepRecordSchemaVersion = 1

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

// FileVariableDTO persiste los tres campos de command.Variable, `shared`
// incluido. Perderlo hacía que el mismo proyecto produjera una huella distinta
// según corriera en modo local o remoto (spec 02 §5.2).
type FileVariableDTO struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Shared bool   `json:"shared,omitempty"`
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
			Name:   variables[i].Name(),
			Value:  variables[i].Value(),
			Shared: variables[i].IsShared(),
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
	if dto.SchemaVersion != fileStepRecordSchemaVersion {
		return domState.StepRecord{}, fmt.Errorf(
			"esquema de registro %d no soportado (este binario lee el %d)",
			dto.SchemaVersion, fileStepRecordSchemaVersion)
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
		variable, err := command.NewVariable(variableDTO.Name, variableDTO.Value, variableDTO.Shared)
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
