package step

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type FileVarStoreDTO struct {
	Name  string
	Value string
}

func toFileVarStoreDTO(varSets []command.Variable) []FileVarStoreDTO {
	if len(varSets) == 0 {
		return []FileVarStoreDTO{}
	}
	dtoVars := make([]FileVarStoreDTO, 0, len(varSets))
	for _, outputVar := range varSets {
		dtoVars = append(dtoVars, FileVarStoreDTO{
			Name:  outputVar.Name(),
			Value: outputVar.Value(),
		})
	}
	return dtoVars
}

func fromFileVarStoreDTO(varsDto []FileVarStoreDTO) ([]command.Variable, error) {
	if varsDto == nil {
		return []command.Variable{}, nil
	}
	vars := make([]command.Variable, 0, len(varsDto))
	for _, dtoEntry := range varsDto {
		outputVar, err := command.NewVariable(dtoEntry.Name, dtoEntry.Value, false)
		if err != nil {
			return []command.Variable{}, err
		}
		vars = append(vars, outputVar)
	}
	return vars, nil
}
