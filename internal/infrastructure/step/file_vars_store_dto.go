package step

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// FileVarStoreDTO persiste los tres campos de command.Variable, isShared
// incluido. Perderlo hacía que el mismo proyecto produjera una huella distinta
// según corriera en modo local o remoto (spec 02 §5.2).
//
// Los archivos .vars escritos antes de este campo se siguen leyendo: gob deja
// los campos ausentes en su valor cero, o sea IsShared=false, que es
// exactamente el comportamiento anterior. No hay migración.
type FileVarStoreDTO struct {
	Name     string
	Value    string
	IsShared bool
}

func toFileVarStoreDTO(varSets []command.Variable) []FileVarStoreDTO {
	dtoVars := make([]FileVarStoreDTO, 0, len(varSets))
	for _, outputVar := range varSets {
		dtoVars = append(dtoVars, FileVarStoreDTO{
			Name:     outputVar.Name(),
			Value:    outputVar.Value(),
			IsShared: outputVar.IsShared(),
		})
	}
	return dtoVars
}

func fromFileVarStoreDTO(varsDto []FileVarStoreDTO) ([]command.Variable, error) {
	vars := make([]command.Variable, 0, len(varsDto))
	for _, dtoEntry := range varsDto {
		outputVar, err := command.NewVariable(dtoEntry.Name, dtoEntry.Value, dtoEntry.IsShared)
		if err != nil {
			return []command.Variable{}, err
		}
		vars = append(vars, outputVar)
	}
	return vars, nil
}
